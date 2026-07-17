package nodelist

import (
	"strings"
	"testing"
)

const sample = `;A Fidonet Nodelist for Thursday, July 16, 2026 -- Day number 197 : 25761
;A comment
Zone,1,North_America,Toronto,Nick_Andre,1-647-847-2083,9600,CM,XX,INA:bbs.example.com,IBN
Region,10,Calif-Nevada,Aptos_CA,Kurt_Weiske,-Unpublished-,300,CM,XX
Host,102,SoCalNet,Los_Angeles_CA,Lee_Green,-Unpublished-,300,CM,XA
,100,Test_BBS,Los_Angeles_CA,Jane_Doe,-Unpublished-,300,CM,INA:test.example,IBN
Hub,500,Local_Hub,Laurel,Frank_Reid,1-757-481-6611,9600,CM,XA
Down,850,Beggar's_Canyon,Henderson_NV,John_Nicpon,-Unpublished-,300,CM
Pvt,900,Private_Node,Somewhere,Sysop_Name,-Unpublished-,300,CM
`

func TestParseHierarchy(t *testing.T) {
	doc, err := Parse(strings.NewReader(sample), "FidoNet")
	if err != nil {
		t.Fatal(err)
	}
	if doc.HeaderDay != 197 {
		t.Fatalf("HeaderDay=%d want 197", doc.HeaderDay)
	}
	if !strings.Contains(doc.HeaderDate, "July 16") {
		t.Fatalf("HeaderDate=%q", doc.HeaderDate)
	}
	if len(doc.Nodes) != 7 {
		t.Fatalf("nodes=%d want 7", len(doc.Nodes))
	}

	want := []struct {
		addr, role, name string
		zone, net, node  int
	}{
		{"1:0/0", "Zone", "North America", 1, 0, 0},
		{"1:10/0", "Region", "Calif-Nevada", 1, 10, 0},
		{"1:102/0", "Host", "SoCalNet", 1, 102, 0},
		{"1:102/100", "Node", "Test BBS", 1, 102, 100},
		{"1:102/500", "Hub", "Local Hub", 1, 102, 500},
		{"1:102/850", "Down", "Beggar's Canyon", 1, 102, 850},
		{"1:102/900", "Pvt", "Private Node", 1, 102, 900},
	}
	for i, w := range want {
		n := doc.Nodes[i]
		if n.NodeNo != w.addr || n.Role != w.role || n.BBSName != w.name {
			t.Errorf("[%d] got %s %s %q want %s %s %q", i, n.NodeNo, n.Role, n.BBSName, w.addr, w.role, w.name)
		}
		if n.Zone != w.zone || n.Net != w.net || n.Node != w.node {
			t.Errorf("[%d] z/n/n %d:%d/%d want %d:%d/%d", i, n.Zone, n.Net, n.Node, w.zone, w.net, w.node)
		}
		if n.Domain != "FidoNet" || n.NodeDay != 197 {
			t.Errorf("[%d] domain/day %s %d", i, n.Domain, n.NodeDay)
		}
	}
}

func TestArchiveName(t *testing.T) {
	cases := map[int]string{
		1:   "Z1DAILY.Z01",
		97:  "Z1DAILY.Z97",
		197: "Z1DAILY.Z97",
		200: "Z1DAILY.Z00",
		365: "Z1DAILY.Z65",
	}
	for day, want := range cases {
		if got := ArchiveName(day); got != want {
			t.Errorf("ArchiveName(%d)=%s want %s", day, got, want)
		}
	}
}

func TestJoinURL(t *testing.T) {
	base := "https://fido-z1.darkrealms.ca/$webfile.send.ZC1./"
	got, err := JoinURL(base, "Z1DAILY.Z97")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "$webfile.send.ZC1.") {
		t.Fatalf("lost special path segment: %s", got)
	}
	if !strings.HasSuffix(got, "Z1DAILY.Z97") {
		t.Fatalf("missing file: %s", got)
	}
}
