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

// buildDOT renders nodelist entries into Graphviz DOT text using the same
// hierarchy rules as node2dot.py (Fernando Toledo / FidoNet zone maps).
func buildDOT(network string, zoneAddr Addr, zoneName, zoneSysop string, nodes []NodeEntry, scope DiagramScope, onlyNet int) string {
	zone := zoneAddr.Zone
	entries := filterDiagramZone(nodes, zone)
	if scope == DiagramPerHub && onlyNet != 0 {
		entries = filterEntriesPerHub(entries, zone, onlyNet)
	}
	entries = orderDiagramEntries(entries, zone)

	g := newDotGraph(network, zone, zoneName, zoneSysop)
	for i := range entries {
		g.addEntry(&entries[i], scope)
	}
	g.ensureZoneNode()
	return g.render()
}

type dotGraph struct {
	digraphName   string
	zone          int
	zoneID        string
	zoneName      string
	zoneSysop     string
	currentRegion string
	currentHost   string
	nodes         map[string]dotNode
	edges         []dotEdge
}

type dotNode struct {
	label    string
	fillType string // zone, region, host, pvt, hold
}

type dotEdge struct {
	from, to string
}

func newDotGraph(network string, zone int, zoneName, zoneSysop string) *dotGraph {
	return &dotGraph{
		digraphName: sanitizeNetworkFileToken(network),
		zone:        zone,
		zoneID:      fmt.Sprintf("%d", zone),
		zoneName:    zoneName,
		zoneSysop:   zoneSysop,
		nodes:       map[string]dotNode{},
	}
}

func (g *dotGraph) addEntry(e *NodeEntry, scope DiagramScope) {
	switch e.Type {
	case "Zone":
		if e.Zone != g.zone {
			return
		}
		sysop := dotLabelText(e.Sysop)
		if sysop == "" {
			sysop = dotLabelText(g.zoneSysop)
		}
		g.nodes[g.zoneID] = dotNode{
			label:    fmt.Sprintf("Zone %d\\n(%s)", g.zone, dotEscape(sysop)),
			fillType: "zone",
		}
	case "Region":
		if e.Zone != g.zone {
			return
		}
		g.currentRegion = fmt.Sprintf("%d:%d", e.Zone, e.Net)
		g.currentHost = ""
		name := dotLabelText(e.Name)
		if name == "" {
			name = dotLabelText(e.Location)
		}
		g.nodes[g.currentRegion] = dotNode{
			label:    fmt.Sprintf("%s\\n(%s)", g.currentRegion, dotEscape(name)),
			fillType: "region",
		}
		g.edges = append(g.edges, dotEdge{g.zoneID, g.currentRegion})
	case "Host":
		if e.Zone != g.zone {
			return
		}
		g.currentHost = fmt.Sprintf("%d:%d", e.Zone, e.Net)
		name := dotLabelText(e.Name)
		sysop := dotLabelText(e.Sysop)
		g.nodes[g.currentHost] = dotNode{
			label:    fmt.Sprintf("%s\\n(%s)\\n(%s)", g.currentHost, dotEscape(name), dotEscape(sysop)),
			fillType: "host",
		}
		g.edges = append(g.edges, g.hostEdge(g.currentHost)...)
	case "Hub", "Node", "Pvt", "Boss", "":
		if scope == DiagramHubsOnly {
			return
		}
		g.addMemberNode(e, "pvt")
	case "Hold", "Down":
		if scope == DiagramHubsOnly {
			return
		}
		g.addMemberNode(e, "hold")
	}
}

func (g *dotGraph) hostEdge(hostID string) []dotEdge {
	if g.currentRegion != "" {
		hubID := g.currentRegion + "/1"
		if _, ok := g.nodes[hubID]; ok {
			return []dotEdge{{hubID, hostID}}
		}
		return []dotEdge{{g.currentRegion, hostID}}
	}
	return []dotEdge{{g.zoneID, hostID}}
}

func (g *dotGraph) addMemberNode(e *NodeEntry, fillType string) {
	if e.Zone != g.zone || e.Node == 0 {
		return
	}
	parent := g.currentHost
	if parent == "" {
		parent = g.currentRegion
	}
	if parent == "" {
		parent = g.zoneID
	}
	nodeID := fmt.Sprintf("%s/%d", parent, e.Node)
	name := dotLabelText(e.Name)
	sysop := dotLabelText(e.Sysop)
	label := nodeID + "\\n" + dotEscape(name)
	if sysop != "" {
		label += "\\n(" + dotEscape(sysop) + ")"
	}
	if e.Type == "Hold" || e.Type == "Down" {
		fillType = "hold"
	}
	g.nodes[nodeID] = dotNode{label: label, fillType: fillType}
	g.edges = append(g.edges, dotEdge{parent, nodeID})
}

func (g *dotGraph) ensureZoneNode() {
	if g.zone == 0 {
		return
	}
	if _, ok := g.nodes[g.zoneID]; ok {
		return
	}
	sysop := dotLabelText(g.zoneSysop)
	g.nodes[g.zoneID] = dotNode{
		label:    fmt.Sprintf("Zone %d\\n(%s)", g.zone, dotEscape(sysop)),
		fillType: "zone",
	}
}

func (g *dotGraph) render() string {
	colorMap := map[string]string{
		"zone":   "lightblue",
		"region": "palegreen",
		"host":   "lightpink",
		"pvt":    "lightyellow",
		"hold":   "lightcoral",
	}
	var b strings.Builder
	fmt.Fprintf(&b, "digraph %s {\r\n", dotEscape(g.digraphName))
	b.WriteString("node [shape=tab, style=filled]\r\n")
	b.WriteString("rankdir=LR\r\n\r\n")

	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		n := g.nodes[id]
		fill := colorMap[n.fillType]
		if fill == "" {
			fill = "lightyellow"
		}
		fmt.Fprintf(&b, "\"%s\" [label=\"%s\", fillcolor=%s]\r\n", dotEscape(id), n.label, fill)
	}
	for _, e := range g.edges {
		fmt.Fprintf(&b, "\"%s\" -> \"%s\"\r\n", dotEscape(e.from), dotEscape(e.to))
	}
	b.WriteString("}\r\n")
	return b.String()
}

func dotLabelText(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "_", " ")
}

// orderDiagramEntries orders nodes like node2dot.py: Zone, then per Region block
// (region → nodes on that net → host nets → nodes on host nets). Without Region
// lines (VirtNet), uses Zone → Host → members per net.
func orderDiagramEntries(entries []NodeEntry, zone int) []NodeEntry {
	var zoneEntry *NodeEntry
	var regions []NodeEntry
	hosts := map[int]NodeEntry{}
	var members []NodeEntry
	for i := range entries {
		e := &entries[i]
		if e.Type != "Zone" && e.Zone != zone {
			continue
		}
		switch e.Type {
		case "Zone":
			zoneEntry = e
		case "Region":
			regions = append(regions, *e)
		case "Host":
			hosts[e.Net] = *e
		default:
			if e.Node != 0 {
				members = append(members, *e)
			}
		}
	}
	sort.Slice(regions, func(i, j int) bool { return regions[i].Net < regions[j].Net })

	var out []NodeEntry
	if zoneEntry != nil {
		out = append(out, *zoneEntry)
	}
	if len(regions) == 0 {
		return orderDiagramEntriesNoRegion(out, hosts, members)
	}

	hostNets := make([]int, 0, len(hosts))
	for n := range hosts {
		hostNets = append(hostNets, n)
	}
	sort.Ints(hostNets)

	for ri, reg := range regions {
		out = append(out, reg)
		nextReg := 0
		if ri+1 < len(regions) {
			nextReg = regions[ri+1].Net
		}
		for _, m := range sortMembers(membersOnNet(members, reg.Net)) {
			out = append(out, m)
		}
		for _, hostNet := range hostNets {
			if hostNet <= reg.Net {
				continue
			}
			if nextReg != 0 && hostNet >= nextReg {
				continue
			}
			if h, ok := hosts[hostNet]; ok {
				out = append(out, h)
				for _, m := range sortMembers(membersOnNet(members, hostNet)) {
					out = append(out, m)
				}
			}
		}
	}
	return out
}

func orderDiagramEntriesNoRegion(out []NodeEntry, hosts map[int]NodeEntry, members []NodeEntry) []NodeEntry {
	hostNets := make([]int, 0, len(hosts))
	for n := range hosts {
		hostNets = append(hostNets, n)
	}
	sort.Ints(hostNets)
	if len(hostNets) == 0 {
		for _, m := range sortMembers(members) {
			out = append(out, m)
		}
		return out
	}
	for _, hostNet := range hostNets {
		out = append(out, hosts[hostNet])
		for _, m := range sortMembers(membersOnNet(members, hostNet)) {
			out = append(out, m)
		}
	}
	return out
}

func membersOnNet(members []NodeEntry, net int) []NodeEntry {
	var out []NodeEntry
	for _, m := range members {
		if m.Net == net {
			out = append(out, m)
		}
	}
	return out
}

func sortMembers(members []NodeEntry) []NodeEntry {
	sort.Slice(members, func(i, j int) bool {
		if members[i].Net != members[j].Net {
			return members[i].Net < members[j].Net
		}
		return members[i].Node < members[j].Node
	})
	return members
}

func dotEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}

func filterDiagramZone(nodes []NodeEntry, zone int) []NodeEntry {
	if zone == 0 {
		return append([]NodeEntry(nil), nodes...)
	}
	out := make([]NodeEntry, 0, len(nodes))
	for _, n := range nodes {
		if n.Type == "Zone" || n.Zone == zone {
			out = append(out, n)
		}
	}
	return out
}

func filterEntriesPerHub(entries []NodeEntry, zone, onlyNet int) []NodeEntry {
	regionNets := map[int]bool{}
	for _, e := range entries {
		if e.Zone == zone && e.Type == "Region" {
			regionNets[e.Net] = true
		}
	}
	out := make([]NodeEntry, 0, len(entries))
	for _, e := range entries {
		switch e.Type {
		case "Zone":
			out = append(out, e)
		case "Region":
			if e.Zone == zone {
				out = append(out, e)
			}
		case "Host":
			if e.Zone == zone && e.Net == onlyNet {
				out = append(out, e)
			}
		default:
			if e.Zone != zone {
				continue
			}
			if e.Net == onlyNet {
				out = append(out, e)
			} else if e.Node == 1 && regionNets[e.Net] {
				out = append(out, e)
			}
		}
	}
	return out
}

func netsForPerHubDiagrams(entries []NodeEntry, zone int) []int {
	seen := map[int]bool{}
	var nets []int
	for _, e := range entries {
		if e.Zone != zone {
			continue
		}
		if e.Type == "Zone" {
			continue
		}
		if e.Type == "Host" && e.Node == 0 {
			if !seen[e.Net] {
				seen[e.Net] = true
				nets = append(nets, e.Net)
			}
			continue
		}
		if e.Type == "Region" {
			if !seen[e.Net] {
				seen[e.Net] = true
				nets = append(nets, e.Net)
			}
			continue
		}
		if e.Node != 0 && !seen[e.Net] {
			seen[e.Net] = true
			nets = append(nets, e.Net)
		}
	}
	sort.Ints(nets)
	return nets
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

// RenderPNG renders DOT source to outPath as PNG using Graphviz dot — bundled
// graphviz/bin/dot next to virtbbs, paths.graphviz_dot, or PATH.
func RenderPNG(dot, outPath string) error {
	dotBin, err := ResolveDotBinary()
	if err != nil {
		return err
	}
	if err := renderPNGWithDot(dotBin, dot, outPath); err != nil {
		if bundledDotNeedsPlugins(err) && graphvizBundleRoot(dotBin) != "" {
			if sys, lookErr := exec.LookPath("dot"); lookErr == nil && sys != dotBin {
				if retryErr := renderPNGWithDot(sys, dot, outPath); retryErr == nil {
					return nil
				}
			}
		}
		return err
	}
	return nil
}

func bundledDotNeedsPlugins(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "not recognized") || strings.Contains(msg, "No formats found")
}

func renderPNGWithDot(dotBin, dot, outPath string) error {
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

	cmd := prepareDotCmd(dotBin, "-Tpng", tmp.Name(), "-o", outPath)
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

	render(prefix+"_Full.png", buildDOT(network, zoneAddr, zoneName, zoneSysop, nodes, DiagramFull, 0))
	render(prefix+"_Hubs.png", buildDOT(network, zoneAddr, zoneName, zoneSysop, nodes, DiagramHubsOnly, 0))
	for _, net := range netsForPerHubDiagrams(nodes, zoneAddr.Zone) {
		name := fmt.Sprintf("Hub_%d-%d.png", zoneAddr.Zone, net)
		render(name, buildDOT(network, zoneAddr, zoneName, zoneSysop, nodes, DiagramPerHub, net))
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
