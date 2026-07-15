package session

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/virtbbs/virtbbs/internal/ansi"
	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/mrc"
	"github.com/virtbbs/virtbbs/internal/node"
)

const (
	mrcRows      = 24
	mrcCols      = 80
	mrcStatusRow = 1
	mrcChatTop   = 2
	mrcChatBot   = 23
	mrcInputRow = 24
	mrcScrollCap = 200
)

// mrcChat is the built-in Multi-Relay Chat client (not a door).
func (s *session) mrcChat() {
	hub := s.deps.MRC
	if hub == nil || !hub.Enabled() {
		s.writeln(ansi.Colorize(ansi.Yellow, "MRC is not enabled. Ask the sysop to configure Admin → MRC."))
		return
	}
	if s.user.SecurityLevel < hub.MinSecurity() {
		s.writeln(ansi.Colorize(ansi.Red, "Insufficient security level for MRC."))
		return
	}
	if !hub.Connected() {
		s.writeln(ansi.Colorize(ansi.Yellow, "MRC is offline / reconnecting. Try again shortly, or ESC to cancel."))
		s.write(ansi.Prompt("Press Enter to retry, or Q to cancel: "))
		ans := strings.ToUpper(strings.TrimSpace(s.readline()))
		if ans == "Q" || ans == "N" {
			return
		}
		if !hub.Connected() {
			s.writeln(ansi.Colorize(ansi.Red, "Still offline."))
			return
		}
	}

	var prefs *mrc.UserPrefs
	handle := mrc.SanitizeName(s.user.Name)
	if hub.Prefs() != nil {
		var err error
		prefs, err = hub.Prefs().Get(s.user.ID, s.user.Name)
		if err == nil && prefs != nil && prefs.Handle != "" {
			handle = prefs.Handle
		}
	}
	if prefs == nil {
		prefs = &mrc.UserPrefs{UserID: s.user.ID, Handle: handle, HandleColor: 11, TextColor: 7, Theme: 1}
	}

	s.write(ansi.Prompt(fmt.Sprintf("MRC handle [%s]: ", handle)))
	if line := strings.TrimSpace(s.readline()); line != "" {
		handle = mrc.SanitizeName(line)
		prefs.Handle = handle
		if hub.Prefs() != nil {
			_ = hub.Prefs().Save(prefs)
		}
	}

	room := hub.DefaultRoom()
	att, err := hub.Attach(s.user.ID, handle, room)
	if err != nil {
		s.writeln(ansi.Colorize(ansi.Red, "Could not join MRC: "+err.Error()))
		return
	}
	defer hub.Detach(att)

	_ = s.deps.Nodes.Update(s.nodeID, node.StatusChat, "MRC: "+room, s.user.ID, s.user.Name, s.user.City)
	s.talkMuted = true
	defer func() {
		s.talkMuted = false
		_ = s.deps.Nodes.Update(s.nodeID, node.StatusMain, "Main Menu", s.user.ID, s.user.Name, s.user.City)
		s.writeln("")
		s.writeln(ansi.Colorize(ansi.Cyan, "Left MRC — back to main menu."))
	}()

	ui := &mrcUI{
		s:     s,
		hub:   hub,
		att:   att,
		prefs: prefs,
		lines: make([]string, 0, 64),
	}
	ui.run()
}

type mrcUI struct {
	s          *session
	hub        *mrc.Hub
	att        *mrc.Attachment
	prefs      *mrc.UserPrefs
	lines      []string
	input      []rune
	mentions   int
	scrollBack int // 0 = live; >0 scrolled up
	quit       bool
	tabIdx     int
	tabBase    string
}

func (ui *mrcUI) run() {
	kr := newMRCKeyReader(ui.s.rw)
	defer kr.stop()

	ui.redraw()
	ui.pushLocal("|15Welcome to MRC|07 — type |11/help|07 for commands, |11ESC|07 or |11/quit|07 to leave.")

	for !ui.quit {
		select {
		case ev, ok := <-ui.att.Inbox:
			if !ok {
				ui.pushLocal("|12Disconnected from MRC hub.")
				ui.quit = true
				continue
			}
			ui.onEvent(ev)
		case b, ok := <-kr.buf:
			if !ok {
				ui.quit = true
				continue
			}
			ui.onByte(kr, b)
		case <-ui.s.ctrl.Done():
			ui.quit = true
		}
	}
}

func (ui *mrcUI) onEvent(ev mrc.Event) {
	if !ev.Local && ui.prefs != nil && ui.prefs.Ignores(ev.From) {
		return
	}
	ts := time.Now().Format("15:04")
	var line string
	switch ev.Kind {
	case mrc.EventPrivate:
		line = fmt.Sprintf("%s |13*%s@%s*|07 %s", ts, ev.From, ev.Site, ev.PipeRaw)
	case mrc.EventTopic:
		ui.att.SetTopic(ev.Body)
		line = fmt.Sprintf("%s |14* topic *|07 %s", ts, ev.Body)
	case mrc.EventUserList:
		line = fmt.Sprintf("%s |08[who] %s|07", ts, ev.Body)
	case mrc.EventSystem, mrc.EventNotice:
		line = fmt.Sprintf("%s |08%s|07", ts, ev.PipeRaw)
		if ev.PipeRaw == "" {
			line = fmt.Sprintf("%s |08%s|07", ts, ev.Body)
		}
	default:
		body := ev.PipeRaw
		if body == "" {
			body = ev.Body
		}
		site := ev.Site
		if site != "" {
			site = "@" + site
		}
		line = fmt.Sprintf("%s |11<%s%s>|07 %s", ts, ev.From, site, body)
		if ui.mentioned(body) {
			ui.mentions++
		}
	}
	ui.pushLine(line)
}

func (ui *mrcUI) mentioned(body string) bool {
	h := strings.ToLower(ui.att.Handle)
	plain := strings.ToLower(mrc.StripPipe(body))
	if h == "" {
		return false
	}
	// word-boundary-ish
	for _, tok := range strings.FieldsFunc(plain, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}) {
		if tok == h {
			return true
		}
	}
	return strings.Contains(plain, "@"+h)
}

func (ui *mrcUI) pushLocal(msg string) {
	ui.pushLine(time.Now().Format("15:04") + " " + msg)
}

func (ui *mrcUI) pushLine(pipeLine string) {
	ui.lines = append(ui.lines, pipeLine)
	if len(ui.lines) > mrcScrollCap {
		ui.lines = ui.lines[len(ui.lines)-mrcScrollCap:]
	}
	if ui.scrollBack == 0 {
		ui.paintChat()
		ui.paintStatus()
		ui.paintInput()
	}
}

func (ui *mrcUI) redraw() {
	ui.s.write(ansi.ClearScreen())
	ui.paintStatus()
	ui.paintChat()
	ui.paintInput()
}

func (ui *mrcUI) paintStatus() {
	room := ui.att.CurrentRoom()
	topic := ui.att.Topic()
	if topic == "" {
		topic = "-"
	}
	men := ""
	if ui.mentions > 0 {
		men = fmt.Sprintf(" [@%d]", ui.mentions)
	}
	st := ui.hub.Status()
	lat := ""
	if !st.Connected {
		lat = " OFFLINE"
	}
	left := fmt.Sprintf(" MRC #%s | %s%s", room, truncateVis(topic, 40), men)
	right := fmt.Sprintf("%s%s ", ui.att.Handle, lat)
	bar := padStatus(left, right)
	ui.s.write(ansi.MoveTo(mrcStatusRow, 1) + ansi.Color(ansi.BrightCyan) + bar + ansi.Reset())
}

func (ui *mrcUI) paintChat() {
	height := mrcChatBot - mrcChatTop + 1
	end := len(ui.lines) - ui.scrollBack
	if end < 0 {
		end = 0
	}
	start := end - height
	if start < 0 {
		start = 0
	}
	row := mrcChatTop
	for i := start; i < end && row <= mrcChatBot; i++ {
		vis := truncateVis(mrc.PipeToANSI(ui.lines[i]), mrcCols)
		ui.s.write(ansi.MoveTo(row, 1) + ansi.Color(ansi.White) + padRightVis(vis, mrcCols) + ansi.Reset())
		row++
	}
	for row <= mrcChatBot {
		ui.s.write(ansi.MoveTo(row, 1) + strings.Repeat(" ", mrcCols))
		row++
	}
}

func (ui *mrcUI) paintInput() {
	prompt := "> "
	body := string(ui.input)
	max := mrcCols - len(prompt) - 1
	if len(body) > max {
		body = body[len(body)-max:]
	}
	line := ansi.Color(ansi.BrightGreen) + prompt + ansi.Color(ansi.BrightWhite) + body + ansi.Reset() + strings.Repeat(" ", max-len([]rune(body))+1)
	ui.s.write(ansi.MoveTo(mrcInputRow, 1) + padRightVis(line, mrcCols))
}

func (ui *mrcUI) onByte(kr *mrcKeyReader, b byte) {
	if ui.s.idleTimer != nil {
		ui.s.idleTimer.Reset(time.Duration(config.Get().Session.IdleTimeoutMins) * time.Minute)
	}
	switch {
	case b == 0x1B: // ESC or CSI
		b2, ok := kr.readByte(50 * time.Millisecond)
		if !ok {
			ui.quit = true
			return
		}
		if b2 == '[' {
			ui.handleCSI(kr)
			return
		}
		ui.quit = true
	case b == '\r' || b == '\n':
		ui.submit()
	case b == 0x08 || b == 0x7F:
		if len(ui.input) > 0 {
			ui.input = ui.input[:len(ui.input)-1]
			ui.tabBase = ""
			ui.paintInput()
		}
	case b == '\t':
		ui.tabComplete()
	case b >= 0x20:
		ui.input = append(ui.input, rune(b))
		ui.tabBase = ""
		ui.paintInput()
	}
}

func (ui *mrcUI) handleCSI(kr *mrcKeyReader) {
	var params []byte
	for {
		b, ok := kr.readByte(50 * time.Millisecond)
		if !ok {
			return
		}
		if b >= 0x40 && b <= 0x7E {
			switch b {
			case 'A': // up — scrollback
				ui.scrollBack++
				if ui.scrollBack > len(ui.lines) {
					ui.scrollBack = len(ui.lines)
				}
				ui.paintChat()
			case 'B': // down
				if ui.scrollBack > 0 {
					ui.scrollBack--
					ui.paintChat()
				}
			case 'H':
				ui.scrollBack = len(ui.lines)
				ui.paintChat()
			case 'F':
				ui.scrollBack = 0
				ui.paintChat()
			}
			return
		}
		params = append(params, b)
		_ = params
	}
}

func (ui *mrcUI) tabComplete() {
	line := string(ui.input)
	i := strings.LastIndexAny(line, " ")
	prefix := line
	head := ""
	if i >= 0 {
		head = line[:i+1]
		prefix = line[i+1:]
	}
	if ui.tabBase == "" {
		ui.tabBase = prefix
		ui.tabIdx = 0
	}
	nick, next := ui.att.CompleteNick(ui.tabBase, ui.tabIdx)
	if nick == "" {
		return
	}
	ui.tabIdx = next
	ui.input = []rune(head + nick)
	ui.paintInput()
}

func (ui *mrcUI) submit() {
	text := strings.TrimSpace(string(ui.input))
	ui.input = nil
	ui.tabBase = ""
	ui.scrollBack = 0
	ui.paintInput()
	if text == "" {
		return
	}
	if strings.HasPrefix(text, "/") {
		ui.slash(text)
		return
	}
	if err := ui.hub.SendChat(ui.att, text); err != nil {
		ui.pushLocal("|12send error: " + err.Error())
		return
	}
	// Echo locally (some servers do not bounce your own line)
	ui.pushLine(fmt.Sprintf("%s |14<%s>|07 %s", time.Now().Format("15:04"), ui.att.Handle, mrc.StripTildesForBody(text)))
}

func (ui *mrcUI) slash(text string) {
	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}
	switch cmd {
	case "quit", "q", "exit":
		ui.quit = true
	case "help", "?":
		ui.pushLocal("|15/join /list /chatters /whoon /msg /me /topic /motd /bbses /info /mentions /quit")
	case "join", "j":
		if arg == "" {
			ui.pushLocal("|12Usage: /join <room>")
			return
		}
		if err := ui.hub.JoinRoom(ui.att, arg); err != nil {
			ui.pushLocal("|12" + err.Error())
			return
		}
		ui.pushLocal("|10Joined " + ui.att.CurrentRoom())
		ui.paintStatus()
		_ = ui.hub.SendServerCmd(ui.att, "USERLIST")
		_ = sUpdateNodeMRC(ui)
	case "list", "rooms":
		_ = ui.hub.SendServerCmd(ui.att, "LIST")
	case "chatters", "who":
		_ = ui.hub.SendServerCmd(ui.att, "CHATTERS")
		_ = ui.hub.SendServerCmd(ui.att, "USERLIST")
	case "whoon":
		_ = ui.hub.SendServerCmd(ui.att, "WHOON")
	case "motd":
		_ = ui.hub.SendServerCmd(ui.att, "MOTD")
	case "bbses":
		_ = ui.hub.SendServerCmd(ui.att, "BBSES")
	case "info":
		if arg == "" {
			_ = ui.hub.SendServerCmd(ui.att, "INFO")
		} else {
			_ = ui.hub.SendServerCmd(ui.att, "INFO "+arg)
		}
	case "topic":
		if arg == "" {
			ui.pushLocal("|14Topic: " + ui.att.Topic())
			return
		}
		_ = ui.hub.SetTopic(ui.att, arg)
	case "msg", "t", "tell", "pm":
		sp := strings.SplitN(arg, " ", 2)
		if len(sp) < 2 {
			ui.pushLocal("|12Usage: /msg <user> <text>")
			return
		}
		if err := ui.hub.SendPrivate(ui.att, sp[0], sp[1]); err != nil {
			ui.pushLocal("|12" + err.Error())
			return
		}
		ui.pushLocal(fmt.Sprintf("|13-> %s|07 %s", sp[0], sp[1]))
	case "me":
		if arg == "" {
			return
		}
		_ = ui.hub.SendAction(ui.att, arg)
		ui.pushLine(fmt.Sprintf("%s |13* %s %s|07", time.Now().Format("15:04"), ui.att.Handle, arg))
	case "mentions":
		ui.pushLocal(fmt.Sprintf("|15Mentions: %d|07 (counter reset)", ui.mentions))
		ui.mentions = 0
		ui.paintStatus()
	case "clear":
		ui.lines = nil
		ui.redraw()
	default:
		// Pass through unknown slash as server helper (!cmd style uses ! not /)
		_ = ui.hub.SendServerCmd(ui.att, strings.TrimPrefix(text, "/"))
	}
}

func sUpdateNodeMRC(ui *mrcUI) error {
	return ui.s.deps.Nodes.Update(ui.s.nodeID, node.StatusChat, "MRC: "+ui.att.CurrentRoom(), ui.s.user.ID, ui.s.user.Name, ui.s.user.City)
}

// ── key reader (mirrors editor pattern; local to session) ────────────────────

type mrcKeyReader struct {
	src  io.Reader
	buf  chan byte
	done chan struct{}
}

func newMRCKeyReader(r io.Reader) *mrcKeyReader {
	kr := &mrcKeyReader{src: r, buf: make(chan byte, 128), done: make(chan struct{})}
	go kr.feed()
	return kr
}

func (kr *mrcKeyReader) stop() {
	select {
	case <-kr.done:
	default:
		close(kr.done)
	}
}

func (kr *mrcKeyReader) feed() {
	b := make([]byte, 1)
	for {
		_, err := kr.src.Read(b)
		if err != nil {
			close(kr.buf)
			return
		}
		select {
		case kr.buf <- b[0]:
		case <-kr.done:
			return
		}
	}
}

func (kr *mrcKeyReader) readByte(timeout time.Duration) (byte, bool) {
	select {
	case b, ok := <-kr.buf:
		return b, ok
	case <-time.After(timeout):
		return 0, false
	case <-kr.done:
		return 0, false
	}
}

func truncateVis(s string, width int) string {
	return ansi.FitVisibleWidth(s, width)
}

func padRightVis(s string, width int) string {
	return ansi.FitVisibleWidth(s, width)
}

func padStatus(left, right string) string {
	lw := ansi.VisibleWidth(left)
	rw := ansi.VisibleWidth(right)
	gap := mrcCols - lw - rw
	if gap < 1 {
		left = ansi.FitVisibleWidth(left, mrcCols-rw-1)
		lw = ansi.VisibleWidth(left)
		gap = mrcCols - lw - rw
		if gap < 1 {
			gap = 1
		}
	}
	return left + strings.Repeat(" ", gap) + right
}
