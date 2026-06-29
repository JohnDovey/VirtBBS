package fido

import (
	"fmt"
	"strings"
	"sync"

	"github.com/virtbbs/virtbbs/internal/conferences"
)

// AreaMappingSaver persists echomail tag → conference ID in VirtBBS.DAT.
// Registered by cmd/virtbbs at startup (fido cannot import config).
type AreaMappingSaver func(networkName, areaTag string, confID int, remove bool) error

// FileAreaMappingSaver persists file-echo tag → files.Dir ID in VirtBBS.DAT.
type FileAreaMappingSaver func(networkName, fileTag string, dirID int64, remove bool) error

var (
	areaMappingMu     sync.Mutex
	areaMappingSaver  AreaMappingSaver
	fileAreaMappingMu sync.Mutex
	fileAreaMappingSaver FileAreaMappingSaver
)

// SetAreaMappingSaver registers the callback that writes [fido.areas] /
// [[fido.networks]].areas after an uplink AreaFix response is applied.
func SetAreaMappingSaver(fn AreaMappingSaver) {
	areaMappingMu.Lock()
	areaMappingSaver = fn
	areaMappingMu.Unlock()
}

// SetFileAreaMappingSaver registers the callback for [fido.file_areas] /
// [[fido.networks]].file_areas after a FileFix response.
func SetFileAreaMappingSaver(fn FileAreaMappingSaver) {
	fileAreaMappingMu.Lock()
	fileAreaMappingSaver = fn
	fileAreaMappingMu.Unlock()
}

func saveAreaMapping(networkName, areaTag string, confID int, remove bool) error {
	areaMappingMu.Lock()
	fn := areaMappingSaver
	areaMappingMu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(networkName, strings.ToUpper(areaTag), confID, remove)
}

func saveFileAreaMapping(networkName, fileTag string, dirID int64, remove bool) error {
	fileAreaMappingMu.Lock()
	fn := fileAreaMappingSaver
	fileAreaMappingMu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(networkName, strings.ToUpper(fileTag), dirID, remove)
}

// AreaFixAction is one subscribe/unsubscribe from an uplink robot reply.
type AreaFixAction struct {
	Tag    string
	Add    bool
	Status string
}

// IsAreaFixResponse reports netmail from an uplink's AreaFix robot.
func IsAreaFixResponse(fromName string) bool {
	return strings.EqualFold(strings.TrimSpace(fromName), AreaFixRobotName)
}

// IsFileFixResponse reports netmail from an uplink's FileFix robot.
func IsFileFixResponse(fromName string) bool {
	return strings.EqualFold(strings.TrimSpace(fromName), FileFixRobotName)
}

// IsUplinkFixRobot reports whether addr is plausibly our uplink's AreaFix/FileFix
// robot (same zone:net as uplink; node 0/1 are common hub robot addresses).
func (n *NetworkDef) IsUplinkFixRobot(addr Addr) bool {
	uplink := n.UplinkAddr()
	if uplink == (Addr{}) {
		return false
	}
	if addr.Zone != uplink.Zone || addr.Net != uplink.Net {
		return false
	}
	if addr.Node == uplink.Node {
		return true
	}
	return addr.Node == 0 || addr.Node == 1
}

// ParseAreaFixResponseBody extracts area actions from an uplink AreaFix/FileFix
// reply. Supports hpt-style status tables and +/- lines in the original-request
// section.
func ParseAreaFixResponseBody(body string) []AreaFixAction {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	lines := strings.Split(body, "\n")

	var actions []AreaFixAction
	seen := map[string]bool{}
	inOriginal := false

	addAction := func(tag string, add bool, status string) {
		tag = strings.ToUpper(strings.TrimSpace(tag))
		if tag == "" {
			return
		}
		key := tag
		if !add {
			key += "-"
		}
		if seen[key] {
			return
		}
		seen[key] = true
		actions = append(actions, AreaFixAction{Tag: tag, Add: add, Status: status})
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "original message") {
			inOriginal = true
			continue
		}
		if strings.HasPrefix(lower, "---") && inOriginal {
			continue
		}

		if inOriginal {
			if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "=") {
				if add, ok := parseAreaFixAddLine(line); ok {
					addAction(add.tag, true, "original")
				}
				continue
			}
			if strings.HasPrefix(line, "-") {
				tag := strings.ToUpper(strings.TrimSpace(line[1:]))
				addAction(tag, false, "original")
			}
			continue
		}

		if strings.HasPrefix(lower, "area") && strings.Contains(lower, "status") {
			continue
		}
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") {
			continue
		}

		if tag, status, ok := parseFixStatusTableLine(line); ok {
			if areaFixStatusIsAdd(status) {
				addAction(tag, true, status)
			} else if areaFixStatusIsRemove(status) {
				addAction(tag, false, status)
			}
			continue
		}

		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "+") || strings.HasPrefix(trim, "=") {
			fields := strings.Fields(trim)
			if len(fields) >= 1 {
				tag := strings.ToUpper(strings.TrimPrefix(strings.TrimPrefix(fields[0], "+"), "="))
				status := "subscribed"
				if len(fields) >= 2 {
					status = strings.ToLower(fields[len(fields)-1])
				}
				if areaFixStatusIsAdd(status) {
					addAction(tag, true, status)
				}
			}
			continue
		}
		if strings.HasPrefix(trim, "-") {
			fields := strings.Fields(trim)
			if len(fields) >= 1 {
				tag := strings.ToUpper(strings.TrimPrefix(fields[0], "-"))
				status := "removed"
				if len(fields) >= 2 {
					status = strings.ToLower(fields[1])
				}
				if areaFixStatusIsRemove(status) {
					addAction(tag, false, status)
				}
			}
		}
	}
	return actions
}

func parseFixStatusTableLine(line string) (tag, status string, ok bool) {
	dot := strings.Index(line, "....")
	if dot < 0 {
		return "", "", false
	}
	tag = strings.TrimSpace(strings.TrimRight(line[:dot], ". "))
	rest := strings.TrimSpace(line[dot:])
	rest = strings.TrimLeft(rest, ".")
	status = strings.ToLower(strings.TrimSpace(rest))
	if tag == "" || status == "" {
		return "", "", false
	}
	return tag, status, true
}

func areaFixStatusIsAdd(status string) bool {
	switch status {
	case "added", "subscribed", "ok", "active", "accepted", "already subscribed", "already active":
		return true
	}
	return strings.Contains(status, "subscrib")
}

func areaFixStatusIsRemove(status string) bool {
	switch status {
	case "removed", "unsubscribed", "deleted", "dropped", "not subscribed":
		return true
	}
	return strings.Contains(status, "unsubscrib") || strings.Contains(status, "removed")
}

func areaFixDisplayName(tag string) string {
	return strings.ReplaceAll(tag, "_", " ")
}

// ProcessAreaFixResponse applies an uplink AreaFix reply: auto-creates echo
// conferences and updates the network's areas map when a saver is registered.
func ProcessAreaFixResponse(nd *NetworkDef, confStore *conferences.Store, pm *Message) ([]string, error) {
	if confStore == nil {
		return nil, fmt.Errorf("areafix response: conference store required")
	}
	if !IsAreaFixResponse(pm.FromName) {
		return nil, nil
	}
	if !nd.IsUplinkFixRobot(pm.OrigAddr) {
		return nil, nil
	}
	RecordAreaFixRecv(nd.Name, "uplink", pm.OrigAddr.String())

	var notes []string
	for _, act := range ParseAreaFixResponseBody(pm.Body) {
		if act.Add {
			n, err := applyAreaFixAdd(nd, confStore, act.Tag)
			if err != nil {
				return notes, err
			}
			if n != "" {
				notes = append(notes, n)
			}
		} else {
			n, err := applyAreaFixRemove(nd, act.Tag)
			if err != nil {
				return notes, err
			}
			if n != "" {
				notes = append(notes, n)
			}
		}
	}
	return notes, nil
}

func applyAreaFixAdd(nd *NetworkDef, confStore *conferences.Store, tag string) (string, error) {
	tag = strings.ToUpper(tag)
	name := areaFixDisplayName(tag)
	conf, err := EnsureEchoConference(confStore, name, nd.Name, tag)
	if err != nil {
		return "", fmt.Errorf("areafix %s: %w", tag, err)
	}
	if err := saveAreaMapping(nd.Name, tag, conf.ID, false); err != nil {
		return "", fmt.Errorf("areafix %s: save mapping: %w", tag, err)
	}
	return fmt.Sprintf("areafix: echo area %s → conference %q (id %d)", tag, conf.Name, conf.ID), nil
}

func applyAreaFixRemove(nd *NetworkDef, tag string) (string, error) {
	tag = strings.ToUpper(tag)
	if err := saveAreaMapping(nd.Name, tag, 0, true); err != nil {
		return "", fmt.Errorf("areafix -%s: save mapping: %w", tag, err)
	}
	return fmt.Sprintf("areafix: removed mapping for %s", tag), nil
}

// ProcessFileFixResponse applies an uplink FileFix reply: auto-creates file
// directories and updates file_areas when a saver is registered.
func ProcessFileFixResponse(nd *NetworkDef, fileArea FileArea, pm *Message) ([]string, error) {
	if fileArea == nil {
		return nil, fmt.Errorf("filefix response: file area store required")
	}
	if !IsFileFixResponse(pm.FromName) {
		return nil, nil
	}
	if !nd.IsUplinkFixRobot(pm.OrigAddr) {
		return nil, nil
	}
	RecordFileFixRecv(nd.Name, "uplink", pm.OrigAddr.String())

	var notes []string
	for _, act := range ParseAreaFixResponseBody(pm.Body) {
		if act.Add {
			n, err := applyFileFixAdd(nd, fileArea, act.Tag)
			if err != nil {
				return notes, err
			}
			if n != "" {
				notes = append(notes, n)
			}
		} else {
			n, err := applyFileFixRemove(nd, act.Tag)
			if err != nil {
				return notes, err
			}
			if n != "" {
				notes = append(notes, n)
			}
		}
	}
	return notes, nil
}

func applyFileFixAdd(nd *NetworkDef, fileArea FileArea, tag string) (string, error) {
	tag = strings.ToUpper(tag)
	name := areaFixDisplayName(tag)
	dirID, _, err := fileArea.EnsureDir(name, nd.Name+" file echo "+tag+" (auto-created)")
	if err != nil {
		return "", fmt.Errorf("filefix %s: %w", tag, err)
	}
	if err := saveFileAreaMapping(nd.Name, tag, dirID, false); err != nil {
		return "", fmt.Errorf("filefix %s: save mapping: %w", tag, err)
	}
	return fmt.Sprintf("filefix: file area %s → directory %q (id %d)", tag, name, dirID), nil
}

func applyFileFixRemove(nd *NetworkDef, tag string) (string, error) {
	tag = strings.ToUpper(tag)
	if err := saveFileAreaMapping(nd.Name, tag, 0, true); err != nil {
		return "", fmt.Errorf("filefix -%s: save mapping: %w", tag, err)
	}
	return fmt.Sprintf("filefix: removed mapping for %s", tag), nil
}
