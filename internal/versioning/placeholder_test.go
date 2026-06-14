package versioning

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func semverAt(s string) Version {
	v, err := SemVer{}.Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

func TestReplacePlaceholders(t *testing.T) {
	v := semverAt("1.2.3")

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// LATEST variants
		{"RLSR_LATEST uppercase", "version: {{RLSR_LATEST}}", "version: 1.2.3", false},
		{"LATEST uppercase", "version: {{LATEST}}", "version: 1.2.3", false},
		{"LATEST lowercase", "version: {{latest}}", "version: 1.2.3", false},
		{"rlsr_latest lowercase", "version: {{rlsr_latest}}", "version: 1.2.3", false},
		{"LATEST mixed case", "version: {{LaTeSt}}", "version: 1.2.3", false},

		// NEXT.MAJOR variants
		{"RLSR_NEXT.MAJOR", "next: {{RLSR_NEXT.MAJOR}}", "next: 2.0.0", false},
		{"NEXT.MAJOR uppercase", "next: {{NEXT.MAJOR}}", "next: 2.0.0", false},
		{"next.major lowercase", "next: {{next.major}}", "next: 2.0.0", false},
		{"rlsr_next.major lowercase", "next: {{rlsr_next.major}}", "next: 2.0.0", false},

		// NEXT.MINOR variants
		{"RLSR_NEXT.MINOR", "next: {{RLSR_NEXT.MINOR}}", "next: 1.3.0", false},
		{"NEXT.MINOR uppercase", "next: {{NEXT.MINOR}}", "next: 1.3.0", false},
		{"next.minor lowercase", "next: {{next.minor}}", "next: 1.3.0", false},
		{"rlsr_next.minor lowercase", "next: {{rlsr_next.minor}}", "next: 1.3.0", false},

		// NEXT.PATCH variants
		{"RLSR_NEXT.PATCH", "next: {{RLSR_NEXT.PATCH}}", "next: 1.2.4", false},
		{"NEXT.PATCH uppercase", "next: {{NEXT.PATCH}}", "next: 1.2.4", false},
		{"next.patch lowercase", "next: {{next.patch}}", "next: 1.2.4", false},
		{"rlsr_next.patch lowercase", "next: {{rlsr_next.patch}}", "next: 1.2.4", false},

		// Multiple placeholders in one string
		{
			name:  "multiple placeholders",
			input: "current={{LATEST}}, next={{NEXT.MINOR}}",
			want:  "current=1.2.3, next=1.3.0",
		},

		// Unrecognised placeholders are left unchanged
		{"unknown placeholder unchanged", "{{UNKNOWN}}", "{{UNKNOWN}}", false},
		{"partial prefix unchanged", "{{RLSR_FOO}}", "{{RLSR_FOO}}", false},

		// No placeholders
		{"no placeholders", "just text", "just text", false},
		{"empty string", "", "", false},

		// Placeholder embedded in larger content
		{
			name:  "placeholder in file-like content",
			input: `{"version": "{{RLSR_LATEST}}"}`,
			want:  `{"version": "1.2.3"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReplacePlaceholders(tt.input, v)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplaceInFile(t *testing.T) {
	v := semverAt("2.0.0")

	t.Run("replaces placeholders in file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "version.txt")
		require.NoError(t, os.WriteFile(path, []byte("v={{LATEST}}, next={{NEXT.MINOR}}"), 0o644))

		require.NoError(t, ReplaceInFile(path, v))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "v=2.0.0, next=2.1.0", string(data))
	})

	t.Run("preserves original file permissions", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "version.sh")
		require.NoError(t, os.WriteFile(path, []byte("VERSION={{LATEST}}"), 0o755))

		require.NoError(t, ReplaceInFile(path, v))

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), info.Mode())
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		err := ReplaceInFile(filepath.Join(t.TempDir(), "missing.txt"), v)
		assert.Error(t, err)
	})
}
