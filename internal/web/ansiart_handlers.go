package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/virtbbs/virtbbs/pkg/ansiart"
)

func ansiArtLibRoot() string {
	return filepath.Join("DoorGames", "AnsiArt", "LIBRARY")
}

func (s *Server) handleAnsiArt(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	lib, err := ansiart.NewLibrary(ansiArtLibRoot())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	entries, _ := lib.ListRecent(20)
	data := struct {
		pageData
		Entries []ansiart.Entry
	}{
		pageData: s.page(r),
		Entries:  entries,
	}
	s.render(w, "ansiart.html", data)
}

func (s *Server) handleAnsiArtConvert(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/ansiart", http.StatusSeeOther)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "upload too large or invalid", 400)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", 400)
		return
	}
	defer file.Close()
	mode := ansiart.Mode(strings.ToLower(r.FormValue("mode")))
	if mode != ansiart.ModeASCII {
		mode = ansiart.ModeANSI
	}
	width, _ := strconv.Atoi(r.FormValue("width"))
	if width <= 0 {
		width = 80
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = hdr.Filename
	}

	lib, err := ansiart.NewLibrary(ansiArtLibRoot())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	tmp, err := os.CreateTemp(lib.InboxDir(), "up-*")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		http.Error(w, err.Error(), 500)
		return
	}
	tmp.Close()

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	srcPath := tmpName + ext
	if err := os.Rename(tmpName, srcPath); err != nil {
		_ = os.Remove(tmpName)
		http.Error(w, err.Error(), 500)
		return
	}
	defer os.Remove(srcPath)

	art, wcols, hrows, err := ansiart.ConvertFile(srcPath, ansiart.Options{
		Mode: mode, Width: width, Title: title, Author: u.Name,
		Source: hdr.Filename, Version: "web",
	})
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ent, err := lib.SaveConversion(u.Name, title, srcPath, art, mode, wcols, hrows)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_ = lib.WriteBulletin(filepath.Join("display", "ANSIART.ANS"), "VirtBBS")

	http.Redirect(w, r, "/ansiart/view?user="+url.QueryEscape(ent.Meta.User)+"&id="+url.QueryEscape(ent.Meta.ID), http.StatusSeeOther)
}

func (s *Server) handleAnsiArtView(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	user := r.URL.Query().Get("user")
	id := r.URL.Query().Get("id")
	dir := filepath.Join(ansiArtLibRoot(), user, id)
	metaRaw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var meta ansiart.EntryMeta
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	artPath := filepath.Join(dir, meta.Result)
	raw, err := os.ReadFile(artPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sauce, _ := ansiart.ReadSAUCE(raw)
	body := ansiart.StripSAUCEForDisplay(raw)
	htmlBody := ansiToHTML(string(body))
	data := struct {
		pageData
		Meta    ansiart.EntryMeta
		Sauce   ansiart.SauceInfo
		Preview template.HTML
		UserQ   string
		IDQ     string
	}{
		pageData: s.page(r),
		Meta:     meta,
		Sauce:    sauce,
		Preview:  template.HTML(htmlBody),
		UserQ:    user,
		IDQ:      id,
	}
	s.render(w, "ansiart_view.html", data)
}

func (s *Server) handleAnsiArtDownload(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	user := r.URL.Query().Get("user")
	id := r.URL.Query().Get("id")
	which := r.URL.Query().Get("which")
	dir := filepath.Join(ansiArtLibRoot(), user, id)
	metaRaw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var meta ansiart.EntryMeta
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	path := filepath.Join(dir, meta.Result)
	name := meta.Result
	if which == "source" {
		path = filepath.Join(dir, meta.SourceRel)
		name = meta.SourceRel
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	http.ServeFile(w, r, path)
}
