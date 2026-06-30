// VirtAnd — QwkPacket.kt
//
// Parses a downloaded QWK packet (from internal/userapi's "qwk.download")
// and builds an uploaded REP packet (for "qwk.upload"), matching the exact
// binary/text layouts produced and consumed by the server's internal/qwk
// package (see VirtBBS/internal/qwk/qwk.go) byte-for-byte:
//
//   MESSAGES.DAT — 128-byte blocks. Block 0 is reserved. Every message
//   starts on a block boundary with a 128-byte header record (see
//   MessageHeader), followed by body blocks soft-wrapped with the 0xE3
//   marker in place of line breaks, space-padded to fill the final block.
//
//   REP packets — a ZIP of flat "<N>.MSG" text files, each:
//     line 1: conference number
//     line 2: reference message number (0 = none)
//     line 3: To
//     line 4: From
//     line 5: Subject
//     line 6: (blank separator)
//     line 7+: body, to end of file
package io.virtbbs.virtand.core

import java.io.ByteArrayOutputStream
import java.util.zip.ZipEntry
import java.util.zip.ZipInputStream
import java.util.zip.ZipOutputStream

private const val BLOCK_SIZE = 128
private const val SOFT_CR = 0xE3 // soft line-break marker (NOT representable in US-ASCII)

// MESSAGES.DAT bytes are decoded as ISO-8859-1 (Latin-1), never US-ASCII or
// UTF-8: it's a lossless 1:1 mapping for every byte value 0x00-0xFF, so the
// SOFT_CR marker (0xE3) round-trips exactly. US-ASCII would silently replace
// any byte > 0x7F (including 0xE3 itself) with U+FFFD, corrupting every body.
private val RAW_BYTES_CHARSET = Charsets.ISO_8859_1

/** One message decoded from a downloaded QWK packet's MESSAGES.DAT. */
data class QwkMessage(
    val conferenceId: Int,
    val msgNumber: Int,
    val date: String,
    val time: String,
    val toName: String,
    val fromName: String,
    val subject: String,
    val body: String,
)

/** One locally-composed reply, queued for upload as part of a REP packet. */
data class QwkReply(
    val conferenceId: Int,
    val refNum: Int,
    val toName: String,
    val fromName: String,
    val subject: String,
    val body: String,
)

/**
 * Parses a downloaded QWK packet's raw ZIP bytes into a flat list of
 * messages — every header record in MESSAGES.DAT in order, regardless of
 * conference (each header already carries its own conference number, so a
 * single sequential walk is equivalent to following the per-conference
 * .NDX files and considerably simpler).
 */
fun parseQwkPacket(zipBytes: ByteArray): List<QwkMessage> {
    val messages = parseQwkMessagesDat(zipBytes)
    return mergeQwkAttachments(zipBytes, messages)
}

/** Reads MESSAGES.DAT only (no ATTACH sidecars). */
internal fun parseQwkMessagesDat(zipBytes: ByteArray): List<QwkMessage> {
    val messagesDat = readZipEntry(zipBytes, "MESSAGES.DAT")
        ?: throw IllegalArgumentException("QWK packet has no MESSAGES.DAT")

    val out = mutableListOf<QwkMessage>()
    // Block 0 is reserved — start at block 1.
    var offset = BLOCK_SIZE
    while (offset + BLOCK_SIZE <= messagesDat.size) {
        val header = decodeHeader(messagesDat, offset)
        val bodyStart = offset + BLOCK_SIZE
        val bodyBlocks = (header.numBlocks - 1).coerceAtLeast(0)
        val bodyEnd = (bodyStart + bodyBlocks * BLOCK_SIZE).coerceAtMost(messagesDat.size)

        out.add(
            QwkMessage(
                conferenceId = header.confNum,
                msgNumber = header.msgNum,
                date = header.date,
                time = header.time,
                toName = header.to,
                fromName = header.from,
                subject = header.subject,
                body = decodeBody(messagesDat, bodyStart, bodyEnd),
            )
        )

        val totalBlocks = header.numBlocks.coerceAtLeast(1)
        offset += totalBlocks * BLOCK_SIZE
    }
    return out
}

/**
 * Builds a REP packet's raw ZIP bytes from queued replies, in the exact
 * text layout internal/qwk.ParseRep expects.
 */
fun buildRepPacket(replies: List<QwkReply>): ByteArray {
    val out = ByteArrayOutputStream()
    ZipOutputStream(out).use { zos ->
        replies.forEachIndexed { i, r ->
            zos.putNextEntry(ZipEntry("${i + 1}.MSG"))
            val text = buildString {
                append(r.conferenceId).append("\r\n")
                append(r.refNum).append("\r\n")
                append(r.toName).append("\r\n")
                append(r.fromName).append("\r\n")
                append(r.subject).append("\r\n")
                append("\r\n")
                append(r.body)
            }
            zos.write(text.toByteArray(Charsets.US_ASCII))
            zos.closeEntry()
        }
    }
    return out.toByteArray()
}

// ── MESSAGES.DAT header record (128 bytes) ──────────────────────────────────

private class MessageHeader(
    val msgNum: Int,
    val date: String,
    val time: String,
    val to: String,
    val from: String,
    val subject: String,
    val numBlocks: Int,
    val confNum: Int,
)

private fun decodeHeader(data: ByteArray, offset: Int): MessageHeader {
    fun field(start: Int, len: Int): String =
        String(data, offset + start, len, RAW_BYTES_CHARSET).trim()

    return MessageHeader(
        msgNum = field(1, 7).toIntOrNull() ?: 0,
        date = field(8, 8),
        time = field(16, 5),
        to = field(21, 25),
        from = field(46, 25),
        subject = field(71, 25),
        numBlocks = field(116, 2).toIntOrNull() ?: 1,
        confNum = field(119, 7).toIntOrNull() ?: 0,
    )
}

private fun decodeBody(data: ByteArray, start: Int, end: Int): String {
    val raw = String(data, start, (end - start).coerceAtLeast(0), RAW_BYTES_CHARSET)
    return raw.replace(SOFT_CR.toChar().toString(), "\r\n").trimEnd(' ')
}

/**
 * Merges VirtBBS QWK extension attachments (ATTACH.IDX + ATTACH sidecar UUE files)
 * into message bodies when uuencode exceeds the classic 2-digit NumBlocks limit.
 */
internal fun mergeQwkAttachments(zipBytes: ByteArray, messages: List<QwkMessage>): List<QwkMessage> {
    val idxBytes = readZipEntry(zipBytes, "ATTACH.IDX") ?: return messages
    val idxText = String(idxBytes, RAW_BYTES_CHARSET)
    val extra = mutableMapOf<Pair<Int, Int>, String>()
    for (rawLine in idxText.split('\n', '\r')) {
        val line = rawLine.trim()
        if (line.isEmpty()) continue
        val parts = line.split(',').map { it.trim() }
        if (parts.size < 4) continue
        val confId = parts[0].toIntOrNull() ?: continue
        val msgNum = parts[1].toIntOrNull() ?: continue
        val zipPath = parts[3]
        val uueBytes = readZipEntry(zipBytes, zipPath) ?: continue
        val uueText = String(uueBytes, RAW_BYTES_CHARSET).trim()
        if (uueText.isEmpty()) continue
        val key = confId to msgNum
        extra[key] = if (key in extra) extra.getValue(key) + "\r\n\r\n" + uueText else uueText
    }
    if (extra.isEmpty()) return messages
    return messages.map { msg ->
        val attach = extra[msg.conferenceId to msg.msgNumber] ?: return@map msg
        msg.copy(
            body = buildString {
                append(msg.body.trimEnd())
                append("\r\n\r\n")
                append(attach)
            },
        )
    }
}

// ── ZIP helpers ──────────────────────────────────────────────────────────────

private fun readZipEntry(zipBytes: ByteArray, name: String): ByteArray? {
    ZipInputStream(zipBytes.inputStream()).use { zis ->
        var entry = zis.nextEntry
        while (entry != null) {
            if (entry.name.equals(name, ignoreCase = true)) {
                return zis.readBytes()
            }
            entry = zis.nextEntry
        }
    }
    return null
}
