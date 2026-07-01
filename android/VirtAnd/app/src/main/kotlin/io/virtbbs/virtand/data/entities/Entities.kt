// VirtAnd — Entities.kt
//
// Room entities for VirtAnd's local cache: conferences, cached messages
// (from User API sync), attachments, file catalog, and upload/reply queues.
package io.virtbbs.virtand.data.entities

import androidx.room.Entity
import androidx.room.ForeignKey
import androidx.room.Index
import androidx.room.PrimaryKey

/** Synthetic conference ID for FidoNet netmail in the local cache (server uses 0). */
const val NETMAIL_CONFERENCE_ID = -1

@Entity(tableName = "conferences")
data class ConferenceEntity(
    @PrimaryKey val id: Int,
    val name: String,
    val description: String,
    val readSec: Int,
    val writeSec: Int,
    val network: String = "Local",
    val total: Int = 0,
    val unread: Int = 0,
    val lastRead: Int = 0,
)

@Entity(
    tableName = "cached_messages",
    indices = [Index(value = ["conferenceId", "msgNumber"], unique = true)],
)
data class CachedMessageEntity(
    @PrimaryKey(autoGenerate = true) val localId: Long = 0,
    val serverMessageId: Long = 0,
    val conferenceId: Int,
    val msgNumber: Int,
    val date: String,
    val time: String,
    val datePostedMs: Long = 0,
    val toName: String,
    val fromName: String,
    val subject: String,
    val body: String,
    val hasAttachment: Boolean = false,
    val isNetmail: Boolean = false,
    val isRead: Boolean = false,
)

@Entity(
    tableName = "message_attachments",
    foreignKeys = [
        ForeignKey(
            entity = CachedMessageEntity::class,
            parentColumns = ["localId"],
            childColumns = ["messageLocalId"],
            onDelete = ForeignKey.CASCADE,
        ),
    ],
    indices = [Index(value = ["messageLocalId", "attachmentId"], unique = true)],
)
data class MessageAttachmentEntity(
    @PrimaryKey(autoGenerate = true) val localId: Long = 0,
    val messageLocalId: Long,
    val attachmentId: Long,
    val filename: String,
    val sizeBytes: Long,
    val localPath: String = "",
)

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

@Entity(tableName = "upload_queue")
data class QueuedUploadEntity(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val dirId: Long,
    val localFileUri: String,
    val filename: String,
    val description: String,
    val createdAt: Long,
)

@Entity(tableName = "download_queue")
data class QueuedDownloadEntity(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val dirId: Long,
    val filename: String,
    val createdAt: Long,
)

@Entity(tableName = "nodelist_versions")
data class NodelistVersionEntity(
    @PrimaryKey val network: String,
    val importedAt: String,
    val nodeCount: Int,
)

@Entity(tableName = "pending_read_updates")
data class PendingReadUpdateEntity(
    @PrimaryKey val conferenceId: Int,
    val msgNumber: Int,
)
