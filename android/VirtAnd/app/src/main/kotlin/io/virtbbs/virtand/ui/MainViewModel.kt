// VirtAnd — MainViewModel.kt
package io.virtbbs.virtand.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import io.virtbbs.virtand.core.UserApiClient
import io.virtbbs.virtand.core.UserApiException
import io.virtbbs.virtand.data.AppDatabase
import io.virtbbs.virtand.data.SessionInfo
import io.virtbbs.virtand.data.entities.CachedMessageEntity
import io.virtbbs.virtand.data.entities.ConferenceEntity
import io.virtbbs.virtand.data.entities.FileDirEntity
import io.virtbbs.virtand.data.entities.FileEntryEntity
import io.virtbbs.virtand.data.entities.QueuedDownloadEntity
import io.virtbbs.virtand.data.entities.QueuedReplyEntity
import io.virtbbs.virtand.data.entities.QueuedUploadEntity
import io.virtbbs.virtand.settings.SettingsStore
import io.virtbbs.virtand.sync.SyncEngine
import io.virtbbs.virtand.sync.SyncResult
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.int
import kotlinx.serialization.json.long
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

data class FidoNodeResult(
    val address: String,
    val name: String,
    val location: String,
    val sysop: String,
)

class MainViewModel(application: Application) : AndroidViewModel(application) {
    private val db = AppDatabase.get(application)
    val settings = SettingsStore(application)
    private val syncEngine = SyncEngine(application)

    val conferences: StateFlow<List<ConferenceEntity>> =
        db.conferenceDao().observeAll().stateIn(viewModelScope, SharingStarted.Eagerly, emptyList())

    val fileDirs: StateFlow<List<FileDirEntity>> =
        db.fileDao().observeDirs().stateIn(viewModelScope, SharingStarted.Eagerly, emptyList())

    val queuedReplies: StateFlow<List<QueuedReplyEntity>> =
        db.messageDao().observeQueuedReplies().stateIn(viewModelScope, SharingStarted.Eagerly, emptyList())

    val queuedUploads: StateFlow<List<QueuedUploadEntity>> =
        db.fileDao().observeQueuedUploads().stateIn(viewModelScope, SharingStarted.Eagerly, emptyList())

    val queuedDownloads: StateFlow<List<QueuedDownloadEntity>> =
        db.fileDao().observeQueuedDownloads().stateIn(viewModelScope, SharingStarted.Eagerly, emptyList())

    private val _sessionInfo = MutableStateFlow(SessionInfo())
    val sessionInfo: StateFlow<SessionInfo> = _sessionInfo

    private val _syncStatus = MutableStateFlow("Not synced yet.")
    val syncStatus: StateFlow<String> = _syncStatus

    private val _syncing = MutableStateFlow(false)
    val syncing: StateFlow<Boolean> = _syncing

    private val _connectionStatus = MutableStateFlow("")
    val connectionStatus: StateFlow<String> = _connectionStatus

    private val _fileSearchResults = MutableStateFlow<List<FileEntryEntity>>(emptyList())
    val fileSearchResults: StateFlow<List<FileEntryEntity>> = _fileSearchResults

    private val _nodeSearchResults = MutableStateFlow<List<FidoNodeResult>>(emptyList())
    val nodeSearchResults: StateFlow<List<FidoNodeResult>> = _nodeSearchResults

    private val _availableNetworks = MutableStateFlow<List<String>>(emptyList())
    val availableNetworks: StateFlow<List<String>> = _availableNetworks

    fun messagesFor(conferenceId: Int) = db.messageDao().observeByConference(conferenceId)
    fun filesFor(dirId: Long) = db.fileDao().observeFiles(dirId)

    fun synchronize() {
        if (_syncing.value) return
        _syncing.value = true
        viewModelScope.launch {
            val result: SyncResult = syncEngine.synchronize()
            result.sessionInfo?.let { _sessionInfo.value = it }
            _syncStatus.value = if (result.error != null) {
                "Sync failed: ${result.error}"
            } else {
                "Synced: ${result.newMessages} new message(s), " +
                    "${result.repliesUploaded} reply(ies) sent, " +
                    "${result.filesUploaded} file(s) uploaded, " +
                    "${result.filesDownloaded} file(s) downloaded" +
                    if (result.nodelistsChanged.isNotEmpty()) {
                        ", nodelist updated: ${result.nodelistsChanged.joinToString()}"
                    } else ""
            }
            _syncing.value = false
        }
    }

    fun testConnection(host: String, port: Int, username: String, password: String) {
        viewModelScope.launch {
            _connectionStatus.value = "Testing…"
            _connectionStatus.value = withContext(Dispatchers.IO) {
                if (host.isBlank() || username.isBlank() || password.isBlank()) {
                    return@withContext "Enter host, username, and password first."
                }
                try {
                    val api = UserApiClient(host.trim(), port, username.trim(), password)
                    val result = api.call("session.whoami") ?: return@withContext "Empty response."
                    val o = result.jsonObject
                    val name = o["name"]?.jsonPrimitive?.content ?: "?"
                    val bbs = o["bbs_name"]?.jsonPrimitive?.content ?: "?"
                    _sessionInfo.value = SessionInfo(
                        userName = name,
                        bbsName = bbs,
                        securityLevel = o["security_level"]?.jsonPrimitive?.int ?: 0,
                        sysop = o["sysop"]?.jsonPrimitive?.content?.toBooleanStrictOrNull() ?: false,
                    )
                    "Connected as $name on $bbs."
                } catch (e: UserApiException) {
                    "Failed: ${e.message}"
                } catch (e: Exception) {
                    "Failed: ${e.message ?: e.toString()}"
                }
            }
        }
    }

    fun markMessageRead(localId: Long) {
        viewModelScope.launch { db.messageDao().markRead(localId) }
    }

    suspend fun getMessage(localId: Long): CachedMessageEntity? =
        db.messageDao().getByLocalId(localId)

    fun queueReply(conferenceId: Int, refNum: Int, toName: String, subject: String, body: String) {
        viewModelScope.launch {
            db.messageDao().queueReply(
                QueuedReplyEntity(
                    conferenceId = conferenceId,
                    refNum = refNum,
                    toName = toName,
                    subject = subject,
                    body = body,
                    createdAt = System.currentTimeMillis(),
                )
            )
        }
    }

    fun queueNewMessage(conferenceId: Int, toName: String, subject: String, body: String) {
        queueReply(conferenceId, refNum = 0, toName = toName, subject = subject, body = body)
    }

    fun removeQueuedReply(id: Long) {
        viewModelScope.launch { db.messageDao().removeQueuedReply(id) }
    }

    fun queueDownload(dirId: Long, filename: String) {
        viewModelScope.launch {
            db.fileDao().queueDownload(
                QueuedDownloadEntity(dirId = dirId, filename = filename, createdAt = System.currentTimeMillis())
            )
        }
    }

    fun queueUpload(dirId: Long, localFileUri: String, filename: String, description: String) {
        viewModelScope.launch {
            db.fileDao().queueUpload(
                QueuedUploadEntity(
                    dirId = dirId,
                    localFileUri = localFileUri,
                    filename = filename,
                    description = description,
                    createdAt = System.currentTimeMillis(),
                )
            )
        }
    }

    fun removeQueuedUpload(item: QueuedUploadEntity) {
        viewModelScope.launch { db.fileDao().removeQueuedUpload(item) }
    }

    fun removeQueuedDownload(item: QueuedDownloadEntity) {
        viewModelScope.launch { db.fileDao().removeQueuedDownload(item) }
    }

    fun searchFiles(host: String, port: Int, username: String, password: String, query: String) {
        viewModelScope.launch {
            _fileSearchResults.value = emptyList()
            if (query.isBlank() || host.isBlank() || username.isBlank() || password.isBlank()) return@launch
            _fileSearchResults.value = withContext(Dispatchers.IO) {
                try {
                    val api = UserApiClient(host.trim(), port, username.trim(), password)
                    val result = api.call("files.search", JsonObject(mapOf("Query" to JsonPrimitive(query))))
                        ?: return@withContext emptyList()
                    result.jsonArray.map { f ->
                        val o = f.jsonObject
                        FileEntryEntity(
                            id = o["ID"]!!.jsonPrimitive.long,
                            dirId = o["DirID"]?.jsonPrimitive?.long ?: 0,
                            filename = o["Filename"]!!.jsonPrimitive.content,
                            size = o["Size"]?.jsonPrimitive?.long ?: 0,
                            description = o["Description"]?.jsonPrimitive?.content ?: "",
                            uploader = o["Uploader"]?.jsonPrimitive?.content ?: "",
                            uploadDate = o["UploadDate"]?.jsonPrimitive?.content ?: "",
                        )
                    }
                } catch (_: Exception) {
                    emptyList()
                }
            }
        }
    }

    fun searchNodes(host: String, port: Int, username: String, password: String, network: String, query: String) {
        viewModelScope.launch {
            _nodeSearchResults.value = emptyList()
            if (query.isBlank() || host.isBlank() || username.isBlank() || password.isBlank()) return@launch
            _nodeSearchResults.value = withContext(Dispatchers.IO) {
                try {
                    val api = UserApiClient(host.trim(), port, username.trim(), password)
                    val result = api.call(
                        "fido.nodes.search",
                        JsonObject(
                            mapOf(
                                "network" to JsonPrimitive(network),
                                "query" to JsonPrimitive(query),
                                "page" to JsonPrimitive(1),
                                "size" to JsonPrimitive(25),
                            )
                        ),
                    ) ?: return@withContext emptyList()
                    val nodes = result.jsonObject["nodes"]?.jsonArray ?: return@withContext emptyList()
                    nodes.map { n ->
                        val o = n.jsonObject
                        val zone = o["zone"]?.jsonPrimitive?.int ?: 0
                        val net = o["net"]?.jsonPrimitive?.int ?: 0
                        val nodeNum = o["node"]?.jsonPrimitive?.int ?: 0
                        val point = o["point"]?.jsonPrimitive?.int ?: 0
                        val address = if (point != 0) "$zone:$net/$nodeNum.$point" else "$zone:$net/$nodeNum"
                        FidoNodeResult(
                            address = address,
                            name = o["name"]?.jsonPrimitive?.content ?: "",
                            location = o["location"]?.jsonPrimitive?.content ?: "",
                            sysop = o["sysop"]?.jsonPrimitive?.content ?: "",
                        )
                    }
                } catch (_: Exception) {
                    emptyList()
                }
            }
        }
    }

    fun saveSettings(host: String, port: Int, username: String, password: String, networks: List<String>) {
        viewModelScope.launch {
            settings.save(host, port, username, password, networks)
        }
    }

    fun fetchAvailableNetworks(host: String, port: Int, username: String, password: String) {
        viewModelScope.launch {
            if (host.isBlank() || username.isBlank() || password.isBlank()) return@launch
            _availableNetworks.value = withContext(Dispatchers.IO) {
                try {
                    val api = UserApiClient(host.trim(), port, username.trim(), password)
                    val result = api.call("fido.networks.list") ?: return@withContext emptyList()
                    result.jsonArray.map { it.jsonPrimitive.content }
                } catch (_: Exception) {
                    emptyList()
                }
            }
        }
    }

    fun conferenceName(conferenceId: Int): String =
        conferences.value.firstOrNull { it.id == conferenceId }?.name ?: "Conf #$conferenceId"

    fun fileDirName(dirId: Long): String =
        fileDirs.value.firstOrNull { it.id == dirId }?.name ?: "Dir #$dirId"
}
