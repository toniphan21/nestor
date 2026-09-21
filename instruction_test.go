package nestor

import (
	"slices"
	"testing"
)

func Test_parseFilePath(t *testing.T) {
	type testCase struct {
		name   string
		in     string
		want   string
		wantOK bool
	}

	common := []testCase{
		{"plain text", "hello", "", false},
		{"empty", "", "", false},
		{"path without scheme", "/tmp/a.txt", "", false},
		{"other scheme", "https://example.com/a.txt", "", false},
		{"invalid escape", "file:///tmp/%zz", "", false},
		{"opaque file url", "file:tmp/a.txt", "", false},
		{"bare scheme", "file:", "", false},
		{"host without path", "file://localhost", "", false},
	}

	unix := []testCase{
		{"absolute", "file:///tmp/a.txt", "/tmp/a.txt", true},
		{"percent encoded", "file:///tmp/my%20file.txt", "/tmp/my file.txt", true},
		{"localhost", "file://localhost/tmp/a.txt", "/tmp/a.txt", true},
		{"uppercase scheme", "FILE:///tmp/a.txt", "/tmp/a.txt", true},
		{"root", "file:///", "/", true},
		{"remote host", "file://server/share/a.txt", "", false},
	}

	windows := []testCase{
		{"drive", "file:///C:/Users/a.txt", `C:\Users\a.txt`, true},
		{"percent encoded", "file:///C:/My%20Docs/a.txt", `C:\My Docs\a.txt`, true},
		{"localhost", "file://localhost/C:/a.txt", `C:\a.txt`, true},
		{"unc", "file://server/share/a.txt", `\\server\share\a.txt`, true},
	}

	suites := []struct {
		kind  OSKind
		tests []testCase
	}{
		{OSMacOS, slices.Concat(common, unix)},
		{OSLinux, slices.Concat(common, unix)},
		{OSWindows, slices.Concat(common, windows)},
	}

	for _, s := range suites {
		t.Run(string(s.kind), func(t *testing.T) {
			for _, tt := range s.tests {
				t.Run(tt.name, func(t *testing.T) {
					got, ok := parseFilePath(tt.in, s.kind)
					if got != tt.want || ok != tt.wantOK {
						t.Errorf("parseFilePath(%q, %s) = (%q, %v), want (%q, %v)",
							tt.in, s.kind, got, ok, tt.want, tt.wantOK)
					}
				})
			}
		})
	}
}

func Test_parseInstruction(t *testing.T) {
	files := []struct {
		kind OSKind
		in   string
		want string
	}{
		{OSMacOS, "file:///tmp/a.txt", "/tmp/a.txt"},
		{OSLinux, "file:///tmp/a.txt", "/tmp/a.txt"},
		{OSWindows, "file:///C:/tmp/a.txt", `C:\tmp\a.txt`},
		{OSWindows, "file://server/share/a.txt", `\\server\share\a.txt`},
	}

	for _, tt := range files {
		t.Run(string(tt.kind)+"/file/"+tt.in, func(t *testing.T) {
			ins := parseInstruction(tt.in, tt.kind)
			got, ok := ins.(*fileInstruction)
			if !ok {
				t.Fatalf("parseInstruction(%q, %s) = %T, want *fileInstruction",
					tt.in, tt.kind, ins)
			}
			if got.path != tt.want {
				t.Errorf("path = %q, want %q", got.path, tt.want)
			}
		})
	}

	literals := []string{
		"hello",
		"",
		"multi\nline\ntext",
		"https://example.com/a.txt",
		"/tmp/a.txt",
		"file:tmp/a.txt",
		"file:///tmp/%zz",
		"note: this has a colon",
	}

	for _, kind := range []OSKind{OSMacOS, OSLinux, OSWindows} {
		inputs := literals
		if kind != OSWindows {
			inputs = append(slices.Clone(literals), "file://server/share/a.txt")
		}

		for _, in := range inputs {
			t.Run(string(kind)+"/literal/"+in, func(t *testing.T) {
				ins := parseInstruction(in, kind)
				got, ok := ins.(*literalInstruction)
				if !ok {
					t.Fatalf("parseInstruction(%q, %s) = %T, want *literalInstruction",
						in, kind, ins)
				}
				if got.value != in {
					t.Errorf("value = %q, want %q", got.value, in)
				}
			})
		}
	}
}
