package fido

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/virtbbs/virtbbs/internal/ansi"
	"github.com/virtbbs/virtbbs/internal/appstats"
)

const (
	bulletinDailyName = "BINKPDAY"
	bulletinAllName   = "BINKPALL"
)

// WriteBinkpStatsBulletins writes/overwrites the daily and all-time ANSI
// bulletin display files in displayDir.
func WriteBinkpStatsBulletins(db *sql.DB, displayDir, bbsName string) error {
	if db == nil {
		return fmt.Errorf("stats database not available")
	}
	if err := os.MkdirAll(displayDir, 0755); err != nil {
		return err
	}
	yesterday := time.Now().Add(-24 * time.Hour)
	dayKey := yesterday.Format("2006-01-02")

	dayStats, err := QueryBinkpStatsForPeriod(db, "", "24h", time.Now())
	if err != nil {
		return fmt.Errorf("daily stats: %w", err)
	}
	allStats, err := QueryBinkpStatsForPeriod(db, "", "all", time.Now())
	if err != nil {
		return fmt.Errorf("all-time stats: %w", err)
	}

	dayText := renderStatsBulletin(bbsName,
		fmt.Sprintf("BinkP Statistics — %s (previous 24h)", dayKey), dayStats)
	allText := renderStatsBulletin(bbsName, "BinkP Statistics — All Time", allStats)

	if err := os.WriteFile(filepath.Join(displayDir, bulletinDailyName+".ANS"), []byte(dayText), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(displayDir, bulletinAllName+".ANS"), []byte(allText), 0644)
}

func renderStatsBulletin(bbsName, title string, q *BinkpStatsQueryResult) string {
	if q == nil {
		q = &BinkpStatsQueryResult{}
	}
	var b strings.Builder
	b.WriteString(ansi.ClearScreen())
	b.WriteString(ansi.Header(title))
	b.WriteString("\r\n")
	b.WriteString(ansi.Colorize(ansi.BrightBlack, bbsName))
	b.WriteString("\r\n\r\n")

	if len(q.Networks) == 0 && len(q.Links) == 0 {
		b.WriteString(ansi.Colorize(ansi.Yellow, "  No BinkP activity recorded for this period."))
		b.WriteString("\r\n")
		return b.String()
	}

	for _, n := range q.Networks {
		b.WriteString(ansi.Bold())
		b.WriteString(ansi.Colorize(ansi.BrightCyan, "  Network: "+n.Network))
		b.WriteString(ansi.Reset())
		b.WriteString("\r\n")
		b.WriteString(ansi.Colorize(ansi.BrightBlack, strings.Repeat("-", 62)))
		b.WriteString("\r\n")

		statLine(&b, "Outbound polls (OK/fail)", fmt.Sprintf("%d / %d", n.PollClientOK, n.PollClientFail))
		statLine(&b, "  files sent/recv", fmt.Sprintf("%d / %d", n.PollClientFilesSent, n.PollClientFilesRecv))
		statLine(&b, "Inbound uplink (OK/fail)", fmt.Sprintf("%d / %d", n.PollServerUplinkOK, n.PollServerUplinkFail))
		statLine(&b, "  files sent/recv", fmt.Sprintf("%d / %d", n.PollServerUplinkSent, n.PollServerUplinkRecv))
		statLine(&b, "Inbound downlink (OK/fail)", fmt.Sprintf("%d / %d", n.PollServerDownlinkOK, n.PollServerDownlinkFail))
		statLine(&b, "  files sent/recv", fmt.Sprintf("%d / %d", n.PollServerDownlinkSent, n.PollServerDownlinkRecv))
		statLine(&b, "Netmail recv/sent", fmt.Sprintf("%d / %d", n.NetmailRecv, n.NetmailSent))
		statLine(&b, "Echomail recv/sent", fmt.Sprintf("%d / %d", n.EchomailRecv, n.EchomailSent))
		statLine(&b, "AreaFix recv/sent", fmt.Sprintf("%d / %d", n.AreaFixRecv, n.AreaFixSent))
		statLine(&b, "FileFix recv/sent", fmt.Sprintf("%d / %d", n.FileFixRecv, n.FileFixSent))
		statLine(&b, "FREQ recv/sent", fmt.Sprintf("%d / %d", n.FreqRecv, n.FreqSent))
		statLine(&b, "TIC recv/sent", fmt.Sprintf("%d / %d", n.TICRecv, n.TICSent))
		statLine(&b, "TIC data recv/sent (MB)", fmt.Sprintf("%s / %s", FormatTICMegabytes(n.TICBytesRecv), FormatTICMegabytes(n.TICBytesSent)))
		statLine(&b, "Toss imported/skipped/held", fmt.Sprintf("%d / %d / %d", n.TossImported, n.TossSkipped, n.TossHeld))
		if bd := n.SkippedBreakdown(); bd != "" {
			statLine(&b, "  skip breakdown", bd)
		}
		statLine(&b, "Packets tossed", fmt.Sprintf("%d", n.TossPackets))
		if n.SessionErrors > 0 {
			statLine(&b, "Session errors", fmt.Sprintf("%d", n.SessionErrors))
		}
		b.WriteString("\r\n")

		links := filterLinks(q.Links, n.Network)
		if len(links) == 0 && networkPollTotal(n) == 0 && n.TossImported == 0 &&
			n.NetmailRecv+n.NetmailSent+n.EchomailRecv+n.EchomailSent == 0 {
			continue
		}
		if len(links) > 0 {
			b.WriteString(ansi.Colorize(ansi.BrightYellow, "  Link detail:"))
			b.WriteString("\r\n")
			for _, l := range links {
				label := l.LinkType + " " + l.PeerKey
				statLine(&b, label, fmt.Sprintf("poll %d/%d  files %d/%d  nm %d/%d  echo %d/%d",
					l.PollOK, l.PollFail, l.FilesSent, l.FilesRecv,
					l.NetmailSent, l.NetmailRecv, l.EchomailSent, l.EchomailRecv))
				if l.AreaFixSent+l.AreaFixRecv+l.FileFixSent+l.FileFixRecv+l.FreqSent+l.FreqRecv+l.TICSent+l.TICRecv > 0 {
					statLine(&b, "  robots/TIC", fmt.Sprintf("AF %d/%d  FF %d/%d  FREQ %d/%d  TIC %d/%d (%.2f/%.2f MB)",
						l.AreaFixRecv, l.AreaFixSent, l.FileFixRecv, l.FileFixSent,
						l.FreqRecv, l.FreqSent,
						l.TICRecv, l.TICSent,
						float64(l.TICBytesRecv)/(1024*1024), float64(l.TICBytesSent)/(1024*1024)))
				}
			}
			b.WriteString("\r\n")
		}
	}

	b.WriteString(ansi.Colorize(ansi.BrightBlack, "  Generated "+time.Now().Format(time.RFC3339)))
	b.WriteString("\r\n")
	return b.String()
}

func networkPollTotal(n BinkpStatsRow) int {
	return n.PollClientOK + n.PollClientFail +
		n.PollServerUplinkOK + n.PollServerUplinkFail +
		n.PollServerDownlinkOK + n.PollServerDownlinkFail
}

func filterLinks(links []BinkpLinkStatsRow, network string) []BinkpLinkStatsRow {
	var out []BinkpLinkStatsRow
	for _, l := range links {
		if l.Network == network {
			out = append(out, l)
		}
	}
	return out
}

func statLine(b *strings.Builder, label, value string) {
	b.WriteString("  ")
	b.WriteString(ansi.Colorize(ansi.Cyan, fmt.Sprintf("%-28s", label)))
	b.WriteString(" ")
	b.WriteString(ansi.Colorize(ansi.White, value))
	b.WriteString("\r\n")
}

// StartBinkpStatsBulletins runs a midnight-aligned goroutine that regenerates
// BINKPDAY.ANS and BINKPALL.ANS in displayDir. Returns a stop function.
func StartBinkpStatsBulletins(db *sql.DB, displayDir, bbsName string) func() {
	stop := make(chan struct{})
	go func() {
		write := func() {
			if err := WriteBinkpStatsBulletins(db, displayDir, bbsName); err != nil {
				LogBinkp("binkp stats bulletin: " + err.Error())
			} else {
				LogBinkp(fmt.Sprintf("binkp stats bulletins updated in %s", displayDir))
			}
			if err := WriteRobotStatsBulletins(db, displayDir, bbsName); err != nil {
				LogBinkp("robot stats bulletin: " + err.Error())
			} else {
				LogBinkp(fmt.Sprintf("robot stats bulletin updated in %s", displayDir))
			}
			if err := appstats.WriteBulletin(db, displayDir, bbsName); err != nil {
				LogBinkp("app stats bulletin: " + err.Error())
			}
		}
		write() // once at startup

		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			timer := time.NewTimer(time.Until(next))
			select {
			case <-stop:
				timer.Stop()
				return
			case <-timer.C:
				write()
			}
		}
	}()
	return func() { close(stop) }
}

// BulletinPaths returns the daily and all-time bulletin file paths.
func BulletinPaths(displayDir string) (daily, allTime string) {
	return filepath.Join(displayDir, bulletinDailyName+".ANS"),
		filepath.Join(displayDir, bulletinAllName+".ANS")
}
