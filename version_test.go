package nestor

import "testing"

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.4.0", "0.3.0", true},
		{"0.3.0", "0.4.0", false},
		{"0.4.0", "0.4.0", false},
		{"1.0.0", "0.9.9", true},
		{"0.9.9", "1.0.0", false},
		{"0.4.1", "0.4.0", true},
		{"0.4", "0.4.0", false},
		{"0.4.0.1", "0.4.0", true},
	}

	for _, c := range cases {
		if got := isNewerVersion(c.a, c.b); got != c.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
