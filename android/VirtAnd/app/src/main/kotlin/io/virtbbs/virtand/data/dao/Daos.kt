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
import io.virtbbs.virtand.data.entities.NodelistVersionEntity
import io.virtbbs.virtand.data.entities.QueuedDownloadEntity
import io.virtbbs.virtand.data.entities.QueuedReplyEntity
import io.virtbbs.virtand.data.entities.QueuedUploadEntity

@Dao
interface ConferenceDao {
    @Query("SELECT * FROM conferences ORDER BY name")
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

    @Query("UPDATE cached_messages SET isRead = 1 WHERE localId = :localId")
    suspend fun markRead(localId: Long)

    @Query("SELECT COALESCE(MAX(msgNumber), 0) FROM cached_messages WHERE conferenceId = :conferenceId")
    suspend fun highMsgNumber(conferenceId: Int): Int

    @Insert(onConflict = OnConflictStrategy.IGNORE)
    suspend fun insertAll(messages: List<CachedMessageEntity>)

    @Update
    suspend fun update(message: CachedMessageEntity)

    // ── Reply queue ──────────────────────────────────────────────────────
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

    // ── Upload / download queues ────────────────────────────────────────
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
