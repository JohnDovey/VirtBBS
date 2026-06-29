package fido

import (
	"testing"

	"github.com/virtbbs/virtbbs/internal/db"
)

func TestTaglineDB_importAndList(t *testing.T) {
	sqlDB, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := MigrateTaglines(sqlDB); err != nil {
		t.Fatal(err)
	}
	tdb := OpenTaglineDB(sqlDB)
	added, err := tdb.ImportLines([]string{`"Hello"`, "Hello", "World"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if added != 2 {
		t.Fatalf("added = %d", added)
	}
	rows, err := tdb.ListAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if len(tdb.EnabledTexts()) != 2 {
		t.Fatal("enabled texts")
	}
}
