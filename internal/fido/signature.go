package fido

import (
	"fmt"
	"strings"

	"github.com/virtbbs/virtbbs/internal/version"
)

const defaultOutboundBBSName = "VirtBBS"

var outboundBBSName = defaultOutboundBBSName

// SetOutboundBBSName sets the BBS name appended to outbound Fido message
// signatures (tear line and Origin). Call from main after config load.
func SetOutboundBBSName(name string) {
	if s := strings.TrimSpace(name); s != "" {
		outboundBBSName = s
	} else {
		outboundBBSName = defaultOutboundBBSName
	}
}

func resolveOutboundBBSName(override string) string {
	if s := strings.TrimSpace(override); s != "" {
		return s
	}
	return outboundBBSName
}

// OutboundTearLine returns the standard VirtBBS tear line for outbound messages.
func OutboundTearLine() string {
	return fmt.Sprintf("--- VirtBBS %s", version.Version)
}

// OutboundOriginLine returns the FTS Origin line for outbound messages.
func OutboundOriginLine(bbsName string, orig Addr) string {
	return fmt.Sprintf(" * Origin: %s (%s)", resolveOutboundBBSName(bbsName), orig.String())
}

// OutboundSignatureLines returns tear line and Origin for outbound messages.
func OutboundSignatureLines(bbsName string, orig Addr) string {
	return OutboundTearLine() + "\r" + OutboundOriginLine(bbsName, orig) + "\r"
}

// AppendOutboundSignature appends the VirtBBS tear line and Origin if not
// already present in body.
func AppendOutboundSignature(body, bbsName string, orig Addr) string {
	tear := OutboundTearLine()
	if strings.Contains(body, tear) {
		return body
	}
	if body != "" && !strings.HasSuffix(body, "\r") {
		body += "\r"
	}
	return body + OutboundSignatureLines(bbsName, orig)
}
