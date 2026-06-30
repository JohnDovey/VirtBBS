package fido

import "testing"

func TestIsNetmailUtilityTest(t *testing.T) {
	tests := []struct {
		name string
		pm   *Message
		want bool
	}{
		{
			name: "ping subject",
			pm:   &Message{Subject: "PING", ToName: "Sysop"},
			want: true,
		},
		{
			name: "ping toName FTSC",
			pm:   &Message{Subject: "", ToName: "PING"},
			want: true,
		},
		{
			name: "trace subject",
			pm:   &Message{Subject: "TRACE", ToName: "Sysop"},
			want: true,
		},
		{
			name: "htick trace via areafix",
			pm:   &Message{Subject: "TRACE", ToName: "AreaFix", Body: "TRACE from 227:1/17 at 30 Jun 26  05:53:05.\r\n"},
			want: true,
		},
		{
			name: "trace reply",
			pm:   &Message{Subject: "TRACE REPLY", ToName: "Sysop"},
			want: true,
		},
		{
			name: "pong",
			pm:   &Message{Subject: "PONG", ToName: "Sysop"},
			want: true,
		},
		{
			name: "areafix subscribe",
			pm:   &Message{Subject: "AreaFix", ToName: "AreaFix", Body: "+GENERAL\r\n"},
			want: false,
		},
		{
			name: "areafix password subject",
			pm:   &Message{Subject: "secret", ToName: "AreaFix", Body: "+GENERAL\r\n"},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNetmailUtilityTest(tc.pm); got != tc.want {
				t.Fatalf("IsNetmailUtilityTest()=%v want %v", got, tc.want)
			}
		})
	}
}

func TestParseFixRequestAuth_traceSubjectFails(t *testing.T) {
	// Documents why toss must skip AreaFix for TRACE: classic AreaFix treats
	// the subject as a password, so Subject "TRACE" fails auth.
	_, ok := parseFixRequestAuth("TRACE", "TRACE from 227:1/17 at 30 Jun 26  05:53:05.\r\n", "secret")
	if ok {
		t.Fatal("expected TRACE subject to fail areafix password auth")
	}
}
