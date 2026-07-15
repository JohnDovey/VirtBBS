package fido

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestSendFile_MGETResume(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	dir := t.TempDir()
	src := filepath.Join(dir, "big.bin")
	payload := []byte("ABCDEFGHIJKLMNOP") // 16 bytes
	if err := os.WriteFile(src, payload, 0644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(src)
	mtime := info.ModTime().Unix()

	bp := &binkpConn{conn: client}
	errCh := make(chan error, 1)
	go func() {
		errCh <- bp.sendFile(src)
	}()

	// Peer: accept M_FILE offset 0, then request resume at 8, then GOT.
	isCmd, cmd, arg, err := readPipeFrame(t, server)
	if err != nil || !isCmd || cmd != bpM_FILE {
		t.Fatalf("first M_FILE: %v %v %q", isCmd, cmd, arg)
	}
	mustWriteCmd(t, server, bpM_GET, fmt.Sprintf("%s 16 %d 8", filepath.Base(src), mtime))

	isCmd, cmd, arg, err = readPipeFrame(t, server)
	if err != nil || !isCmd || cmd != bpM_FILE {
		t.Fatalf("resume M_FILE: %v %v %q", isCmd, cmd, arg)
	}
	_, _, _, off, ok := parseBinkpFileArg(string(arg))
	if !ok || off != 8 {
		t.Fatalf("resume offset: arg=%q", arg)
	}
	// Drain data frames until we have 8 bytes remaining.
	got := 0
	for got < 8 {
		isCmd, cmd, payload, err := readPipeFrame(t, server)
		if err != nil {
			t.Fatal(err)
		}
		if isCmd {
			t.Fatalf("unexpected cmd %d during data", cmd)
		}
		got += len(payload)
	}
	mustWriteCmd(t, server, bpM_GOT, fmt.Sprintf("%s 16 %d", filepath.Base(src), mtime))

	if err := <-errCh; err != nil {
		t.Fatalf("sendFile: %v", err)
	}
}

func TestWriteWaZooREQ_roundTrip(t *testing.T) {
	dir := t.TempDir()
	dest, _ := ParseAddr("1:2/3")
	path, err := WriteWaZooREQ(dir, dest, []string{"readme.txt", "game.zip"})
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := ParseREQFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 2 || cmds[0] != "readme.txt" {
		t.Fatalf("cmds=%v", cmds)
	}
}

func TestSplitFreqFilePassword(t *testing.T) {
	n, p := splitFreqFilePassword("secret.zip !hunter2")
	if n != "secret.zip" || p != "hunter2" {
		t.Fatalf("bark form: %q %q", n, p)
	}
	n, p = splitFreqFilePassword("secret.zip hunter2")
	if n != "secret.zip" || p != "hunter2" {
		t.Fatalf("space form: %q %q", n, p)
	}
}

func readPipeFrame(t *testing.T, r io.Reader) (isCmd bool, cmd byte, payload []byte, err error) {
	t.Helper()
	var hdr uint16
	if err := binary.Read(r, binary.BigEndian, &hdr); err != nil {
		return false, 0, nil, err
	}
	isCmd = hdr&0x8000 != 0
	n := int(hdr & 0x7fff)
	payload = make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return false, 0, nil, err
	}
	if isCmd {
		if len(payload) == 0 {
			return true, 0, nil, nil
		}
		return true, payload[0], payload[1:], nil
	}
	return false, 0, payload, nil
}
