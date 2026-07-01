// VirtAnd — UserApiClient.kt
//
// JSON-over-TCP client for VirtBBS's internal/userapi. There's no HTTP
// server on the BBS side — just a raw newline-delimited-JSON socket protocol
// (see internal/userapi/server.go), so this opens a plain java.net.Socket per
// call rather than using an HTTP client library.
package io.virtbbs.virtand.core

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import java.io.ByteArrayOutputStream
import java.net.Socket
import java.nio.charset.StandardCharsets

class UserApiException(message: String) : Exception(message)

/**
 * One JSON/TCP call against internal/userapi. Opens a fresh connection,
 * sends one newline-delimited JSON request, reads one newline-delimited
 * JSON response line, and closes.
 */
class UserApiClient(
    var host: String = "127.0.0.1",
    var port: Int = 9998,
    var username: String = "",
    var password: String = "",
) {
    private val json = Json { ignoreUnknownKeys = true }

    /** Sends [method] with [params] (any JSON-serializable value, or null) and returns the "result" field. */
    fun call(method: String, params: JsonElement? = null, readTimeoutMs: Int = timeoutFor(method)): JsonElement? {
        val req = buildJsonRequest(method, params, username, password)
        val reqLine = json.encodeToString(JsonObject.serializer(), req) + "\n"

        Socket(host, port).use { socket ->
            socket.soTimeout = readTimeoutMs
            val out = socket.getOutputStream()
            out.write(reqLine.toByteArray(StandardCharsets.UTF_8))
            out.flush()

            val respLine = readResponseLine(socket.getInputStream())
            val resp = try {
                json.parseToJsonElement(respLine).jsonObject
            } catch (e: Exception) {
                throw UserApiException(
                    "$method: invalid JSON response (${respLine.length} bytes): ${e.message}"
                )
            }
            val error = resp["error"]
            if (error != null && error != JsonNull) {
                val msg = error.jsonPrimitive.content
                if (msg.isNotEmpty()) throw UserApiException(msg)
            }
            return resp["result"]
        }
    }

    companion object {
        /** Matches internal/userapi maxLineSize — single-line JSON must fit. */
        private const val MAX_LINE_BYTES = 16 * 1024 * 1024
        private const val DEFAULT_TIMEOUT_MS = 30_000
        private const val BULK_TIMEOUT_MS = 300_000

        private val bulkMethods = setOf(
            "messages.sync",
            "messages.post",
            "messages.attachment.download",
            "files.download",
            "files.upload",
        )

        fun timeoutFor(method: String): Int =
            if (method in bulkMethods) BULK_TIMEOUT_MS else DEFAULT_TIMEOUT_MS

        /**
         * Reads one newline-terminated line without BufferedReader.readLine()'s
         * practical size limits and with an explicit byte cap matching the server.
         */
        internal fun readResponseLine(input: java.io.InputStream): String {
            val out = ByteArrayOutputStream(64 * 1024)
            val chunk = ByteArray(64 * 1024)
            while (true) {
                val n = input.read(chunk)
                if (n == -1) break
                for (i in 0 until n) {
                    val b = chunk[i]
                    if (b == '\n'.code.toByte()) {
                        return out.toString(StandardCharsets.UTF_8.name())
                    }
                    if (out.size() >= MAX_LINE_BYTES) {
                        throw UserApiException("Response exceeds maximum size ($MAX_LINE_BYTES bytes).")
                    }
                    out.write(b.toInt())
                }
            }
            if (out.size() == 0) throw UserApiException("Empty response from server.")
            return out.toString(StandardCharsets.UTF_8.name())
        }
    }

    /** Quick connectivity check — true if an authenticated call succeeds. */
    fun testConnection(): Boolean = try {
        call("conferences.list")
        true
    } catch (_: Exception) {
        false
    }
}

/**
 * Builds the {"method", "params", "auth": {"username","password"}} envelope as a
 * JsonObject, matching internal/userapi's Request{Method,Params,Auth} shape.
 */
fun buildJsonRequest(
    method: String,
    params: JsonElement?,
    username: String,
    password: String,
): JsonObject {
    val map = linkedMapOf<String, JsonElement>(
        "method" to JsonPrimitive(method),
        "auth" to JsonObject(
            mapOf(
                "username" to JsonPrimitive(username),
                "password" to JsonPrimitive(password),
            )
        ),
    )
    if (params != null) map["params"] = params
    return JsonObject(map)
}

/** Go encodes nil slices as JSON null; treat null and JsonNull as an empty array. */
fun JsonElement?.asJsonArrayOrEmpty(): JsonArray = when (this) {
    null, is JsonNull -> JsonArray(emptyList())
    else -> this.jsonArray
}
