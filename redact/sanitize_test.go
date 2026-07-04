package redact

import "testing"

func TestLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text unchanged", "hello world", "hello world"},
		{"utf-8 passthrough", "größe Bäume — ✓", "größe Bäume — ✓"},
		{"tab kept", "a\tb", "a\tb"},
		{"sgr color stripped", "\x1b[31mred\x1b[0m", "red"},
		{"osc title spoof stripped", "\x1b]0;PWNED\x07hello", "hello"},
		{"osc 52 clipboard stripped", "\x1b]52;c;bWFsaWNpb3Vz\x07hello", "hello"},
		{"osc with st terminator", "\x1b]0;PWNED\x1b\\hello", "hello"},
		{"dcs stripped", "\x1bPqpayload\x1b\\ok", "ok"},
		{"bare c0 removed", "a\x07b\x08c\rd", "abcd"},
		// ESC+letter is a valid two-byte escape sequence; the parser
		// consumes both, which is fine (over-stripping is safe).
		{"bare esc removed", "a\x1bb", "a"},
		{"del removed", "a\x7fb", "ab"},
		{"c1 removed", "a\u0085b", "ab"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Line(tt.in); got != tt.want {
				t.Errorf("Line(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
