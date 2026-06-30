package io.virtbbs.virtand.core

import java.io.ByteArrayInputStream
import java.nio.charset.StandardCharsets
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

class UserApiClientTest {
    @Test
    fun readResponseLine_readsUntilNewline() {
        val input = ByteArrayInputStream("{\"ok\":true}\n".toByteArray(StandardCharsets.UTF_8))
        assertEquals("{\"ok\":true}", UserApiClient.readResponseLine(input))
    }

    @Test
    fun readResponseLine_handlesLargePayload() {
        val payload = "x".repeat(100_000)
        val input = ByteArrayInputStream("$payload\n".toByteArray(StandardCharsets.UTF_8))
        assertEquals(payload, UserApiClient.readResponseLine(input))
    }

    @Test
    fun readResponseLine_emptyStreamThrows() {
        val input = ByteArrayInputStream(ByteArray(0))
        assertFailsWith<UserApiException> {
            UserApiClient.readResponseLine(input)
        }
    }

    @Test
    fun asJsonArrayOrEmpty_treatsNullAsEmpty() {
        assertEquals(0, JsonNull.asJsonArrayOrEmpty().size)
        assertEquals(0, null.asJsonArrayOrEmpty().size)
        assertEquals(2, JsonArray(listOf(JsonPrimitive(1), JsonPrimitive(2))).asJsonArrayOrEmpty().size)
    }

    @Test
    fun timeoutFor_bulkMethodsUseLongerTimeout() {
        assertEquals(300_000, UserApiClient.timeoutFor("qwk.download"))
        assertEquals(30_000, UserApiClient.timeoutFor("conferences.list"))
    }
}
