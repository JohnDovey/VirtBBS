// Command nodegui is a VirtBBS door (and standalone TUI) for downloading and
// browsing the FidoNet Zone 1 daily nodelist in SQLite.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JohnDovey/NodeGUI/internal/about"
	"github.com/JohnDovey/NodeGUI/internal/config"
	"github.com/JohnDovey/NodeGUI/internal/nodelist"
	"github.com/JohnDovey/NodeGUI/internal/store"
	"github.com/JohnDovey/NodeGUI/internal/ui"
)

func main() {
	dbPath := flag.String("db", "", "Path to SQLite database")
	baseURL := flag.String("url", "", "Override download base URL (default from settings / built-in)")
	domain := flag.String("domain", "", "Network domain stored with nodes (default FidoNet)")
	doImport := flag.Bool("import", false, "Download and import latest Z1DAILY, then exit (sysop/local only)")
	importFile := flag.String("import-file", "", "Import a local nodelist or zip, then exit (sysop/local only)")
	doorfile := flag.String("doorfile", "", "path to DOOR.SYS drop file")
	local := flag.Bool("local", false, "run without DOOR.SYS (local/sysop testing; all features enabled)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("nodegui %s\n", Version)
		os.Exit(0)
	}

	isSysop := *local
	var sess *DoorSession

	if !*local {
		path := ResolveDoorFile(*doorfile, flag.Args())
		if path != "" {
			var err error
			sess, err = ParseDoorSYS(path)
			if err != nil {
				fatal(err)
			}
			isSysop = sess.IsSysop()
		} else if *doImport || *importFile != "" {
			// Headless import without a drop file is allowed (standalone CLI).
			isSysop = true
		} else {
			// Interactive without DOOR.SYS: treat as local/dev run.
			isSysop = true
		}
	}

	if *dbPath == "" {
		*dbPath = defaultDB()
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fatal(err)
	}
	defer st.Close()

	cfg, err := config.Load(*dbPath)
	if err != nil {
		fatal(err)
	}
	if *baseURL != "" {
		cfg.BaseURL = *baseURL
	}
	if *domain != "" {
		cfg.Domain = *domain
	}

	// Headless import modes (no TUI) — sysop / local only.
	if *importFile != "" {
		if !isSysop {
			fatal(fmt.Errorf("import is sysop-only (security >= %d)", SysopSecurityMin))
		}
		if err := runImportFile(st, &cfg, *dbPath, *importFile); err != nil {
			fatal(err)
		}
		return
	}
	if *doImport {
		if !isSysop {
			fatal(fmt.Errorf("import is sysop-only (security >= %d)", SysopSecurityMin))
		}
		if err := runImportRemote(st, &cfg, *dbPath); err != nil {
			fatal(err)
		}
		return
	}

	about.Version = Version
	caller := ""
	if sess != nil {
		caller = sess.UserName
	}
	model := ui.New(*dbPath, st, cfg, Version, isSysop, caller)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fatal(err)
	}
}

func runImportRemote(st *store.Store, cfg *config.Settings, dbPath string) error {
	fmt.Fprintf(os.Stderr, "nodegui: downloading from %s\n", cfg.BaseURL)
	im := &nodelist.Importer{Store: st, Client: nodelist.NewClient(), Domain: cfg.Domain}
	res, err := im.ImportRemote(cfg.BaseURL)
	if err != nil {
		return err
	}
	cfg.MarkImport(res.Source, res.NodeDay, res.Nodes)
	if err := config.Save(dbPath, *cfg); err != nil {
		return err
	}
	fmt.Printf("imported %d nodes (day %d) from %s\n", res.Nodes, res.NodeDay, res.Source)
	return nil
}

func runImportFile(st *store.Store, cfg *config.Settings, dbPath, path string) error {
	fmt.Fprintf(os.Stderr, "nodegui: importing %s\n", path)
	im := &nodelist.Importer{Store: st, Domain: cfg.Domain}
	res, err := im.ImportFile(path)
	if err != nil {
		return err
	}
	cfg.MarkImport(res.Source, res.NodeDay, res.Nodes)
	if err := config.Save(dbPath, *cfg); err != nil {
		return err
	}
	fmt.Printf("imported %d nodes (day %d) from %s\n", res.Nodes, res.NodeDay, res.Source)
	return nil
}

func defaultDB() string {
	if v := os.Getenv("NODEGUI_DB"); v != "" {
		return v
	}
	// Prefer a data/ directory next to the executable when installed;
	// fall back to cwd for dev runs.
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		// If running from `go run` cache, use cwd instead.
		if !strings.Contains(dir, "go-build") {
			return filepath.Join(dir, "data", "nodelist.sqlite3")
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "nodelist.sqlite3"
	}
	return filepath.Join(cwd, "data", "nodelist.sqlite3")
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "nodegui: %v\n", err)
	os.Exit(1)
}
