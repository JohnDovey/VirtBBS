// VirtAnd — AppDatabase.kt
package io.virtbbs.virtand.data

import android.content.Context
import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase
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
    version = 2,
    exportSchema = false,
)
abstract class AppDatabase : RoomDatabase() {
    abstract fun conferenceDao(): ConferenceDao
    abstract fun messageDao(): MessageDao
    abstract fun fileDao(): FileDao
    abstract fun nodelistDao(): NodelistDao

    companion object {
        @Volatile private var instance: AppDatabase? = null

        private val MIGRATION_1_2 = object : Migration(1, 2) {
            override fun migrate(db: SupportSQLiteDatabase) {
                db.execSQL(
                    "ALTER TABLE conferences ADD COLUMN network TEXT NOT NULL DEFAULT 'Local'",
                )
            }
        }

        fun get(context: Context): AppDatabase =
            instance ?: synchronized(this) {
                instance ?: Room.databaseBuilder(context, AppDatabase::class.java, "virtand.db")
                    .addMigrations(MIGRATION_1_2)
                    .build()
                    .also { instance = it }
            }
    }
}
