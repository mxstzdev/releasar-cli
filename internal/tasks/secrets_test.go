package tasks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSecretScan(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string // relative path → content
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "clean directory returns nil",
			files:   map[string]string{"readme.txt": "hello world"},
			wantErr: false,
		},
		{
			name: "AWS access key triggers finding",
			files: map[string]string{
				// Matches the aws-access-token rule: AKIA[A-Z2-7]{16}
				// Key does not end with EXAMPLE (which is allowlisted by the default config).
				"config.env": "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7ABCDE23\n",
			},
			wantErr:   true,
			errSubstr: "potential secret",
		},
		{
			name:    "empty directory returns nil",
			files:   map[string]string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				path := filepath.Join(dir, name)
				require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
			}

			err := RunSecretScan(dir, false)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunSecretScanCustomConfig(t *testing.T) {
	dir := t.TempDir()

	// A .gitleaks.toml with no rules overrides the defaults — nothing is detected
	// even if the directory contains a pattern the default ruleset would catch.
	customCfg := `title = "test: no rules"`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitleaks.toml"), []byte(customCfg), 0o600))

	// AWS key that default rules would catch; custom zero-rule config ignores it.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "creds.env"), []byte("AWS_ACCESS_KEY_ID=AKIAIOSFODNN7ABCDE23\n"), 0o600))

	err := RunSecretScan(dir, false)
	assert.NoError(t, err, "custom config with no rules should suppress all detections")
}
