// ============================================================================
// VirtBBS — A modern BBS server inspired by PCBoard BBS
//           (Clark Development Company, 1987-1996)
//
// Copyright (c) 2026 John Dovey <dovey.john@gmail.com>
//
// MIT License
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.
//
// Change History:
//   v0.0.1  2026-06-24  Initial implementation
//   v0.0.2  2026-06-24  Phase 10: Fix node.kick NodeID param; add From to broadcast
//   v0.0.5  2026-06-24  Phase 14: Rich callers tab with Entry data, search, daily stats
//   v0.0.5  2026-06-24  Fix config tab (json tags, merge-save); fix grey message text;
//                        add FidoNet tab (config, toss, scan); add full session/paths config
// ============================================================================

// virtbbs-gui is the VirtBBS sysop console — a Fyne GUI that connects to a
// remote (or local) VirtBBS server via the management API.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/virtbbs/virtbbs/internal/version"
)

// apiClient is a minimal JSON-over-TCP client for the VirtBBS API.
type apiClient struct {
	host     string
	port     int
	user     string
	password string
}

type apiRequest struct {
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
	Auth   struct {
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"auth"`
}

type apiResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (c *apiClient) call(method string, params any, out any) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.host, c.port), 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	req := apiRequest{Method: method, Params: params}
	req.Auth.User = c.user
	req.Auth.Password = c.password

	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return err
	}

	sc := bufio.NewScanner(conn)
	if !sc.Scan() {
		return fmt.Errorf("no response")
	}
	var resp apiResponse
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("api error: %s", resp.Error)
	}
	if out != nil && resp.Result != nil {
		return json.Unmarshal(resp.Result, out)
	}
	return nil
}

// --- GUI ---

func main() {
	a := app.NewWithID("io.virtbbs.sysop")
	w := a.NewWindow(fmt.Sprintf("VirtBBS Sysop Console v%s", version.Version))
	w.Resize(fyne.NewSize(1024, 680))

	// Connection settings (persisted in Fyne preferences)
	prefs := a.Preferences()
	client := &apiClient{
		host:     prefs.StringWithFallback("api_host", "localhost"),
		port:     prefs.IntWithFallback("api_port", 9999),
		user:     prefs.StringWithFallback("api_user", "Sysop"),
		password: "",
	}

	// Build tabs
	tabs := container.NewAppTabs(
		buildSettingsTab(client, prefs, w),
		buildNodeTab(client, w),
		buildUsersTab(client, w),
		buildMessagesTab(client, w),
		buildConferencesTab(client, w),
		buildCallersTab(client),
		buildConfigTab(client, w),
		buildFidoTab(client, w),
	)

	w.SetContent(tabs)
	w.ShowAndRun()
}

// ── Node Monitor ──────────────────────────────────────────────────────────────

func buildNodeTab(c *apiClient, w fyne.Window) *container.TabItem {
	table := widget.NewTable(
		func() (int, int) { return 1, 5 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) {},
	)
	status := widget.NewLabel("Not connected")
	refresh := widget.NewButton("Refresh", func() {
		var nodes []struct {
			ID        int    `json:"ID"`
			Status    string `json:"Status"`
			UserName  string `json:"UserName"`
			City      string `json:"City"`
			Operation string `json:"Operation"`
		}
		if err := c.call("nodes.list", nil, &nodes); err != nil {
			status.SetText("Error: " + err.Error())
			return
		}
		headers := []string{"Node", "Status", "User", "City", "Operation"}
		rows := make([][]string, len(nodes))
		for i, n := range nodes {
			rows[i] = []string{fmt.Sprintf("%d", n.ID), n.Status, n.UserName, n.City, n.Operation}
		}
		table.Length = func() (int, int) { return len(nodes) + 1, 5 }
		table.CreateCell = func() fyne.CanvasObject { return widget.NewLabel("") }
		table.UpdateCell = func(id widget.TableCellID, o fyne.CanvasObject) {
			lbl := o.(*widget.Label)
			if id.Row == 0 {
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				lbl.SetText(headers[id.Col])
				return
			}
			lbl.TextStyle = fyne.TextStyle{}
			lbl.SetText(rows[id.Row-1][id.Col])
		}
		table.Refresh()
		status.SetText(fmt.Sprintf("%d node(s) active", len(nodes)))
	})

	kickBtn := widget.NewButton("Kick Node…", func() {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("Node ID")
		dialog.ShowCustomConfirm("Kick Node", "Kick", "Cancel", entry, func(ok bool) {
			if !ok {
				return
			}
			var id int
			fmt.Sscanf(entry.Text, "%d", &id)
			if err := c.call("node.kick", map[string]any{"NodeID": id}, nil); err != nil {
				dialog.ShowError(err, w)
			}
		}, w)
	})

	broadcastBtn := widget.NewButton("Broadcast…", func() {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("Message to all nodes")
		dialog.ShowCustomConfirm("Broadcast Message", "Send", "Cancel", entry, func(ok bool) {
			if !ok || entry.Text == "" {
				return
			}
			if err := c.call("node.broadcast", map[string]any{"From": "Sysop", "Message": entry.Text}, nil); err != nil {
				dialog.ShowError(err, w)
			}
		}, w)
	})

	return container.NewTabItem("Nodes", container.NewBorder(
		container.NewHBox(refresh, kickBtn, broadcastBtn, status), nil, nil, nil, table,
	))
}

// ── Users ─────────────────────────────────────────────────────────────────────

func buildUsersTab(c *apiClient, w fyne.Window) *container.TabItem {
	list := widget.NewList(
		func() int { return 0 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, o fyne.CanvasObject) {},
	)
	var userNames []string
	var userIDs []int64

	refresh := widget.NewButton("Refresh", func() {
		var result []struct {
			ID   int64  `json:"ID"`
			Name string `json:"Name"`
			City string `json:"City"`
			SecurityLevel int `json:"SecurityLevel"`
			TimesOnline   int `json:"TimesOnline"`
		}
		if err := c.call("users.list", nil, &result); err != nil {
			dialog.ShowError(err, w)
			return
		}
		userNames = make([]string, len(result))
		userIDs = make([]int64, len(result))
		for i, u := range result {
			userNames[i] = fmt.Sprintf("%-25s  %-20s  Sec:%-3d  Calls:%d", u.Name, u.City, u.SecurityLevel, u.TimesOnline)
			userIDs[i] = u.ID
		}
		list.Length = func() int { return len(userNames) }
		list.UpdateItem = func(id widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(userNames[id])
		}
		list.Refresh()
	})

	deleteBtn := widget.NewButton("Delete Selected", nil)
	resetPassBtn := widget.NewButton("Reset Password…", nil)

	var selectedIdx widget.ListItemID = -1
	list.OnSelected = func(id widget.ListItemID) { selectedIdx = id }

	deleteBtn.OnTapped = func() {
		if selectedIdx < 0 || selectedIdx >= widget.ListItemID(len(userIDs)) {
			return
		}
		dialog.ShowConfirm("Delete User", "Are you sure?", func(ok bool) {
			if !ok {
				return
			}
			if err := c.call("users.delete", map[string]any{"ID": userIDs[selectedIdx]}, nil); err != nil {
				dialog.ShowError(err, w)
				return
			}
			refresh.OnTapped()
		}, w)
	}

	resetPassBtn.OnTapped = func() {
		if selectedIdx < 0 || selectedIdx >= widget.ListItemID(len(userIDs)) {
			return
		}
		entry := widget.NewPasswordEntry()
		dialog.ShowCustomConfirm("New Password", "Set", "Cancel", entry, func(ok bool) {
			if !ok {
				return
			}
			err := c.call("users.setpassword", map[string]any{"ID": userIDs[selectedIdx], "Password": entry.Text}, nil)
			if err != nil {
				dialog.ShowError(err, w)
			}
		}, w)
	}

	toolbar := container.NewHBox(refresh, deleteBtn, resetPassBtn)
	return container.NewTabItem("Users", container.NewBorder(toolbar, nil, nil, nil, list))
}

// ── Messages ──────────────────────────────────────────────────────────────────

func buildMessagesTab(c *apiClient, w fyne.Window) *container.TabItem {
	type msgRow struct {
		MsgNumber int    `json:"MsgNumber"`
		FromName  string `json:"FromName"`
		ToName    string `json:"ToName"`
		Subject   string `json:"Subject"`
		Body      string `json:"Body"`
	}

	confEntry := widget.NewEntry()
	confEntry.SetText("0")
	confEntry.SetPlaceHolder("Conference ID")

	// Message list on the left
	var msgs []msgRow
	msgList := widget.NewList(
		func() int { return len(msgs) },
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id < len(msgs) {
				m := msgs[id]
				o.(*widget.Label).SetText(fmt.Sprintf("#%-5d  %-20s → %-20s  %s",
					m.MsgNumber, m.FromName, m.ToName, m.Subject))
			}
		},
	)

	// Message body on the right — use a non-disabled entry so text is readable
	bodyLabel := widget.NewLabel("")
	bodyLabel.Wrapping = fyne.TextWrapWord
	bodyScroll := container.NewVScroll(bodyLabel)

	msgList.OnSelected = func(id widget.ListItemID) {
		if id < len(msgs) {
			m := msgs[id]
			bodyLabel.SetText(fmt.Sprintf(
				"Msg #%d\nFrom: %s\nTo:   %s\nSubj: %s\n\n%s",
				m.MsgNumber, m.FromName, m.ToName, m.Subject, m.Body,
			))
		}
	}

	deleteBtn := widget.NewButton("Delete Selected", func() {
		if msgList.Length() == 0 {
			return
		}
		// No selection API in Fyne List — inform the user
		dialog.ShowInformation("Delete", "Select a message in the list first, then use the API or BBS session to delete.", w)
	})

	fetch := widget.NewButton("Fetch", func() {
		confID := 0
		fmt.Sscanf(confEntry.Text, "%d", &confID)
		if err := c.call("messages.list", map[string]any{"ConferenceID": confID, "Limit": 50}, &msgs); err != nil {
			dialog.ShowError(err, w)
			return
		}
		msgList.Refresh()
		bodyLabel.SetText("")
	})

	top := container.NewBorder(nil, nil,
		widget.NewLabel("Conference:"),
		container.NewHBox(fetch, deleteBtn),
		confEntry,
	)
	split := container.NewHSplit(msgList, bodyScroll)
	split.SetOffset(0.4)
	return container.NewTabItem("Messages", container.NewBorder(top, nil, nil, nil, split))
}

// ── Callers Log ───────────────────────────────────────────────────────────────

func buildCallersTab(c *apiClient) *container.TabItem {
	type callerEntry struct {
		Timestamp     string `json:"timestamp"`
		UserName      string `json:"user_name"`
		City          string `json:"city"`
		RemoteAddr    string `json:"remote_addr"`
		SecurityLevel int    `json:"security_level"`
		Node          int    `json:"node"`
		Action        string `json:"action"`
		DurationSecs  int    `json:"duration_secs"`
		MsgsRead      int    `json:"msgs_read"`
		MsgsLeft      int    `json:"msgs_left"`
		FilesDown     int    `json:"files_down"`
		FilesUp       int    `json:"files_up"`
	}

	// Use a Label (non-disabled) inside a scroll for readable text
	output := widget.NewLabel("")
	output.Wrapping = fyne.TextWrapOff
	outputScroll := container.NewVScroll(output)

	statsLabel := widget.NewLabel("")
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search name / city…")

	renderEntries := func(entries []callerEntry) {
		var sb strings.Builder
		for _, e := range entries {
			ts := e.Timestamp
			if len(ts) >= 16 {
				ts = ts[:16]
			}
			line := fmt.Sprintf("%-16s  %-25s  %-20s  Sec:%-3d  N%-2d  %-8s",
				ts, e.UserName, e.City, e.SecurityLevel, e.Node, e.Action)
			if e.DurationSecs > 0 {
				m := e.DurationSecs / 60
				s := e.DurationSecs % 60
				line += fmt.Sprintf("  %dm%02ds", m, s)
			}
			if e.MsgsRead > 0 || e.MsgsLeft > 0 {
				line += fmt.Sprintf("  M:%d/%d", e.MsgsRead, e.MsgsLeft)
			}
			if e.FilesDown > 0 || e.FilesUp > 0 {
				line += fmt.Sprintf("  F:↓%d↑%d", e.FilesDown, e.FilesUp)
			}
			sb.WriteString(line + "\n")
		}
		output.SetText(sb.String())
	}

	refreshBtn := widget.NewButton("Refresh", func() {
		var entries []callerEntry
		if err := c.call("callers.list", map[string]any{"N": 200}, &entries); err != nil {
			output.SetText("Error: " + err.Error())
			return
		}
		renderEntries(entries)

		var stats struct {
			Unique int `json:"unique"`
			Total  int `json:"total"`
		}
		if err := c.call("callers.stats", nil, &stats); err == nil {
			statsLabel.SetText(fmt.Sprintf("Today: %d calls, %d unique callers", stats.Total, stats.Unique))
		}
	})

	searchBtn := widget.NewButton("Search", func() {
		q := strings.TrimSpace(searchEntry.Text)
		var entries []callerEntry
		if err := c.call("callers.search", map[string]any{"Query": q, "N": 200}, &entries); err != nil {
			output.SetText("Error: " + err.Error())
			return
		}
		renderEntries(entries)
	})

	top := container.NewVBox(
		statsLabel,
		container.NewBorder(nil, nil, nil,
			container.NewHBox(refreshBtn, searchBtn),
			searchEntry),
	)
	return container.NewTabItem("Callers", container.NewBorder(top, nil, nil, nil, outputScroll))
}

// ── Settings (connection) ─────────────────────────────────────────────────────

func buildSettingsTab(c *apiClient, prefs fyne.Preferences, w fyne.Window) *container.TabItem {
	hostEntry := widget.NewEntry()
	hostEntry.SetText(prefs.StringWithFallback("api_host", "localhost"))
	portEntry := widget.NewEntry()
	portEntry.SetText(fmt.Sprintf("%d", prefs.IntWithFallback("api_port", 9999)))
	userEntry := widget.NewEntry()
	userEntry.SetText(prefs.StringWithFallback("api_user", "Sysop"))
	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("API password")

	statusLbl := widget.NewLabel("")

	connBtn := widget.NewButton("Connect / Test", func() {
		c.host = hostEntry.Text
		c.user = userEntry.Text
		c.password = passEntry.Text
		fmt.Sscanf(portEntry.Text, "%d", &c.port)
		prefs.SetString("api_host", c.host)
		prefs.SetInt("api_port", c.port)
		prefs.SetString("api_user", c.user)

		var cfg any
		if err := c.call("config.get", nil, &cfg); err != nil {
			statusLbl.SetText("❌ " + err.Error())
			return
		}
		statusLbl.SetText("✓ Connected to VirtBBS API")
	})

	form := widget.NewForm(
		widget.NewFormItem("API Host", hostEntry),
		widget.NewFormItem("API Port", portEntry),
		widget.NewFormItem("Sysop User", userEntry),
		widget.NewFormItem("Sysop Password", passEntry),
	)
	return container.NewTabItem("Connection", container.NewVBox(form, connBtn, statusLbl))
}

// ── Conferences ───────────────────────────────────────────────────────────────

func buildConferencesTab(c *apiClient, w fyne.Window) *container.TabItem {
	type confRow struct {
		ID          int    `json:"ID"`
		Name        string `json:"Name"`
		Description string `json:"Description"`
		Public      bool   `json:"Public"`
		ReadSec     int    `json:"ReadSec"`
		WriteSec    int    `json:"WriteSec"`
	}
	var confs []confRow
	var selected int = -1

	list := widget.NewList(
		func() int { return len(confs) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id < len(confs) {
				pub := "private"
				if confs[id].Public {
					pub = "public"
				}
				o.(*widget.Label).SetText(fmt.Sprintf("[%d] %-20s  %-30s  (%s)  R:%d W:%d",
					confs[id].ID, confs[id].Name, confs[id].Description, pub,
					confs[id].ReadSec, confs[id].WriteSec))
			}
		},
	)
	list.OnSelected = func(id widget.ListItemID) { selected = id }

	refresh := widget.NewButton("Refresh", func() {
		if err := c.call("conferences.list", nil, &confs); err != nil {
			dialog.ShowError(err, w)
			return
		}
		list.Refresh()
	})

	newBtn := widget.NewButton("New…", func() {
		nameE := widget.NewEntry()
		descE := widget.NewEntry()
		pubCheck := widget.NewCheck("Public", nil)
		pubCheck.SetChecked(true)
		readSecE := widget.NewEntry()
		readSecE.SetText("10")
		writeSecE := widget.NewEntry()
		writeSecE.SetText("10")
		form := widget.NewForm(
			widget.NewFormItem("Name", nameE),
			widget.NewFormItem("Description", descE),
			widget.NewFormItem("", pubCheck),
			widget.NewFormItem("Read Security", readSecE),
			widget.NewFormItem("Write Security", writeSecE),
		)
		dialog.ShowCustomConfirm("New Conference", "Create", "Cancel", form, func(ok bool) {
			if !ok || nameE.Text == "" {
				return
			}
			var rs, ws int
			fmt.Sscanf(readSecE.Text, "%d", &rs)
			fmt.Sscanf(writeSecE.Text, "%d", &ws)
			params := map[string]any{
				"Name": nameE.Text, "Description": descE.Text,
				"Public": pubCheck.Checked,
				"ReadSec": rs, "WriteSec": ws, "SysopSec": 110,
			}
			if err := c.call("conferences.create", params, nil); err != nil {
				dialog.ShowError(err, w)
				return
			}
			refresh.OnTapped()
		}, w)
	})

	deleteBtn := widget.NewButton("Delete", func() {
		if selected < 0 || selected >= len(confs) {
			return
		}
		conf := confs[selected]
		dialog.ShowConfirm("Delete Conference",
			fmt.Sprintf("Delete '%s' (ID %d) and all its messages?", conf.Name, conf.ID),
			func(ok bool) {
				if !ok {
					return
				}
				if err := c.call("conferences.delete", map[string]any{"ID": conf.ID}, nil); err != nil {
					dialog.ShowError(err, w)
					return
				}
				refresh.OnTapped()
			}, w)
	})

	toolbar := container.NewHBox(refresh, newBtn, deleteBtn)
	return container.NewTabItem("Conferences", container.NewBorder(toolbar, nil, nil, nil, list))
}

// ── Config Editor ─────────────────────────────────────────────────────────────
// Loads the full config from the server, lets the sysop edit key fields,
// then sends the whole config back (merge-safe on the server side).

func buildConfigTab(c *apiClient, w fyne.Window) *container.TabItem {
	// We store the last loaded raw config so we can send back a full merge.
	type sessionCfg struct {
		IdleTimeoutMins int    `json:"idle_timeout_mins"`
		TimePerCallMins int    `json:"time_per_call_mins"`
		DisplayDir      string `json:"display_dir"`
		NewUserSecurity int    `json:"new_user_security"`
	}
	type pathsCfg struct {
		DB        string `json:"db"`
		Files     string `json:"files"`
		Logs      string `json:"logs"`
		CallerLog string `json:"caller_log"`
	}
	type netCfg struct {
		TelnetPort int    `json:"telnet_port"`
		SSHPort    int    `json:"ssh_port"`
		APIPort    int    `json:"api_port"`
		APIBind    string `json:"api_bind"`
	}
	type bbsCfg struct {
		Name     string `json:"name"`
		MaxNodes int    `json:"max_nodes"`
	}
	type sysopCfg struct {
		Name string `json:"name"`
	}
	type fullCfg struct {
		Network netCfg     `json:"network"`
		BBS     bbsCfg     `json:"bbs"`
		Sysop   sysopCfg   `json:"sysop"`
		Paths   pathsCfg   `json:"paths"`
		Session sessionCfg `json:"session"`
	}

	// BBS section
	bbsNameE   := widget.NewEntry()
	maxNodesE  := widget.NewEntry()
	sysopNameE := widget.NewEntry()

	// Network section
	telnetPortE := widget.NewEntry()
	sshPortE    := widget.NewEntry()
	apiPortE    := widget.NewEntry()
	apiBindE    := widget.NewEntry()

	// Paths section
	dbPathE      := widget.NewEntry()
	filesPathE   := widget.NewEntry()
	logsPathE    := widget.NewEntry()
	callerLogE   := widget.NewEntry()

	// Session section
	idleTimeoutE  := widget.NewEntry()
	timePerCallE  := widget.NewEntry()
	displayDirE   := widget.NewEntry()
	newUserSecE   := widget.NewEntry()

	status := widget.NewLabel("")

	loadBtn := widget.NewButton("Load from Server", func() {
		var cfg fullCfg
		if err := c.call("config.get", nil, &cfg); err != nil {
			status.SetText("Error: " + err.Error())
			return
		}
		bbsNameE.SetText(cfg.BBS.Name)
		maxNodesE.SetText(fmt.Sprintf("%d", cfg.BBS.MaxNodes))
		sysopNameE.SetText(cfg.Sysop.Name)
		telnetPortE.SetText(fmt.Sprintf("%d", cfg.Network.TelnetPort))
		sshPortE.SetText(fmt.Sprintf("%d", cfg.Network.SSHPort))
		apiPortE.SetText(fmt.Sprintf("%d", cfg.Network.APIPort))
		apiBindE.SetText(cfg.Network.APIBind)
		dbPathE.SetText(cfg.Paths.DB)
		filesPathE.SetText(cfg.Paths.Files)
		logsPathE.SetText(cfg.Paths.Logs)
		callerLogE.SetText(cfg.Paths.CallerLog)
		idleTimeoutE.SetText(fmt.Sprintf("%d", cfg.Session.IdleTimeoutMins))
		timePerCallE.SetText(fmt.Sprintf("%d", cfg.Session.TimePerCallMins))
		displayDirE.SetText(cfg.Session.DisplayDir)
		newUserSecE.SetText(fmt.Sprintf("%d", cfg.Session.NewUserSecurity))
		status.SetText("Loaded.")
	})

	saveBtn := widget.NewButton("Save to Server", func() {
		var maxNodes, telnet, ssh, apiPort, idle, timeCall, newSec int
		fmt.Sscanf(maxNodesE.Text, "%d", &maxNodes)
		fmt.Sscanf(telnetPortE.Text, "%d", &telnet)
		fmt.Sscanf(sshPortE.Text, "%d", &ssh)
		fmt.Sscanf(apiPortE.Text, "%d", &apiPort)
		fmt.Sscanf(idleTimeoutE.Text, "%d", &idle)
		fmt.Sscanf(timePerCallE.Text, "%d", &timeCall)
		fmt.Sscanf(newUserSecE.Text, "%d", &newSec)

		// Send only the sections we edited — the server merges into current config.
		params := map[string]any{
			"bbs":     map[string]any{"name": bbsNameE.Text, "max_nodes": maxNodes},
			"sysop":   map[string]any{"name": sysopNameE.Text},
			"network": map[string]any{
				"telnet_port": telnet, "ssh_port": ssh,
				"api_port": apiPort, "api_bind": apiBindE.Text,
			},
			"paths": map[string]any{
				"db": dbPathE.Text, "files": filesPathE.Text,
				"logs": logsPathE.Text, "caller_log": callerLogE.Text,
			},
			"session": map[string]any{
				"idle_timeout_mins": idle, "time_per_call_mins": timeCall,
				"display_dir": displayDirE.Text, "new_user_security": newSec,
			},
		}
		if err := c.call("config.update", params, nil); err != nil {
			status.SetText("Error: " + err.Error())
			return
		}
		status.SetText("Saved! Restart server to apply port/path changes.")
	})

	form := widget.NewForm(
		widget.NewFormItem("── BBS ──────────────", widget.NewLabel("")),
		widget.NewFormItem("BBS Name", bbsNameE),
		widget.NewFormItem("Max Nodes", maxNodesE),
		widget.NewFormItem("Sysop Name", sysopNameE),
		widget.NewFormItem("── Network ──────────", widget.NewLabel("")),
		widget.NewFormItem("Telnet Port", telnetPortE),
		widget.NewFormItem("SSH Port", sshPortE),
		widget.NewFormItem("API Port", apiPortE),
		widget.NewFormItem("API Bind", apiBindE),
		widget.NewFormItem("── Paths ────────────", widget.NewLabel("")),
		widget.NewFormItem("Database", dbPathE),
		widget.NewFormItem("Files Dir", filesPathE),
		widget.NewFormItem("Logs Dir", logsPathE),
		widget.NewFormItem("Caller Log", callerLogE),
		widget.NewFormItem("── Session ──────────", widget.NewLabel("")),
		widget.NewFormItem("Idle Timeout (min)", idleTimeoutE),
		widget.NewFormItem("Time Per Call (min)", timePerCallE),
		widget.NewFormItem("Display Dir", displayDirE),
		widget.NewFormItem("New User Security", newUserSecE),
	)

	return container.NewTabItem("Config", container.NewBorder(
		container.NewHBox(loadBtn, saveBtn, status),
		nil, nil, nil,
		container.NewVScroll(form),
	))
}

// ── FidoNet ───────────────────────────────────────────────────────────────────

func buildFidoTab(c *apiClient, w fyne.Window) *container.TabItem {
	type fidoCfg struct {
		Enabled     bool           `json:"enabled"`
		Address     string         `json:"address"`
		Uplink      string         `json:"uplink"`
		Password    string         `json:"password"`
		InboundDir  string         `json:"inbound_dir"`
		OutboundDir string         `json:"outbound_dir"`
		Areas       map[string]int `json:"areas"`
	}

	enabledCheck := widget.NewCheck("Enable FidoNet", nil)
	addressE     := widget.NewEntry()
	addressE.SetPlaceHolder("Zone:Net/Node (e.g. 1:234/567)")
	uplinkE      := widget.NewEntry()
	uplinkE.SetPlaceHolder("Hub address")
	passwordE    := widget.NewEntry()
	passwordE.SetPlaceHolder("Session password")
	inboundE     := widget.NewEntry()
	inboundE.SetPlaceHolder("fido/inbound")
	outboundE    := widget.NewEntry()
	outboundE.SetPlaceHolder("fido/outbound")

	// Areas display — editable label "TAG=confID, TAG=confID, ..."
	areasE := widget.NewMultiLineEntry()
	areasE.SetPlaceHolder("FIDO_GENERAL=1\nVIRTBBS_SUPPORT=2")
	areasE.SetMinRowsVisible(4)

	status := widget.NewLabel("")

	// Results area for toss/scan output
	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord
	resultScroll := container.NewVScroll(resultLabel)

	loadBtn := widget.NewButton("Load from Server", func() {
		var full struct {
			Fido fidoCfg `json:"fido"`
		}
		if err := c.call("config.get", nil, &full); err != nil {
			status.SetText("Error: " + err.Error())
			return
		}
		f := full.Fido
		enabledCheck.SetChecked(f.Enabled)
		addressE.SetText(f.Address)
		uplinkE.SetText(f.Uplink)
		passwordE.SetText(f.Password)
		inboundE.SetText(f.InboundDir)
		outboundE.SetText(f.OutboundDir)
		// Format areas map as lines
		var areaLines []string
		for tag, id := range f.Areas {
			areaLines = append(areaLines, fmt.Sprintf("%s=%d", tag, id))
		}
		areasE.SetText(strings.Join(areaLines, "\n"))
		status.SetText("Loaded.")
	})

	saveBtn := widget.NewButton("Save to Server", func() {
		// Parse areas from the text box
		areas := map[string]int{}
		for _, line := range strings.Split(areasE.Text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			var id int
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &id)
			areas[strings.TrimSpace(parts[0])] = id
		}
		params := map[string]any{
			"fido": map[string]any{
				"enabled":      enabledCheck.Checked,
				"address":      addressE.Text,
				"uplink":       uplinkE.Text,
				"password":     passwordE.Text,
				"inbound_dir":  inboundE.Text,
				"outbound_dir": outboundE.Text,
				"areas":        areas,
			},
		}
		if err := c.call("config.update", params, nil); err != nil {
			status.SetText("Error: " + err.Error())
			return
		}
		status.SetText("FidoNet config saved.")
	})

	tossBtn := widget.NewButton("Toss Inbound", func() {
		resultLabel.SetText("Tossing…")
		var result struct {
			Packets  int      `json:"Packets"`
			Imported int      `json:"Imported"`
			Skipped  int      `json:"Skipped"`
			Errors   []string `json:"Errors"`
		}
		if err := c.call("fido.toss", nil, &result); err != nil {
			resultLabel.SetText("Error: " + err.Error())
			return
		}
		msg := fmt.Sprintf("Toss complete: %d packets, %d imported, %d skipped",
			result.Packets, result.Imported, result.Skipped)
		if len(result.Errors) > 0 {
			msg += "\nErrors:\n  " + strings.Join(result.Errors, "\n  ")
		}
		resultLabel.SetText(msg)
	})

	scanBtn := widget.NewButton("Scan Outbound", func() {
		resultLabel.SetText("Scanning…")
		var result struct {
			Scanned  int      `json:"Scanned"`
			PKTFiles int      `json:"PKTFiles"`
			Errors   []string `json:"Errors"`
		}
		if err := c.call("fido.scan", nil, &result); err != nil {
			resultLabel.SetText("Error: " + err.Error())
			return
		}
		msg := fmt.Sprintf("Scan complete: %d messages in %d PKT(s)", result.Scanned, result.PKTFiles)
		if len(result.Errors) > 0 {
			msg += "\nErrors:\n  " + strings.Join(result.Errors, "\n  ")
		}
		resultLabel.SetText(msg)
	})

	pollBtn := widget.NewButton("Poll Uplink (BinkP)", func() {
		resultLabel.SetText("Polling uplink…")
		var result struct {
			Sent     []string `json:"Sent"`
			Received []string `json:"Received"`
		}
		if err := c.call("fido.poll", map[string]string{"network": ""}, &result); err != nil {
			resultLabel.SetText("Poll error: " + err.Error())
			return
		}
		resultLabel.SetText(fmt.Sprintf("Poll complete: sent %d, received %d file(s).",
			len(result.Sent), len(result.Received)))
	})

	form := widget.NewForm(
		widget.NewFormItem("", enabledCheck),
		widget.NewFormItem("Node Address", addressE),
		widget.NewFormItem("Uplink", uplinkE),
		widget.NewFormItem("Password", passwordE),
		widget.NewFormItem("Inbound Dir", inboundE),
		widget.NewFormItem("Outbound Dir", outboundE),
		widget.NewFormItem("Echo Areas (TAG=confID)", areasE),
	)

	btnBar := container.NewHBox(loadBtn, saveBtn, tossBtn, scanBtn, pollBtn)
	top := container.NewVBox(btnBar, status, form)

	configTab := container.NewTabItem("Config", container.NewBorder(
		top, nil, nil, nil, resultScroll,
	))

	// ── Nodelist Browser tab ─────────────────────────────────────────────────

	nodelistTab := buildNodelistBrowserTab(c, w)

	// ── Netmail compose tab ───────────────────────────────────────────────────

	netmailTab := buildNetmailTab(c, w)

	// ── Conference echo-flags tab ─────────────────────────────────────────────

	echoTab := buildConferenceEchoTab(c, w)

	inner := container.NewAppTabs(configTab, nodelistTab, netmailTab, echoTab)
	return container.NewTabItem("FidoNet", inner)
}

// buildNodelistBrowserTab builds the Nodelist Browser sub-tab.
func buildNodelistBrowserTab(c *apiClient, _ fyne.Window) *container.TabItem {
	type nodeEntry struct {
		Network  string `json:"network"`
		Zone     int    `json:"zone"`
		Net      int    `json:"net"`
		Node     int    `json:"node"`
		Point    int    `json:"point"`
		Name     string `json:"name"`
		Location string `json:"location"`
		Sysop    string `json:"sysop"`
		Phone    string `json:"phone"`
		Baud     int    `json:"baud"`
		Flags    string `json:"flags"`
		Type     string `json:"type"`
		Active   bool   `json:"active"`
	}
	type searchResult struct {
		Nodes []*nodeEntry `json:"nodes"`
		Total int          `json:"total"`
		Page  int          `json:"page"`
		Pages int          `json:"pages"`
	}

	networkE := widget.NewEntry()
	networkE.SetText("FidoNet")
	queryE := widget.NewEntry()
	queryE.SetPlaceHolder("sysop name / address / location")
	statusL := widget.NewLabel("")

	var nodes []*nodeEntry
	var curPage, totalPages int
	curPage = 1

	detailL := widget.NewLabel("")
	detailL.Wrapping = fyne.TextWrapWord
	detailScroll := container.NewVScroll(detailL)

	list := widget.NewList(
		func() int { return len(nodes) },
		func() fyne.CanvasObject {
			return widget.NewLabel("node")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			n := nodes[id]
			addr := fmt.Sprintf("%d:%d/%d", n.Zone, n.Net, n.Node)
			if n.Point != 0 {
				addr += fmt.Sprintf(".%d", n.Point)
			}
			obj.(*widget.Label).SetText(fmt.Sprintf("%-18s %-22s %s", addr, n.Sysop, n.Location))
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		n := nodes[id]
		addr := fmt.Sprintf("%d:%d/%d", n.Zone, n.Net, n.Node)
		if n.Point != 0 {
			addr += fmt.Sprintf(".%d", n.Point)
		}
		status := "Active"
		if !n.Active {
			status = "Down/Hold"
		}
		detailL.SetText(fmt.Sprintf(
			"Address:  %s\nNetwork:  %s\nType:     %s (%s)\nBBS Name: %s\nSysop:    %s\nLocation: %s\nPhone:    %s\nBaud:     %d\nFlags:    %s",
			addr, n.Network, n.Type, status, n.Name, n.Sysop, n.Location, n.Phone, n.Baud, n.Flags,
		))
	}

	doSearch := func() {
		params := map[string]any{
			"network": networkE.Text,
			"query":   queryE.Text,
			"page":    curPage,
			"size":    30,
		}
		var res searchResult
		if err := c.call("fido.nodes.search", params, &res); err != nil {
			statusL.SetText("Error: " + err.Error())
			return
		}
		nodes = res.Nodes
		totalPages = res.Pages
		if totalPages < 1 {
			totalPages = 1
		}
		statusL.SetText(fmt.Sprintf("Page %d/%d  (%d total nodes)", res.Page, res.Pages, res.Total))
		list.Refresh()
	}

	searchBtn := widget.NewButton("Search", func() {
		curPage = 1
		doSearch()
	})
	prevBtn := widget.NewButton("◀ Prev", func() {
		if curPage > 1 {
			curPage--
			doSearch()
		}
	})
	nextBtn := widget.NewButton("Next ▶", func() {
		if curPage < totalPages {
			curPage++
			doSearch()
		}
	})

	topBar := container.NewVBox(
		container.NewGridWithColumns(3,
			widget.NewLabel("Network"), networkE, widget.NewLabel(""),
		),
		container.NewBorder(nil, nil, nil,
			container.NewHBox(searchBtn, prevBtn, nextBtn),
			queryE,
		),
		statusL,
	)

	split := container.NewHSplit(list, detailScroll)
	split.SetOffset(0.55)

	return container.NewTabItem("Nodelist", container.NewBorder(topBar, nil, nil, nil, split))
}

// buildNetmailTab builds the NetMail compose sub-tab.
func buildNetmailTab(c *apiClient, _ fyne.Window) *container.TabItem {
	fromAddrE := widget.NewEntry()
	fromAddrE.SetPlaceHolder("Your FidoNet address (auto-filled)")
	toAddrE := widget.NewEntry()
	toAddrE.SetPlaceHolder("1:234/567 or 1:234/567.1")
	toNameE := widget.NewEntry()
	toNameE.SetPlaceHolder("Recipient name")
	subjectE := widget.NewEntry()
	bodyE := widget.NewMultiLineEntry()
	bodyE.SetMinRowsVisible(8)
	crashCheck := widget.NewCheck("Crash (direct delivery)", nil)
	networkE := widget.NewEntry()
	networkE.SetPlaceHolder("FidoNet (blank=primary)")
	statusL := widget.NewLabel("")

	// Auto-load from address.
	go func() {
		var cfg struct {
			Fido struct {
				Address string `json:"address"`
			} `json:"fido"`
		}
		if err := c.call("config.get", nil, &cfg); err == nil {
			fromAddrE.SetText(cfg.Fido.Address)
		}
	}()

	sendBtn := widget.NewButton("Send NetMail", func() {
		statusL.SetText("Sending…")
		params := map[string]any{
			"from_name": "Sysop",
			"from_addr": fromAddrE.Text,
			"to_name":   toNameE.Text,
			"to_addr":   toAddrE.Text,
			"subject":   subjectE.Text,
			"body":      bodyE.Text,
			"crash":     crashCheck.Checked,
			"network":   networkE.Text,
		}
		var result map[string]any
		if err := c.call("fido.netmail.send", params, &result); err != nil {
			statusL.SetText("Error: " + err.Error())
			return
		}
		statusL.SetText(fmt.Sprintf("Queued (PKT written). ID=%v", result["id"]))
		toAddrE.SetText("")
		toNameE.SetText("")
		subjectE.SetText("")
		bodyE.SetText("")
	})

	form := widget.NewForm(
		widget.NewFormItem("From Address", fromAddrE),
		widget.NewFormItem("To Address", toAddrE),
		widget.NewFormItem("To Name", toNameE),
		widget.NewFormItem("Subject", subjectE),
		widget.NewFormItem("Network", networkE),
		widget.NewFormItem("", crashCheck),
		widget.NewFormItem("Body", bodyE),
	)
	return container.NewTabItem("NetMail", container.NewBorder(
		container.NewVBox(sendBtn, statusL), nil, nil, nil, container.NewVScroll(form),
	))
}

// buildConferenceEchoTab builds the Conference Echo Flags sub-tab.
func buildConferenceEchoTab(c *apiClient, _ fyne.Window) *container.TabItem {
	type conf struct {
		ID          int    `json:"ID"`
		Name        string `json:"Name"`
		Echo        bool   `json:"Echo"`
		EchoTag     string `json:"EchoTag"`
		UplinkAddr  string `json:"UplinkAddr"`
		Network     string `json:"Network"`
	}

	var confs []conf
	statusL := widget.NewLabel("")

	echoCheck   := widget.NewCheck("Echomail Area", nil)
	echoTagE    := widget.NewEntry()
	echoTagE.SetPlaceHolder("AREA_TAG")
	uplinkE     := widget.NewEntry()
	uplinkE.SetPlaceHolder("override uplink (blank=default)")
	echoNetworkE := widget.NewEntry()
	echoNetworkE.SetPlaceHolder("FidoNet (blank=primary)")

	var selectedIdx int = -1

	list := widget.NewList(
		func() int { return len(confs) },
		func() fyne.CanvasObject { return widget.NewLabel("conf") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			co := confs[id]
			flag := "local"
			if co.Echo {
				flag = "echo:" + co.EchoTag
			}
			obj.(*widget.Label).SetText(fmt.Sprintf("%3d  %-22s  %s", co.ID, co.Name, flag))
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		selectedIdx = id
		co := confs[id]
		echoCheck.SetChecked(co.Echo)
		echoTagE.SetText(co.EchoTag)
		uplinkE.SetText(co.UplinkAddr)
		echoNetworkE.SetText(co.Network)
	}

	loadBtn := widget.NewButton("Refresh", func() {
		if err := c.call("conferences.list", nil, &confs); err != nil {
			statusL.SetText("Error: " + err.Error())
			return
		}
		list.Refresh()
		statusL.SetText(fmt.Sprintf("Loaded %d conferences.", len(confs)))
	})

	saveBtn := widget.NewButton("Save Selected", func() {
		if selectedIdx < 0 || selectedIdx >= len(confs) {
			statusL.SetText("Select a conference first.")
			return
		}
		co := confs[selectedIdx]
		params := map[string]any{
			"ID":         co.ID,
			"Name":       co.Name,
			"Echo":       echoCheck.Checked,
			"EchoTag":    echoTagE.Text,
			"UplinkAddr": uplinkE.Text,
			"Network":    echoNetworkE.Text,
		}
		if err := c.call("conferences.update", params, nil); err != nil {
			statusL.SetText("Error: " + err.Error())
			return
		}
		confs[selectedIdx].Echo = echoCheck.Checked
		confs[selectedIdx].EchoTag = echoTagE.Text
		confs[selectedIdx].UplinkAddr = uplinkE.Text
		confs[selectedIdx].Network = echoNetworkE.Text
		list.Refresh()
		statusL.SetText("Saved.")
	})

	editForm := widget.NewForm(
		widget.NewFormItem("", echoCheck),
		widget.NewFormItem("AREA Tag", echoTagE),
		widget.NewFormItem("Override Uplink", uplinkE),
		widget.NewFormItem("Network", echoNetworkE),
	)

	right := container.NewVBox(editForm, saveBtn)
	split := container.NewHSplit(list, right)
	split.SetOffset(0.6)

	top := container.NewVBox(container.NewHBox(loadBtn, statusL))
	return container.NewTabItem("Echo Flags", container.NewBorder(top, nil, nil, nil, split))
}
