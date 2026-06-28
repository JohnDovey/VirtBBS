package files

import "time"

// CatalogStats holds system-wide file catalog counters.
type CatalogStats struct {
	Total      int
	Today      int
	LastMonth  int
}

// GetCatalogStats returns counts of catalogued files (excluding missing placeholders).
func (s *Store) GetCatalogStats() (CatalogStats, error) {
	var st CatalogStats
	monthAgo := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")

	row := s.db.QueryRow(`SELECT COUNT(*) FROM files WHERE description != ?`, missingFileDesc)
	if err := row.Scan(&st.Total); err != nil {
		return st, err
	}
	row = s.db.QueryRow(`SELECT COUNT(*) FROM files WHERE description != ? AND upload_date = ?`,
		missingFileDesc, today)
	if err := row.Scan(&st.Today); err != nil {
		return st, err
	}
	row = s.db.QueryRow(`SELECT COUNT(*) FROM files WHERE description != ? AND upload_date >= ?`,
		missingFileDesc, monthAgo)
	if err := row.Scan(&st.LastMonth); err != nil {
		return st, err
	}
	return st, nil
}

// CountUploadsByDay returns file upload counts keyed by YYYY-MM-DD for rows
// with upload_date on or after the start of the last days window.
func (s *Store) CountUploadsByDay(days int) (map[string]int, error) {
	if days <= 0 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	rows, err := s.db.Query(`SELECT upload_date, COUNT(*) FROM files
		WHERE description != ? AND upload_date >= ?
		GROUP BY upload_date`, missingFileDesc, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var date string
		var n int
		if err := rows.Scan(&date, &n); err != nil {
			return nil, err
		}
		out[date] = n
	}
	return out, rows.Err()
}

// UpdateDescription changes the catalog description for a file.
func (s *Store) UpdateDescription(dirID int64, filename, description string) error {
	_, err := s.db.Exec(`UPDATE files SET description=? WHERE dir_id=? AND filename=?`,
		description, dirID, filename)
	return err
}