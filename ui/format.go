package ui

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/eqms/actop/redact"
)

// safe strips terminal control sequences from text that originates from the
// container CLI. Names, image references, volume sources and env values cross
// the same trust boundary as log output: whoever publishes an image picks
// them, and an embedded OSC 52 (clipboard write), title-set or query sequence
// would be acted on by the host terminal the moment it is drawn. Every render
// path that touches CLI-supplied text goes through safe, or through pad and
// displayWidth, which apply it.
func safe(s string) string { return redact.Line(s) }

// displayWidth is the number of cells s occupies once rendered: rune count
// after sanitizing, so stripped escape bytes never inflate a column width.
func displayWidth(s string) int { return utf8.RuneCountInString(safe(s)) }

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

// sparkBlocks are the eight block glyphs used for sparkline levels.
var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// sparkline renders the last `width` values as a block-glyph graph.
// Values are scaled against max(observed, floor) so idle noise does not
// render as full bars; for percent metrics pass floor=10.
func sparkline(values []float64, width int, floor float64) string {
	if width <= 0 || len(values) == 0 {
		return ""
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}
	scale := floor
	for _, v := range values {
		if v > scale {
			scale = v
		}
	}
	if scale <= 0 {
		scale = 1
	}
	out := make([]rune, 0, width)
	for _, v := range values {
		idx := int(v / scale * float64(len(sparkBlocks)-1))
		idx = max(idx, 0)
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		out = append(out, sparkBlocks[idx])
	}
	return string(out)
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
