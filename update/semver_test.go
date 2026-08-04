package update

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Version
		ok   bool
	}{
		{"1.2.0", Version{1, 2, 0}, true},
		{"v1.2.0", Version{1, 2, 0}, true},
		{"v0.6.1", Version{0, 6, 1}, true},
		{"1.2", Version{}, false},
		{"1.2.0.1", Version{}, false},
		{"v1.2.0-beta.1", Version{}, false},
		{"1.2.x", Version{}, false},
		{"", Version{}, false},
		{"v-1.2.0", Version{}, false},
		{" 1.2.0", Version{}, false},
		// A release tag is attacker-controllable in the general case; the
		// parser is the first filter that keeps terminal escapes out of the
		// render path.
		{"1.2.0\x1b]52;c;cGF5bG9hZA==\x07", Version{}, false},
	}
	for _, c := range cases {
		got, ok := Parse(c.in)
		if ok != c.ok {
			t.Errorf("Parse(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

// Numeric comparison, not lexicographic: "1.10.0" must outrank "1.9.0".
func TestLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "2.0.0", true},
		{"1.9.0", "1.10.0", true},
		{"1.2.0", "1.2.1", true},
		{"1.2.0", "1.2.0", false},
		{"2.0.0", "1.9.9", false},
		{"0.6.1", "0.7.0", true},
	}
	for _, c := range cases {
		a, _ := Parse(c.a)
		b, _ := Parse(c.b)
		if got := a.Less(b); got != c.want {
			t.Errorf("%s.Less(%s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// The canonical form is what gets rendered — never the bytes the remote sent.
func TestString(t *testing.T) {
	v, _ := Parse("v1.2.0")
	if v.String() != "1.2.0" {
		t.Errorf("String() = %q, want %q", v.String(), "1.2.0")
	}
}
