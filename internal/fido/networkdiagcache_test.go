package fido

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNetworkDiagramCache_writeAndList(t *testing.T) {
	dir := t.TempDir()
	prefix := NetworkDiagPrefix("Fido Net")
	pngs := map[string][]byte{
		prefix + "_Full.png":  []byte("full-png"),
		prefix + "_Hubs.png":  []byte("hubs-png"),
		"Hub_1-105.png":       []byte("hub-png"),
	}
	if err := WriteNetworkDiagramCache(dir, "Fido Net", 1, pngs); err != nil {
		t.Fatal(err)
	}
	url := DiagramCacheWebURL("Fido Net", "full")
	if url != "/static/network-maps/Fido_Net/full.png" {
		t.Fatalf("url = %q", url)
	}
	entries := ListNetworkDiagramCache(dir, "Fido Net", 1)
	if len(entries) != 3 {
		t.Fatalf("entries = %d", len(entries))
	}
	if entries[0].Key != "full" || entries[1].Key != "hubs" {
		t.Fatalf("order: %+v", entries)
	}
	path := filepath.Join(NetworkDiagramCacheDir(dir), prefix, "full.png")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "full-png" {
		t.Fatalf("cached data = %q", data)
	}
}
