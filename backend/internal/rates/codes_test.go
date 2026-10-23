package rates

import "testing"

func TestCodeKey(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"0450", "450"},
		{"450", "450"},
		{" 450 ", "450"},
		{"00731", "731"},
		{"99213", "99213"},
		{"j1100", "J1100"},
		{"0000", "0"},
	} {
		if got := codeKey(c.in); got != c.want {
			t.Errorf("codeKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDisplayCode(t *testing.T) {
	for _, c := range []struct{ codeType, code, want string }{
		{"RC", "450", "0450"},
		{"CPT", "731", "00731"}, // three digits like a revenue code, but CPT pads to five
		{"CPT", "99213", "99213"},
		{"HCPCS", "J1100", "J1100"},
		{"RC", "0450", "0450"}, // already full width
	} {
		if got := displayCode(c.codeType, c.code); got != c.want {
			t.Errorf("displayCode(%q, %q) = %q, want %q", c.codeType, c.code, got, c.want)
		}
	}
}
