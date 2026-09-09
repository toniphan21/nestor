package nestor

import (
	"errors"
	"reflect"
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
			want:  "/src",
		},
		{
			name:  "direct ro",
			mount: SandboxSpecMount{Type: MountTypeDirect, Path: "/src", ReadOnly: true},
			want:  "/src",
		},
		{
			name:  "direct with at",
			mount: SandboxSpecMount{Type: MountTypeDirect, Path: "/src", At: "/work"},
			want:  "/work",
		},
		{
			name:  "worktree",
			mount: SandboxSpecMount{Type: MountTypeGitWorktree, Path: "/src"},
			want:  "/src",
		},
		{
			name:  "worktree with same at",
			mount: SandboxSpecMount{Type: MountTypeGitWorktree, Path: "/src", At: "/src"},
			want:  "/src",
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

func TestSandboxSpec_Resolve(t *testing.T) {
	tests := []struct {
		name      string
		mounts    []SandboxSpecMount
		path      string
		wantMount *SandboxSpecMount
		wantRel   string
	}{
		{
			name:      "exact mount path",
			mounts:    []SandboxSpecMount{{Path: "/home/proj"}},
			path:      "/home/proj",
			wantMount: &SandboxSpecMount{Path: "/home/proj"},
			wantRel:   ".",
		},
		{
			name:      "nested path",
			mounts:    []SandboxSpecMount{{Path: "/home/proj"}},
			path:      "/home/proj/cmd/nestor",
			wantMount: &SandboxSpecMount{Path: "/home/proj"},
			wantRel:   "cmd/nestor",
		},
		{
			name:      "trailing slash on host",
			mounts:    []SandboxSpecMount{{Path: "/home/proj/"}},
			path:      "/home/proj/internal",
			wantMount: &SandboxSpecMount{Path: "/home/proj/"},
			wantRel:   "internal",
		},
		{
			name:      "unclean path is normalized",
			mounts:    []SandboxSpecMount{{Path: "/home/proj"}},
			path:      "/home/proj/./a/../b",
			wantMount: &SandboxSpecMount{Path: "/home/proj"},
			wantRel:   "b",
		},
		{
			name:      "escapes mount via dotdot",
			mounts:    []SandboxSpecMount{{Path: "/home/proj"}},
			path:      "/home/proj/../secrets",
			wantMount: nil,
			wantRel:   "",
		},
		{
			name:      "sibling with shared prefix is not inside",
			mounts:    []SandboxSpecMount{{Path: "/home/proj"}},
			path:      "/home/project",
			wantMount: nil,
			wantRel:   "",
		},
		{
			name:      "outside any mount",
			mounts:    []SandboxSpecMount{{Path: "/home/proj"}},
			path:      "/etc/passwd",
			wantMount: nil,
			wantRel:   "",
		},
		{
			name:      "relative path against absolute mount",
			mounts:    []SandboxSpecMount{{Path: "/home/proj"}},
			path:      "proj/cmd",
			wantMount: nil,
			wantRel:   "",
		},
		{
			name: "second mount matches",
			mounts: []SandboxSpecMount{
				{Path: "/opt/cache", At: "/cache"},
				{Path: "/home/proj"},
			},
			path:      "/home/proj/main.go",
			wantMount: &SandboxSpecMount{Path: "/home/proj"},
			wantRel:   "main.go",
		},
		{
			name:      "root mount",
			mounts:    []SandboxSpecMount{{Path: "/"}},
			path:      "/etc/hosts",
			wantMount: &SandboxSpecMount{Path: "/"},
			wantRel:   "etc/hosts",
		},
		{
			name:      "no mounts",
			mounts:    nil,
			path:      "/home/proj",
			wantMount: nil,
			wantRel:   "",
		},
		{
			name: "overlapping mounts pick the first, not the most specific",
			mounts: []SandboxSpecMount{
				{Path: "/home/nestor"},
				{Path: "/home/nestor/proj"},
			},
			path:      "/home/nestor/main.go",
			wantMount: &SandboxSpecMount{Path: "/home/nestor"},
			wantRel:   "main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SandboxSpec{Mounts: tt.mounts}

			mount, rel := s.Resolve(tt.path)
			if rel != tt.wantRel {
				t.Errorf("rel = %v, want %v", rel, tt.wantRel)
			}
			if !reflect.DeepEqual(mount, tt.wantMount) {
				t.Errorf("mount = %v, want %v", mount, tt.wantMount)
			}
		})
	}
}
