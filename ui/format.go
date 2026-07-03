package ui

import (
	"fmt"
	"time"
)

// byteSize renders a byte count human-readable (fixed-ish width).
func byteSize(n int64) string {
	const k = 1024
	switch {
	case n >= k*k*k:
		return fmt.Sprintf("%.1fG", float64(n)/(k*k*k))
	case n >= k*k:
		return fmt.Sprintf("%.0fM", float64(n)/(k*k))
	case n >= k:
		return fmt.Sprintf("%.0fK", float64(n)/k)
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// rate renders bytes/second.
func rate(bps float64) string {
	return byteSize(int64(bps)) + "/s"
}

// shortDuration renders an uptime like ctop: 2d4h, 5h12m, 3m2s.
func shortDuration(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd%dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh%dm", hours, mins)
	case mins > 0:
		return fmt.Sprintf("%dm%ds", mins, secs)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}
