package nestor

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplate_makeAgentName_stressTest(t *testing.T) {
	taken := make(map[string]struct{})
	template := DefaultTemplate()
	n := 10_000_000
	for i := range n {
		name := template.MakeAgentID()
		_, have := taken[name]
		if have {
			t.Fatalf("agent name collision at %d when doing n=%d", i, n)
		}
		taken[name] = struct{}{}
	}
	fmt.Println(template.MakeAgentID())
}

func TestTemplate_sanitize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "go-reshard", "go-reshard"},
		{"uppercase", "Go-Reshard", "go-reshard"},
		{"spaces", "my project", "my-project"},
		{"keeps dots and underscores", "hello_world.tar.gz", "hello_world.tar.gz"},
		{"strips leading dot", ".hidden", "hidden"},
		{"trims separators", "--foo--", "foo"},
		{"unsafe chars", "a/b:c*d", "a-b-c-d"},
		{"non-ascii", "projekt-müller", "projekt-m-ller"},
		{"non-ascii leading", "Ünicode", "nicode"},
		{"empty", "", ""},
		{"only dots", "...", ""},
		{"only separator", "/", ""},
		{"truncates", strings.Repeat("a", 45), strings.Repeat("a", 40)},
		{"truncates then trims", strings.Repeat("a", 39) + "-bcd", strings.Repeat("a", 39)},
	}

	for _, tt := range tests {
		template := &Template{}
		t.Run(tt.name, func(t *testing.T) {
			if got := template.sanitize(tt.in); got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTemplate_sanitizeIsFilesystemSafe(t *testing.T) {
	inputs := []string{
		"normal", "Wéird Näme!", "../escape", "a\x00b", "  ", "-.-",
		strings.Repeat("x", 300), "\ttab\n", "sub/dir", `back\slash`,
	}

	for _, in := range inputs {
		template := &Template{}
		got := template.sanitize(in)
		if got == "" {
			continue
		}
		if len(got) > maxBase {
			t.Errorf("sanitize(%q) = %q, too long (%d)", in, got, len(got))
		}
		if strings.ContainsAny(got, `/\:`) || strings.ContainsRune(got, 0) {
			t.Errorf("sanitize(%q) = %q, contains a path separator", in, got)
		}
		if got[0] == '.' || got[0] == '-' {
			t.Errorf("sanitize(%q) = %q, starts with %q", in, got, got[0])
		}
		if got != filepath.Base(got) {
			t.Errorf("sanitize(%q) = %q, not a single path element", in, got)
		}
	}
}
