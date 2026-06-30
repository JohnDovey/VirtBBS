package userapi

import (
	"encoding/json"
	"testing"
)

func TestQwkDownloadParamsUnmarshal(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"omitted", `{"method":"qwk.download","auth":{"username":"a","password":"b"}}`},
		{"null", `{"method":"qwk.download","params":null,"auth":{"username":"a","password":"b"}}`},
		{"empty_object", `{"method":"qwk.download","params":{},"auth":{"username":"a","password":"b"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req Request
			if err := json.Unmarshal([]byte(tc.raw), &req); err != nil {
				t.Fatalf("request: %v", err)
			}
			var p struct{ ConferenceIDs []int }
			if len(req.Params) > 0 {
				if err := json.Unmarshal(req.Params, &p); err != nil {
					t.Fatalf("params: %v", err)
				}
			}
		})
	}
}
