// VirtAnd — SettingsStore.kt
package io.virtbbs.virtand.settings

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map

private val Context.dataStore by preferencesDataStore(name = "virtand_settings")

class SettingsStore(private val context: Context) {
    private object Keys {
        val HOST = stringPreferencesKey("host")
        val USER_API_PORT = intPreferencesKey("user_api_port")
        val USERNAME = stringPreferencesKey("username")
        val PASSWORD = stringPreferencesKey("password")
        val SUBSCRIBED_NETWORKS = stringPreferencesKey("subscribed_networks")
        val PURGE_DAYS = intPreferencesKey("purge_days")
    }

    val host: Flow<String> = context.dataStore.data.map { it[Keys.HOST] ?: "" }
    val userApiPort: Flow<Int> = context.dataStore.data.map { it[Keys.USER_API_PORT] ?: 9998 }
    val username: Flow<String> = context.dataStore.data.map { it[Keys.USERNAME] ?: "" }
    val password: Flow<String> = context.dataStore.data.map { it[Keys.PASSWORD] ?: "" }
    val subscribedNetworks: Flow<List<String>> = context.dataStore.data.map {
        (it[Keys.SUBSCRIBED_NETWORKS] ?: "FidoNet").split(",").filter { n -> n.isNotBlank() }
    }
    val purgeDays: Flow<Int> = context.dataStore.data.map { it[Keys.PURGE_DAYS] ?: 7 }

    suspend fun save(
        host: String,
        userApiPort: Int,
        username: String,
        password: String,
        subscribedNetworks: List<String>,
        purgeDays: Int = 7,
    ) {
        context.dataStore.edit { prefs ->
            prefs[Keys.HOST] = host
            prefs[Keys.USER_API_PORT] = userApiPort
            prefs[Keys.USERNAME] = username
            prefs[Keys.PASSWORD] = password
            prefs.remove(stringPreferencesKey("token"))
            prefs[Keys.SUBSCRIBED_NETWORKS] = subscribedNetworks.joinToString(",")
            prefs[Keys.PURGE_DAYS] = purgeDays.coerceAtLeast(1)
        }
    }

    suspend fun snapshot(): Snapshot = Snapshot(
        host = host.first(),
        userApiPort = userApiPort.first(),
        username = username.first(),
        password = password.first(),
        subscribedNetworks = subscribedNetworks.first(),
        purgeDays = purgeDays.first(),
    )

    data class Snapshot(
        val host: String,
        val userApiPort: Int,
        val username: String,
        val password: String,
        val subscribedNetworks: List<String>,
        val purgeDays: Int = 7,
    )
}
