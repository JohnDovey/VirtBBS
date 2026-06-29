package fido

import (
	"strings"
	"testing"
)

func TestBuildDOT_node2dotHierarchy(t *testing.T) {
	nodes := []NodeEntry{
		{Zone: 4, Net: 4, Node: 0, Name: "Z4", Sysop: "Fernando_Toledo", Type: "Zone"},
		{Zone: 4, Net: 80, Node: 0, Name: "Rede_Brasil", Type: "Region"},
		{Zone: 4, Net: 80, Node: 1, Name: "Internet_HUB", Sysop: "Flavio_Bessa", Type: "Hub"},
		{Zone: 4, Net: 801, Node: 0, Name: "Rede_Brasileira", Sysop: "Flavio_Bessa", Type: "Host"},
		{Zone: 4, Net: 801, Node: 10, Name: "SoftSolutions_BBS", Sysop: "Alexandre_Figueiredo", Type: "Node"},
		{Zone: 4, Net: 801, Node: 204, Name: "Micronet_BBS", Sysop: "Luiz_Pacheco", Type: "Hold"},
	}

	dot := buildDOT("FidoNet", Addr{Zone: 4}, "Z4", "Fernando Toledo", nodes, DiagramFull, 0)

	for _, want := range []string{
		"shape=tab",
		"rankdir=LR",
		`"4" [label="ZONA 4\n(Fernando Toledo)", fillcolor=lightblue]`,
		`"4:80" [label="4:80\n(Rede Brasil)", fillcolor=palegreen]`,
		`"4:80/1"`,
		`fillcolor=lightyellow`,
		`"4:801"`,
		`fillcolor=lightpink`,
		`"4:801/10"`,
		`"4:801/204"`,
		`fillcolor=lightcoral`,
		`"4" -> "4:80"`,
		`"4:80" -> "4:80/1"`,
		`"4:80/1" -> "4:801"`,
		`"4:801" -> "4:801/10"`,
		`"4:801" -> "4:801/204"`,
	} {
		if !strings.Contains(dot, want) {
			t.Fatalf("DOT missing %q\n---\n%s", want, dot)
		}
	}
}

func TestBuildDOT_virtNetWithoutRegion(t *testing.T) {
	nodes := []NodeEntry{
		{Zone: 300, Net: 300, Node: 0, Name: "VirtNet", Sysop: "Hub Sysop", Type: "Zone"},
		{Zone: 300, Net: 1, Node: 0, Name: "Net_One", Sysop: "NC", Type: "Host"},
		{Zone: 300, Net: 1, Node: 17, Name: "My_BBS", Sysop: "Alice", Type: "Node"},
	}

	dot := buildDOT("VirtNet", Addr{Zone: 300}, "VirtNet", "Hub Sysop", nodes, DiagramFull, 0)
	for _, want := range []string{
		`"300" -> "300:1"`,
		`"300:1" -> "300:1/17"`,
		`fillcolor=lightpink`,
	} {
		if !strings.Contains(dot, want) {
			t.Fatalf("DOT missing %q\n---\n%s", want, dot)
		}
	}
}

func TestBuildDOT_hubsOnlyOmitsMembers(t *testing.T) {
	nodes := []NodeEntry{
		{Zone: 4, Net: 4, Node: 0, Sysop: "Z Sysop", Type: "Zone"},
		{Zone: 4, Net: 80, Node: 0, Name: "Region", Type: "Region"},
		{Zone: 4, Net: 801, Node: 0, Name: "Host", Sysop: "H", Type: "Host"},
		{Zone: 4, Net: 801, Node: 10, Name: "Node", Sysop: "N", Type: "Node"},
	}
	dot := buildDOT("FidoNet", Addr{Zone: 4}, "", "", nodes, DiagramHubsOnly, 0)
	if strings.Contains(dot, "4:801/10") {
		t.Fatalf("hubs-only should omit members:\n%s", dot)
	}
	if !strings.Contains(dot, `"4:801"`) {
		t.Fatalf("hubs-only should keep host:\n%s", dot)
	}
}
