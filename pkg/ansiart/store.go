package ansiart

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Library is the on-disk conversion store under DoorGames/AnsiArt/LIBRARY.
type Library struct {
	Root string
}

// EntryMeta is persisted next to each conversion.
type EntryMeta struct {
	ID        string    `json:"id"`
	User      string    `json:"user"`
	Title     string    `json:"title"`
	Mode      Mode      `json:"mode"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Source    string    `json:"source"`
	Result    string    `json:"result"` // relative to entry dir
	SourceRel string    `json:"source_rel"`
	Created   time.Time `json:"created"`
	Public    bool      `json:"public"`
}

// Entry is one conversion folder.
type Entry struct {
	Dir  string
	Meta EntryMeta
}

// NewLibrary ensures root exists.
func NewLibrary(root string) (*Library, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	_ = os.MkdirAll(filepath.Join(root, "inbox"), 0755)
	return &Library{Root: root}, nil
}

// InboxDir returns the shared inbox path.
func (L *Library) InboxDir() string {
	return filepath.Join(L.Root, "inbox")
}

// SaveConversion stores source + art and meta under LIBRARY/<user>/<id>/.
func (L *Library) SaveConversion(user, title, sourcePath string, art []byte, mode Mode, width, height int) (*Entry, error) {
	user = sanitizeName(user)
	if user == "" {
		user = "anonymous"
	}
	id := time.Now().Format("20060102-150405")
	dir := filepath.Join(L.Root, user, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	srcBase := filepath.Base(sourcePath)
	srcDest := filepath.Join(dir, "source"+extOf(srcBase))
	if err := copyFile(sourcePath, srcDest); err != nil {
		// maybe already in place — try rename/link skip
		data, rerr := os.ReadFile(sourcePath)
		if rerr != nil {
			return nil, err
		}
		if werr := os.WriteFile(srcDest, data, 0644); werr != nil {
			return nil, werr
		}
	}
	ext := ".ans"
	if mode == ModeASCII {
		ext = ".asc"
	}
	resultName := "result" + ext
	resultPath := filepath.Join(dir, resultName)
	if err := os.WriteFile(resultPath, art, 0644); err != nil {
		return nil, err
	}
	meta := EntryMeta{
		ID:        id,
		User:      user,
		Title:     title,
		Mode:      mode,
		Width:     width,
		Height:    height,
		Source:    srcBase,
		Result:    resultName,
		SourceRel: filepath.Base(srcDest),
		Created:   time.Now(),
		Public:    true,
	}
	raw, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), raw, 0644); err != nil {
		return nil, err
	}
	return &Entry{Dir: dir, Meta: meta}, nil
}

// ListRecent returns newest-first entries (cap n).
func (L *Library) ListRecent(n int) ([]Entry, error) {
	var out []Entry
	_ = filepath.WalkDir(L.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || !d.IsDir() {
			return nil
		}
		metaPath := filepath.Join(path, "meta.json")
		raw, err := os.ReadFile(metaPath)
		if err != nil {
			return nil
		}
		var m EntryMeta
		if json.Unmarshal(raw, &m) != nil {
			return nil
		}
		out = append(out, Entry{Dir: path, Meta: m})
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		return out[i].Meta.Created.After(out[j].Meta.Created)
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out, nil
}

// ResultPath returns absolute path to result art.
func (e *Entry) ResultPath() string {
	return filepath.Join(e.Dir, e.Meta.Result)
}

// SourcePath returns absolute path to saved source image.
func (e *Entry) SourcePath() string {
	return filepath.Join(e.Dir, e.Meta.SourceRel)
}

// WriteBulletin writes a recent-gallery ANSI bulletin.
func (L *Library) WriteBulletin(path string, bbsName string) error {
	entries, _ := L.ListRecent(15)
	var b strings.Builder
	b.WriteString("\x1b[2J\x1b[H")
	b.WriteString("\x1b[1m\x1b[96m=[ \x1b[97mAnsiArt Gallery\x1b[96m ]=\x1b[0m\r\n")
	b.WriteString(fmt.Sprintf("\x1b[90m%s\x1b[0m\r\n\r\n", bbsName))
	if len(entries) == 0 {
		b.WriteString("  (no conversions yet)\r\n")
	}
	for i, e := range entries {
		mode := strings.ToUpper(string(e.Meta.Mode))
		b.WriteString(fmt.Sprintf("  %2d. %-18s  %-20s  %s %dx%d  %s\r\n",
			i+1, trunc(e.Meta.User, 18), trunc(e.Meta.Title, 20), mode, e.Meta.Width, e.Meta.Height,
			e.Meta.Created.Format("2006-01-02 15:04")))
	}
	b.WriteString(fmt.Sprintf("\r\n\x1b[90m  Generated %s\x1b[0m\r\n", time.Now().Format(time.RFC3339)))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('_')
		}
	}
	return b.String()
}

func extOf(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return ext
	default:
		return ".bin"
	}
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0644)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "~"
}
