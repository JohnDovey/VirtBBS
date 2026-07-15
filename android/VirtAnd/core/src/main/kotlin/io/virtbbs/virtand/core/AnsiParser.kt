// VirtAnd — AnsiParser.kt
//
// Minimal SGR (Select Graphic Rendition) ANSI parser for BBS message bodies.
// Strips unsupported CSI sequences; maps foreground/background colors and bold.
package io.virtbbs.virtand.core

data class AnsiColor(val red: Int, val green: Int, val blue: Int)

data class AnsiSpan(
    val text: String,
    val foreground: AnsiColor? = null,
    val background: AnsiColor? = null,
    val bold: Boolean = false,
)

private val FG_STANDARD = arrayOf(
    AnsiColor(0, 0, 0),       // 30 black
    AnsiColor(170, 0, 0),     // 31 red
    AnsiColor(0, 170, 0),     // 32 green
    AnsiColor(170, 170, 0),   // 33 yellow
    AnsiColor(0, 0, 170),     // 34 blue
    AnsiColor(170, 0, 170),   // 35 magenta
    AnsiColor(0, 170, 170),   // 36 cyan
    AnsiColor(170, 170, 170), // 37 white
)

private val FG_BRIGHT = arrayOf(
    AnsiColor(85, 85, 85),    // 90 bright black
    AnsiColor(255, 85, 85),   // 91 bright red
    AnsiColor(85, 255, 85),   // 92 bright green
    AnsiColor(255, 255, 85),  // 93 bright yellow
    AnsiColor(85, 85, 255),   // 94 bright blue
    AnsiColor(255, 85, 255),  // 95 bright magenta
    AnsiColor(85, 255, 255),  // 96 bright cyan
    AnsiColor(255, 255, 255), // 97 bright white
)

private val BG_STANDARD = arrayOf(
    AnsiColor(0, 0, 0),
    AnsiColor(170, 0, 0),
    AnsiColor(0, 170, 0),
    AnsiColor(170, 170, 0),
    AnsiColor(0, 0, 170),
    AnsiColor(170, 0, 170),
    AnsiColor(0, 170, 170),
    AnsiColor(170, 170, 170),
)

/**
 * Parse SGR ANSI escape sequences into styled spans. Unsupported CSI sequences
 * are stripped. Carriage returns are removed (CP437/ISO-8859-1 bodies often use CR).
 */
fun parseAnsi(input: String): List<AnsiSpan> {
    var bold = false
    var fg: AnsiColor? = null
    var bg: AnsiColor? = null
    val spans = mutableListOf<AnsiSpan>()
    val buf = StringBuilder()

    fun flush() {
        if (buf.isEmpty()) return
        spans.add(AnsiSpan(buf.toString(), fg, bg, bold))
        buf.clear()
    }

    var i = 0
    while (i < input.length) {
        val ch = input[i]
        if (ch == '\u001B' && i + 1 < input.length && input[i + 1] == '[') {
            var j = i + 2
            while (j < input.length) {
                val c = input[j]
                if (c in '@'..'~') break
                j++
            }
            if (j < input.length) {
                val cmd = input[j]
                if (cmd == 'm') {
                    flush()
                    val updated = applySgr(input.substring(i + 2, j), bold, fg, bg)
                    bold = updated.first
                    fg = updated.second
                    bg = updated.third
                }
                i = j + 1
                continue
            }
        }
        if (ch == '\r') {
            i++
            continue
        }
        buf.append(ch)
        i++
    }
    flush()
    return spans.ifEmpty { listOf(AnsiSpan("")) }
}

private fun applySgr(
    params: String,
    bold: Boolean,
    fg: AnsiColor?,
    bg: AnsiColor?,
): Triple<Boolean, AnsiColor?, AnsiColor?> {
    var curBold = bold
    var curFg = fg
    var curBg = bg

    val codes = if (params.isBlank()) listOf(0) else params.split(";").mapNotNull { it.toIntOrNull() }
    for (code in codes) {
        when (code) {
            0 -> {
                curBold = false
                curFg = null
                curBg = null
            }
            1 -> curBold = true
            22 -> curBold = false
            in 30..37 -> curFg = FG_STANDARD[code - 30]
            in 90..97 -> curFg = FG_BRIGHT[code - 90]
            in 40..47 -> curBg = BG_STANDARD[code - 40]
            39 -> curFg = null
            49 -> curBg = null
        }
    }
    return Triple(curBold, curFg, curBg)
}

/** Plain text with all ANSI sequences stripped (for list previews). */
fun stripAnsi(input: String): String = parseAnsi(input).joinToString("") { it.text }
