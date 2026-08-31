package nestor

import (
	"bytes"
	"embed"
	"path/filepath"
	"reflect"
	"testing"
)

//go:embed testdata/profile
var fixtures embed.FS

func Test_ParseProfiles(t *testing.T) {
	cases := []struct {
		name     string
		file     string
		err      error
		expected []Profile
	}{
		{
			name: "happy path",
			file: "default.yml",
			expected: []Profile{
				{
					Name:          "claude",
					DefaultTarget: "base",
					DefaultModel:  "claude-opus-5",
					Targets:       []string{"base", "go"},
					Models: map[string][]string{
						"claude-opus-5":     {"opus", "opus:latest", "opus:5"},
						"claude-opus-4-8":   {"opus:4", "opus:4.8"},
						"claude-sonnet-5":   {"sonnet", "sonnet:latest", "sonnet:5"},
						"claude-sonnet-4-6": {"sonnet:4", "sonnet:4.6"},
						"claude-haiku-4-5":  {"haiku", "haiku:latest", "haiku:4", "haiku:4.5"},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			b, err := fixtures.ReadFile(filepath.Join("testdata", "profile", tc.file))
			if err != nil {
				t.Fatal(err)
			}

			result, err := ParseProfiles(bytes.NewBuffer(b))
			if tc.err != nil {
				if err == nil {
					t.Errorf("expected error, got none")
					return
				}

				if got, want := err.Error(), tc.err.Error(); got != want {
					t.Errorf("got: %q, want: %q", got, want)
				}
				return
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("got: %v, want: %v", result, tc.expected)
			}
		})
	}
}
