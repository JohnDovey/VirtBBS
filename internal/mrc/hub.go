package mrc

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxMsgLen         = 140
	iamhereInterval   = 50 * time.Second
	reconnectMin      = 5 * time.Second
	reconnectMax      = 60 * time.Second
	dialTimeout       = 15 * time.Second
	writeIdleTimeout  = 30 * time.Second
)

// Hub maintains one outbound MRC relay connection for the whole BBS.
type Hub struct {
	prefs *PrefsStore

	mu       sync.Mutex
	cfg      Resolved
	conn     net.Conn
	attached map[string]*Attachment

	running    bool
	enabled    bool
	connected  bool
	reconnect  bool
	lastErr    string
	lastConn   time.Time
	stopCh     chan struct{}
	applyCh    chan Resolved
	started    sync.Once
	platform   string
	attachSeq  atomic.Uint64
}

// NewHub creates a hub that can be started and reconfigured live.
func NewHub(prefs *PrefsStore, platform string) *Hub {
	if platform == "" {
		platform = "VirtBBS"
	}
	return &Hub{
		prefs:    prefs,
		attached: make(map[string]*Attachment),
		stopCh:   make(chan struct{}),
		applyCh:  make(chan Resolved, 1),
		platform: platform,
	}
}

// Prefs returns the prefs store (may be nil in tests).
func (h *Hub) Prefs() *PrefsStore { return h.prefs }

// Start begins the hub supervisor goroutine (idempotent).
func (h *Hub) Start() {
	h.started.Do(func() {
		h.mu.Lock()
		h.running = true
		h.mu.Unlock()
		go h.loop()
	})
}

// Stop shuts down the hub.
func (h *Hub) Stop() {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return
	}
	h.running = false
	h.mu.Unlock()
	select {
	case <-h.stopCh:
	default:
		close(h.stopCh)
	}
	h.dropConn()
}

// ApplyConfig updates settings and reconnects as needed. Safe from admin handlers.
func (h *Hub) ApplyConfig(r Resolved) {
	h.Start()
	select {
	case h.applyCh <- r:
	default:
		// Replace pending config
		select {
		case <-h.applyCh:
		default:
		}
		h.applyCh <- r
	}
}

// Status returns a snapshot for admin UI.
func (h *Hub) Status() Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	st := Status{
		Enabled:      h.enabled,
		Connected:    h.connected,
		Reconnecting: h.reconnect && h.enabled,
		Addr:         h.cfg.Addr(),
		BBSName:      h.cfg.BBSName,
		Attached:     len(h.attached),
		LastError:    h.lastErr,
		LastConnect:  h.lastConn,
		UseTLS:       h.cfg.UseTLS,
	}
	return st
}

// Enabled reports whether MRC is configured on.
func (h *Hub) Enabled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.enabled
}

// Connected reports live TCP session.
func (h *Hub) Connected() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.connected
}

// Config returns the current resolved config.
func (h *Hub) Config() Resolved {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cfg
}

// MinSecurity returns the security gate.
func (h *Hub) MinSecurity() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cfg.MinSecurity <= 0 {
		return 10
	}
	return h.cfg.MinSecurity
}

// DefaultRoom returns the join room.
func (h *Hub) DefaultRoom() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cfg.DefaultRoom == "" {
		return "lobby"
	}
	return h.cfg.DefaultRoom
}

func (h *Hub) loop() {
	var backoff time.Duration
	for {
		select {
		case <-h.stopCh:
			h.detachAll()
			h.dropConn()
			return
		case r := <-h.applyCh:
			h.applyLocked(r)
			backoff = 0
		default:
		}

		h.mu.Lock()
		en := h.enabled
		h.mu.Unlock()
		if !en {
			select {
			case <-h.stopCh:
				h.detachAll()
				h.dropConn()
				return
			case r := <-h.applyCh:
				h.applyLocked(r)
				backoff = 0
			}
			continue
		}

		err := h.session()
		if err != nil && err != io.EOF {
			h.mu.Lock()
			h.lastErr = err.Error()
			h.connected = false
			h.reconnect = true
			h.mu.Unlock()
			log.Printf("mrc: session ended: %v", err)
		} else {
			h.mu.Lock()
			h.connected = false
			h.mu.Unlock()
		}

		h.mu.Lock()
		en = h.enabled
		h.mu.Unlock()
		if !en {
			continue
		}

		if backoff == 0 {
			backoff = reconnectMin
		} else {
			backoff *= 2
			if backoff > reconnectMax {
				backoff = reconnectMax
			}
		}
		select {
		case <-h.stopCh:
			h.detachAll()
			h.dropConn()
			return
		case r := <-h.applyCh:
			h.applyLocked(r)
			backoff = 0
		case <-time.After(backoff):
		}
	}
}

func (h *Hub) applyLocked(r Resolved) {
	h.mu.Lock()
	was := h.enabled
	h.cfg = r
	h.enabled = r.Enabled
	h.mu.Unlock()
	if was && !r.Enabled {
		h.detachAll()
		h.dropConn()
		return
	}
	if r.Enabled {
		// Force redial with new identity/host
		h.dropConn()
	}
}

func (h *Hub) session() error {
	h.mu.Lock()
	cfg := h.cfg
	h.reconnect = true
	h.mu.Unlock()

	conn, err := h.dial(cfg)
	if err != nil {
		return err
	}

	h.mu.Lock()
	h.conn = conn
	h.connected = true
	h.reconnect = false
	h.lastConn = time.Now()
	h.lastErr = ""
	h.mu.Unlock()
	defer h.dropConn()

	if _, err := io.WriteString(conn, cfg.HandshakeString()+"\n"); err != nil {
		return fmt.Errorf("handshake: %w", err)
	}
	h.sendMeta(cfg)

	// Re-announce attached users after reconnect
	h.mu.Lock()
	atts := make([]*Attachment, 0, len(h.attached))
	for _, a := range h.attached {
		atts = append(atts, a)
	}
	h.mu.Unlock()
	for _, a := range atts {
		room := a.CurrentRoom()
		_ = h.writePacket(Packet{FromUser: a.Handle, FromSite: cfg.BBSName, FromRoom: room, ToUser: "SERVER", ToRoom: room, Body: "IAMHERE"})
		_ = h.writePacket(Packet{FromUser: a.Handle, FromSite: cfg.BBSName, FromRoom: "", ToUser: "SERVER", ToRoom: room, Body: "NEWROOM::" + room})
		a.Deliver(Event{Kind: EventNotice, Body: "Reconnected to MRC network.", Local: true})
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.readLoop(conn)
	}()

	ticker := time.NewTicker(iamhereInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			h.sendShutdown(cfg)
			return io.EOF
		case r := <-h.applyCh:
			h.applyLocked(r)
			h.mu.Lock()
			en := h.enabled
			h.mu.Unlock()
			if !en {
				h.sendShutdown(cfg)
				return nil
			}
			// Host/identity changed — end session to redial
			return fmt.Errorf("config changed")
		case err := <-errCh:
			return err
		case <-ticker.C:
			h.heartbeat()
		}
	}
}

func (h *Hub) dial(cfg Resolved) (net.Conn, error) {
	addr := cfg.Addr()
	d := net.Dialer{Timeout: dialTimeout}
	if cfg.UseTLS {
		return tls.DialWithDialer(&d, "tcp", addr, &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // MRC relays often use atypical certs
			MinVersion:         tls.VersionTLS12,
		})
	}
	return d.Dial("tcp", addr)
}

func (h *Hub) sendMeta(cfg Resolved) {
	// CAPABILITIES / BBSMETA / INFO* — best-effort, servers vary
	_ = h.writePacket(Packet{FromUser: "CLIENT", FromSite: cfg.BBSName, ToUser: "SERVER", Body: "CAPABILITIES"})
	meta := fmt.Sprintf("BBSMETA:NAME=%s:SYSOP=%s", SanitizeField(cfg.BBSPretty), SanitizeField(cfg.Sysop))
	_ = h.writePacket(Packet{FromUser: cfg.BBSName, FromSite: cfg.BBSName, ToUser: "SERVER", Body: meta})
	if cfg.Description != "" {
		_ = h.writePacket(Packet{FromUser: cfg.BBSName, FromSite: cfg.BBSName, ToUser: "SERVER", Body: "INFONODE:" + SanitizeField(cfg.Description)})
	}
	if cfg.Telnet != "" {
		_ = h.writePacket(Packet{FromUser: cfg.BBSName, FromSite: cfg.BBSName, ToUser: "SERVER", Body: "INFOTEL:" + SanitizeField(cfg.Telnet)})
	}
	if cfg.SSH != "" {
		_ = h.writePacket(Packet{FromUser: cfg.BBSName, FromSite: cfg.BBSName, ToUser: "SERVER", Body: "INFOSSH:" + SanitizeField(cfg.SSH)})
	}
	if cfg.Website != "" {
		_ = h.writePacket(Packet{FromUser: cfg.BBSName, FromSite: cfg.BBSName, ToUser: "SERVER", Body: "INFOWEB:" + SanitizeField(cfg.Website)})
	}
}

func (h *Hub) sendShutdown(cfg Resolved) {
	_ = h.writePacket(Packet{FromUser: "CLIENT", FromSite: cfg.BBSName, ToUser: "SERVER", MsgExt: "ALL", Body: "SHUTDOWN"})
	time.Sleep(100 * time.Millisecond)
}

func (h *Hub) heartbeat() {
	h.mu.Lock()
	cfg := h.cfg
	atts := make([]*Attachment, 0, len(h.attached))
	for _, a := range h.attached {
		atts = append(atts, a)
	}
	h.mu.Unlock()

	pid := fmt.Sprintf("%d", os.Getpid())
	_ = h.writePacket(Packet{FromUser: "CLIENT", FromSite: cfg.BBSName, FromRoom: pid, ToUser: "SERVER", Body: "IMALIVE:" + cfg.BBSName})
	for _, a := range atts {
		room := a.CurrentRoom()
		_ = h.writePacket(Packet{FromUser: a.Handle, FromSite: cfg.BBSName, FromRoom: room, ToUser: "SERVER", ToRoom: room, Body: "IAMHERE"})
	}
}

func (h *Hub) readLoop(conn net.Conn) error {
	r := bufio.NewReader(conn)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Minute))
		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		pkt, ok := ParsePacket(line)
		if !ok {
			continue
		}
		h.handlePacket(pkt)
	}
}

func (h *Hub) handlePacket(pkt Packet) {
	bodyUp := strings.ToUpper(strings.TrimSpace(pkt.Body))

	if pkt.IsServerPing() || bodyUp == "PING" || strings.HasPrefix(bodyUp, "PING") {
		h.mu.Lock()
		cfg := h.cfg
		h.mu.Unlock()
		pid := fmt.Sprintf("%d", os.Getpid())
		_ = h.writePacket(Packet{FromUser: "CLIENT", FromSite: cfg.BBSName, FromRoom: pid, ToUser: "SERVER", Body: "IMALIVE:" + cfg.BBSName})
		return
	}

	if strings.HasPrefix(bodyUp, "USERLIST:") {
		names := strings.Split(pkt.Body[len("USERLIST:"):], ",")
		h.mu.Lock()
		for _, a := range h.attached {
			if roomMatch(a.CurrentRoom(), pkt.FromRoom) || roomMatch(a.CurrentRoom(), pkt.ToRoom) || pkt.ToRoom == "" {
				a.SetNicks(names)
				a.Deliver(Event{Kind: EventUserList, Body: strings.Join(names, ", "), Raw: pkt})
			}
		}
		h.mu.Unlock()
		return
	}

	if strings.HasPrefix(bodyUp, "TOPIC:") {
		topic := pkt.Body
		if i := strings.Index(topic, ":"); i >= 0 {
			topic = topic[i+1:]
		}
		h.fanoutRoom(pkt, Event{Kind: EventTopic, From: pkt.FromUser, Site: pkt.FromSite, Room: pkt.ToRoom, Body: topic, PipeRaw: pkt.Body, Raw: pkt})
		return
	}

	// Private message: ToUser is a local handle
	if pkt.ToUser != "" && !strings.EqualFold(pkt.ToUser, "SERVER") && !strings.EqualFold(pkt.ToUser, "CLIENT") && !strings.EqualFold(pkt.ToUser, "ALL") {
		h.mu.Lock()
		for _, a := range h.attached {
			if strings.EqualFold(a.Handle, pkt.ToUser) {
				a.NoteNick(pkt.FromUser)
				a.Deliver(Event{Kind: EventPrivate, From: pkt.FromUser, Site: pkt.FromSite, Room: pkt.FromRoom, Body: pkt.Body, PipeRaw: pkt.Body, Raw: pkt})
			}
		}
		h.mu.Unlock()
		return
	}

	if pkt.IsChatMessage() {
		h.fanoutRoom(pkt, Event{Kind: EventChat, From: pkt.FromUser, Site: pkt.FromSite, Room: firstNonEmpty(pkt.ToRoom, pkt.FromRoom), Body: pkt.Body, PipeRaw: pkt.Body, Raw: pkt})
		return
	}

	// System / verb responses → notice to all or room
	h.fanoutRoom(pkt, Event{Kind: EventSystem, From: pkt.FromUser, Site: pkt.FromSite, Room: firstNonEmpty(pkt.ToRoom, pkt.FromRoom), Body: pkt.Body, PipeRaw: pkt.Body, Raw: pkt})
}

func (h *Hub) fanoutRoom(pkt Packet, ev Event) {
	room := firstNonEmpty(pkt.ToRoom, pkt.FromRoom, ev.Room)
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, a := range h.attached {
		if room == "" || roomMatch(a.CurrentRoom(), room) || strings.EqualFold(pkt.ToUser, "ALL") {
			if ev.From != "" {
				a.NoteNick(ev.From)
			}
			a.Deliver(ev)
		}
	}
}

func roomMatch(a, b string) bool {
	return strings.EqualFold(SanitizeName(a), SanitizeName(b))
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func (h *Hub) writePacket(p Packet) error {
	h.mu.Lock()
	conn := h.conn
	h.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeIdleTimeout))
	_, err := io.WriteString(conn, p.Encode())
	return err
}

func (h *Hub) dropConn() {
	h.mu.Lock()
	conn := h.conn
	h.conn = nil
	h.connected = false
	h.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

func (h *Hub) detachAll() {
	h.mu.Lock()
	atts := make([]*Attachment, 0, len(h.attached))
	for id, a := range h.attached {
		atts = append(atts, a)
		delete(h.attached, id)
	}
	cfg := h.cfg
	h.mu.Unlock()
	for _, a := range atts {
		room := a.CurrentRoom()
		_ = h.writePacket(Packet{FromUser: a.Handle, FromSite: cfg.BBSName, FromRoom: room, ToUser: "SERVER", ToRoom: room, Body: "LOGOFF"})
		a.closeInbox()
	}
}

// Attach registers a local user with the hub.
func (h *Hub) Attach(userID int64, handle, room string) (*Attachment, error) {
	h.mu.Lock()
	if !h.enabled {
		h.mu.Unlock()
		return nil, fmt.Errorf("MRC is disabled")
	}
	if !h.connected {
		h.mu.Unlock()
		return nil, fmt.Errorf("MRC is offline — reconnecting")
	}
	cfg := h.cfg
	handle = SanitizeName(handle)
	if handle == "" {
		h.mu.Unlock()
		return nil, fmt.Errorf("empty MRC handle")
	}
	if room == "" {
		room = cfg.DefaultRoom
	}
	room = SanitizeName(room)
	id := fmt.Sprintf("%d-%d", userID, h.attachSeq.Add(1))
	a := newAttachment(id, userID, handle, room)
	h.attached[id] = a
	h.mu.Unlock()

	_ = h.writePacket(Packet{FromUser: handle, FromSite: cfg.BBSName, FromRoom: room, ToUser: "SERVER", ToRoom: room, Body: "IAMHERE"})
	_ = h.writePacket(Packet{FromUser: handle, FromSite: cfg.BBSName, FromRoom: "", ToUser: "SERVER", ToRoom: room, Body: "NEWROOM::" + room})
	_ = h.writePacket(Packet{FromUser: "CLIENT", FromSite: cfg.BBSName, FromRoom: room, ToUser: "SERVER", MsgExt: "ALL", Body: "USERLIST"})
	return a, nil
}

// Detach removes a local user.
func (h *Hub) Detach(a *Attachment) {
	if a == nil {
		return
	}
	h.mu.Lock()
	delete(h.attached, a.ID)
	cfg := h.cfg
	h.mu.Unlock()
	room := a.CurrentRoom()
	_ = h.writePacket(Packet{FromUser: a.Handle, FromSite: cfg.BBSName, FromRoom: room, ToUser: "SERVER", ToRoom: room, Body: "LOGOFF"})
	a.closeInbox()
}

// JoinRoom switches an attachment to another room.
func (h *Hub) JoinRoom(a *Attachment, newRoom string) error {
	if a == nil {
		return fmt.Errorf("nil attachment")
	}
	newRoom = SanitizeName(newRoom)
	if newRoom == "" {
		return fmt.Errorf("empty room")
	}
	old := a.CurrentRoom()
	h.mu.Lock()
	cfg := h.cfg
	h.mu.Unlock()
	a.SetRoom(newRoom)
	return h.writePacket(Packet{
		FromUser: a.Handle, FromSite: cfg.BBSName, FromRoom: old,
		ToUser: "SERVER", ToRoom: newRoom,
		Body: "NEWROOM:" + old + ":" + newRoom,
	})
}

// SendChat sends one or more room chat lines (auto-split to 140).
func (h *Hub) SendChat(a *Attachment, text string) error {
	if a == nil {
		return fmt.Errorf("nil attachment")
	}
	h.mu.Lock()
	cfg := h.cfg
	h.mu.Unlock()
	room := a.CurrentRoom()
	for _, chunk := range SplitMessage(text, maxMsgLen) {
		if err := h.writePacket(Packet{FromUser: a.Handle, FromSite: cfg.BBSName, FromRoom: room, ToRoom: room, Body: chunk}); err != nil {
			return err
		}
	}
	return nil
}

// SendPrivate sends a PM.
func (h *Hub) SendPrivate(a *Attachment, toUser, text string) error {
	if a == nil {
		return fmt.Errorf("nil attachment")
	}
	toUser = SanitizeName(toUser)
	h.mu.Lock()
	cfg := h.cfg
	h.mu.Unlock()
	for _, chunk := range SplitMessage(text, maxMsgLen) {
		if err := h.writePacket(Packet{FromUser: a.Handle, FromSite: cfg.BBSName, ToUser: toUser, Body: chunk}); err != nil {
			return err
		}
	}
	return nil
}

// SendAction sends /me style action (as chat with leading text — server-dependent).
func (h *Hub) SendAction(a *Attachment, text string) error {
	return h.SendChat(a, "/me "+strings.TrimSpace(text))
}

// SendServerCmd sends a raw body command from this user (LIST, MOTD, WHOON, …).
func (h *Hub) SendServerCmd(a *Attachment, cmd string) error {
	if a == nil {
		return fmt.Errorf("nil attachment")
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	h.mu.Lock()
	cfg := h.cfg
	h.mu.Unlock()
	room := a.CurrentRoom()
	up := strings.ToUpper(cmd)
	// Some commands use CLIENT as FromUser
	from := a.Handle
	if up == "LIST" || up == "STATS" || up == "BBSES" || strings.HasPrefix(up, "INFO") {
		from = cfg.BBSName
	}
	return h.writePacket(Packet{FromUser: from, FromSite: cfg.BBSName, FromRoom: room, ToUser: "SERVER", ToRoom: room, Body: SanitizeField(cmd)})
}

// SetTopic requests a topic change.
func (h *Hub) SetTopic(a *Attachment, topic string) error {
	topic = StripTildesForBody(topic)
	return h.SendServerCmd(a, "TOPIC "+topic)
}
