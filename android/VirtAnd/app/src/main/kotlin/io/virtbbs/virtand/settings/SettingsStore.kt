// VirtAnd — SettingsStore.kt
package io.virtbbs.virtand.settings

import android.content.Context
import androidx.datastore.preferences.core.booleanPreferencesKey
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
        val RENDER_ANSI = booleanPreferencesKey("render_ansi")
        val LOCAL_FILE_SEARCH = booleanPreferencesKey("local_file_search")
    }

    val host: Flow<String> = context.dataStore.data.map { it[Keys.HOST] ?: "" }
    val userApiPort: Flow<Int> = context.dataStore.data.map { it[Keys.USER_API_PORT] ?: 9998 }
    val username: Flow<String> = context.dataStore.data.map { it[Keys.USERNAME] ?: "" }
    val password: Flow<String> = context.dataStore.data.map { it[Keys.PASSWORD] ?: "" }
    val subscribedNetworks: Flow<List<String>> = context.dataStore.data.map {
        (it[Keys.SUBSCRIBED_NETWORKS] ?: "FidoNet").split(",").filter { n -> n.isNotBlank() }
    }
    val purgeDays: Flow<Int> = context.dataStore.data.map { it[Keys.PURGE_DAYS] ?: 7 }
    val renderAnsi: Flow<Boolean> = context.dataStore.data.map { it[Keys.RENDER_ANSI] ?: true }
    val localFileSearch: Flow<Boolean> = context.dataStore.data.map { it[Keys.LOCAL_FILE_SEARCH] ?: true }

    suspend fun save(
        host: String,
        userApiPort: Int,
        username: String,
        password: String,
        subscribedNetworks: List<String>,
        purgeDays: Int = 7,
        renderAnsi: Boolean = true,
        localFileSearch: Boolean = true,
    ) {
        context.dataStore.edit { prefs ->
            prefs[Keys.HOST] = host
            prefs[Keys.USER_API_PORT] = userApiPort
            prefs[Keys.USERNAME] = username
            prefs[Keys.PASSWORD] = password
            prefs.remove(stringPreferencesKey("token"))
            prefs[Keys.SUBSCRIBED_NETWORKS] = subscribedNetworks.joinToString(",")
            prefs[Keys.PURGE_DAYS] = purgeDays.coerceAtLeast(1)
            prefs[Keys.RENDER_ANSI] = renderAnsi
            prefs[Keys.LOCAL_FILE_SEARCH] = localFileSearch
        }
    }

    suspend fun setRenderAnsi(enabled: Boolean) {
        context.dataStore.edit { prefs -> prefs[Keys.RENDER_ANSI] = enabled }
    }

    suspend fun setLocalFileSearch(enabled: Boolean) {
        context.dataStore.edit { prefs -> prefs[Keys.LOCAL_FILE_SEARCH] = enabled }
    }

    suspend fun snapshot(): Snapshot = Snapshot(
        host = host.first(),
        userApiPort = userApiPort.first(),
        username = username.first(),
        password = password.first(),
        subscribedNetworks = subscribedNetworks.first(),
        purgeDays = purgeDays.first(),
        renderAnsi = renderAnsi.first(),
        localFileSearch = localFileSearch.first(),
    )

    data class Snapshot(
        val host: String,
        val userApiPort: Int,
        val username: String,
        val password: String,
        val subscribedNetworks: List<String>,
        val purgeDays: Int = 7,
        val renderAnsi: Boolean = true,
        val localFileSearch: Boolean = true,
    )
}
