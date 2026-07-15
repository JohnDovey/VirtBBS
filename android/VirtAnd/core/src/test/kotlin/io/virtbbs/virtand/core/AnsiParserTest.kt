package io.virtbbs.virtand.core

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class AnsiParserTest {
    @Test
    fun plainText_singleSpan() {
        val spans = parseAnsi("Hello BBS")
        assertEquals(1, spans.size)
        assertEquals("Hello BBS", spans[0].text)
    }

    @Test
    fun redForeground() {
        val spans = parseAnsi("\u001B[31mRed\u001B[0m normal")
        assertEquals(2, spans.size)
        assertEquals("Red", spans[0].text)
        assertEquals(AnsiColor(170, 0, 0), spans[0].foreground)
        assertEquals(" normal", spans[1].text)
        assertEquals(null, spans[1].foreground)
    }

    @Test
    fun boldAndBrightBlue() {
        val spans = parseAnsi("\u001B[1;94mBold blue\u001B[0m")
        assertEquals(1, spans.size)
        assertTrue(spans[0].bold)
        assertEquals(AnsiColor(85, 85, 255), spans[0].foreground)
        assertEquals("Bold blue", spans[0].text)
    }

    @Test
    fun backgroundColor() {
        val spans = parseAnsi("\u001B[42mGreen bg\u001B[0m")
        assertEquals(AnsiColor(0, 170, 0), spans[0].background)
    }

    @Test
    fun stripsUnsupportedCsi() {
        val spans = parseAnsi("before\u001B[Kafter")
        assertEquals(1, spans.size)
        assertEquals("beforeafter", spans[0].text)
    }

    @Test
    fun stripsCarriageReturns() {
        assertEquals("line1\nline2", stripAnsi("line1\r\nline2"))
    }

    @Test
    fun emptyResetParam() {
        val spans = parseAnsi("\u001B[mtext")
        assertEquals("text", spans[0].text)
    }
}
