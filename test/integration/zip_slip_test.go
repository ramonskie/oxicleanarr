package integration

import (
	"path/filepath"
	"testing"
)

func TestIsWithinDir(t *testing.T) {
	tests := []struct {
		name string
		base string
		path string
		want bool
	}{
		{"exact base", "/tmp/out", "/tmp/out", true},
		{"child file", "/tmp/out", "/tmp/out/sub/file.dll", true},
		{"parent escape", "/tmp/out", "/tmp/out/../secret.dll", false},
		{"sibling dir", "/tmp/out", "/tmp/other/secret.dll", false},
		{"dotdot prefix", "/tmp/out", filepath.Join("..", "secret.dll"), false},
		{"root escape", "/tmp/out", "/etc/passwd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWithinDir(tt.base, tt.path)
			if got != tt.want {
				t.Errorf("isWithinDir(%q, %q) = %v, want %v", tt.base, tt.path, got, tt.want)
			}
		})
	}
}
