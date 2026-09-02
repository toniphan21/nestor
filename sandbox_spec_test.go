package nestor

import (
	"errors"
	"testing"
)

func TestSandboxMountTarget(t *testing.T) {
	tests := []struct {
		name  string
		mount SandboxSpecMount
		want  string
		err   error
	}{
		{
			name:  "direct rw",
			mount: SandboxSpecMount{Type: MountTypeDirect, Path: "/src"},
			want:  "/src:rw",
		},
		{
			name:  "direct ro",
			mount: SandboxSpecMount{Type: MountTypeDirect, Path: "/src", ReadOnly: true},
			want:  "/src:ro",
		},
		{
			name:  "direct with at",
			mount: SandboxSpecMount{Type: MountTypeDirect, Path: "/src", At: "/work"},
			want:  "/work:rw",
		},
		{
			name:  "worktree",
			mount: SandboxSpecMount{Type: MountTypeGitWorktree, Path: "/src"},
			want:  "/src:rw",
		},
		{
			name:  "worktree with same at",
			mount: SandboxSpecMount{Type: MountTypeGitWorktree, Path: "/src", At: "/src"},
			want:  "/src:rw",
		},
		{
			name:  "worktree read-only",
			mount: SandboxSpecMount{Type: MountTypeGitWorktree, Path: "/src", ReadOnly: true},
			err:   ErrNotAllowed,
		},
		{
			name:  "worktree with different at",
			mount: SandboxSpecMount{Type: MountTypeGitWorktree, Path: "/src", At: "/work"},
			err:   ErrNotAllowed,
		},
		{
			name:  "unknown type",
			mount: SandboxSpecMount{Type: mountType("bogus"), Path: "/src"},
			err:   ErrNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.mount.Target()
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
