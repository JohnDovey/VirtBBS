package fido

import (
	"database/sql"
	"strings"
)

// DownlinkState holds per-downlink AreaFix options (pause, compressor preference).
type DownlinkState struct {
	Paused     bool
	Compressor string
}

func (a *AreaFixDB) ensureDownlinkSchema() {
	_, _ = a.db.Exec(`CREATE TABLE IF NOT EXISTS fido_areafix_downlink_state (
			network       TEXT NOT NULL,
			downlink_addr TEXT NOT NULL,
			paused        INTEGER NOT NULL DEFAULT 0,
			compressor    TEXT NOT NULL DEFAULT '',
			updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (network, downlink_addr)
		)`)
}

// DownlinkStateFor returns pause/compressor settings for a downlink.
func (a *AreaFixDB) DownlinkStateFor(network, downlinkAddr string) (DownlinkState, error) {
	a.ensureDownlinkSchema()
	var paused int
	var compressor string
	err := a.db.QueryRow(`SELECT paused, compressor FROM fido_areafix_downlink_state
		WHERE network=? AND downlink_addr=?`, network, downlinkAddr).Scan(&paused, &compressor)
	if err == sql.ErrNoRows {
		return DownlinkState{}, nil
	}
	if err != nil {
		return DownlinkState{}, err
	}
	return DownlinkState{Paused: paused != 0, Compressor: compressor}, nil
}

// SetDownlinkPaused sets or clears the pause flag for a downlink.
func (a *AreaFixDB) SetDownlinkPaused(network, downlinkAddr string, paused bool) error {
	a.ensureDownlinkSchema()
	p := 0
	if paused {
		p = 1
	}
	_, err := a.db.Exec(`INSERT INTO fido_areafix_downlink_state (network, downlink_addr, paused, updated_at)
		VALUES (?,?,?,datetime('now'))
		ON CONFLICT(network, downlink_addr) DO UPDATE SET
		 paused=excluded.paused, updated_at=excluded.updated_at`,
		network, downlinkAddr, p)
	return err
}

// SetDownlinkCompressor stores the downlink's preferred outbound compressor name.
func (a *AreaFixDB) SetDownlinkCompressor(network, downlinkAddr, compressor string) error {
	a.ensureDownlinkSchema()
	compressor = strings.TrimSpace(strings.ToLower(compressor))
	_, err := a.db.Exec(`INSERT INTO fido_areafix_downlink_state (network, downlink_addr, compressor, updated_at)
		VALUES (?,?,?,datetime('now'))
		ON CONFLICT(network, downlink_addr) DO UPDATE SET
		 compressor=excluded.compressor, updated_at=excluded.updated_at`,
		network, downlinkAddr, compressor)
	return err
}

// IsDownlinkPaused reports whether echomail fan-out to this downlink is held.
func (a *AreaFixDB) IsDownlinkPaused(network, downlinkAddr string) bool {
	st, err := a.DownlinkStateFor(network, downlinkAddr)
	if err != nil {
		return false
	}
	return st.Paused
}

// SupportedAreaFixCompressors lists valid %COMPRESS names.
var SupportedAreaFixCompressors = []string{"none", "zlib"}

func validAreaFixCompressor(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, c := range SupportedAreaFixCompressors {
		if name == c {
			return true
		}
	}
	return false
}
