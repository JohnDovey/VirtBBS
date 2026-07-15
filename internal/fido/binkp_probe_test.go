package fido

import (
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"
	"time"
)

func TestIsBinkpProbeError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{io.EOF, true},
		{io.ErrUnexpectedEOF, true},
		{syscall.ECONNRESET, true},
		{fmt.Errorf("read tcp 1.2.3.4:24554->1.2.3.4:1: read: connection reset by peer"), true},
		{fmt.Errorf("write: broken pipe"), true},
		{&timeoutError{}, true},
		{fmt.Errorf("remote M_ERR during handshake: nope"), false},
		{fmt.Errorf("remote busy"), false},
		{errors.New("authentication failed: bad"), false},
	}
	for _, tc := range cases {
		if got := isBinkpProbeError(tc.err); got != tc.want {
			t.Errorf("isBinkpProbeError(%v)=%v want %v", tc.err, got, tc.want)
		}
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestNoteBinkpProbe_rateLimits(t *testing.T) {
	binkpProbeMu.Lock()
	binkpProbeWindow = time.Time{}
	binkpProbeByHost = map[string]int{}
	binkpProbeTotal = 0
	binkpProbeMu.Unlock()

	addr := &net.TCPAddr{IP: net.ParseIP("192.168.0.125"), Port: 12345}
	for i := 0; i < 5; i++ {
		noteBinkpProbe(addr, io.EOF)
	}

	binkpProbeMu.Lock()
	defer binkpProbeMu.Unlock()
	if binkpProbeTotal != 5 {
		t.Fatalf("total=%d want 5", binkpProbeTotal)
	}
	if binkpProbeByHost["192.168.0.125"] != 5 {
		t.Fatalf("by host=%v", binkpProbeByHost)
	}
}

func TestRemoteHost(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("192.168.0.125"), Port: 49613}
	if got := remoteHost(addr); got != "192.168.0.125" {
		t.Fatalf("got %q", got)
	}
}
