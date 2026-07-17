// Package ui is a ServiceMonitor-style Bubble Tea TUI for browsing the nodelist.
package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/JohnDovey/NodeGUI/internal/config"
	"github.com/JohnDovey/NodeGUI/internal/nodelist"
	"github.com/JohnDovey/NodeGUI/internal/store"
)

// Layout constants. lipgloss Width/Height set the *content* box; rounded borders
// add 1 cell on each side (2 total per axis).
const (
	borderX   = 2 // left + right border
	borderY   = 2 // top + bottom border
	panelGap  = 1 // space between left and right panels
	minLeftW  = 22
	minRightW = 28
	prefLeftW = 42
)

// Model is the root bubbletea model.
type Model struct {
	dbPath  string
	store   *store.Store
	cfg     config.Settings
	version string
	isSysop bool   // import / source URL changes require sysop
	caller  string // optional DOOR.SYS user name for header

	nodes   []store.Node
	total   int // unfiltered count
	matched int // filtered count
	stats   store.Stats
	cursor  int
	offset  int // left-list scroll offset
	width   int
	height  int
	ready   bool

	// Computed layout (content widths/heights, not including borders).
	leftW  int
	rightW int
	bodyH  int // panel content height

	filter  string
	detail  viewport.Model
	flash   string
	flashAt time.Time

	busy    bool
	busyMsg string

	// overlays: search / config / open-file
	mode  mode
	input textinput.Model

	// About drawer (slides in from the left), same pattern as wShare admin.
	showAbout    bool
	aboutClosing bool
	aboutWidth   int
	aboutView    viewport.Model
}

type mode int

const (
	modeNormal mode = iota
	modeSearch
	modeConfig
	modeOpenFile
)

type tickMsg time.Time

type refreshMsg struct {
	nodes   []store.Node
	total   int
	matched int
	stats   store.Stats
	err     error
}

type actionDoneMsg struct {
	err error
	msg string
	cfg config.Settings
}

// New constructs the UI model. version is shown on the About drawer.
// When isSysop is false, import and download-source edits are disabled.
func New(dbPath string, st *store.Store, cfg config.Settings, version string, isSysop bool, caller string) Model {
	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = 40

	return Model{
		dbPath:    dbPath,
		store:     st,
		cfg:       cfg,
		version:   version,
		isSysop:   isSysop,
		caller:    caller,
		detail:    viewport.New(40, 12),
		aboutView: viewport.New(aboutMaxWidth-2, 20),
		input:     ti,
		leftW:     prefLeftW,
		rightW:    40,
		bodyH:     12,
	}
}

func (m *Model) denySysop(action string) (tea.Model, tea.Cmd) {
	m.flash = errStyle.Render("✗ " + action + " is sysop-only")
	m.flashAt = time.Now()
	return m, nil
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) refreshCmd() tea.Cmd {
	st := m.store
	filter := m.filter
	return func() tea.Msg {
		total, err := st.Count("")
		if err != nil {
			return refreshMsg{err: err}
		}
		matched, err := st.Count(filter)
		if err != nil {
			return refreshMsg{err: err}
		}
		nodes, err := st.List(filter, 20000)
		if err != nil {
			return refreshMsg{err: err}
		}
		stats, err := st.Stats()
		if err != nil {
			return refreshMsg{err: err}
		}
		return refreshMsg{nodes: nodes, total: total, matched: matched, stats: stats}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.relayout()
		m.layoutAbout()
		m.ready = true
		m.detail.SetContent(m.renderDetail())
		if m.showAbout {
			m.refreshAboutContent()
		}
		return m, nil

	case aboutAnimMsg:
		return m.tickAboutAnim()

	case tickMsg:
		if time.Since(m.flashAt) > 5*time.Second {
			m.flash = ""
		}
		return m, tickCmd()

	case refreshMsg:
		if msg.err != nil {
			m.flash = errStyle.Render("✗ " + msg.err.Error())
			m.flashAt = time.Now()
			return m, nil
		}
		m.nodes = msg.nodes
		m.total = msg.total
		m.matched = msg.matched
		m.stats = msg.stats
		if m.cursor >= len(m.nodes) && m.cursor > 0 {
			m.cursor = len(m.nodes) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.ensureVisible()
		m.detail.SetContent(m.renderDetail())
		return m, nil

	case actionDoneMsg:
		m.busy = false
		m.busyMsg = ""
		if msg.err != nil {
			m.flash = errStyle.Render("✗ " + msg.err.Error())
		} else {
			m.flash = upStyle.Render("✓ " + msg.msg)
			if msg.cfg.BaseURL != "" {
				m.cfg = msg.cfg
			}
		}
		m.flashAt = time.Now()
		return m, m.refreshCmd()

	case tea.KeyMsg:
		if m.showAbout {
			return m.updateAbout(msg)
		}
		if m.busy {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.mode != modeNormal {
			return m.updateOverlay(msg)
		}
		return m.updateNormal(msg)
	}

	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg)
	return m, cmd
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
			m.detail.GotoTop()
			m.detail.SetContent(m.renderDetail())
		}
	case "down", "j":
		if m.cursor < len(m.nodes)-1 {
			m.cursor++
			m.ensureVisible()
			m.detail.GotoTop()
			m.detail.SetContent(m.renderDetail())
		}
	case "home", "g":
		m.cursor = 0
		m.ensureVisible()
		m.detail.GotoTop()
		m.detail.SetContent(m.renderDetail())
	case "end", "G":
		if len(m.nodes) > 0 {
			m.cursor = len(m.nodes) - 1
			m.ensureVisible()
			m.detail.GotoTop()
			m.detail.SetContent(m.renderDetail())
		}
	case "pgup", "ctrl+u":
		m.detail.HalfViewUp()
	case "pgdown", "ctrl+d":
		m.detail.HalfViewDown()
	case "left", "h":
		page := m.visibleRows()
		m.cursor -= page
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.ensureVisible()
		m.detail.SetContent(m.renderDetail())
	case "right", "l":
		page := m.visibleRows()
		m.cursor += page
		if m.cursor >= len(m.nodes) {
			m.cursor = len(m.nodes) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.ensureVisible()
		m.detail.SetContent(m.renderDetail())
	case "/", "f":
		return m.beginOverlay(modeSearch, "filter nodes…", m.filter)
	case "c":
		if !m.isSysop {
			return m.denySysop("source URL")
		}
		return m.beginOverlay(modeConfig, "download base URL…", m.cfg.BaseURL)
	case "o":
		if !m.isSysop {
			return m.denySysop("import")
		}
		return m.beginOverlay(modeOpenFile, "path to nodelist or zip…", "")
	case "i":
		if !m.isSysop {
			return m.denySysop("import")
		}
		m.busy = true
		m.busyMsg = "downloading Z1DAILY…"
		return m, m.importRemoteCmd()
	case "r":
		return m, m.refreshCmd()
	case "?":
		return m.openAbout()
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.cursor = 0
			return m, m.refreshCmd()
		}
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg)
	return m, cmd
}

func (m Model) beginOverlay(md mode, placeholder, value string) (tea.Model, tea.Cmd) {
	m.mode = md
	m.input.Placeholder = placeholder
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.input.Focus()
	m.relayout() // free vertical space for the input bar
	m.detail.SetContent(m.renderDetail())
	return m, textinput.Blink
}

func (m Model) updateOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input.Blur()
		m.relayout()
		m.detail.SetContent(m.renderDetail())
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		val := strings.TrimSpace(m.input.Value())
		md := m.mode
		m.mode = modeNormal
		m.input.Blur()
		m.relayout()
		switch md {
		case modeSearch:
			m.filter = val
			m.cursor = 0
			return m, m.refreshCmd()
		case modeConfig:
			return m, m.saveConfigCmd(val)
		case modeOpenFile:
			if val == "" {
				m.flash = errStyle.Render("✗ path required")
				m.flashAt = time.Now()
				m.detail.SetContent(m.renderDetail())
				return m, nil
			}
			m.busy = true
			m.busyMsg = "importing " + filepath.Base(val) + "…"
			return m, m.importFileCmd(val)
		}
		m.detail.SetContent(m.renderDetail())
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) saveConfigCmd(baseURL string) tea.Cmd {
	if baseURL == "" {
		baseURL = config.DefaultBaseURL
	}
	dbPath := m.dbPath
	cfg := m.cfg
	cfg.BaseURL = baseURL
	return func() tea.Msg {
		if err := config.Save(dbPath, cfg); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{msg: "source URL saved", cfg: cfg}
	}
}

func (m Model) importRemoteCmd() tea.Cmd {
	st := m.store
	cfg := m.cfg
	dbPath := m.dbPath
	return func() tea.Msg {
		im := &nodelist.Importer{Store: st, Client: nodelist.NewClient(), Domain: cfg.Domain}
		res, err := im.ImportRemote(cfg.BaseURL)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		cfg.MarkImport(res.Source, res.NodeDay, res.Nodes)
		_ = config.Save(dbPath, cfg)
		msg := fmt.Sprintf("imported %d nodes (day %d) in %s", res.Nodes, res.NodeDay, res.Duration.Round(time.Millisecond))
		return actionDoneMsg{msg: msg, cfg: cfg}
	}
}

func (m Model) importFileCmd(path string) tea.Cmd {
	st := m.store
	cfg := m.cfg
	dbPath := m.dbPath
	return func() tea.Msg {
		im := &nodelist.Importer{Store: st, Domain: cfg.Domain}
		res, err := im.ImportFile(path)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		cfg.MarkImport(res.Source, res.NodeDay, res.Nodes)
		_ = config.Save(dbPath, cfg)
		msg := fmt.Sprintf("imported %d nodes from file (day %d)", res.Nodes, res.NodeDay)
		return actionDoneMsg{msg: msg, cfg: cfg}
	}
}

// relayout recomputes panel sizes from the current terminal dimensions.
// Safe to call with width/height still zero (uses sensible floors).
func (m *Model) relayout() {
	w, h := m.width, m.height
	if w < 40 {
		w = 40
	}
	if h < 12 {
		h = 12
	}

	// Vertical: header (1) + help (1) + optional input bar (3 content+border) + body panels.
	// Panel Height is content height; border adds borderY outside.
	// visual_body = bodyH + borderY
	// 1 + (bodyH+borderY) + 1 [+ input] <= h
	chrome := 2 // header + help
	if m.mode != modeNormal {
		chrome += 3 // bordered one-line input bar ≈ 3 rows
	}
	bodyOuter := h - chrome
	if bodyOuter < 6 {
		bodyOuter = 6
	}
	m.bodyH = bodyOuter - borderY
	if m.bodyH < 4 {
		m.bodyH = 4
	}

	// Horizontal: leftOuter + gap + rightOuter <= w
	// outer = contentW + borderX
	avail := w - panelGap - 2*borderX
	if avail < minLeftW+minRightW {
		// Extremely narrow: still split something usable.
		avail = minLeftW + minRightW
	}

	left := prefLeftW
	if avail < prefLeftW+minRightW {
		left = avail - minRightW
	}
	if left < minLeftW {
		left = minLeftW
	}
	// Prefer ~40% left on wide screens, but never above prefLeftW unless needed.
	if avail > 120 {
		left = prefLeftW
	} else if avail > 80 {
		left = min(prefLeftW, max(minLeftW, avail*2/5))
	} else {
		left = min(prefLeftW, max(minLeftW, avail/3))
	}
	right := avail - left
	if right < minRightW {
		right = minRightW
		left = avail - right
		if left < minLeftW {
			// Give both a share of whatever we have.
			left = max(12, avail/3)
			right = avail - left
		}
	}

	m.leftW = left
	m.rightW = right

	// Viewport fills the right panel under the "Details" title line.
	m.detail.Width = max(10, m.rightW)
	m.detail.Height = max(3, m.bodyH-1) // title row

	// Text input tracks terminal width.
	m.input.Width = max(10, min(m.width-12, 72))

	m.ensureVisible()
}

// visibleRows is how many 2-line node entries fit in the left panel list area.
func (m Model) visibleRows() int {
	// title + blank line = 2, optional footer ~1, each node = 2 lines
	inner := m.bodyH - 3
	if inner < 2 {
		inner = 2
	}
	rows := inner / 2
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m *Model) ensureVisible() {
	rows := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m Model) View() string {
	if !m.ready {
		return "loading…"
	}

	header := m.renderHeader()
	left := m.renderLeft()
	right := m.renderRight()
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", panelGap), right)

	// About slides out from the left over the main layout (wShare-style).
	if m.showAbout && m.aboutWidth > 0 {
		drawer := m.renderAboutDrawer()
		restW := m.width - m.aboutWidth - 1
		if restW < 10 {
			restW = 10
		}
		mainClip := lipgloss.NewStyle().MaxWidth(restW).Render(body)
		body = lipgloss.JoinHorizontal(lipgloss.Top, drawer, " ", mainClip)
	}

	help := m.renderHelp()
	flash := m.flash
	if m.busy {
		flash = missStyle.Render("… " + m.busyMsg)
	}
	if flash != "" {
		flash = "  " + flash
	}
	// Keep help+flash on one visual line, clipped to width.
	footer := truncateWidth(help+flash, m.width)

	parts := []string{header, body, footer}
	if m.mode != modeNormal && !m.showAbout {
		parts = append(parts, m.renderInputBar())
	}

	view := lipgloss.JoinVertical(lipgloss.Left, parts...)
	// Hard-clip to terminal so a resize never leaves trailing overflow.
	return lipgloss.NewStyle().
		MaxWidth(max(1, m.width)).
		MaxHeight(max(1, m.height)).
		Render(view)
}

func (m Model) renderHeader() string {
	title := titleStyle.Render(" NodeGUI ")
	meta := dimStyle.Render(shortPath(m.dbPath, max(8, m.width/4)))
	if m.caller != "" {
		meta += " " + dimStyle.Render("· "+truncateRunes(m.caller, 16))
	}
	if m.stats.HasData {
		meta += " " + dimStyle.Render(fmt.Sprintf("· %d nodes · day %d", m.stats.Total, m.stats.NodeDay))
	}
	if m.filter != "" {
		meta += " " + missStyle.Render(fmt.Sprintf("· filter %q (%d)", truncateRunes(m.filter, 16), m.matched))
	}
	line := title + " " + meta
	return truncateWidth(line, m.width)
}

func (m Model) renderHelp() string {
	if m.showAbout {
		return helpStyle.Render(
			keyStyle.Render("↑↓") + " scroll  " +
				keyStyle.Render("esc") + "/" + keyStyle.Render("?") + " close",
		)
	}
	if m.mode != modeNormal {
		return helpStyle.Render(
			keyStyle.Render("enter") + " confirm  " +
				keyStyle.Render("esc") + " cancel",
		)
	}
	keys := keyStyle.Render("↑↓") + " select  "
	if m.isSysop {
		keys += keyStyle.Render("i") + " import  " +
			keyStyle.Render("o") + " open  " +
			keyStyle.Render("c") + " source  "
	}
	keys += keyStyle.Render("/") + " filter  " +
		keyStyle.Render("r") + " refresh  " +
		keyStyle.Render("?") + " about  " +
		keyStyle.Render("q") + " quit"
	return helpStyle.Render(keys)
}

func (m Model) renderInputBar() string {
	label := "Filter"
	switch m.mode {
	case modeConfig:
		label = "Source"
	case modeOpenFile:
		label = "Open"
	}
	// Content width so outer (content + borders) fits the terminal.
	w := m.width - borderX
	if w < 20 {
		w = 20
	}
	return panelStyle.Width(w).Render(
		panelTitleStyle.Render(label) + "  " + m.input.View(),
	)
}

func (m Model) renderLeft() string {
	var b strings.Builder
	title := "Nodes"
	if m.matched != m.total && m.filter != "" {
		title = fmt.Sprintf("Nodes (%d/%d)", m.matched, m.total)
	} else if m.total > 0 {
		title = fmt.Sprintf("Nodes (%d)", m.total)
	}
	b.WriteString(panelTitleStyle.Render(title) + "\n\n")

	if len(m.nodes) == 0 {
		if m.total == 0 {
			if m.isSysop {
				b.WriteString(dimStyle.Render("No nodes loaded.\nPress i to import Z1DAILY."))
			} else {
				b.WriteString(dimStyle.Render("No nodes loaded.\nAsk a sysop to import the nodelist."))
			}
		} else {
			b.WriteString(dimStyle.Render("No matches.\nEsc clears filter."))
		}
	}

	// Inner text width (content box minus a little padding feel).
	innerW := m.leftW - 2
	if innerW < 10 {
		innerW = m.leftW
	}

	rows := m.visibleRows()
	end := m.offset + rows
	if end > len(m.nodes) {
		end = len(m.nodes)
	}
	for i := m.offset; i < end; i++ {
		n := m.nodes[i]
		nameW := innerW - 13
		if nameW < 4 {
			nameW = 4
		}
		label := fmt.Sprintf("%-12s %s", truncateRunes(n.NodeNo, 12), truncateRunes(n.BBSName, nameW))
		line := normalItemStyle.Render(padRight(truncateRunes(label, innerW), innerW))
		if i == m.cursor {
			line = selectedStyle.Render(padRight(truncateRunes(label, innerW), innerW))
		}
		b.WriteString(line)
		b.WriteByte('\n')
		meta := fmt.Sprintf("  %s  %s", roleBadge(n.Role), truncateRunes(n.Location, max(4, innerW-12)))
		b.WriteString(dimStyle.Render(truncateRunes(stripRough(meta), innerW)))
		b.WriteByte('\n')
	}
	if len(m.nodes) > 0 && (m.offset > 0 || end < len(m.nodes)) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  [%d–%d/%d]", m.offset+1, end, len(m.nodes))))
	}

	return panelStyle.
		Width(m.leftW).
		Height(m.bodyH).
		Render(b.String())
}

func (m Model) renderRight() string {
	title := panelTitleStyle.Render("Details")
	inner := title + "\n" + m.detail.View()
	return panelStyle.
		Width(m.rightW).
		Height(m.bodyH).
		Render(inner)
}

func (m Model) renderDetail() string {
	dw := max(16, m.detail.Width)

	if len(m.nodes) == 0 {
		var b strings.Builder
		b.WriteString(nameStyle.Render("FidoNet Nodelist Manager") + "\n\n")
		writeWrapped(&b, "Download the Zone 1 daily nodelist and browse it offline.", dw, true)
		b.WriteByte('\n')
		b.WriteString(panelTitleStyle.Render("Source") + "\n")
		writeWrapped(&b, m.cfg.BaseURL, max(12, dw-2), false)
		b.WriteByte('\n')
		b.WriteString(panelTitleStyle.Render("Actions") + "\n")
		if m.isSysop {
			fmt.Fprintf(&b, "  %s  download & import latest Z1DAILY\n", keyStyle.Render("i"))
			fmt.Fprintf(&b, "  %s  import a local nodelist or .zip\n", keyStyle.Render("o"))
			fmt.Fprintf(&b, "  %s  edit download base URL\n", keyStyle.Render("c"))
		} else {
			writeWrapped(&b, "Import and source URL changes are sysop-only. You can browse and filter once a list is loaded.", max(12, dw-2), true)
		}
		if m.cfg.LastImportAt != "" {
			b.WriteByte('\n')
			b.WriteString(panelTitleStyle.Render("Last import") + "\n")
			writeWrapped(&b, m.cfg.LastImportAt, max(12, dw-2), true)
			writeWrapped(&b, m.cfg.LastSource, max(12, dw-2), true)
		}
		return b.String()
	}
	if m.cursor < 0 || m.cursor >= len(m.nodes) {
		return dimStyle.Render("Select a node from the left panel.")
	}
	n := m.nodes[m.cursor]

	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s\n", nameStyle.Render(n.NodeNo), roleBadge(n.Role))
	// BBS name may be long — wrap under the address line.
	for i, line := range wrapText(n.BBSName, max(8, dw-2)) {
		if i == 0 {
			b.WriteString(nameStyle.Render(line))
		} else {
			b.WriteString(nameStyle.Render(line))
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	b.WriteString(panelTitleStyle.Render("Identity") + "\n")
	kv(&b, "domain", n.Domain, dw)
	kv(&b, "zone", fmt.Sprintf("%d", n.Zone), dw)
	kv(&b, "net", fmt.Sprintf("%d", n.Net), dw)
	kv(&b, "node", fmt.Sprintf("%d", n.Node), dw)
	kv(&b, "role", n.Role, dw)

	b.WriteByte('\n')
	b.WriteString(panelTitleStyle.Render("Contact") + "\n")
	kv(&b, "sysop", n.Sysop, dw)
	kv(&b, "location", n.Location, dw)
	kv(&b, "phone", n.Phone, dw)
	kv(&b, "maxbaud", n.MaxBaud, dw)

	b.WriteByte('\n')
	b.WriteString(panelTitleStyle.Render("Flags") + "\n")
	if n.Flags == "" {
		b.WriteString(dimStyle.Render("  (none)") + "\n")
	} else {
		for _, line := range wrapFlags(n.Flags, max(12, dw-4)) {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}

	b.WriteByte('\n')
	b.WriteString(panelTitleStyle.Render("Meta") + "\n")
	kv(&b, "nodeday", fmt.Sprintf("%d", n.NodeDay), dw)
	if !n.Updated.IsZero() {
		kv(&b, "updated", n.Updated.Local().Format(time.RFC1123), dw)
	}
	if m.cfg.LastSource != "" {
		kv(&b, "source", m.cfg.LastSource, dw)
	}

	return b.String()
}

// kv writes a labeled field, wrapping the value across lines when needed
// (URLs, long sysop names, flags-like strings, etc.).
func kv(b *strings.Builder, k, v string, width int) {
	const (
		gutter = 2 // leading spaces
		gap    = 2 // spaces between label and value
		labelW = 10
	)
	prefix := gutter + labelW + gap // columns before value on first line
	valW := width - prefix
	if valW < 8 {
		valW = 8
	}
	lines := wrapText(v, valW)
	if len(lines) == 0 {
		lines = []string{""}
	}
	fmt.Fprintf(b, "%s%s%s%s\n",
		strings.Repeat(" ", gutter),
		dimStyle.Render(fmt.Sprintf("%-*s", labelW, k)),
		strings.Repeat(" ", gap),
		lines[0],
	)
	cont := strings.Repeat(" ", prefix)
	for _, line := range lines[1:] {
		fmt.Fprintf(b, "%s%s\n", cont, line)
	}
}

// writeWrapped writes text wrapped to width, each line indented by two spaces.
// When dim is true, lines use dimStyle.
func writeWrapped(b *strings.Builder, s string, width int, dim bool) {
	if width < 8 {
		width = 8
	}
	for _, line := range wrapText(s, width) {
		text := "  " + line
		if dim {
			text = dimStyle.Render(text)
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}
}

// wrapText hard-wraps s to at most width runes per line. Prefers breaking after
// common URL/path separators so source URLs remain readable.
func wrapText(s string, width int) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if width < 1 {
		width = 1
	}
	runes := []rune(s)
	if len(runes) <= width {
		return []string{s}
	}

	var lines []string
	for len(runes) > 0 {
		if len(runes) <= width {
			lines = append(lines, string(runes))
			break
		}
		// Prefer a break at the last separator within the window.
		cut := width
		for i := width; i > width/3; i-- {
			switch runes[i-1] {
			case '/', '\\', ' ', '\t', '-', '_', '.', '?', '&', '=', ':', ',', ';':
				cut = i
				goto gotCut
			}
		}
	gotCut:
		lines = append(lines, string(runes[:cut]))
		runes = runes[cut:]
		// Drop a single leading space on continuation lines.
		if len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	return lines
}

func roleBadge(role string) string {
	switch strings.ToLower(role) {
	case "zone", "region", "host":
		return upStyle.Render(role)
	case "hub":
		return roleStyle.Render(role)
	case "down":
		return errStyle.Render(role)
	case "hold", "pvt":
		return missStyle.Render(role)
	default:
		return dimStyle.Render(role)
	}
}

func wrapFlags(flags string, width int) []string {
	if width < 12 {
		width = 12
	}
	parts := strings.Split(flags, ",")
	var lines []string
	var cur strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if cur.Len() == 0 {
			cur.WriteString(p)
			continue
		}
		if cur.Len()+1+len(p) > width {
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(p)
			continue
		}
		cur.WriteByte(',')
		cur.WriteString(p)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	if len(lines) == 0 {
		return []string{flags}
	}
	return lines
}

func padRight(s string, n int) string {
	plain := stripRough(s)
	// Use display width roughly as rune count (styles already stripped).
	w := len([]rune(plain))
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// truncateWidth clips a styled string to at most width terminal cells.
func truncateWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

func shortPath(p string, max int) string {
	if max < 8 {
		max = 8
	}
	if len(p) <= max {
		return p
	}
	// keep tail (filename + parent hint)
	return "…" + p[len(p)-(max-1):]
}

func stripRough(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
