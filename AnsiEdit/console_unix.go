//go:build !windows

package main

func prepareConsole() error { return nil }

func restoreConsole() {}
