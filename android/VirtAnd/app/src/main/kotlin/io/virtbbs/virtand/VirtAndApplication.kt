// VirtAnd — VirtAndApplication.kt
package io.virtbbs.virtand

import android.app.Application
import io.virtbbs.virtand.sync.SyncWorker

class VirtAndApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        SyncWorker.schedulePeriodic(this)
    }
}
