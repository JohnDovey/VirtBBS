package fido

import (
	"strings"
	"testing"
)

func TestIsRobotNetmailMessage(t *testing.T) {
	pm := &Message{FromName: AreaFixRobotName, Subject: "AreaFix response"}
	if !IsRobotNetmailMessage(pm) {
		t.Fatal("expected AreaFix response")
	}
	pm = &Message{FromName: "Sysop", Subject: "Hello"}
	if IsRobotNetmailMessage(pm) {
		t.Fatal("ordinary netmail should not be robot")
	}
}

func TestStripRobotNetmailFooter(t *testing.T) {
	body := "Area list\r\n\r\n--- htick/lnx 1.9 2023-12-21 areafix\r\n"
	got := StripRobotNetmailFooter(body)
	if strings.Contains(got, "--- htick") {
		t.Fatalf("tear not stripped: %q", got)
	}
	if !strings.Contains(got, "Area list") {
		t.Fatalf("text lost: %q", got)
	}
}

func TestBuildBody_noSignatureForRobot(t *testing.T) {
	SetOutboundBBSName("VirtBBS Test")
	t.Cleanup(func() { SetOutboundBBSName("") })

	body := buildBody(&NetmailMsg{
		Body:        "%HELP\r\n",
		NoSignature: true,
	}, Addr{Zone: 227, Net: 1, Node: 17}, Addr{Zone: 227, Net: 1, Node: 1})
	if strings.Contains(body, OutboundTearLine()) {
		t.Fatalf("robot netmail must not include tear line: %q", body)
	}
	if !strings.Contains(body, "%HELP") {
		t.Fatalf("missing body: %q", body)
	}
}
