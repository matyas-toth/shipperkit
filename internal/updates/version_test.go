package updates

import "testing"

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		candidate, current string
		want               bool
	}{
		{"1.0.0-rc.9", "1.0.0-rc.8", true},
		{"1.0.0-rc.8", "1.0.0-rc.8", false},
		{"1.0.0-rc.8", "1.0.0-rc.9", false},
		{"1.0.0-rc.8.nightly.2", "1.0.0-rc.8.nightly.1", true},
		{"1.0.0-rc.8.nightly.10", "1.0.0-rc.8.nightly.2", true},
		{"1.0.0-rc.8.nightly.1", "1.0.0-rc.8", true},
		{"1.0.0-rc.9", "1.0.0-rc.8.nightly.99", true},
		{"1.0.0-rc.10.nightly.1", "1.0.0-rc.9.nightly.99", true},
		{"1.0.0-rc.8", "1.0.0-rc.8.nightly.1", false},
		{"1.0.0", "1.0.0-rc.99", true},
		{"1.0.0-rc.8.nightly.bb", "1.0.0-rc.8-edge.aa", true},
		{"v1.0.0-rc.9", "1.0.0-rc.8", true},
		{"1.0.0-alpha", "1.0.0-alpha.1", false},
		{"1.0.0-alpha.1", "1.0.0-alpha.beta", false},
		{"1.0.0-beta.11", "1.0.0-beta.2", true},
		{"1.2.3", "1.2.2", true},
		{"2.0.0", "1.99.99", true},
		{"1.0.0-rc.8+meta", "1.0.0-rc.8", false},
		{"not-a-version", "1.0.0", false},
		{"1.0.0", "", false},
	} {
		if got := Newer(tc.candidate, tc.current); got != tc.want {
			t.Fatalf("Newer(%q, %q) = %v, want %v", tc.candidate, tc.current, got, tc.want)
		}
	}
}
