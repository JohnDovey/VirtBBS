// VirtAnd — SyncWorker.kt
//
// Background sync via WorkManager. Note the real constraint from the plan:
// Doze/battery restrictions mean this will not run promptly — WorkManager's
// periodic work has a 15-minute minimum interval, plus further deferral
// under battery optimization. That's accepted v1 behavior here, since the
// spec's primary flow is the user tapping "Synchronize" manually; this
// worker is a convenience for catching new mail when the app isn't open,
// not a real-time delivery mechanism.
package io.virtbbs.virtand.sync

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import androidx.work.Constraints
import java.util.concurrent.TimeUnit

class SyncWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        val result = SyncEngine(applicationContext).synchronize()
        return if (result.error == null) Result.success() else Result.retry()
    }

    companion object {
        private const val WORK_NAME = "virtand_periodic_sync"

        fun schedulePeriodic(context: Context) {
            val constraints = Constraints.Builder()
                .setRequiredNetworkType(NetworkType.CONNECTED)
                .build()

            val request = PeriodicWorkRequestBuilder<SyncWorker>(15, TimeUnit.MINUTES)
                .setConstraints(constraints)
                .build()

            WorkManager.getInstance(context)
                .enqueueUniquePeriodicWork(WORK_NAME, ExistingPeriodicWorkPolicy.KEEP, request)
        }

        fun cancelPeriodic(context: Context) {
            WorkManager.getInstance(context).cancelUniqueWork(WORK_NAME)
        }
    }
}
