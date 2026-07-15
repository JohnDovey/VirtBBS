package main

// Version is the AnsiEdit semver. Bump patch on every change;
// minor for significant features; major only when explicitly requested.
const Version = "1.0.9"

// Copyright holder and contact (embedded in -version and About).
const (
	CopyrightHolder = "John Dovey <dovey.john@gmail.com>"
	CopyrightYear   = "2026"
	CopyrightNotice = "Copyright (c) " + CopyrightYear + " " + CopyrightHolder
	LicenseName     = "MIT License"
	GitHubRepoURL   = "https://github.com/JohnDovey/VirtBBS"
	AboutBlurb      = "AnsiEdit is a standalone fullscreen ANSI art editor for classic BBS " +
		".ANS/.ASC screens. Create and edit art with CP437/truecolor, import images " +
		"(HBFS/ASCII), insert styled text with fonts and drop shadows, and edit " +
		"ACiD SAUCE + COMNT metadata. It ships with VirtBBS but runs as its own " +
		"console program — not a BBS door."
)
