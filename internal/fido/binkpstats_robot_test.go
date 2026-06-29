package fido

import (
	"testing"
	"time"

	"github.com/virtbbs/virtbbs/internal/db"
	"github.com/virtbbs/virtbbs/internal/messages"
)

func TestRobotStats_recordAndQuery(t *testing.T) {
	sqlDB, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := messages.Open(sqlDB); err != nil {
		t.Fatal(err)
	}
	InitBinkpStats(sqlDB)

	RecordAreaFixSent("FidoNet", "uplink", "1:2/3")
	RecordAreaFixRecv("FidoNet", "downlink", "1:4/5")
	RecordFileFixSent("FidoNet", "uplink", "1:2/3")
	RecordTICSent("FidoNet", "downlink", "1:4/5", 1024*1024)

	res, err := QueryBinkpStats(sqlDB, "FidoNet", "day", statsPeriodKey("day", testNow()))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Networks) != 1 {
		t.Fatalf("networks = %d", len(res.Networks))
	}
	n := res.Networks[0]
	if n.AreaFixSent != 1 || n.AreaFixRecv != 1 || n.FileFixSent != 1 || n.TICSent != 1 {
		t.Fatalf("network stats: %+v", n)
	}
	if n.TICBytesSent < 1024*1024 {
		t.Fatalf("tic bytes = %d", n.TICBytesSent)
	}
}

func testNow() time.Time {
	return time.Now()
}
