package fido

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/virtbbs/virtbbs/internal/ansi"
)

const bulletinRobotName = "ROBOTSTAT"

const robotFreqDetailLimit = 15

// WriteRobotStatsBulletins writes ROBOTSTAT.ANS — robot traffic only (AreaFix,
// FileFix, FREQ, TIC, and per-link robot detail) for the previous 24h and all time.
func WriteRobotStatsBulletins(db *sql.DB, displayDir, bbsName string) error {
	if db == nil {
		return fmt.Errorf("stats database not available")
	}
	if err := os.MkdirAll(displayDir, 0755); err != nil {
		return err
	}

	dayStats, err := QueryBinkpStatsForPeriod(db, "", "24h", time.Now())
	if err != nil {
		return fmt.Errorf("robot stats 24h: %w", err)
	}
	allStats, err := QueryBinkpStatsForPeriod(db, "", "all", time.Now())
	if err != nil {
		return fmt.Errorf("robot stats all-time: %w", err)
	}

	dayKey := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	text := renderRobotStatsBulletin(db, bbsName, dayKey, dayStats, allStats)
	return os.WriteFile(filepath.Join(displayDir, bulletinRobotName+".ANS"), []byte(text), 0644)
}

func renderRobotStatsBulletin(db *sql.DB, bbsName, dayKey string, dayStats, allStats *BinkpStatsQueryResult) string {
	var b strings.Builder
	b.WriteString(ansi.ClearScreen())
	b.WriteString(ansi.Header("Robot Statistics"))
	b.WriteString("\r\n")
	b.WriteString(ansi.Colorize(ansi.BrightBlack, bbsName))
	b.WriteString("\r\n\r\n")
	b.WriteString(ansi.Colorize(ansi.BrightYellow, "FidoNet robots: AreaFix, FileFix, FREQ, and TIC file echo."))
	b.WriteString("\r\n")
	b.WriteString(ansi.Colorize(ansi.BrightBlack, "Recv = inbound requests to this BBS; Sent = replies/files we originated."))
	b.WriteString("\r\n\r\n")

	appendRobotPeriod(&b, fmt.Sprintf("Previous 24 Hours (%s)", dayKey), dayStats)
	b.WriteString("\r\n")
	appendRobotPeriod(&b, "All Time", allStats)

	if db != nil {
		appendFreqDetail(&b, db)
	}

	b.WriteString(ansi.Colorize(ansi.BrightBlack, "  Generated "+time.Now().Format(time.RFC3339)))
	b.WriteString("\r\n")
	return b.String()
}

func appendRobotPeriod(b *strings.Builder, title string, q *BinkpStatsQueryResult) {
	b.WriteString(ansi.Bold())
	b.WriteString(ansi.Colorize(ansi.BrightCyan, "  "+title))
	b.WriteString(ansi.Reset())
	b.WriteString("\r\n")
	b.WriteString(ansi.Colorize(ansi.BrightBlack, strings.Repeat("-", 62)))
	b.WriteString("\r\n")

	if q == nil || !robotPeriodHasData(q) {
		b.WriteString(ansi.Colorize(ansi.Yellow, "    No robot activity recorded for this period."))
		b.WriteString("\r\n")
		return
	}

	for _, n := range q.Networks {
		if !networkHasRobotActivity(n) {
			continue
		}
		b.WriteString("\r\n")
		b.WriteString(ansi.Colorize(ansi.BrightGreen, "  Network: "+n.Network))
		b.WriteString("\r\n")
		robotStatLine(b, "AreaFix", n.AreaFixRecv, n.AreaFixSent, "")
		robotStatLine(b, "FileFix", n.FileFixRecv, n.FileFixSent, "")
		robotStatLine(b, "FREQ", n.FreqRecv, n.FreqSent, "")
		robotStatLine(b, "TIC", n.TICRecv, n.TICSent,
			fmt.Sprintf("%s recv / %s sent MB", FormatTICMegabytes(n.TICBytesRecv), FormatTICMegabytes(n.TICBytesSent)))

		links := robotLinksForNetwork(q.Links, n.Network)
		if len(links) > 0 {
			b.WriteString(ansi.Colorize(ansi.BrightYellow, "    Per-node robot detail:"))
			b.WriteString("\r\n")
			for _, l := range links {
				label := fmt.Sprintf("      %s %s", l.LinkType, l.PeerKey)
				robotStatLine(b, label, l.AreaFixRecv+l.FileFixRecv+l.FreqRecv+l.TICRecv,
					l.AreaFixSent+l.FileFixSent+l.FreqSent+l.TICSent,
					fmt.Sprintf("AF %d/%d  FF %d/%d  FREQ %d/%d  TIC %d/%d",
						l.AreaFixRecv, l.AreaFixSent, l.FileFixRecv, l.FileFixSent,
						l.FreqRecv, l.FreqSent, l.TICRecv, l.TICSent))
			}
		}
	}
}

func appendFreqDetail(b *strings.Builder, db *sql.DB) {
	freq, err := QueryFreqStats(db, "", robotFreqDetailLimit)
	if err != nil || freq == nil || (len(freq.Files) == 0 && len(freq.Nodes) == 0) {
		return
	}
	b.WriteString("\r\n")
	b.WriteString(ansi.Bold())
	b.WriteString(ansi.Colorize(ansi.BrightCyan, "  FREQ detail (all time)"))
	b.WriteString(ansi.Reset())
	b.WriteString("\r\n")
	b.WriteString(ansi.Colorize(ansi.BrightBlack, strings.Repeat("-", 62)))
	b.WriteString("\r\n")

	if len(freq.Files) > 0 {
		b.WriteString(ansi.Colorize(ansi.BrightYellow, "    Top requested files:"))
		b.WriteString("\r\n")
		for _, f := range freq.Files {
			b.WriteString("      ")
			b.WriteString(ansi.Colorize(ansi.Cyan, fmt.Sprintf("%-28s", truncateRobotLabel(f.Filename, 28))))
			b.WriteString(ansi.Colorize(ansi.White, fmt.Sprintf("req %4d   %s MB", f.RequestCount, FormatTICMegabytes(f.BytesSent))))
			b.WriteString("\r\n")
		}
	}
	if len(freq.Nodes) > 0 {
		b.WriteString(ansi.Colorize(ansi.BrightYellow, "    Top requesters:"))
		b.WriteString("\r\n")
		for _, n := range freq.Nodes {
			b.WriteString("      ")
			b.WriteString(ansi.Colorize(ansi.Cyan, fmt.Sprintf("%-28s", truncateRobotLabel(n.RequesterAddr, 28))))
			b.WriteString(ansi.Colorize(ansi.White, fmt.Sprintf("req %4d   files %4d   %s MB",
				n.RequestCount, n.FilesSent, FormatTICMegabytes(n.BytesSent))))
			b.WriteString("\r\n")
		}
	}
}

func truncateRobotLabel(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func robotStatLine(b *strings.Builder, label string, recv, sent int, extra string) {
	if recv == 0 && sent == 0 && extra == "" {
		return
	}
	b.WriteString("    ")
	b.WriteString(ansi.Colorize(ansi.Cyan, fmt.Sprintf("%-22s", label)))
	b.WriteString(ansi.Colorize(ansi.White, fmt.Sprintf("recv %4d   sent %4d", recv, sent)))
	if extra != "" {
		b.WriteString(ansi.Colorize(ansi.BrightBlack, "   "+extra))
	}
	b.WriteString("\r\n")
}

func robotPeriodHasData(q *BinkpStatsQueryResult) bool {
	for _, n := range q.Networks {
		if networkHasRobotActivity(n) {
			return true
		}
	}
	for _, l := range q.Links {
		if linkHasRobotActivity(l) {
			return true
		}
	}
	return false
}

func networkHasRobotActivity(n BinkpStatsRow) bool {
	return n.AreaFixRecv+n.AreaFixSent+n.FileFixRecv+n.FileFixSent+
		n.FreqRecv+n.FreqSent+n.TICRecv+n.TICSent > 0
}

func linkHasRobotActivity(l BinkpLinkStatsRow) bool {
	return l.AreaFixRecv+l.AreaFixSent+l.FileFixRecv+l.FileFixSent+
		l.FreqRecv+l.FreqSent+l.TICRecv+l.TICSent > 0
}

func robotLinksForNetwork(links []BinkpLinkStatsRow, network string) []BinkpLinkStatsRow {
	var out []BinkpLinkStatsRow
	for _, l := range links {
		if l.Network != network || !linkHasRobotActivity(l) {
			continue
		}
		out = append(out, l)
	}
	return out
}

// RobotBulletinPath returns the Robot Stats bulletin display file path.
func RobotBulletinPath(displayDir string) string {
	return filepath.Join(displayDir, bulletinRobotName+".ANS")
}
