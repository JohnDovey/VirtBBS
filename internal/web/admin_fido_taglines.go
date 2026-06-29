package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/fido"
)

func (s *Server) handleAdminFidoTaglines(w http.ResponseWriter, r *http.Request) {
	_, ok := s.requireSysop(w, r)
	if !ok {
		return
	}
	db := s.Deps.Messages.DB()
	_ = fido.MigrateTaglines(db)
	tdb := fido.OpenTaglineDB(db)

	data := struct {
		pageData
		Taglines   []fido.TaglineRow
		ImportPath string
		Flash      string
		Error      string
	}{
		pageData: s.page(r),
		ImportPath: strings.TrimSpace(config.Get().Fido.TaglinesFile),
	}

	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		action := r.FormValue("action")
		switch action {
		case "add":
			if _, err := tdb.Upsert(r.FormValue("text"), "manual"); err != nil {
				data.Error = err.Error()
			} else {
				data.Flash = "Tagline added."
			}
		case "update":
			id := parseAdminID(r.FormValue("id"))
			enabled := r.FormValue("enabled") == "1"
			if err := tdb.SetText(id, r.FormValue("text"), enabled); err != nil {
				data.Error = err.Error()
			} else {
				data.Flash = "Tagline updated."
			}
		case "delete":
			if err := tdb.Delete(parseAdminID(r.FormValue("id"))); err != nil {
				data.Error = err.Error()
			} else {
				data.Flash = "Tagline deleted."
			}
		case "import_path":
			path := strings.TrimSpace(r.FormValue("import_path"))
			added, err := tdb.ImportFile(path)
			if err != nil {
				data.Error = err.Error()
			} else {
				data.Flash = fmt.Sprintf("Imported %d new tagline(s) from file.", added)
			}
		case "import_text":
			lines := strings.Split(r.FormValue("import_text"), "\n")
			added, err := tdb.ImportLines(lines, "import")
			if err != nil {
				data.Error = err.Error()
			} else {
				data.Flash = fmt.Sprintf("Merged %d new tagline(s).", added)
			}
		}
	}

	if rows, err := tdb.ListAll(); err == nil {
		data.Taglines = rows
	} else if data.Error == "" {
		data.Error = err.Error()
	}
	s.render(w, "admin_fido_taglines.html", data)
}

func parseAdminID(s string) int64 {
	var id int64
	fmt.Sscanf(strings.TrimSpace(s), "%d", &id)
	return id
}
