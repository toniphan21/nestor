package cli

import "testing"

func TestEscapeCLIFlagValue(t *testing.T) {
	tests := map[string]string{
		"test":      "test",
		"nhat phan": "'nhat phan'",
		"'test_":    `"'test_"`,
		"":          "''",
		"a$b":       "'a$b'",
		`it's "$x"`: `"it's \"\$x\""`,
		"--flag=v1": "--flag=v1",
	}
	for in, want := range tests {
		if got := escapeCLIFlagValue(in); got != want {
			t.Errorf("escapeCLIFlagValue(%q) = %q, want %q", in, got, want)
		}
	}
}
