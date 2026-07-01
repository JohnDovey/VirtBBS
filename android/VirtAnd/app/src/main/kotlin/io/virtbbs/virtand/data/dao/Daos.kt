// VirtAnd — Daos.kt
package io.virtbbs.virtand.data.dao

import androidx.room.Dao
import androidx.room.Delete
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import androidx.room.Update
import kotlinx.coroutines.flow.Flow
import io.virtbbs.virtand.data.entities.CachedMessageEntity
import io.virtbbs.virtand.data.entities.ConferenceEntity
import io.virtbbs.virtand.data.entities.FileDirEntity
import io.virtbbs.virtand.data.entities.FileEntryEntity
import io.virtbbs.virtand.data.entities.MessageAttachmentEntity
import io.virtbbs.virtand.data.entities.NodelistVersionEntity
import io.virtbbs.virtand.data.entities.PendingReadUpdateEntity
import io.virtbbs.virtand.data.entities.QueuedDownloadEntity
import io.virtbbs.virtand.data.entities.QueuedReplyEntity
import io.virtbbs.virtand.data.entities.QueuedUploadEntity

@Dao
interface ConferenceDao {
    @Query("SELECT * FROM conferences ORDER BY network, name")
    fun observeAll(): Flow<List<ConferenceEntity>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertAll(conferences: List<ConferenceEntity>)

    @Query("DELETE FROM conferences")
    suspend fun clear()
}

@Dao
interface MessageDao {
    @Query("SELECT * FROM cached_messages WHERE conferenceId = :conferenceId ORDER BY msgNumber DESC")
    fun observeByConference(conferenceId: Int): Flow<List<CachedMessageEntity>>

    @Query("SELECT * FROM cached_messages WHERE localId = :localId")
    suspend fun getByLocalId(localId: Long): CachedMessageEntity?

    @Query("SELECT * FROM cached_messages WHERE conferenceId = :conferenceId AND msgNumber = :msgNumber")
    suspend fun getByConferenceAndNumber(conferenceId: Int, msgNumber: Int): CachedMessageEntity?

    @Query("UPDATE cached_messages SET isRead = 1 WHERE localId = :localId")
    suspend fun markRead(localId: Long)

    @Query("SELECT COALESCE(MAX(msgNumber), 0) FROM cached_messages WHERE conferenceId = :conferenceId")
    suspend fun highMsgNumber(conferenceId: Int): Int

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertAll(messages: List<CachedMessageEntity>)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insert(message: CachedMessageEntity): Long

    @Update
    suspend fun update(message: CachedMessageEntity)

    @Query("SELECT * FROM cached_messages WHERE datePostedMs > 0 AND datePostedMs < :cutoffMs")
    suspend fun messagesOlderThan(cutoffMs: Long): List<CachedMessageEntity>

    @Query("DELETE FROM cached_messages WHERE localId IN (:ids)")
    suspend fun deleteByIds(ids: List<Long>)

    @Query("SELECT * FROM reply_queue ORDER BY createdAt")
    fun observeQueuedReplies(): Flow<List<QueuedReplyEntity>>

    @Insert
    suspend fun queueReply(reply: QueuedReplyEntity)

    @Query("DELETE FROM reply_queue WHERE id = :id")
    suspend fun removeQueuedReply(id: Long)

    @Query("SELECT * FROM reply_queue")
    suspend fun allQueuedReplies(): List<QueuedReplyEntity>

    @Query("DELETE FROM reply_queue WHERE id IN (:ids)")
    suspend fun removeQueuedReplies(ids: List<Long>)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertPendingRead(update: PendingReadUpdateEntity)

    @Query("SELECT * FROM pending_read_updates")
    suspend fun allPendingReads(): List<PendingReadUpdateEntity>

    @Query("DELETE FROM pending_read_updates WHERE conferenceId = :conferenceId")
    suspend fun clearPendingRead(conferenceId: Int)
}

@Dao
interface AttachmentDao {
    @Query("SELECT * FROM message_attachments WHERE messageLocalId = :messageLocalId ORDER BY filename")
    fun observeForMessage(messageLocalId: Long): Flow<List<MessageAttachmentEntity>>

    @Query("SELECT * FROM message_attachments WHERE messageLocalId = :messageLocalId")
    suspend fun forMessage(messageLocalId: Long): List<MessageAttachmentEntity>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertAll(attachments: List<MessageAttachmentEntity>)

    @Query("DELETE FROM message_attachments WHERE messageLocalId IN (:messageLocalIds)")
    suspend fun deleteForMessages(messageLocalIds: List<Long>)
}

@Dao
interface FileDao {
    @Query("SELECT * FROM file_dirs ORDER BY name")
    fun observeDirs(): Flow<List<FileDirEntity>>

    @Query("SELECT * FROM file_entries WHERE dirId = :dirId ORDER BY filename")
    fun observeFiles(dirId: Long): Flow<List<FileEntryEntity>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertDirs(dirs: List<FileDirEntity>)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertFiles(files: List<FileEntryEntity>)

    @Query("DELETE FROM file_entries WHERE dirId = :dirId")
    suspend fun clearFiles(dirId: Long)

    @Query("SELECT * FROM upload_queue ORDER BY createdAt")
    fun observeQueuedUploads(): Flow<List<QueuedUploadEntity>>

    @Insert
    suspend fun queueUpload(upload: QueuedUploadEntity)

    @Query("SELECT * FROM upload_queue")
    suspend fun allQueuedUploads(): List<QueuedUploadEntity>

    @Delete
    suspend fun removeQueuedUpload(upload: QueuedUploadEntity)

    @Query("SELECT * FROM download_queue ORDER BY createdAt")
    fun observeQueuedDownloads(): Flow<List<QueuedDownloadEntity>>

    @Insert
    suspend fun queueDownload(download: QueuedDownloadEntity)

    @Query("SELECT * FROM download_queue")
    suspend fun allQueuedDownloads(): List<QueuedDownloadEntity>

    @Delete
    suspend fun removeQueuedDownload(download: QueuedDownloadEntity)
}

@Dao
interface NodelistDao {
    @Query("SELECT * FROM nodelist_versions WHERE network = :network")
    suspend fun get(network: String): NodelistVersionEntity?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(version: NodelistVersionEntity)
}
