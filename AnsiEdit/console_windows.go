//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

var (
	savedOutMode uint32
	savedInMode  uint32
	hadOutMode   bool
	hadInMode    bool
)

// prepareConsole enables Windows virtual-terminal processing so ANSI SGR/CSI
// sequences and UTF-8 box-drawing render in PowerShell and classic conhost.
func prepareConsole() error {
	out := windows.Handle(os.Stdout.Fd())
	in := windows.Handle(os.Stdin.Fd())

	var mode uint32
	if err := windows.GetConsoleMode(out, &mode); err != nil {
		return nil // not attached to a console
	}
	savedOutMode = mode
	hadOutMode = true
	if err := windows.SetConsoleMode(out, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return err
	}

	if err := windows.GetConsoleMode(in, &mode); err == nil {
		savedInMode = mode
		hadInMode = true
		_ = windows.SetConsoleMode(in, mode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	}

	_ = windows.SetConsoleOutputCP(65001)
	_ = windows.SetConsoleCP(65001)
	return nil
}

func restoreConsole() {
	if hadOutMode {
		out := windows.Handle(os.Stdout.Fd())
		_ = windows.SetConsoleMode(out, savedOutMode)
	}
	if hadInMode {
		in := windows.Handle(os.Stdin.Fd())
		_ = windows.SetConsoleMode(in, savedInMode)
	}
}
