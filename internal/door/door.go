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
//   v0.0.5  2026-06-24  Phase 12: Door game support — drop files, PTY exec, I/O bridge
// ============================================================================

// Package door implements PCBoard-compatible door game support.
//
// PCBoard "doors" are external programs launched by the BBS. The BBS writes a
// "drop file" (DOOR.SYS or DORINFOx.DEF) containing caller information, then
// executes the door program.  The door reads the drop file to find out who is
// calling and communicates with the caller through the BBS's I/O handle.
//
// VirtBBS supports two drop file formats:
//   - DOOR.SYS  — the most common format (GAP / PCBoard style, 52 lines)
//   - DORINFO1.DEF — RBBS-style format, also widely supported
//
// The door is executed in a pseudo-terminal (PTY) so that programs that need
// a real tty (ncurses, etc.) work correctly.  I/O is bridged between the PTY
// and the caller's rw connection.
package door

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/creack/pty"
)

// DropFileType selects which drop file format is written.
type DropFileType string

const (
	DropDoorSYS  DropFileType = "DOOR.SYS"
	DropDORINFO  DropFileType = "DORINFO1.DEF"
)

// Config describes a single door game entry.
type Config struct {
	Name           string       `toml:"name"             json:"name"`
	Description    string       `toml:"description"      json:"description"`
	Cmd            string       `toml:"cmd"              json:"cmd"`
	Args           []string     `toml:"args"             json:"args"`
	WorkDir        string       `toml:"work_dir"         json:"work_dir"`
	DropFile       DropFileType `toml:"drop_file"        json:"drop_file"`
	AppendDropFile bool         `toml:"append_drop_file" json:"append_drop_file"`
	MinSecurity    int          `toml:"min_security"     json:"min_security"`
}

// Session holds the per-call data used to write drop files.
type Session struct {
	NodeID      int
	UserName    string
	FirstName   string
	LastName    string
	City        string
	PhoneHome   string
	PhoneBiz    string
	SecurityLevel int
	TimesOnline int
	TimeLeftMins int
	ANSI        bool
	BaudRate    int    // always 38400 for Telnet/SSH
	BBSName     string
	SysopName   string
	Credits     int // remaining upload credit (DOOR.SYS line 50)
}

// Run writes a drop file and executes the door program, bridging I/O to rw.
// The drop file is written into a temporary directory named after the node.
// When the door exits, the temporary directory is removed.
func Run(rw io.ReadWriteCloser, cfg Config, sess Session) error {
	if cfg.Cmd == "" {
		return fmt.Errorf("door %q: no executable configured", cfg.Name)
	}

	// Resolve the drop file directory — node-specific so concurrent nodes don't clash.
	dropDir := filepath.Join(os.TempDir(), fmt.Sprintf("virtbbs-door-node%d", sess.NodeID))
	if err := os.MkdirAll(dropDir, 0755); err != nil {
		return fmt.Errorf("create drop dir: %w", err)
	}
	defer os.RemoveAll(dropDir)

	// Write the requested drop file format.
	dropType := cfg.DropFile
	if dropType == "" {
		dropType = DropDoorSYS
	}
	var dropPath string
	switch dropType {
	case DropDORINFO:
		dropPath = filepath.Join(dropDir, "DORINFO1.DEF")
		if err := writeDORINFO(dropPath, sess); err != nil {
			return fmt.Errorf("write DORINFO1.DEF: %w", err)
		}
	default:
		dropPath = filepath.Join(dropDir, "DOOR.SYS")
		if err := writeDoorSYS(dropPath, sess); err != nil {
			return fmt.Errorf("write DOOR.SYS: %w", err)
		}
	}

	// Build argument list.
	args := append([]string{}, cfg.Args...)
	if cfg.AppendDropFile {
		args = append(args, dropPath)
	}

	// Resolve working directory and executable.
	// Relative Cmd paths with Dir set are resolved after chdir in the child,
	// so "DoorGames/foo/foo" + work_dir "DoorGames/foo" fails. Always give
	// pty.Start an absolute binary path; Dir remains the door's cwd.
	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(cfg.Cmd)
	}
	if workDir != "" && !filepath.IsAbs(workDir) {
		if abs, err := filepath.Abs(workDir); err == nil {
			workDir = abs
		}
	}
	cmdPath, err := resolveDoorCmd(cfg.Cmd, workDir)
	if err != nil {
		return fmt.Errorf("door %q: %w", cfg.Name, err)
	}

	// Execute the door in a pseudo-terminal.
	cmd := exec.Command(cmdPath, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("DOORFILE=%s", dropPath),
		fmt.Sprintf("NODE=%d", sess.NodeID),
		"TERM=ansi",
	)

	ptm, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("start door PTY: %w", err)
	}
	defer ptm.Close()

	// Bridge: PTY → caller
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(rw, ptm)
	}()

	// Bridge: caller → PTY
	go func() {
		_, _ = io.Copy(ptm, rw)
	}()

	// Wait for door to finish or for the caller to disconnect.
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	select {
	case <-waitDone:
		// Door exited normally — wait for output to flush.
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	case <-done:
		// Caller disconnected — kill the door.
		_ = cmd.Process.Kill()
		<-waitDone
	}

	return nil
}

// resolveDoorCmd returns an absolute path to the door executable.
// Candidates (in order): absolute Cmd; Cmd relative to process cwd; basename
// under workDir; Cmd joined under workDir.
func resolveDoorCmd(cmd, workDir string) (string, error) {
	if cmd == "" {
		return "", fmt.Errorf("no executable configured")
	}
	try := func(p string) (string, bool) {
		if p == "" {
			return "", false
		}
		if !filepath.IsAbs(p) {
			if abs, err := filepath.Abs(p); err == nil {
				p = abs
			}
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
		return "", false
	}

	if filepath.IsAbs(cmd) {
		if p, ok := try(cmd); ok {
			return p, nil
		}
		return "", fmt.Errorf("executable not found: %s", cmd)
	}

	// Relative to BBS process cwd (as configured in VirtBBS.DAT).
	if p, ok := try(cmd); ok {
		return p, nil
	}
	// Basename inside work directory (e.g. cmd=mathmaze, work_dir=DoorGames/MathMaze).
	base := filepath.Base(cmd)
	if workDir != "" {
		if p, ok := try(filepath.Join(workDir, base)); ok {
			return p, nil
		}
		if p, ok := try(filepath.Join(workDir, cmd)); ok {
			return p, nil
		}
	}
	return "", fmt.Errorf("executable not found: %s (work_dir=%s)", cmd, workDir)
}

// ─── Drop file writers ────────────────────────────────────────────────────────

// writeDoorSYS writes a DOOR.SYS drop file (GAP/PCBoard format, 52 lines).
//
// Format documented at: https://archive.org/details/GAP_BBS (and PCBoard developer docs)
func writeDoorSYS(path string, s Session) error {
	baud := s.BaudRate
	if baud == 0 {
		baud = 38400
	}
	ansiFlag := "0" // 0=ANSI, 1=no-ANSI (inverted in DOOR.SYS)
	if !s.ANSI {
		ansiFlag = "1"
	}
	timeLeft := s.TimeLeftMins
	if timeLeft <= 0 {
		timeLeft = 60
	}
	now := time.Now()

	lines := []string{
		fmt.Sprintf("COM1:"),                          // 1  COM port
		fmt.Sprintf("%d,N,8,1", baud),                // 2  baud,parity,databits,stopbits
		"0",                                           // 3  parity (0=none)
		fmt.Sprintf("%d", s.NodeID),                   // 4  node number
		fmt.Sprintf("%d", baud),                       // 5  DTE baud rate
		"Y",                                           // 6  screen display (Y=local)
		"Y",                                           // 7  printer toggle (Y)
		"Y",                                           // 8  page bell (Y)
		"Y",                                           // 9  caller alarm (Y)
		s.UserName,                                    // 10 caller's full name
		s.City,                                        // 11 caller's city
		s.PhoneHome,                                   // 12 home phone
		s.PhoneBiz,                                    // 13 business phone
		"",                                            // 14 password (blank — security)
		fmt.Sprintf("%d", s.SecurityLevel),            // 15 security level
		fmt.Sprintf("%d", s.TimesOnline),              // 16 times called
		now.Format("01/02/06"),                        // 17 last date called
		fmt.Sprintf("%d", timeLeft*60),                // 18 seconds remaining
		fmt.Sprintf("%d", timeLeft),                   // 19 minutes remaining
		ansiFlag,                                      // 20 ANSI (0=yes, 1=no)
		"24",                                          // 21 page length
		"0",                                           // 22 kbytes downloaded
		"0",                                           // 23 kbytes downloaded today
		"0",                                           // 24 kbytes uploaded
		"0",                                           // 25 kbytes uploaded total
		now.Format("01/02/06"),                        // 26 today's date
		now.Format("15:04"),                           // 27 current time
		"0",                                           // 28 error-correcting connection (1=yes)
		"0",                                           // 29 ANSI graphics mode
		"24",                                          // 30 screen length
		"0",                                           // 31 multi-task (0=single)
		"",                                            // 32 read-only messages
		"",                                            // 33 default protocol
		"0",                                           // 34 graphics mode
		"0",                                           // 35 number of files downloaded
		"0",                                           // 36 number of files uploaded
		"0",                                           // 37 minutes online today
		"0",                                           // 38 kbytes uploaded today
		"0",                                           // 39 kbytes downloaded today
		"",                                            // 40 user comment
		"0",                                           // 41 doors opened
		"0",                                           // 42 messages left
		"0",                                           // 43 chat status
		s.BBSName,                                     // 44 BBS name
		s.SysopName,                                   // 45 sysop name
		"0",                                           // 46 file ratio
		"0",                                           // 47 byte ratio
		"0",                                           // 48 daily download limit
		"0",                                           // 49 remaining downloads today
		fmt.Sprintf("%d", s.Credits),                  // 50 remaining upload credit
		"0",                                           // 51 total messages
		"0",                                           // 52 total files
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0644)
}

// writeDORINFO writes a DORINFO1.DEF drop file (RBBS format).
//
// Format: plain text, each value on its own line.
func writeDORINFO(path string, s Session) error {
	baud := s.BaudRate
	if baud == 0 {
		baud = 38400
	}
	ansiFlag := "1" // 1=ANSI, 0=TTY
	if !s.ANSI {
		ansiFlag = "0"
	}
	timeLeft := s.TimeLeftMins
	if timeLeft <= 0 {
		timeLeft = 60
	}
	first := s.FirstName
	last := s.LastName
	if first == "" {
		parts := strings.Fields(s.UserName)
		if len(parts) >= 2 {
			first = parts[0]
			last = strings.Join(parts[1:], " ")
		} else {
			first = s.UserName
		}
	}

	lines := []string{
		s.BBSName,                          // 1  BBS name
		"COM1",                             // 2  COM port
		fmt.Sprintf("%d", baud),            // 3  baud rate
		"0",                                // 4  network type (0=none)
		first,                              // 5  caller first name
		last,                               // 6  caller last name
		s.City,                             // 7  caller city
		ansiFlag,                           // 8  ANSI (1=yes, 0=no)
		fmt.Sprintf("%d", s.SecurityLevel), // 9  security level
		fmt.Sprintf("%d", timeLeft),        // 10 time left (minutes)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0644)
}
