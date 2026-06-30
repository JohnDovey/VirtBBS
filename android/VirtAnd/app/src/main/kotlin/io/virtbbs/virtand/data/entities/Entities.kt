// VirtAnd — Entities.kt
//
// Room entities for VirtAnd's local cache: conferences, cached messages
// (from QWK downloads), the file catalog (browsed offline after one
// sync), the nodelist version stamp per subscribed network, and the
// upload/reply queues that only execute on an explicit "synchronize" —
// per the plan's "store and forward" design.
package io.virtbbs.virtand.data.entities

import androidx.room.Entity
import androidx.room.Index
import androidx.room.PrimaryKey

@Entity(tableName = "conferences")
data class ConferenceEntity(
    @PrimaryKey val id: Int,
    val name: String,
    val description: String,
    val readSec: Int,
    val writeSec: Int,
    /** Display network for grouping — "Local" for non-echomail areas. */
    val network: String = "Local",
    /** BBS message stats (Total/Unread/LastRead), refreshed on sync. */
    val total: Int = 0,
    val unread: Int = 0,
    val lastRead: Int = 0,
)

/** A message cached locally after a QWK download — read-only until synced again. */
@Entity(
    tableName = "cached_messages",
    indices = [Index(value = ["conferenceId", "msgNumber"], unique = true)],
)
data class CachedMessageEntity(
    @PrimaryKey(autoGenerate = true) val localId: Long = 0,
    val conferenceId: Int,
    val msgNumber: Int,
    val date: String,
    val time: String,
    val toName: String,
    val fromName: String,
    val subject: String,
    val body: String,
    val isRead: Boolean = false,
)

/** A reply composed offline, queued until the next "synchronize" uploads it as part of a REP packet. */
@Entity(tableName = "reply_queue")
data class QueuedReplyEntity(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val conferenceId: Int,
    val refNum: Int,
    val toName: String,
    val subject: String,
    val body: String,
    val createdAt: Long,
)

@Entity(tableName = "file_dirs")
data class FileDirEntity(
    @PrimaryKey val id: Long,
    val name: String,
    val description: String,
    val readSec: Int,
    val uploadSec: Int,
)

@Entity(tableName = "file_entries")
data class FileEntryEntity(
    @PrimaryKey val id: Long,
    val dirId: Long,
    val filename: String,
    val size: Long,
    val description: String,
    val uploader: String,
    val uploadDate: String,
)

/** A file the user picked locally, queued until "synchronize" actually uploads it. */
@Entity(tableName = "upload_queue")
data class QueuedUploadEntity(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val dirId: Long,
    val localFileUri: String,
    val filename: String,
    val description: String,
    val createdAt: Long,
)

/** A file the user picked for download, queued until "synchronize" fetches it. */
@Entity(tableName = "download_queue")
data class QueuedDownloadEntity(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val dirId: Long,
    val filename: String,
    val createdAt: Long,
)

/** Last known nodelist version stamp per subscribed network — mirrors fido.nodelist.version. */
@Entity(tableName = "nodelist_versions")
data class NodelistVersionEntity(
    @PrimaryKey val network: String,
    val importedAt: String,
    val nodeCount: Int,
)
