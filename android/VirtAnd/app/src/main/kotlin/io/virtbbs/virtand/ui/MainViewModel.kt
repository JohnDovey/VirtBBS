// VirtAnd — MainViewModel.kt
package io.virtbbs.virtand.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import io.virtbbs.virtand.data.AppDatabase
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
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch

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

    private val _syncStatus = MutableStateFlow<String>("Not synced yet.")
    val syncStatus: StateFlow<String> = _syncStatus

    private val _syncing = MutableStateFlow(false)
    val syncing: StateFlow<Boolean> = _syncing

    fun messagesFor(conferenceId: Int) = db.messageDao().observeByConference(conferenceId)
    fun filesFor(dirId: Long) = db.fileDao().observeFiles(dirId)

    fun synchronize() {
        if (_syncing.value) return
        _syncing.value = true
        viewModelScope.launch {
            val result: SyncResult = syncEngine.synchronize()
            _syncStatus.value = if (result.error != null) {
                "Sync failed: ${result.error}"
            } else {
                "Synced: ${result.newMessages} new message(s), " +
                    "${result.repliesUploaded} reply(ies) sent, " +
                    "${result.filesUploaded} file(s) uploaded, " +
                    "${result.filesDownloaded} file(s) downloaded" +
                    if (result.nodelistsChanged.isNotEmpty()) ", nodelist updated: ${result.nodelistsChanged.joinToString()}" else ""
            }
            _syncing.value = false
        }
    }

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

    fun saveSettings(host: String, port: Int, token: String, networks: List<String>) {
        viewModelScope.launch {
            settings.save(host, port, token, networks)
        }
    }
}
