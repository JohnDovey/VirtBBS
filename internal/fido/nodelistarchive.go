package fido

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const nodelistArchiveDizName = "FILE_ID.DIZ"

// NodelistFileIDDiz returns the FILE_ID.DIZ description for a generated
// NODELIST.Z## or NODEDIFF.Z## archive.
func NodelistFileIDDiz(network, bbsName, bbsAddr, innerFilename string, isDiff bool, when time.Time) string {
	kind := "Nodelist"
	if isDiff {
		kind = "Nodelist diff"
	}
	if bbsName == "" {
		bbsName = "VirtBBS"
	}
	if bbsAddr == "" {
		bbsAddr = "unknown"
	}
	_, week := when.ISOWeek()
	return fmt.Sprintf("%s for Network %s, for week %02d and day %03d, generated on %s on %s (%s)",
		kind, network, week, when.YearDay(), when.Format("2006-01-02 15:04:05"), bbsName, bbsAddr)
}

// BuildNodelistZip builds a ZIP archive containing innerFilename, its plain
// nodelist/diff payload, and FILE_ID.DIZ — the usual Fido distribution shape.
func BuildNodelistZip(innerFilename string, payload []byte, diz string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(innerFilename)
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(payload); err != nil {
		return nil, err
	}
	d, err := zw.Create(nodelistArchiveDizName)
	if err != nil {
		return nil, err
	}
	if _, err := d.Write([]byte(diz)); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeNodelistZArchive writes a Fido-style Z## ZIP (outer name = innerFilename)
// into dir, with FILE_ID.DIZ describing the archive.
func writeNodelistZArchive(dir, innerFilename string, payload []byte, nd *NetworkDef, bbsName string, isDiff bool, when time.Time) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	addr := nd.Address
	if addr == "" {
		addr = nd.NodeAddr().String()
	}
	diz := NodelistFileIDDiz(nd.Name, bbsName, addr, innerFilename, isDiff, when)
	zipData, err := BuildNodelistZip(innerFilename, payload, diz)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, innerFilename), zipData, 0644)
}
