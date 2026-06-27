// VirtAnd — AppScreens.kt
package io.virtbbs.virtand.ui

import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.virtbbs.virtand.data.entities.CachedMessageEntity
import io.virtbbs.virtand.data.entities.QueuedDownloadEntity
import io.virtbbs.virtand.data.entities.QueuedReplyEntity
import io.virtbbs.virtand.data.entities.QueuedUploadEntity
import kotlinx.coroutines.launch

@Composable
fun MessagesScreen(viewModel: MainViewModel) {
    val conferences by viewModel.conferences.collectAsState()
    var selectedConference by remember { mutableStateOf<Int?>(null) }
    var selectedMessageId by remember { mutableStateOf<Long?>(null) }

    when {
        selectedMessageId != null -> MessageDetailScreen(
            viewModel = viewModel,
            localId = selectedMessageId!!,
            onBack = { selectedMessageId = null },
        )
        selectedConference == null -> {
            LazyColumn {
                items(conferences) { conf ->
                    ListItem(
                        headlineContent = { Text(conf.name) },
                        supportingContent = { Text(conf.description) },
                        modifier = Modifier
                            .padding(4.dp)
                            .clickable { selectedConference = conf.id },
                    )
                }
            }
        }
        else -> {
            val conferenceId = selectedConference!!
            val confName = conferences.firstOrNull { it.id == conferenceId }?.name ?: "Messages"
            val messages by viewModel.messagesFor(conferenceId).collectAsState(initial = emptyList())
            var showCompose by remember { mutableStateOf(false) }

            Column {
                Row(modifier = Modifier.padding(8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedButton(onClick = { selectedConference = null }) { Text("< Conferences") }
                    Button(onClick = { showCompose = true }) { Text("New Message") }
                }
                Text(confName, modifier = Modifier.padding(horizontal = 12.dp), fontWeight = FontWeight.Bold)
                LazyColumn {
                    items(messages, key = { it.localId }) { msg ->
                        MessageListItem(
                            msg = msg,
                            onOpen = { selectedMessageId = msg.localId },
                        )
                    }
                }
            }

            if (showCompose) {
                ComposeDialog(
                    title = "New Message",
                    onDismiss = { showCompose = false },
                    onSend = { to, subject, body ->
                        viewModel.queueNewMessage(conferenceId, to, subject, body)
                        showCompose = false
                    },
                )
            }
        }
    }
}

@Composable
private fun MessageListItem(msg: CachedMessageEntity, onOpen: () -> Unit) {
    ListItem(
        headlineContent = {
            Text(
                "${msg.fromName} → ${msg.toName}: ${msg.subject}",
                fontWeight = if (msg.isRead) FontWeight.Normal else FontWeight.Bold,
            )
        },
        supportingContent = {
            Text(
                "${msg.date} ${msg.time} — ${msg.body.replace("\r", "").take(120)}",
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
        },
        modifier = Modifier
            .padding(4.dp)
            .clickable(onClick = onOpen),
    )
}

@Composable
fun MessageDetailScreen(viewModel: MainViewModel, localId: Long, onBack: () -> Unit) {
    val scope = rememberCoroutineScope()
    var msg by remember { mutableStateOf<CachedMessageEntity?>(null) }
    var showReply by remember { mutableStateOf(false) }

    androidx.compose.runtime.LaunchedEffect(localId) {
        msg = viewModel.getMessage(localId)
        viewModel.markMessageRead(localId)
    }

    val message = msg
    if (message == null) {
        Column(modifier = Modifier.padding(16.dp)) {
            OutlinedButton(onClick = onBack) { Text("Back") }
            Text("Loading…")
        }
        return
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(12.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        OutlinedButton(onClick = onBack) { Text("< Back") }
        Text("From: ${message.fromName}", fontWeight = FontWeight.Bold)
        Text("To: ${message.toName}")
        Text("Subject: ${message.subject}")
        Text("${message.date} ${message.time}", style = MaterialTheme.typography.bodySmall)
        Text(message.body.replace("\r", ""))
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = { showReply = true }) { Text("Reply") }
        }
    }

    if (showReply) {
        ComposeDialog(
            title = "Reply",
            initialSubject = "Re: ${message.subject}",
            onDismiss = { showReply = false },
            onSend = { to, subject, body ->
                viewModel.queueReply(message.conferenceId, message.msgNumber, to, subject, body)
                showReply = false
                scope.launch { onBack() }
            },
        )
    }
}

@Composable
private fun ComposeDialog(
    title: String,
    initialSubject: String = "",
    onDismiss: () -> Unit,
    onSend: (to: String, subject: String, body: String) -> Unit,
) {
    var to by remember { mutableStateOf("All") }
    var subject by remember { mutableStateOf(initialSubject) }
    var body by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                TextField(value = to, onValueChange = { to = it }, label = { Text("To") })
                TextField(value = subject, onValueChange = { subject = it }, label = { Text("Subject") })
                TextField(
                    value = body,
                    onValueChange = { body = it },
                    label = { Text("Message") },
                    minLines = 4,
                )
            }
        },
        confirmButton = {
            TextButton(
                onClick = {
                    if (subject.isNotBlank() && body.isNotBlank()) {
                        onSend(to.trim(), subject.trim(), body)
                    }
                },
            ) { Text("Queue") }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("Cancel") }
        },
    )
}

@Composable
fun FilesScreen(viewModel: MainViewModel) {
    val dirs by viewModel.fileDirs.collectAsState()
    val host by viewModel.settings.host.collectAsState(initial = "")
    val token by viewModel.settings.token.collectAsState(initial = "")
    val port by viewModel.settings.userApiPort.collectAsState(initial = 9998)
    val searchResults by viewModel.fileSearchResults.collectAsState()
    var selectedDir by remember { mutableStateOf<Long?>(null) }
    var searchQuery by remember { mutableStateOf("") }
    var pendingUpload by remember { mutableStateOf<Triple<Long, Uri, String>?>(null) }
    val context = LocalContext.current

    val pickFile = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null && selectedDir != null) {
            context.contentResolver.takePersistableUriPermission(
                uri, android.content.Intent.FLAG_GRANT_READ_URI_PERMISSION,
            )
            val filename = uri.lastPathSegment?.substringAfterLast('/') ?: "upload.bin"
            pendingUpload = Triple(selectedDir!!, uri, filename)
        }
    }

    Column(modifier = Modifier.fillMaxSize()) {
        Row(modifier = Modifier.padding(8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            TextField(
                value = searchQuery,
                onValueChange = { searchQuery = it },
                label = { Text("Search files") },
                modifier = Modifier.weight(1f),
            )
            Button(onClick = { viewModel.searchFiles(host, port, token, searchQuery) }) {
                Text("Search")
            }
        }

        if (searchResults.isNotEmpty()) {
            Text("Search results", modifier = Modifier.padding(horizontal = 12.dp), fontWeight = FontWeight.Bold)
            LazyColumn(modifier = Modifier.weight(1f, fill = false)) {
                items(searchResults) { f ->
                    ListItem(
                        headlineContent = { Text(f.filename) },
                        supportingContent = { Text("${f.size} bytes — ${f.description}") },
                    )
                }
            }
        }

        if (selectedDir == null) {
            LazyColumn {
                items(dirs) { dir ->
                    ListItem(
                        headlineContent = { Text(dir.name) },
                        supportingContent = { Text(dir.description) },
                        modifier = Modifier
                            .padding(4.dp)
                            .clickable { selectedDir = dir.id },
                    )
                }
            }
        } else {
            val dirId = selectedDir!!
            val files by viewModel.filesFor(dirId).collectAsState(initial = emptyList())

            Row(modifier = Modifier.padding(8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(onClick = { selectedDir = null }) { Text("< File Areas") }
                Button(onClick = { pickFile.launch(arrayOf("*/*")) }) { Text("Upload") }
            }
            LazyColumn {
                items(files) { f ->
                    ListItem(
                        headlineContent = { Text(f.filename) },
                        supportingContent = { Text("${f.size} bytes — ${f.description}") },
                        trailingContent = {
                            Button(onClick = { viewModel.queueDownload(dirId, f.filename) }) {
                                Text("Queue DL")
                            }
                        },
                        modifier = Modifier.padding(4.dp),
                    )
                }
            }
        }
    }

    pendingUpload?.let { (dirId, uri, filename) ->
        var description by remember { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = { pendingUpload = null },
            title = { Text("Upload description") },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(filename)
                    TextField(
                        value = description,
                        onValueChange = { description = it },
                        label = { Text("Description") },
                    )
                }
            },
            confirmButton = {
                TextButton(onClick = {
                    viewModel.queueUpload(dirId, uri.toString(), filename, description.trim())
                    pendingUpload = null
                }) { Text("Queue upload") }
            },
            dismissButton = {
                TextButton(onClick = { pendingUpload = null }) { Text("Cancel") }
            },
        )
    }
}

@Composable
fun QueueScreen(viewModel: MainViewModel) {
    val replies by viewModel.queuedReplies.collectAsState()
    val uploads by viewModel.queuedUploads.collectAsState()
    val downloads by viewModel.queuedDownloads.collectAsState()

    LazyColumn(modifier = Modifier.padding(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        item {
            Text("Pending replies (${replies.size})", fontWeight = FontWeight.Bold)
        }
        items(replies) { r ->
            QueueCard(
                title = viewModel.conferenceName(r.conferenceId),
                detail = "${r.subject}\nTo: ${r.toName}\n${r.body.take(80)}",
                onRemove = { viewModel.removeQueuedReply(r.id) },
            )
        }
        item {
            Text("Pending uploads (${uploads.size})", fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 8.dp))
        }
        items(uploads) { u ->
            QueueCard(
                title = viewModel.fileDirName(u.dirId),
                detail = "${u.filename}\n${u.description.ifBlank { "(no description)" }}",
                onRemove = { viewModel.removeQueuedUpload(u) },
            )
        }
        item {
            Text("Pending downloads (${downloads.size})", fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 8.dp))
        }
        items(downloads) { d ->
            QueueCard(
                title = viewModel.fileDirName(d.dirId),
                detail = d.filename,
                onRemove = { viewModel.removeQueuedDownload(d) },
            )
        }
        if (replies.isEmpty() && uploads.isEmpty() && downloads.isEmpty()) {
            item { Text("Nothing queued — compose replies or file transfers, then tap Synchronize.") }
        }
    }
}

@Composable
private fun QueueCard(title: String, detail: String, onRemove: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(title, fontWeight = FontWeight.Bold)
            Text(detail, style = MaterialTheme.typography.bodySmall)
            OutlinedButton(onClick = onRemove) { Text("Remove") }
        }
    }
}

@Composable
fun SettingsScreen(viewModel: MainViewModel) {
    val host by viewModel.settings.host.collectAsState(initial = "")
    val token by viewModel.settings.token.collectAsState(initial = "")
    val port by viewModel.settings.userApiPort.collectAsState(initial = 9998)
    val networks by viewModel.settings.subscribedNetworks.collectAsState(initial = listOf("FidoNet"))
    val connectionStatus by viewModel.connectionStatus.collectAsState()
    val nodeResults by viewModel.nodeSearchResults.collectAsState()

    var hostField by remember(host) { mutableStateOf(host) }
    var portField by remember(port) { mutableStateOf(port.toString()) }
    var tokenField by remember(token) { mutableStateOf(token) }
    var networksField by remember(networks) { mutableStateOf(networks.joinToString(",")) }
    var nodeQuery by remember { mutableStateOf("") }
    var nodeNetwork by remember(networks) { mutableStateOf(networks.firstOrNull() ?: "FidoNet") }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text("Generate a token on the BBS: Profile → [T]okens → [G]enerate.")
        TextField(value = hostField, onValueChange = { hostField = it }, label = { Text("Host") })
        TextField(value = portField, onValueChange = { portField = it }, label = { Text("User API Port") })
        TextField(value = tokenField, onValueChange = { tokenField = it }, label = { Text("API Token") })
        TextField(
            value = networksField,
            onValueChange = { networksField = it },
            label = { Text("Subscribed FidoNet networks (comma-separated)") },
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = {
                viewModel.saveSettings(
                    hostField,
                    portField.toIntOrNull() ?: 9998,
                    tokenField,
                    networksField.split(",").map { it.trim() }.filter { it.isNotEmpty() },
                )
            }) { Text("Save") }
            OutlinedButton(onClick = {
                viewModel.testConnection(hostField, portField.toIntOrNull() ?: 9998, tokenField)
            }) { Text("Test connection") }
        }
        if (connectionStatus.isNotBlank()) {
            Text(connectionStatus, style = MaterialTheme.typography.bodySmall)
        }

        Text("FidoNet node lookup", fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 12.dp))
        TextField(value = nodeNetwork, onValueChange = { nodeNetwork = it }, label = { Text("Network") })
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            TextField(
                value = nodeQuery,
                onValueChange = { nodeQuery = it },
                label = { Text("Search nodes") },
                modifier = Modifier.weight(1f),
            )
            Button(onClick = {
                viewModel.searchNodes(hostField, portField.toIntOrNull() ?: 9998, tokenField, nodeNetwork, nodeQuery)
            }) { Text("Go") }
        }
        nodeResults.forEach { n ->
            ListItem(
                headlineContent = { Text("${n.address} — ${n.name}") },
                supportingContent = { Text("${n.location} (sysop: ${n.sysop})") },
            )
        }
    }
}
