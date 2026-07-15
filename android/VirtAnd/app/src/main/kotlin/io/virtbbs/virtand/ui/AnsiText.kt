// VirtAnd — AnsiText.kt
package io.virtbbs.virtand.ui

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
import io.virtbbs.virtand.core.AnsiColor
import io.virtbbs.virtand.core.AnsiSpan
import io.virtbbs.virtand.core.parseAnsi

private fun AnsiColor.toComposeColor(): Color = Color(red, green, blue)

fun ansiToAnnotatedString(input: String): AnnotatedString = buildAnnotatedString {
    for (span in parseAnsi(input)) {
        val style = SpanStyle(
            color = span.foreground?.toComposeColor() ?: Color.Unspecified,
            background = span.background?.toComposeColor() ?: Color.Unspecified,
            fontWeight = if (span.bold) FontWeight.Bold else FontWeight.Normal,
            fontFamily = FontFamily.Monospace,
        )
        withStyle(style) { append(span.text) }
    }
}

@Composable
fun AnsiText(text: String, modifier: Modifier = Modifier) {
    Text(
        text = ansiToAnnotatedString(text),
        modifier = modifier,
        fontFamily = FontFamily.Monospace,
    )
}
