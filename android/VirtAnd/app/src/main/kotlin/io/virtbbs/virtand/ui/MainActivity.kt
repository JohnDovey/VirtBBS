// VirtAnd — MainActivity.kt
package io.virtbbs.virtand.ui

import android.Manifest
import android.content.Intent
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import io.virtbbs.virtand.notification.SyncNotifications

class MainActivity : ComponentActivity() {
    private val viewModel: MainViewModel by viewModels()
    private val openTabRoute = mutableStateOf<String?>(null)

    private val requestNotificationPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { /* best-effort; sync still works without notifications */ }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        openTabRoute.value = intent.getStringExtra(SyncNotifications.EXTRA_OPEN_TAB)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            requestNotificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
        setContent {
            val tabToOpen = openTabRoute.value
            MaterialTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    VirtAndApp(
                        viewModel = viewModel,
                        openTabRoute = tabToOpen,
                        onOpenTabHandled = { openTabRoute.value = null },
                    )
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        openTabRoute.value = intent.getStringExtra(SyncNotifications.EXTRA_OPEN_TAB)
    }
}

private sealed class Tab(val route: String, val label: String) {
    data object Messages : Tab("messages", "Messages")
    data object Files : Tab("files", "Files")
    data object Queue : Tab("queue", "Queue")
    data object Settings : Tab("settings", "Settings")
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun VirtAndApp(
    viewModel: MainViewModel,
    openTabRoute: String? = null,
    onOpenTabHandled: () -> Unit = {},
) {
    var splashDone by remember { mutableStateOf(false) }
    val host by viewModel.settings.host.collectAsState(initial = "")
    val username by viewModel.settings.username.collectAsState(initial = "")

    if (!splashDone) {
        SplashScreen(viewModel) { splashDone = true }
        return
    }

    LaunchedEffect(splashDone) {
        if (splashDone && host.isNotBlank() && username.isNotBlank()) {
            viewModel.synchronize()
        }
    }

    val nav = rememberNavController()
    LaunchedEffect(openTabRoute) {
        if (openTabRoute == SyncNotifications.TAB_MESSAGES) {
            nav.navigate(Tab.Messages.route) { launchSingleTop = true }
            onOpenTabHandled()
        }
    }
    val syncing by viewModel.syncing.collectAsState()
    val syncStatus by viewModel.syncStatus.collectAsState()
    val session by viewModel.sessionInfo.collectAsState()
    val queuedReplies by viewModel.queuedReplies.collectAsState()
    val queuedUploads by viewModel.queuedUploads.collectAsState()
    val queuedDownloads by viewModel.queuedDownloads.collectAsState()
    val queueCount = queuedReplies.size + queuedUploads.size + queuedDownloads.size

    val title = when {
        session.bbsName.isNotBlank() && session.userName.isNotBlank() ->
            "${session.bbsName} — ${session.userName}"
        session.userName.isNotBlank() -> session.userName
        else -> "VirtAnd"
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text(title)
                        if (session.securityLevel > 0) {
                            Text(
                                "Security ${session.securityLevel}${if (session.sysop) " · Sysop" else ""}",
                                style = MaterialTheme.typography.bodySmall,
                            )
                        }
                    }
                },
                actions = {
                    IconButton(onClick = { viewModel.synchronize() }, enabled = !syncing) {
                        Icon(Icons.Filled.Refresh, contentDescription = "Synchronize")
                    }
                },
            )
        },
        bottomBar = {
            val backStackEntry by nav.currentBackStackEntryAsState()
            val current = backStackEntry?.destination?.route
            NavigationBar {
                listOf(Tab.Messages, Tab.Files, Tab.Queue, Tab.Settings).forEach { tab ->
                    val badge = if (tab == Tab.Queue && queueCount > 0) " ($queueCount)" else ""
                    NavigationBarItem(
                        selected = current == tab.route,
                        onClick = { nav.navigate(tab.route) { launchSingleTop = true } },
                        icon = {},
                        label = { Text(tab.label + badge) },
                    )
                }
            }
        },
    ) { scaffoldPadding ->
        Column(modifier = Modifier.padding(scaffoldPadding)) {
            Text(syncStatus, modifier = Modifier.padding(8.dp), style = MaterialTheme.typography.bodySmall)
            NavHost(
                navController = nav,
                startDestination = Tab.Messages.route,
                modifier = Modifier.fillMaxSize(),
            ) {
                composable(Tab.Messages.route) { MessagesScreen(viewModel) }
                composable(Tab.Files.route) { FilesScreen(viewModel) }
                composable(Tab.Queue.route) { QueueScreen(viewModel) }
                composable(Tab.Settings.route) { SettingsScreen(viewModel) }
            }
        }
    }
}
