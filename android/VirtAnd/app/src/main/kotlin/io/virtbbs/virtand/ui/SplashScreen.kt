// VirtAnd — SplashScreen.kt
package io.virtbbs.virtand.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp

@Composable
fun SplashScreen(viewModel: MainViewModel, onDismiss: () -> Unit) {
    val host by viewModel.settings.host.collectAsState(initial = "")
    val username by viewModel.settings.username.collectAsState(initial = "")
    val password by viewModel.settings.password.collectAsState(initial = "")
    val port by viewModel.settings.userApiPort.collectAsState(initial = 9998)

    var hostField by remember(host) { mutableStateOf(host) }
    var portField by remember(port) { mutableStateOf(port.toString()) }
    var usernameField by remember(username) { mutableStateOf(username) }
    var passwordField by remember(password) { mutableStateOf(password) }
    var showLogin by remember { mutableStateOf(host.isBlank() || username.isBlank()) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(24.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text("VirtAnd", style = MaterialTheme.typography.headlineLarge, fontWeight = FontWeight.Bold)
        Spacer(modifier = Modifier.height(8.dp))
        Text(
            "Your pocket connection to the VirtBBS bulletin board system.",
            textAlign = TextAlign.Center,
            style = MaterialTheme.typography.bodyLarge,
        )
        Spacer(modifier = Modifier.height(32.dp))

        if (showLogin) {
            TextField(value = hostField, onValueChange = { hostField = it }, label = { Text("Host") })
            TextField(value = portField, onValueChange = { portField = it }, label = { Text("User API Port") })
            TextField(value = usernameField, onValueChange = { usernameField = it }, label = { Text("Username") })
            TextField(
                value = passwordField,
                onValueChange = { passwordField = it },
                label = { Text("Password") },
                visualTransformation = PasswordVisualTransformation(),
            )
            Spacer(modifier = Modifier.height(16.dp))
            Button(onClick = {
                viewModel.saveSettings(
                    hostField.trim(),
                    portField.toIntOrNull() ?: 9998,
                    usernameField.trim(),
                    passwordField,
                    listOf("FidoNet"),
                )
                onDismiss()
            }) { Text("Login") }
        } else {
            Text("Signed in as $username", style = MaterialTheme.typography.bodyMedium)
            Text("$host:${portField.toIntOrNull() ?: 9998}", style = MaterialTheme.typography.bodySmall)
            Spacer(modifier = Modifier.height(16.dp))
            Button(onClick = onDismiss) { Text("OK") }
            OutlinedButton(onClick = { showLogin = true }) { Text("Change login") }
        }
    }
}
