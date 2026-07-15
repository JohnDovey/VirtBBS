package web

import (
	"os"
	"strings"
	"testing"
)

func TestAnsiToHTML_BinkpDay(t *testing.T) {
	raw, err := os.ReadFile("../../display/BINKPDAY.ANS")
	if err != nil {
		t.Skip(err)
	}
	html := ansiToHTML(string(raw))
	if strings.Contains(html, "\x1b") {
		t.Error("unconverted escape sequences remain")
	}
	if !strings.Contains(html, `class="ansi-bold ansi-fg-bright-cyan"`) {
		t.Error("expected bold bright-cyan header styling")
	}
	if !strings.Contains(html, "Outbound polls (OK/fail)    ") {
		t.Error("expected padded stat label whitespace preserved in HTML")
	}
}

func TestAnsiToHTML_accumulatesSGR(t *testing.T) {
	html := ansiToHTML("\x1b[1m\x1b[96mbold cyan\x1b[0m plain")
	if !strings.Contains(html, `<span class="ansi-bold ansi-fg-bright-cyan">bold cyan</span>`) {
		t.Fatalf("SGR state not accumulated: %q", html)
	}
	if !strings.Contains(html, " plain") {
		t.Fatalf("reset not applied: %q", html)
	}
}

func TestAnsiToHTML_truecolor(t *testing.T) {
	html := ansiToHTML("\x1b[38;2;255;128;0m\x1b[48;2;10;20;30morange\x1b[0m")
	if !strings.Contains(html, `style="color:rgb(255,128,0);background-color:rgb(10,20,30)"`) {
		t.Fatalf("truecolor style missing: %q", html)
	}
	if !strings.Contains(html, "orange") {
		t.Fatalf("text missing: %q", html)
	}
}
