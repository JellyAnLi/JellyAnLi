package updater

import "testing"

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v1.0.1", "v1.0.0", true},
		{"v1.1.0", "v1.0.9", true},
		{"v2.0.0", "v1.9.9", true},
		{"v1.0.0", "v1.0.0", false},
		{"v1.0.0", "v1.0.1", false},
		{"v1.0.0", "v1.0.0-beta", true},
		{"1.0.1", "1.0.0", true},
		{"v1.0.0-beta.2", "v1.0.0-beta.1", false},
		{"v1.3.0", "main", false},
		{"v1.3.0", "dev", false},
		{"v1.3.0", "v1.3.1-main", false},
		{"v1.3.2", "v1.3.1-main", true},
	}

	for _, tt := range tests {
		got := IsNewerVersion(tt.latest, tt.current)
		if got != tt.want {
			t.Errorf("IsNewerVersion(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}
