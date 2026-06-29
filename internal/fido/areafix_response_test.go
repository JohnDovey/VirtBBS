package fido

import "testing"

func TestParseAreaFixResponseBody_hpt(t *testing.T) {
	body := ` Area                                                Status
 --------------------------------------------------  -------------------------
 LVLY_ANNOUNCE ....................................  added

Following is the original message text
--------------------------------------
+LVLY_ANNOUNCE

--------------------------------------

--- hpt/lnx 1.9 2024-03-02 areafix
 * Origin: Areafix robot (227:1/1)
`
	actions := ParseAreaFixResponseBody(body)
	if len(actions) != 1 {
		t.Fatalf("got %d actions, want 1: %+v", len(actions), actions)
	}
	if actions[0].Tag != "LVLY_ANNOUNCE" || !actions[0].Add {
		t.Fatalf("unexpected action: %+v", actions[0])
	}
}

func TestParseAreaFixResponseBody_virtbbs(t *testing.T) {
	body := "AreaFix response for Bob (1:2/3)\r\n\r\n  +FSX_GEN                 subscribed\r\n"
	actions := ParseAreaFixResponseBody(body)
	if len(actions) != 1 || actions[0].Tag != "FSX_GEN" || !actions[0].Add {
		t.Fatalf("unexpected: %+v", actions)
	}
}

func TestParseAreaFixResponseBody_remove(t *testing.T) {
	body := "OLD_TAG ....................................  removed\r\n"
	actions := ParseAreaFixResponseBody(body)
	if len(actions) != 1 || actions[0].Tag != "OLD_TAG" || actions[0].Add {
		t.Fatalf("unexpected: %+v", actions)
	}
}

func TestNetworkDef_IsUplinkFixRobot(t *testing.T) {
	nd := &NetworkDef{Uplink: "227:1/1"}
	if !nd.IsUplinkFixRobot(Addr{Zone: 227, Net: 1, Node: 1}) {
		t.Fatal("expected 227:1/1 robot")
	}
	if !nd.IsUplinkFixRobot(Addr{Zone: 227, Net: 1, Node: 0}) {
		t.Fatal("expected 227:1/0 host robot")
	}
	if nd.IsUplinkFixRobot(Addr{Zone: 227, Net: 1, Node: 17}) {
		t.Fatal("member node should not match")
	}
}
