package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/virtbbs/virtbbs/pkg/ansiart"
	"github.com/virtbbs/virtbbs/pkg/transfer"
)

type App struct {
	term   *Terminal
	cfg    Config
	lib    *ansiart.Library
	player string
	last   *ansiart.Entry
}

func Run(term *Terminal, cfg Config, player string) {
	lib, err := ansiart.NewLibrary(cfg.LibraryDir)
	if err != nil {
		term.Printf("%s\r\n", color(cBrightRed, "Library error: "+err.Error()))
		return
	}
	app := &App{term: term, cfg: cfg, lib: lib, player: player}
	for {
		app.drawMenu()
		ch, err := term.ReadChoice()
		if err != nil || ch == "Q" {
			_ = lib.WriteBulletin(cfg.BulletinPath, "VirtBBS")
			term.Clear()
			term.Printf("%s\r\n", color(cBrightCyan, "Thanks for using AnsiArt "+Version))
			return
		}
		switch ch {
		case "1", "U":
			app.uploadZmodem()
		case "2", "I":
			app.convertFromInbox()
		case "3", "C":
			app.convertPath("")
		case "4", "G":
			app.gallery()
		case "5", "D":
			app.downloadLast()
		case "6", "P":
			app.previewLast()
		default:
			term.Print(color(cBrightRed, "Unknown choice.") + "\r\n")
			pause(term)
		}
	}
}

func (a *App) drawMenu() {
	t := a.term
	t.Clear()
	t.Print(color(cBrightCyan, "=[ ")+color(cBrightWhite, "AnsiArt "+Version)+color(cBrightCyan, " ]=")+"\r\n")
	t.Printf("%s\r\n\r\n", color(cBrightYellow, a.player))
	t.Print("  1. Upload image (Zmodem)\r\n")
	t.Print("  2. Convert from inbox/\r\n")
	t.Print("  3. Convert local path (-local)\r\n")
	t.Print("  4. Gallery / preview\r\n")
	t.Print("  5. Download last result (Zmodem)\r\n")
	t.Print("  6. Preview last result\r\n")
	t.Print("  Q. Quit\r\n\r\n")
	if a.last != nil {
		t.Printf("%s Last: %s (%s %dx%d)\r\n\r\n",
			color(cBrightBlack, ""), a.last.Meta.Title, a.last.Meta.Mode, a.last.Meta.Width, a.last.Meta.Height)
	}
	t.Print(color(cCyan, "Choice: "))
}

func (a *App) uploadZmodem() {
	a.term.Print("\r\nStart your Zmodem send now...\r\n")
	path, err := transfer.ReceiveFile(struct {
		io.Reader
		io.Writer
	}{a.term.in, a.term.out}, a.lib.InboxDir())
	if err != nil {
		a.term.Printf("%s\r\n", color(cBrightRed, "Upload failed: "+err.Error()))
		pause(a.term)
		return
	}
	a.term.Printf("%s\r\n", color(cBrightGreen, "Saved: "+filepath.Base(path)))
	a.convertPath(path)
}

func (a *App) convertFromInbox() {
	entries, _ := os.ReadDir(a.lib.InboxDir())
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp":
			files = append(files, filepath.Join(a.lib.InboxDir(), e.Name()))
		}
	}
	if len(files) == 0 {
		a.term.Print(color(cBrightYellow, "Inbox empty. Upload via Zmodem or /ansiart web.") + "\r\n")
		pause(a.term)
		return
	}
	a.term.Print("\r\nInbox files:\r\n")
	for i, f := range files {
		a.term.Printf("  %d. %s\r\n", i+1, filepath.Base(f))
	}
	a.term.Print("Number: ")
	ch, err := a.term.ReadChoice()
	if err != nil || ch == "Q" {
		return
	}
	var n int
	fmt.Sscanf(ch, "%d", &n)
	if n < 1 || n > len(files) {
		a.term.Print(color(cBrightRed, "Invalid.") + "\r\n")
		pause(a.term)
		return
	}
	a.convertPath(files[n-1])
}

func (a *App) convertPath(path string) {
	if path == "" {
		path = a.term.PromptLine("\r\nImage path: ")
		if path == "" {
			return
		}
	}
	a.term.Print("\r\nMode: 1=ANSI truecolor  2=ASCII\r\nChoice: ")
	ch, _ := a.term.ReadChoice()
	mode := ansiart.ModeANSI
	if ch == "2" {
		mode = ansiart.ModeASCII
	}
	a.term.Printf("Width [%d]: ", a.cfg.DefaultWidth)
	ws := a.term.PromptLine("")
	width := a.cfg.DefaultWidth
	if ws != "" {
		fmt.Sscanf(ws, "%d", &width)
	}
	title := a.term.PromptLine("Title (Enter=filename): ")
	if title == "" {
		title = filepath.Base(path)
	}
	a.term.Print(color(cBrightBlack, "Converting...") + "\r\n")
	art, w, h, err := ansiart.ConvertFile(path, ansiart.Options{
		Mode: mode, Width: width, Title: title, Author: a.player,
		Source: filepath.Base(path), Version: Version,
	})
	if err != nil {
		a.term.Printf("%s\r\n", color(cBrightRed, err.Error()))
		pause(a.term)
		return
	}
	ent, err := a.lib.SaveConversion(a.player, title, path, art, mode, w, h)
	if err != nil {
		a.term.Printf("%s\r\n", color(cBrightRed, "Save failed: "+err.Error()))
		pause(a.term)
		return
	}
	a.last = ent
	_ = a.lib.WriteBulletin(a.cfg.BulletinPath, "VirtBBS")
	a.term.Printf("%s %s %dx%d → %s\r\n", color(cBrightGreen, "OK"), mode, w, h, ent.ResultPath())
	a.previewLast()
}

func (a *App) gallery() {
	list, err := a.lib.ListRecent(20)
	if err != nil || len(list) == 0 {
		a.term.Print(color(cBrightYellow, "No conversions yet.") + "\r\n")
		pause(a.term)
		return
	}
	a.term.Clear()
	a.term.Print(color(cBrightCyan, "Gallery") + "\r\n\r\n")
	for i, e := range list {
		a.term.Printf("  %2d. %-16s %-18s %s %dx%d\r\n",
			i+1, e.Meta.User, e.Meta.Title, e.Meta.Mode, e.Meta.Width, e.Meta.Height)
	}
	a.term.Print("\r\nNumber to preview (Q=back): ")
	ch, _ := a.term.ReadChoice()
	if ch == "Q" {
		return
	}
	var n int
	fmt.Sscanf(ch, "%d", &n)
	if n < 1 || n > len(list) {
		return
	}
	a.last = &list[n-1]
	a.previewLast()
}

func (a *App) previewLast() {
	if a.last == nil {
		a.term.Print("Nothing to preview.\r\n")
		pause(a.term)
		return
	}
	data, err := os.ReadFile(a.last.ResultPath())
	if err != nil {
		a.term.Printf("%s\r\n", color(cBrightRed, err.Error()))
		pause(a.term)
		return
	}
	info, _ := ansiart.ReadSAUCE(data)
	body := ansiart.StripSAUCEForDisplay(data)
	a.term.Clear()
	a.term.Printf("%s by %s [%s %dx%d]\r\n\r\n",
		color(cBrightYellow, info.Title), info.Author, a.last.Meta.Mode, info.Width, info.Height)
	a.term.Write(body)
	a.term.Print("\r\n")
	pause(a.term)
}

func (a *App) downloadLast() {
	if a.last == nil {
		a.term.Print("Nothing to download.\r\n")
		pause(a.term)
		return
	}
	a.term.Print("\r\nStart your Zmodem receive now...\r\n")
	rw := struct {
		io.Reader
		io.Writer
	}{a.term.in, a.term.out}
	if err := transfer.SendFile(rw, a.last.ResultPath()); err != nil {
		a.term.Printf("%s\r\n", color(cBrightRed, "Download failed: "+err.Error()))
	} else {
		a.term.Print(color(cBrightGreen, "Sent.") + "\r\n")
	}
	pause(a.term)
}

func pause(t *Terminal) {
	t.Print(color(cBrightBlack, "Press any key...") + "\r\n")
	_, _ = t.ReadKey()
}
