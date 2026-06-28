package web

import "fmt"

const bytesPerMegabyte = 1024 * 1024

// formatDataSize renders a byte count for display: megabytes (one decimal)
// when size is at least 1 MiB, otherwise bytes with thousands separators.
func formatDataSize(bytes int64, locale string) string {
	if bytes >= bytesPerMegabyte {
		mb := float64(bytes) / float64(bytesPerMegabyte)
		return fmt.Sprintf("%.1f %s", mb, tr(locale, "common.megabytes"))
	}
	return fmt.Sprintf("%d %s", bytes, tr(locale, "common.bytes"))
}
