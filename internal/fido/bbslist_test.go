package fido

import (
	"database/sql"
	"testing"

	"github.com/virtbbs/virtbbs/internal/db"
	"github.com/virtbbs/virtbbs/internal/messages"
)

func TestBBSListRecordAndQuery(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := migrateBBSList(db); err != nil {
		t.Fatal(err)
	}

	our := Addr{Zone: 1, Net: 100, Node: 1}
	remote := Addr{Zone: 1, Net: 200, Node: 5, Point: 3}

	recordEnd(db, "FidoNet", our, remote, "Alice", true, false)
	recordEnd(db, "FidoNet", our, remote, "Alice", false, true)

	page, err := ListBBSNodesEchomail(db, 1, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Fatalf("echomail nodes: got %d want 1", page.Total)
	}
	if page.Nodes[0].EchomailCount != 1 {
		t.Fatalf("echomail count: got %d want 1", page.Nodes[0].EchomailCount)
	}

	nmPage, err := ListBBSNodesNetmail(db, 1, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if nmPage.Total != 1 || nmPage.Nodes[0].NetmailCount != 1 {
		t.Fatalf("netmail page: %+v", nmPage)
	}

	users, err := ListBBSUsersForNode(db, "FidoNet", remote.BossString())
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].EchomailCount != 1 || users[0].NetmailCount != 1 {
		t.Fatalf("users: %+v", users)
	}

	charts, err := QueryBBSListCharts(db, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !charts.HasData {
		t.Fatal("expected chart data")
	}

	// Our own node should not be recorded.
	recordEnd(db, "FidoNet", our, our, "Sysop", true, false)
	page2, _ := ListBBSNodesEchomail(db, 1, 10, "")
	if page2.Total != 1 {
		t.Fatalf("our node recorded: total=%d", page2.Total)
	}
}

func TestBBSListBackfillSingleConnection(t *testing.T) {
	sqlDB, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	if _, err := sqlDB.Exec(`CREATE TABLE messages (
		fido_network TEXT, fido_origin TEXT, from_name TEXT, echo INTEGER, conference_id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO messages (fido_network, fido_origin, from_name, echo, conference_id)
		VALUES ('FidoNet', '1:234/5', 'Bob', 1, 1),
		       ('FidoNet', '1:234/6', 'Carol', 0, 0)`); err != nil {
		t.Fatal(err)
	}

	if err := migrateBBSList(sqlDB); err != nil {
		t.Fatal(err)
	}

	page, err := ListBBSNodesEchomail(sqlDB, 1, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total < 1 {
		t.Fatalf("expected backfill nodes, got %+v", page)
	}
}

func TestBBSListNetworkInferenceAndRepair(t *testing.T) {
	sqlDB, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := messages.Open(sqlDB); err != nil {
		t.Fatal(err)
	}

	addr, _ := ParseAddr("227:1/1")
	_, err = sqlDB.Exec(`INSERT INTO fido_nodes
		(network, zone, net, node_num, point, name, location, sysop, phone, baud, flags, node_type, is_active)
		VALUES ('LovlyNet', ?, ?, ?, 0, 'Lovely BBS', 'Somewhere', 'John', '', 33600, 'CM,IBN', 'Node', 1)`,
		addr.Zone, addr.Net, addr.Node)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrateBBSList(sqlDB); err != nil {
		t.Fatal(err)
	}
	_, err = sqlDB.Exec(`INSERT INTO fido_bbs_nodes
		(network, node_addr, zone, net, node_num, echomail_count, netmail_count, last_seen)
		VALUES ('FidoNet', '227:1/1', ?, ?, ?, 0, 5, '2026-01-01T00:00:00Z')`,
		addr.Zone, addr.Net, addr.Node)
	if err != nil {
		t.Fatal(err)
	}

	networks := []NetworkDef{
		{Name: "FidoNet", Address: "1:100/1"},
		{Name: "LovlyNet", Address: "227:1/1"},
	}
	InitBBSList(sqlDB, networks)

	var network string
	if err := sqlDB.QueryRow(`SELECT network FROM fido_bbs_nodes WHERE node_addr='227:1/1'`).Scan(&network); err != nil {
		t.Fatal(err)
	}
	if network != "LovlyNet" {
		t.Fatalf("network after repair: got %q want LovlyNet", network)
	}

	ndb := OpenNodelistDB(sqlDB)
	node := &BBSListNode{Network: "FidoNet", NodeAddr: "227:1/1"}
	enrichBBSNode(ndb, node)
	if node.Network != "LovlyNet" || node.Name != "Lovely BBS" || node.Sysop != "John" {
		t.Fatalf("enriched node: %+v", node)
	}
}
