package nestor

import "testing"

func TestSandbox_resolveWorkDir(t *testing.T) {
	tests := []struct {
		name   string
		mounts []SandboxMount
		path   string
		want   string
		wantOK bool
	}{
		{
			name:   "exact mount root",
			mounts: []SandboxMount{{Host: "/home/nhat/proj", Target: "/work"}},
			path:   "/home/nhat/proj",
			want:   "/work",
			wantOK: true,
		},
		{
			name:   "nested path",
			mounts: []SandboxMount{{Host: "/home/nhat/proj", Target: "/work"}},
			path:   "/home/nhat/proj/cmd/nestor",
			want:   "/work/cmd/nestor",
			wantOK: true,
		},
		{
			name:   "trailing slash on host",
			mounts: []SandboxMount{{Host: "/home/nhat/proj/", Target: "/work"}},
			path:   "/home/nhat/proj/internal",
			want:   "/work/internal",
			wantOK: true,
		},
		{
			name:   "unclean path is normalized",
			mounts: []SandboxMount{{Host: "/home/nhat/proj", Target: "/work"}},
			path:   "/home/nhat/proj/./a/../b",
			want:   "/work/b",
			wantOK: true,
		},
		{
			name:   "escapes mount via dotdot",
			mounts: []SandboxMount{{Host: "/home/nhat/proj", Target: "/work"}},
			path:   "/home/nhat/proj/../secrets",
			wantOK: false,
		},
		{
			name:   "sibling with shared prefix is not inside",
			mounts: []SandboxMount{{Host: "/home/nhat/proj", Target: "/work"}},
			path:   "/home/nhat/project",
			wantOK: false,
		},
		{
			name:   "outside any mount",
			mounts: []SandboxMount{{Host: "/home/nhat/proj", Target: "/work"}},
			path:   "/etc/passwd",
			wantOK: false,
		},
		{
			name:   "relative path against absolute mount",
			mounts: []SandboxMount{{Host: "/home/nhat/proj", Target: "/work"}},
			path:   "proj/cmd",
			wantOK: false,
		},
		{
			name: "second mount matches",
			mounts: []SandboxMount{
				{Host: "/opt/cache", Target: "/cache"},
				{Host: "/home/nhat/proj", Target: "/work"},
			},
			path:   "/home/nhat/proj/main.go",
			want:   "/work/main.go",
			wantOK: true,
		},
		{
			name:   "root mount",
			mounts: []SandboxMount{{Host: "/", Target: "/host"}},
			path:   "/etc/hosts",
			want:   "/host/etc/hosts",
			wantOK: true,
		},
		{
			name:   "no mounts",
			mounts: nil,
			path:   "/home/nhat/proj",
			wantOK: false,
		},
		{
			name: "overlapping mounts pick the first, not the most specific",
			mounts: []SandboxMount{
				{Host: "/home/nhat", Target: "/home"},
				{Host: "/home/nhat/proj", Target: "/work"},
			},
			path:   "/home/nhat/proj/main.go",
			want:   "/home/proj/main.go",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounts := make(map[string]SandboxMount)
			for _, m := range tt.mounts {
				mounts[m.Host] = m
			}
			s := &sandboxImpl{data: &sandboxData{Mounts: mounts}}

			got, ok := s.resolveWorkDir(tt.path)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if !ok && got != "" {
				t.Errorf("got %q, want empty string when not resolved", got)
			}
		})
	}
}
