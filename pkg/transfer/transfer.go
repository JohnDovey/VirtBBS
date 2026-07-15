// Package transfer re-exports VirtBBS Zmodem helpers for external doors.
package transfer

import (
	"io"

	internalxfer "github.com/virtbbs/virtbbs/internal/transfer"
)

// SendFile transmits path to the remote client over rw using Zmodem.
func SendFile(rw io.ReadWriter, path string) error {
	return internalxfer.SendFile(rw, path)
}

// ReceiveFile receives a Zmodem upload into destDir and returns the saved path.
func ReceiveFile(rw io.ReadWriter, destDir string) (string, error) {
	return internalxfer.ReceiveFile(rw, destDir)
}
