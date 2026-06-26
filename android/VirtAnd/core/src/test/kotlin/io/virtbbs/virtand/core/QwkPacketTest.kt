// VirtAnd — QwkPacketTest.kt
// Round-trip verification against the exact MESSAGES.DAT/REP layouts
// produced and consumed by the server (internal/qwk/qwk.go).
package io.virtbbs.virtand.core

import java.io.ByteArrayOutputStream
import java.util.zip.ZipEntry
import java.util.zip.ZipInputStream
import java.util.zip.ZipOutputStream
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private const val BLOCK_SIZE = 128

/** Minimal test-only QWK packet encoder, mirroring internal/qwk.go's layout, used to verify parseQwkPacket. */
private fun encodeTestQwkPacket(messages: List<QwkMessage>): ByteArray {
    val dat = ByteArrayOutputStream()
    dat.write(ByteArray(BLOCK_SIZE) { ' '.code.toByte() }) // reserved block 0

    fun leftPad(n: Int, width: Int): String {
        val s = n.toString()
        return if (s.length >= width) s.take(width) else s + " ".repeat(width - s.length)
    }
    fun fixed(s: String, width: Int): String =
        if (s.length >= width) s.take(width) else s + " ".repeat(width - s.length)
    fun encodeBody(body: String): ByteArray {
        val out = ByteArrayOutputStream()
        body.replace("\r\n", "\n").split("\n").forEach { line ->
            out.write(line.toByteArray(Charsets.ISO_8859_1))
            out.write(0xE3)
        }
        while (out.size() % BLOCK_SIZE != 0) out.write(' '.code)
        return out.toByteArray()
    }

    messages.forEach { m ->
        val bodyBlocks = encodeBody(m.body)
        val numBlocks = 1 + bodyBlocks.size / BLOCK_SIZE
        val header = buildString {
            append(' ') // status
            append(leftPad(m.msgNumber, 7))
            append(fixed(m.date, 8))
            append(fixed(m.time, 5))
            append(fixed(m.toName, 25))
            append(fixed(m.fromName, 25))
            append(fixed(m.subject, 25))
            append(fixed("", 12)) // password
            append(leftPad(0, 8)) // ref num
            append(leftPad(numBlocks, 2))
            append(' ') // active
            append(leftPad(m.conferenceId, 7))
            append(' ') // net tag
            append(' ') // reserved
        }
        check(header.length == BLOCK_SIZE) { "test header encode produced ${header.length} bytes" }
        dat.write(header.toByteArray(Charsets.US_ASCII))
        dat.write(bodyBlocks)
    }

    val out = ByteArrayOutputStream()
    ZipOutputStream(out).use { zos ->
        zos.putNextEntry(ZipEntry("MESSAGES.DAT"))
        zos.write(dat.toByteArray())
        zos.closeEntry()
    }
    return out.toByteArray()
}

class QwkPacketTest {
    @Test
    fun parsesMultipleMessagesFromMessagesDat() {
        val originals = listOf(
            QwkMessage(1, 10, "06-26-26", "12:00", "All", "Sysop", "Hello", "Line one\r\nLine two"),
            QwkMessage(1, 11, "06-26-26", "12:05", "All", "Sysop", "Second", "Just one line"),
        )
        val packet = encodeTestQwkPacket(originals)
        val parsed = parseQwkPacket(packet)

        assertEquals(2, parsed.size)
        parsed.forEachIndexed { i, m ->
            assertEquals(originals[i].conferenceId, m.conferenceId)
            assertEquals(originals[i].msgNumber, m.msgNumber)
            assertEquals(originals[i].toName, m.toName)
            assertEquals(originals[i].fromName, m.fromName)
            assertEquals(originals[i].subject, m.subject)
            // encodeBody terminates every line (including the last) with the
            // soft-CR marker, so the decoded body carries one trailing
            // "\r\n" versus the original — same behavior as the server's
            // own round-trip test (internal/qwk/qwk_test.go).
            assertEquals(originals[i].body, m.body.removeSuffix("\r\n"))
        }
    }

    @Test
    fun buildsRepPacketInExpectedTextLayout() {
        val replies = listOf(
            QwkReply(conferenceId = 1, refNum = 0, toName = "All", fromName = "PointUser", subject = "Re: Hello", body = "My reply.\r\nSecond line."),
        )
        val zipBytes = buildRepPacket(replies)

        ZipInputStream(zipBytes.inputStream()).use { zis ->
            val entry = zis.nextEntry
            assertEquals("1.MSG", entry?.name)
            val text = zis.readBytes().toString(Charsets.US_ASCII)
            val lines = text.split("\r\n")
            assertEquals("1", lines[0])
            assertEquals("0", lines[1])
            assertEquals("All", lines[2])
            assertEquals("PointUser", lines[3])
            assertEquals("Re: Hello", lines[4])
            assertEquals("", lines[5])
            assertTrue(text.endsWith("My reply.\r\nSecond line."))
        }
    }
}
