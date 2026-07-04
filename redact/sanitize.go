package redact

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Line strips ANSI escape sequences (CSI/OSC/DCS/...) and all control
// characters except tab from s. Container output crosses a trust boundary:
// a process can emit OSC 52 (clipboard write), title-set or query sequences
// that the host terminal would act on when the line is rendered.
func Line(s string) string {
	s = ansi.Strip(s)
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return r
		case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
			return -1
		}
		return r
	}, s)
}
