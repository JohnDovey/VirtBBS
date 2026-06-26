// VirtAnd — AppDatabase.kt
package io.virtbbs.virtand.data

import android.content.Context
import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase
import io.virtbbs.virtand.data.dao.ConferenceDao
import io.virtbbs.virtand.data.dao.FileDao
import io.virtbbs.virtand.data.dao.MessageDao
import io.virtbbs.virtand.data.dao.NodelistDao
import io.virtbbs.virtand.data.entities.CachedMessageEntity
import io.virtbbs.virtand.data.entities.ConferenceEntity
import io.virtbbs.virtand.data.entities.FileDirEntity
import io.virtbbs.virtand.data.entities.FileEntryEntity
import io.virtbbs.virtand.data.entities.NodelistVersionEntity
import io.virtbbs.virtand.data.entities.QueuedDownloadEntity
import io.virtbbs.virtand.data.entities.QueuedReplyEntity
import io.virtbbs.virtand.data.entities.QueuedUploadEntity

@Database(
    entities = [
        ConferenceEntity::class,
        CachedMessageEntity::class,
        QueuedReplyEntity::class,
        FileDirEntity::class,
        FileEntryEntity::class,
        QueuedUploadEntity::class,
        QueuedDownloadEntity::class,
        NodelistVersionEntity::class,
    ],
    version = 1,
    exportSchema = false,
)
abstract class AppDatabase : RoomDatabase() {
    abstract fun conferenceDao(): ConferenceDao
    abstract fun messageDao(): MessageDao
    abstract fun fileDao(): FileDao
    abstract fun nodelistDao(): NodelistDao

    companion object {
        @Volatile private var instance: AppDatabase? = null

        fun get(context: Context): AppDatabase =
            instance ?: synchronized(this) {
                instance ?: Room.databaseBuilder(context, AppDatabase::class.java, "virtand.db")
                    .build()
                    .also { instance = it }
            }
    }
}
