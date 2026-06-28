package fido

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestBinkpOutboundFilesForUplinkIncludesAllOUTSubdirs(t *testing.T) {
	dir := t.TempDir()
	uplink, _ := ParseAddr("227:1/1")
	indirect, _ := ParseAddr("1:153/150")

	uplinkSub := filepath.Join(dir, outboundSubdirName(uplink))
	indirectSub := filepath.Join(dir, outboundSubdirName(indirect))
	for _, sub := range []string{uplinkSub, indirectSub} {
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(uplinkSub, "uplink.pkt"), []byte("u"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(indirectSub, "indirect.pkt"), []byte("i"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "root.pkt"), []byte("r"), 0644); err != nil {
		t.Fatal(err)
	}

	nd := &NetworkDef{OutboundDir: dir, Uplink: uplink.String()}
	got := binkpOutboundFilesFor(nd, nil, uplink)
	if len(got) != 3 {
		t.Fatalf("got %d files, want 3: %v", len(got), got)
	}
}

func TestBinkpOutboundFilesForDownlinkOnlyOwnSubdir(t *testing.T) {
	dir := t.TempDir()
	peer, _ := ParseAddr("300:1/5")
	other, _ := ParseAddr("300:1/6")

	peerSub := filepath.Join(dir, outboundSubdirName(peer))
	otherSub := filepath.Join(dir, outboundSubdirName(other))
	for _, sub := range []string{peerSub, otherSub} {
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(peerSub, "peer.pkt"), []byte("p"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherSub, "other.pkt"), []byte("o"), 0644); err != nil {
		t.Fatal(err)
	}

	nd := &NetworkDef{OutboundDir: dir}
	dl := &Downlink{Address: peer.String()}
	got := binkpOutboundFilesFor(nd, dl, peer)
	if len(got) != 1 || filepath.Base(got[0]) != "peer.pkt" {
		t.Fatalf("got %v, want only peer.pkt", got)
	}
}

func outboundSubdirName(a Addr) string {
	return fmt.Sprintf("%04X%04X.OUT", a.Zone*0x100+a.Net, a.Node)
}
