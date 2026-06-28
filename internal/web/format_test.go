package web

import "testing"

func TestFormatDataSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{512, "512 bytes"},
		{1048575, "1048575 bytes"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
	}
	for _, tc := range tests {
		got := formatDataSize(tc.bytes, "en")
		if got != tc.want {
			t.Errorf("formatDataSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}
