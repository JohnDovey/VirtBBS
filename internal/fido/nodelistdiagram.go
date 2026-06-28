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
//   v0.13.0 2026-06-27  VirtNet: Go rewrite of a Python FidoNet-nodelist-
//                        to-Graphviz-DOT converter (node2dot.py, see
//                        https://gist.github.com/ftoledo/8c17113d30e847a5e69f92bf0bbab82c),
//                        sourced from fido_members instead of a parsed
//                        nodelist file: full hierarchy, hubs-only topology,
//                        and one diagram per hub+its members, rendered to
//                        PNG via the external `dot` CLI (same as the source
//                        script — neither it nor Go has a built-in
//                        Graphviz renderer, so writing one from scratch
//                        isn't justified).
//   v1.6.1  2026-06-28  Diagram all networks from fido_nodes; zip as
//                        <Network>_diags.zip with matching PNG prefixes.
// ============================================================================

// Package fido — nodelistdiagram.go
package fido

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// DiagramScope selects which subset of the network hierarchy buildDOT draws.
type DiagramScope int

const (
	// DiagramFull draws the complete hierarchy: zone -> every net's host -> every member.
	DiagramFull DiagramScope = iota
	// DiagramHubsOnly draws zone -> every net's host, omitting member nodes (topology skeleton).
	DiagramHubsOnly
	// DiagramPerHub draws one net's host plus only its direct members.
	DiagramPerHub
)

// NetworkDiagZipName returns the registered zip filename for a network's diagrams.
func NetworkDiagZipName(network string) string {
	return NetworkDiagPrefix(network) + "_diags.zip"
}

// NetworkDiagPrefix returns the filename prefix used for diagram PNGs inside the zip.
func NetworkDiagPrefix(network string) string {
	return sanitizeNetworkFileToken(network)
}

func sanitizeNetworkFileToken(network string) string {
	s := strings.TrimSpace(network)
	if s == "" {
		return "Network"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-':
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "Network"
	}
	return b.String()
}

// buildDOT renders nodelist entries (grouped by net for DiagramPerHub) into
// Graphviz DOT text, mirroring node2dot.py's conventions.
func buildDOT(network string, zoneAddr Addr, zoneName, zoneSysop string, byNet map[int][]NodeEntry, scope DiagramScope, onlyNet int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "digraph %s {\r\n", dotEscape(sanitizeNetworkFileToken(network)))
	b.WriteString("  node [shape=box, style=filled];\r\n")

	zoneID := "zone"
	fmt.Fprintf(&b, "  %s [label=\"%d\\n%s\\n%s\", fillcolor=lightblue];\r\n",
		zoneID, zoneAddr.Zone, dotEscape(zoneName), dotEscape(zoneSysop))

	nets := sortedNetKeys(byNet)
	for _, net := range nets {
		if scope == DiagramPerHub && net != onlyNet {
			continue
		}
		entries := byNet[net]
		host := findNetHost(entries)

		hostID := fmt.Sprintf("host_%d", net)
		hostName, hostSysop := zoneName, zoneSysop
		hostNode := "1"
		if host != nil {
			hostName, hostSysop = host.Name, host.Sysop
			if host.Node != 0 {
				hostNode = fmt.Sprintf("%d", host.Node)
			}
		}
		fmt.Fprintf(&b, "  %s [label=\"%d:%d/%s\\n%s\\n%s\", fillcolor=palegreen];\r\n",
			hostID, zoneAddr.Zone, net, hostNode, dotEscape(hostName), dotEscape(hostSysop))
		fmt.Fprintf(&b, "  %s -> %s;\r\n", zoneID, hostID)

		if scope == DiagramHubsOnly {
			continue
		}
		for i, e := range entries {
			if !isDiagramMember(&e, host) {
				continue
			}
			memberID := fmt.Sprintf("m_%d_%d", net, i)
			fmt.Fprintf(&b, "  %s [label=\"%s\\n%s\\n%s\", fillcolor=lightpink];\r\n",
				memberID, dotEscape(e.Addr4D()), dotEscape(e.Name), dotEscape(e.Sysop))
			fmt.Fprintf(&b, "  %s -> %s;\r\n", hostID, memberID)
		}
	}

	b.WriteString("}\r\n")
	return b.String()
}

func dotEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}

func groupNodesByNet(nodes []NodeEntry) map[int][]NodeEntry {
	out := map[int][]NodeEntry{}
	for _, n := range nodes {
		if n.Type == "Zone" {
			continue
		}
		out[n.Net] = append(out[n.Net], n)
	}
	return out
}

func sortedNetKeys(byNet map[int][]NodeEntry) []int {
	nets := make([]int, 0, len(byNet))
	for n := range byNet {
		nets = append(nets, n)
	}
	sort.Ints(nets)
	return nets
}

func findNetHost(entries []NodeEntry) *NodeEntry {
	for i := range entries {
		e := &entries[i]
		if e.Node == 0 && (e.Type == "Host" || e.Type == "Region") {
			return e
		}
	}
	for i := range entries {
		e := &entries[i]
		if e.Type == "Host" {
			return e
		}
	}
	return nil
}

func isDiagramMember(e *NodeEntry, host *NodeEntry) bool {
	if host != nil && e.ID == host.ID {
		return false
	}
	if e.Node == 0 && (e.Type == "Host" || e.Type == "Region") {
		return false
	}
	return true
}

func zoneLabelForDiagram(nd *NetworkDef, nodes []NodeEntry, bbsName, sysopName string) (Addr, string, string) {
	zone := nd.NodeAddr().Zone
	if zone == 0 {
		for _, n := range nodes {
			if n.Zone != 0 {
				zone = n.Zone
				break
			}
		}
	}
	zName, zSysop := bbsName, sysopName
	if zName == "" {
		zName = "VirtBBS"
	}
	if zSysop == "" {
		zSysop = "Sysop"
	}
	for _, n := range nodes {
		if n.Zone == zone && n.Type == "Zone" && n.Node == 0 {
			zName, zSysop = n.Name, n.Sysop
			break
		}
	}
	return Addr{Zone: zone}, zName, zSysop
}

// RenderPNG writes dot to a temp .dot file and shells out to the `dot` CLI
// to render it to outPath as a PNG. If `dot` isn't found on PATH, returns
// a descriptive error rather than panicking — the caller should log it and
// skip that diagram, not fail the whole regeneration (see GenerateDiagramsFromNodes).
func RenderPNG(dot, outPath string) error {
	dotBin, err := exec.LookPath("dot")
	if err != nil {
		return fmt.Errorf("graphviz 'dot' not found on PATH: %w", err)
	}
	tmp, err := os.CreateTemp("", "virtnet-*.dot")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(dot); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	cmd := exec.Command(dotBin, "-Tpng", tmp.Name(), "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dot render failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// GenerateDiagramsFromNodes builds full, hubs-only, and per-net DOT graphs
// for network from fido_nodes rows, renders each to PNG, and returns a map
// of {filename: pngBytes} ready for writeMultiZipAndRegister.
func GenerateDiagramsFromNodes(network string, nd *NetworkDef, bbsName, sysopName string, nodes []NodeEntry) (map[string][]byte, []string) {
	byNet := groupNodesByNet(nodes)
	prefix := NetworkDiagPrefix(network)
	zoneAddr, zoneName, zoneSysop := zoneLabelForDiagram(nd, nodes, bbsName, sysopName)

	pngs := map[string][]byte{}
	var warnings []string

	render := func(name, dot string) {
		tmpOut, err := os.CreateTemp("", "virtnet-*.png")
		if err != nil {
			warnings = append(warnings, err.Error())
			return
		}
		defer os.Remove(tmpOut.Name())
		tmpOut.Close()
		if err := RenderPNG(dot, tmpOut.Name()); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", name, err))
			return
		}
		data, err := os.ReadFile(tmpOut.Name())
		if err != nil {
			warnings = append(warnings, err.Error())
			return
		}
		pngs[name] = data
	}

	render(prefix+"_Full.png", buildDOT(network, zoneAddr, zoneName, zoneSysop, byNet, DiagramFull, 0))
	render(prefix+"_Hubs.png", buildDOT(network, zoneAddr, zoneName, zoneSysop, byNet, DiagramHubsOnly, 0))
	for _, net := range sortedNetKeys(byNet) {
		name := fmt.Sprintf("Hub_%d-%d.png", zoneAddr.Zone, net)
		render(name, buildDOT(network, zoneAddr, zoneName, zoneSysop, byNet, DiagramPerHub, net))
	}

	return pngs, warnings
}

// GenerateDiagrams builds diagrams from hub member rows (legacy wrapper).
func GenerateDiagrams(zoneAddr Addr, hubBBSName, hubSysopName string, members []*Member) (map[string][]byte, []string) {
	network := "VirtNet"
	nodes := make([]NodeEntry, 0, len(members))
	for _, m := range members {
		if network == "VirtNet" && m.Network != "" {
			network = m.Network
		}
		t := "Node"
		if m.IsHost {
			t = "Host"
		}
		nodes = append(nodes, NodeEntry{
			ID: m.ID, Zone: m.Zone, Net: m.Net, Node: m.NodeNum, Point: m.Point,
			Name: m.BBSName, Sysop: m.SysopName, Type: t, Active: m.IsActive,
		})
	}
	nd := &NetworkDef{Name: network, Address: zoneAddr.String()}
	if nd.NodeAddr().Zone == 0 && zoneAddr.Zone != 0 {
		nd.Address = fmt.Sprintf("%d:0/0", zoneAddr.Zone)
	}
	return GenerateDiagramsFromNodes(network, nd, hubBBSName, hubSysopName, nodes)
}
