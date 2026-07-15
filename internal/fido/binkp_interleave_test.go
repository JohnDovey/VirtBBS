package fido

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// TestWaitForGOT_interleavedInbound verifies binkd-style M_FILE batches arriving
// while we wait for M_GOT on our outbound file are received and acknowledged.
func TestWaitForGOT_interleavedInbound(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	destDir := t.TempDir()
	bp := &binkpConn{conn: client, inboundDir: destDir}

	// Drain responses VirtBBS sends (M_GOT for inbound file) so the pipe cannot deadlock.
	go io.Copy(io.Discard, server)

	done := make(chan error, 1)
	go func() {
		done <- bp.waitForGOT("out.pkt", 4, 1700000001)
	}()

	// Remote sends its own file while we wait for GOT on out.pkt.
	mustWriteCmd(t, server, bpM_FILE, "in.pkt 5 1700000000 0")
	mustWriteData(t, server, []byte("hello"))
	mustWriteCmd(t, server, bpM_GOT, "out.pkt 4 1700000001")

	if err := <-done; err != nil {
		t.Fatalf("waitForGOT: %v", err)
	}
	if len(bp.earlyReceived) != 1 || bp.earlyReceived[0] != "in.pkt" {
		t.Fatalf("earlyReceived = %v", bp.earlyReceived)
	}
	if _, err := osReadFile(destDir, "in.pkt"); err != nil {
		t.Fatalf("inbound file: %v", err)
	}
}

func mustWriteCmd(t *testing.T, w io.Writer, cmd byte, arg string) {
	t.Helper()
	data := append([]byte{cmd}, []byte(arg)...)
	hdr := uint16(0x8000) | uint16(len(data))
	if err := binary.Write(w, binary.BigEndian, hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
}

func mustWriteData(t *testing.T, w io.Writer, data []byte) {
	t.Helper()
	hdr := uint16(len(data))
	if err := binary.Write(w, binary.BigEndian, hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
}

func osReadFile(dir, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, name))
}
