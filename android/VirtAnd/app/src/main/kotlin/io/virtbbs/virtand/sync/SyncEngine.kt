// VirtAnd — SyncEngine.kt
//
// Single "synchronize" entry point for User API sync (messages, files, nodelist).
package io.virtbbs.virtand.sync

import android.content.Context
import io.virtbbs.virtand.core.UserApiClient
import io.virtbbs.virtand.core.asJsonArrayOrEmpty
import io.virtbbs.virtand.data.AppDatabase
import io.virtbbs.virtand.data.SessionInfo
import io.virtbbs.virtand.data.entities.CachedMessageEntity
import io.virtbbs.virtand.data.entities.ConferenceEntity
import io.virtbbs.virtand.data.entities.FileDirEntity
import io.virtbbs.virtand.data.entities.FileEntryEntity
import io.virtbbs.virtand.data.entities.MessageAttachmentEntity
import io.virtbbs.virtand.data.entities.NodelistVersionEntity
import io.virtbbs.virtand.data.entities.PendingReadUpdateEntity
import io.virtbbs.virtand.settings.SettingsStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import java.io.File
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Base64

data class SyncResult(
    val newMessages: Int,
    val repliesUploaded: Int,
    val filesUploaded: Int,
    val filesDownloaded: Int,
    val nodelistsChanged: List<String>,
    val sessionInfo: SessionInfo? = null,
    val error: String? = null,
)

class SyncEngine(private val context: Context) {
    private val db = AppDatabase.get(context)
    private val settings = SettingsStore(context)

    suspend fun synchronize(): SyncResult = withContext(Dispatchers.IO) {
        val cfg = settings.snapshot()
        if (cfg.host.isBlank() || cfg.username.isBlank() || cfg.password.isBlank()) {
            return@withContext SyncResult(0, 0, 0, 0, emptyList(), error = "Not configured — set host, username, and password first.")
        }
        val api = UserApiClient(
            host = cfg.host,
            port = cfg.userApiPort,
            username = cfg.username,
            password = cfg.password,
        )

        try {
            purgeOldMessages(cfg.purgeDays)
            val sessionInfo = refreshSessionInfo(api)
            refreshConferences(api)
            syncPendingReadPointers(api)
            val newMessages = downloadAndImportMessages(api)
            refreshFileCatalog(api)
            val filesDownloaded = executeQueuedDownloads(api)
            val filesUploaded = executeQueuedUploads(api)
            val nodelistsChanged = checkNodelists(api, cfg.subscribedNetworks)
            val repliesUploaded = uploadQueuedReplies(api)
            reportUsageStats(api, newMessages, repliesUploaded, filesDownloaded, filesUploaded)

            SyncResult(newMessages, repliesUploaded, filesUploaded, filesDownloaded, nodelistsChanged, sessionInfo)
        } catch (e: Exception) {
            SyncResult(0, 0, 0, 0, emptyList(), error = e.message ?: e.toString())
        }
    }

    suspend fun purgeOldMessages(purgeDays: Int) {
        if (purgeDays <= 0) return
        val cutoffMs = System.currentTimeMillis() - purgeDays.toLong() * 24 * 60 * 60 * 1000
        runBlockingDb {
            val stale = db.messageDao().messagesOlderThan(cutoffMs)
            if (stale.isEmpty()) return@runBlockingDb
            val localIds = stale.map { it.localId }
            val attachments = localIds.flatMap { db.attachmentDao().forMessage(it) }
            for (a in attachments) {
                if (a.localPath.isNotBlank()) {
                    File(a.localPath).delete()
                }
            }
            db.attachmentDao().deleteForMessages(localIds)
            db.messageDao().deleteByIds(localIds)
        }
    }

    private inline fun <T> apiStep(step: String, block: () -> T): T = try {
        block()
    } catch (e: Exception) {
        throw Exception("$step: ${e.message ?: e}", e)
    }

    private fun refreshSessionInfo(api: UserApiClient): SessionInfo? = apiStep("session.whoami") {
        val result = api.call("session.whoami") ?: return@apiStep null
        val o = result.jsonObject
        SessionInfo(
            userName = o["name"]?.jsonPrimitive?.content ?: "",
            bbsName = o["bbs_name"]?.jsonPrimitive?.content ?: "",
            securityLevel = o["security_level"]?.jsonPrimitive?.int ?: 0,
            sysop = o["sysop"]?.jsonPrimitive?.content?.toBooleanStrictOrNull() ?: false,
        )
    }

    private fun refreshConferences(api: UserApiClient) = apiStep("conferences.list") {
        val result = api.call("conferences.list") ?: return@apiStep
        val entities = result.asJsonArrayOrEmpty().map { c ->
            val o = c.jsonObject
            ConferenceEntity(
                id = o["ID"]!!.jsonPrimitive.int,
                name = o["Name"]!!.jsonPrimitive.content,
                description = o["Description"]?.jsonPrimitive?.content ?: "",
                readSec = o["ReadSec"]?.jsonPrimitive?.int ?: 0,
                writeSec = o["WriteSec"]?.jsonPrimitive?.int ?: 0,
                network = conferenceNetworkLabel(o),
                total = o["Total"]?.jsonPrimitive?.int ?: 0,
                unread = o["Unread"]?.jsonPrimitive?.int ?: 0,
                lastRead = o["LastRead"]?.jsonPrimitive?.int ?: 0,
            )
        }
        runBlockingDb { db.conferenceDao().upsertAll(entities) }
    }

    private fun conferenceNetworkLabel(o: JsonObject): String {
        val echo = o["Echo"]?.jsonPrimitive?.content?.toBooleanStrictOrNull() ?: false
        if (!echo) return "Local"
        val raw = o["Network"]?.jsonPrimitive?.content?.trim().orEmpty()
        return raw.ifEmpty { "FidoNet" }
    }

    private fun syncPendingReadPointers(api: UserApiClient) = apiStep("messages.mark_read") {
        val pending = runBlockingDb { db.messageDao().allPendingReads() }
        for (p in pending) {
            try {
                api.call(
                    "messages.mark_read",
                    JsonObject(
                        mapOf(
                            "ConferenceID" to JsonPrimitive(p.conferenceId),
                            "MsgNumber" to JsonPrimitive(p.msgNumber),
                        )
                    ),
                )
                runBlockingDb { db.messageDao().clearPendingRead(p.conferenceId) }
            } catch (_: Exception) {
                // Retry on next sync.
            }
        }
    }

    private fun downloadAndImportMessages(api: UserApiClient): Int = apiStep("messages.sync") {
        val result = api.call("messages.sync") ?: return@apiStep 0
        val messages = result.jsonObject["Messages"]?.asJsonArrayOrEmpty() ?: JsonArray(emptyList())
        var newCount = 0

        for (item in messages) {
            val o = item.jsonObject
            val confId = o["ConferenceID"]!!.jsonPrimitive.int
            val msgNum = o["MsgNumber"]!!.jsonPrimitive.int
            val existing = runBlockingDb { db.messageDao().getByConferenceAndNumber(confId, msgNum) }
            if (existing != null) continue

            val datePosted = o["DatePosted"]?.jsonPrimitive?.content ?: ""
            val (dateStr, timeStr, postedMs) = parseDatePosted(datePosted)
            val entity = CachedMessageEntity(
                serverMessageId = o["ID"]?.jsonPrimitive?.long ?: 0,
                conferenceId = confId,
                msgNumber = msgNum,
                date = dateStr,
                time = timeStr,
                datePostedMs = postedMs,
                toName = o["ToName"]?.jsonPrimitive?.content ?: "",
                fromName = o["FromName"]?.jsonPrimitive?.content ?: "",
                subject = o["Subject"]?.jsonPrimitive?.content ?: "",
                body = o["Body"]?.jsonPrimitive?.content ?: "",
                hasAttachment = o["HasAttachment"]?.jsonPrimitive?.booleanOrNull ?: false,
            )
            val localId = runBlockingDb { db.messageDao().insert(entity) }
            newCount++

            val atts = o["Attachments"]?.asJsonArrayOrEmpty() ?: JsonArray(emptyList())
            if (atts.isNotEmpty()) {
                val attachDir = File(context.getExternalFilesDir(null), "attachments/$confId").apply { mkdirs() }
                val saved = mutableListOf<MessageAttachmentEntity>()
                for (a in atts) {
                    val ao = a.jsonObject
                    val attachId = ao["ID"]!!.jsonPrimitive.long
                    val filename = ao["Filename"]!!.jsonPrimitive.content
                    val size = ao["SizeBytes"]?.jsonPrimitive?.long ?: 0
                    var localPath = ""
                    try {
                        val dl = api.call(
                            "messages.attachment.download",
                            JsonObject(
                                mapOf(
                                    "ConferenceID" to JsonPrimitive(confId),
                                    "MsgNumber" to JsonPrimitive(msgNum),
                                    "AttachmentID" to JsonPrimitive(attachId),
                                )
                            ),
                        )
                        val b64 = dl?.jsonObject?.get("data")?.jsonPrimitive?.content
                        if (b64 != null) {
                            val bytes = Base64.getDecoder().decode(b64)
                            val safeName = "${msgNum}_${attachId}_$filename"
                            val outFile = File(attachDir, safeName)
                            outFile.writeBytes(bytes)
                            localPath = outFile.absolutePath
                        }
                    } catch (_: Exception) {
                        // Attachment metadata saved; file can retry on next sync.
                    }
                    saved.add(
                        MessageAttachmentEntity(
                            messageLocalId = localId,
                            attachmentId = attachId,
                            filename = filename,
                            sizeBytes = size,
                            localPath = localPath,
                        )
                    )
                }
                runBlockingDb { db.attachmentDao().upsertAll(saved) }
            }
        }
        newCount
    }

    private fun parseDatePosted(iso: String): Triple<String, String, Long> {
        if (iso.isBlank()) return Triple("", "", 0)
        return try {
            val instant = Instant.parse(iso)
            val zdt = instant.atZone(ZoneId.systemDefault())
            val date = DateTimeFormatter.ofPattern("MM/dd/yy").format(zdt)
            val time = DateTimeFormatter.ofPattern("HH:mm").format(zdt)
            Triple(date, time, instant.toEpochMilli())
        } catch (_: Exception) {
            Triple(iso.take(10), "", 0)
        }
    }

    private fun refreshFileCatalog(api: UserApiClient) = apiStep("files.dirs.list") {
        val dirsResult = api.call("files.dirs.list") ?: return@apiStep
        val dirs = dirsResult.asJsonArrayOrEmpty().map { d ->
            val o = d.jsonObject
            FileDirEntity(
                id = o["ID"]!!.jsonPrimitive.long,
                name = o["Name"]!!.jsonPrimitive.content,
                description = o["Description"]?.jsonPrimitive?.content ?: "",
                readSec = o["ReadSec"]?.jsonPrimitive?.int ?: 0,
                uploadSec = o["UploadSec"]?.jsonPrimitive?.int ?: 0,
            )
        }
        runBlockingDb { db.fileDao().upsertDirs(dirs) }

        for (dir in dirs) {
            val filesResult = api.call("files.list", JsonObject(mapOf("DirID" to JsonPrimitive(dir.id))))
                ?: continue
            val files = filesResult.asJsonArrayOrEmpty().map { f ->
                val o = f.jsonObject
                FileEntryEntity(
                    id = o["ID"]!!.jsonPrimitive.long,
                    dirId = dir.id,
                    filename = o["Filename"]!!.jsonPrimitive.content,
                    size = o["Size"]?.jsonPrimitive?.long ?: 0,
                    description = o["Description"]?.jsonPrimitive?.content ?: "",
                    uploader = o["Uploader"]?.jsonPrimitive?.content ?: "",
                    uploadDate = o["UploadDate"]?.jsonPrimitive?.content ?: "",
                )
            }
            runBlockingDb {
                db.fileDao().clearFiles(dir.id)
                db.fileDao().upsertFiles(files)
            }
        }
    }

    private fun executeQueuedDownloads(api: UserApiClient): Int {
        val queued = runBlockingDb { db.fileDao().allQueuedDownloads() }
        var count = 0
        for (q in queued) {
            try {
                val result = api.call(
                    "files.download",
                    JsonObject(mapOf("DirID" to JsonPrimitive(q.dirId), "Filename" to JsonPrimitive(q.filename))),
                )
                val b64 = result?.jsonObject?.get("data")?.jsonPrimitive?.content
                    ?: throw IllegalStateException("empty files.download response")
                val bytes = Base64.getDecoder().decode(b64)
                val dir = File(context.getExternalFilesDir(null), "downloads").apply { mkdirs() }
                File(dir, q.filename).writeBytes(bytes)
                runBlockingDb { db.fileDao().removeQueuedDownload(q) }
                count++
            } catch (_: Exception) {
            }
        }
        return count
    }

    private fun executeQueuedUploads(api: UserApiClient): Int {
        val queued = runBlockingDb { db.fileDao().allQueuedUploads() }
        var count = 0
        for (q in queued) {
            try {
                val bytes = context.contentResolver.openInputStream(android.net.Uri.parse(q.localFileUri))
                    ?.use { it.readBytes() } ?: continue
                val b64 = Base64.getEncoder().encodeToString(bytes)
                api.call(
                    "files.upload",
                    JsonObject(
                        mapOf(
                            "DirID" to JsonPrimitive(q.dirId),
                            "Filename" to JsonPrimitive(q.filename),
                            "Description" to JsonPrimitive(q.description),
                            "Data" to JsonPrimitive(b64),
                        )
                    ),
                )
                runBlockingDb { db.fileDao().removeQueuedUpload(q) }
                count++
            } catch (_: Exception) {
            }
        }
        return count
    }

    private fun checkNodelists(api: UserApiClient, networks: List<String>): List<String> {
        val changed = mutableListOf<String>()
        for (network in networks) {
            try {
                val result = api.call("fido.nodelist.version", JsonObject(mapOf("network" to JsonPrimitive(network))))
                    ?: continue
                val o = result.jsonObject
                val importedAt = o["imported_at"]?.jsonPrimitive?.content ?: continue
                val nodeCount = o["node_count"]?.jsonPrimitive?.int ?: 0
                val cached = runBlockingDb { db.nodelistDao().get(network) }
                if (cached == null || cached.importedAt != importedAt || cached.nodeCount != nodeCount) {
                    runBlockingDb { db.nodelistDao().upsert(NodelistVersionEntity(network, importedAt, nodeCount)) }
                    changed.add(network)
                }
            } catch (_: Exception) {
            }
        }
        return changed
    }

    private fun uploadQueuedReplies(api: UserApiClient): Int {
        val queued = runBlockingDb { db.messageDao().allQueuedReplies() }
        if (queued.isEmpty()) return 0

        val replies = JsonArray(
            queued.map { r ->
                JsonObject(
                    mapOf(
                        "ConferenceID" to JsonPrimitive(r.conferenceId),
                        "RefNum" to JsonPrimitive(r.refNum),
                        "ToName" to JsonPrimitive(r.toName),
                        "Subject" to JsonPrimitive(r.subject),
                        "Body" to JsonPrimitive(r.body),
                    )
                )
            }
        )
        api.call("messages.post", JsonObject(mapOf("Replies" to replies)))
        runBlockingDb { db.messageDao().removeQueuedReplies(queued.map { it.id }) }
        return queued.size
    }

    private fun reportUsageStats(
        api: UserApiClient,
        messagesDownloaded: Int,
        messagesUploaded: Int,
        filesDownloaded: Int,
        filesUploaded: Int,
    ) {
        if (messagesDownloaded == 0 && messagesUploaded == 0 && filesDownloaded == 0 && filesUploaded == 0) return
        try {
            api.call(
                "app.stats.report",
                JsonObject(
                    mapOf(
                        "messages_downloaded" to JsonPrimitive(messagesDownloaded),
                        "messages_uploaded" to JsonPrimitive(messagesUploaded),
                        "files_downloaded" to JsonPrimitive(filesDownloaded),
                        "files_uploaded" to JsonPrimitive(filesUploaded),
                    )
                ),
            )
        } catch (_: Exception) {
        }
    }

    private fun <T> runBlockingDb(block: suspend () -> T): T =
        kotlinx.coroutines.runBlocking { block() }
}
