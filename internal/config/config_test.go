package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(dir string)
		env        map[string]string
		wantErr    string
		assertFunc func(t *testing.T, cfg *Config)
	}{
		{
			name: "loads releasar.json with all defaults applied",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{}`)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "main", cfg.Git.DefaultBranch)
				assert.Equal(t, "main", cfg.Git.DevelopmentBranch)
				assert.Equal(t, "v", cfg.Git.TagPrefix)
				assert.Equal(t, "origin", cfg.Git.Remote)
				assert.Equal(t, "chore(release): {{tag}}", cfg.Git.ReleaseCommitTemplate)
				assert.Equal(t, "semver", cfg.Versioning.Scheme)
				assert.True(t, cfg.Changelog.Enabled)
				assert.Equal(t, "CHANGELOG.md", cfg.Changelog.Path)
				assert.Equal(t, PackageManagerNone, cfg.DetectedPackageManager)
			},
		},
		{
			name: "loads from package.json releasar key and detects npm",
			setup: func(dir string) {
				writeFile(t, dir, "package.json", `{"releasar": {"scm": {"provider": "github"}}}`)
			},
			env: map[string]string{"GITHUB_TOKEN": "tok"},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, PackageManagerNpm, cfg.DetectedPackageManager)
				assert.Equal(t, "github", cfg.SCM.Provider)
				assert.Equal(t, "package.json", filepath.Base(cfg.SourceFile))
			},
		},
		{
			name: "loads from composer.json releasar key and detects composer",
			setup: func(dir string) {
				writeFile(t, dir, "composer.json", `{"releasar": {"scm": {"provider": "gitlab"}}}`)
			},
			env: map[string]string{"GITLAB_TOKEN": "tok"},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, PackageManagerComposer, cfg.DetectedPackageManager)
				assert.Equal(t, "gitlab", cfg.SCM.Provider)
			},
		},
		{
			name: "npm takes precedence when both manifests present without lock files",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{}`)
				writeFile(t, dir, "package.json", `{}`)
				writeFile(t, dir, "composer.json", `{}`)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, PackageManagerNpm, cfg.DetectedPackageManager)
			},
		},
		{
			name: "detects yarn via yarn.lock",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{}`)
				writeFile(t, dir, "package.json", `{}`)
				writeFile(t, dir, "yarn.lock", ``)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, PackageManagerYarn, cfg.DetectedPackageManager)
			},
		},
		{
			name: "detects pnpm via pnpm-lock.yaml",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{}`)
				writeFile(t, dir, "package.json", `{}`)
				writeFile(t, dir, "pnpm-lock.yaml", ``)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, PackageManagerPnpm, cfg.DetectedPackageManager)
			},
		},
		{
			name: "detects bun via bun.lockb",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{}`)
				writeFile(t, dir, "package.json", `{}`)
				writeFile(t, dir, "bun.lockb", ``)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, PackageManagerBun, cfg.DetectedPackageManager)
			},
		},
		{
			name: "detects bun via bun.lock",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{}`)
				writeFile(t, dir, "package.json", `{}`)
				writeFile(t, dir, "bun.lock", ``)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, PackageManagerBun, cfg.DetectedPackageManager)
			},
		},
		{
			name: "bun takes precedence over pnpm and yarn when multiple lock files present",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{}`)
				writeFile(t, dir, "package.json", `{}`)
				writeFile(t, dir, "bun.lockb", ``)
				writeFile(t, dir, "pnpm-lock.yaml", ``)
				writeFile(t, dir, "yarn.lock", ``)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, PackageManagerBun, cfg.DetectedPackageManager)
			},
		},
		{
			name:    "returns error when no config file found",
			setup:   func(dir string) {},
			wantErr: "no releasar config found in",
		},
		{
			name: "changelog disabled round-trips",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{"changelog": {"enabled": false}}`)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.False(t, cfg.Changelog.Enabled)
			},
		},
		{
			name: "scm provider github defaults tokenEnv to GITHUB_TOKEN",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{"scm": {"provider": "github"}}`)
			},
			env: map[string]string{"GITHUB_TOKEN": "tok"},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "GITHUB_TOKEN", cfg.SCM.TokenEnv)
			},
		},
		{
			name: "missing tokenEnv returns error",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{"scm": {"provider": "github"}}`)
			},
			wantErr: `env var "GITHUB_TOKEN" (required for scm.github) is not set`,
		},
		{
			name: "task refs round-trip",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{"tasks": {"test": ["npm::test", "lint"], "build": ["npm::build"]}}`)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, []TaskRef{"npm::test", "lint"}, cfg.Tasks.Test)
				assert.Equal(t, []TaskRef{"npm::build"}, cfg.Tasks.Build)
			},
		},
		{
			name: "hooks round-trip",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{"hooks": {"beforeRelease": "bash pre.sh", "afterRelease": "bash post.sh"}}`)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "bash pre.sh", cfg.Hooks.BeforeRelease)
				assert.Equal(t, "bash post.sh", cfg.Hooks.AfterRelease)
			},
		},
		{
			name: "releaseCommitTemplate round-trips",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{"git": {"releaseCommitTemplate": "chore(release): {{package}} {{tag}}"}}`)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "chore(release): {{package}} {{tag}}", cfg.Git.ReleaseCommitTemplate)
			},
		},
		{
			name: "pm provider and host round-trip",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{"pm": {"provider": "npm", "host": "https://registry.npmjs.org", "tokenEnv": "NPM_TOKEN"}}`)
			},
			env: map[string]string{"NPM_TOKEN": "tok"},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "npm", cfg.PackageManager.Provider)
				assert.Equal(t, "https://registry.npmjs.org", cfg.PackageManager.Host)
				assert.Equal(t, "NPM_TOKEN", cfg.PackageManager.TokenEnv)
			},
		},
		{
			name: "developmentBranch defaults to defaultBranch when unset",
			setup: func(dir string) {
				writeFile(t, dir, "releasar.json", `{"git": {"defaultBranch": "master"}}`)
			},
			assertFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "master", cfg.Git.DefaultBranch)
				assert.Equal(t, "master", cfg.Git.DevelopmentBranch)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			tc.setup(dir)

			cfg, err := Load(dir)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			if tc.assertFunc != nil {
				tc.assertFunc(t, cfg)
			}
		})
	}
}

func TestParseTaskRef(t *testing.T) {
	tests := []struct {
		ref        TaskRef
		wantPM     string
		wantScript string
	}{
		{"npm::test", "npm", "test"},
		{"composer::build", "composer", "build"},
		{"lint", "", "lint"},
		{"test", "", "test"},
	}

	for _, tc := range tests {
		pm, script := ParseTaskRef(tc.ref)
		assert.Equal(t, tc.wantPM, pm, "pm for %q", tc.ref)
		assert.Equal(t, tc.wantScript, script, "script for %q", tc.ref)
	}
}
