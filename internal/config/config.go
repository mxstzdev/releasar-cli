package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PackageManagerKind identifies the detected package manager.
type PackageManagerKind string

const (
	PackageManagerNone     PackageManagerKind = ""
	PackageManagerNpm      PackageManagerKind = "npm"
	PackageManagerYarn     PackageManagerKind = "yarn"
	PackageManagerPnpm     PackageManagerKind = "pnpm"
	PackageManagerBun      PackageManagerKind = "bun"
	PackageManagerComposer PackageManagerKind = "composer"
)

// Config is the resolved, validated configuration with defaults applied.
type Config struct {
	SourceFile string // absolute path of the file that was loaded
	DetectedPackageManager PackageManagerKind // inferred from manifest presence in the working directory

	Git        GitConfig
	Versioning VersioningConfig
	SCM        SCMConfig
	PackageManager PackageManagerConfig
	Tracker    TrackerConfig
	Changelog  ChangelogConfig
	Tasks      TasksConfig
	Hooks      HooksConfig
}

type GitConfig struct {
	DefaultBranch          string
	DevelopmentBranch      string
	TagPrefix              string
	Remote                 string
	ReleaseCommitTemplate  string // vars: {{package}}, {{version}}, {{tag}}
	ReleaseBranchTemplate  string // vars: {{version}}, {{tag}}; empty = use built-in heuristic
}

type VersioningConfig struct {
	Scheme            string   // "semver" | "calver"
	PlaceholderExclude []string // glob patterns relative to workingDir; matched files are skipped during placeholder substitution
}

type SCMConfig struct {
	Provider string // "github" | "gitlab" | "gitea" | "forgejo" | "codeberg"
	Host     string
	TokenEnv string // name of the env var holding the API token
}

type PackageManagerConfig struct {
	Provider string // explicit override of DetectedPackageManager
	Host     string // private/self-hosted registry URL
	TokenEnv string // name of the env var holding the registry token
}

// TrackerConfig holds configuration for the issue tracker integration.
type TrackerConfig struct {
	Provider   string // "github" | "gitea" | "forgejo" | "codeberg" | "jira" | "openproject"
	Host       string // base API URL; adapter uses provider default if empty
	TokenEnv   string // name of the env var holding the API token
	EmailEnv   string // name of the env var holding the account email (Jira only)
	ProjectKey string // Jira: project key (e.g. "MYPROJ"); OpenProject: numeric project ID
}

type ChangelogConfig struct {
	Enabled bool
	Path    string
}

// TaskRef is a PM script reference in the form "scriptName" or "pm::scriptName".
type TaskRef string

// ParseTaskRef splits "npm::test" into ("npm", "test").
// A bare "test" returns ("", "test"); the PM is resolved from DetectedPackageManager at run time.
func ParseTaskRef(ref TaskRef) (pm, script string) {
	parts := strings.SplitN(string(ref), "::", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}

type TasksConfig struct {
	Test           []TaskRef
	Build          []TaskRef
	SecretScanning bool
}

type HooksConfig struct {
	BeforeRelease string
	AfterRelease  string
}

// Load resolves and validates the releasar configuration from workingDir.
// Must be called after LoadDotEnv so that tokenEnv references resolve correctly.
func Load(workingDir string) (*Config, error) {
	workingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("resolving working directory: %w", err)
	}

	raw, sourceFile, err := loadRaw(workingDir)
	if err != nil {
		return nil, err
	}

	detected := detectPackageManager(workingDir)
	cfg := applyDefaults(raw, detected)
	cfg.SourceFile = sourceFile
	cfg.DetectedPackageManager = detected

	if err := validateTokens(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadLayered loads config from rootDir first, then overlays it with config from workingDir.
// When rootDir == workingDir it behaves identically to Load(workingDir).
// Must be called after LoadDotEnv so that tokenEnv references resolve correctly.
func LoadLayered(rootDir, workingDir string) (*Config, error) {
	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolving root directory: %w", err)
	}
	workingDir, err = filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("resolving working directory: %w", err)
	}

	if rootDir == workingDir {
		return Load(workingDir)
	}

	// Root config is optional (monorepo roots may have none).
	baseRaw, _, _ := loadRaw(rootDir)
	if baseRaw == nil {
		baseRaw = &rawConfig{}
	}

	overlayRaw, sourceFile, err := loadRaw(workingDir)
	if err != nil {
		return nil, err
	}

	merged := mergeRaw(baseRaw, overlayRaw)
	detected := detectPackageManager(workingDir)
	cfg := applyDefaults(merged, detected)
	cfg.SourceFile = sourceFile
	cfg.DetectedPackageManager = detected

	if err := validateTokens(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// loadRaw finds and parses the config file, returning the raw struct and its absolute path.
func loadRaw(workingDir string) (*rawConfig, string, error) {
	// 1. releasar.json
	path := filepath.Join(workingDir, "releasar.json")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("reading %s: %w", path, err)
	}
	if err == nil {
		var raw rawConfig
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, "", fmt.Errorf("parsing %s: %w", path, err)
		}
		return &raw, path, nil
	}

	// 2. package.json → "releasar" key
	path = filepath.Join(workingDir, "package.json")
	if raw, err := extractEmbedded(path, "releasar"); err == nil {
		return raw, path, nil
	} else if !errors.Is(err, errKeyAbsent) {
		return nil, "", err
	}

	// 3. composer.json → "releasar" key
	path = filepath.Join(workingDir, "composer.json")
	if raw, err := extractEmbedded(path, "releasar"); err == nil {
		return raw, path, nil
	} else if !errors.Is(err, errKeyAbsent) {
		return nil, "", err
	}

	return nil, "", fmt.Errorf("no releasar config found in %s", workingDir)
}

var errKeyAbsent = errors.New("key absent")

// extractEmbedded reads a JSON file and extracts the named key as a rawConfig.
// Returns errKeyAbsent if the file exists but the key is not present.
func extractEmbedded(path, key string) (*rawConfig, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errKeyAbsent
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var outer map[string]json.RawMessage
	if err := json.Unmarshal(data, &outer); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	raw, ok := outer[key]
	if !ok {
		return nil, errKeyAbsent
	}

	var cfg rawConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %q in %s: %w", key, path, err)
	}
	return &cfg, nil
}

// detectPackageManager infers the package manager from lock file and manifest presence.
// Lock files take precedence over package.json alone to differentiate npm alternatives.
func detectPackageManager(workingDir string) PackageManagerKind {
	hasFile := func(name string) bool {
		_, err := os.Stat(filepath.Join(workingDir, name))
		return err == nil
	}
	switch {
	case hasFile("bun.lockb") || hasFile("bun.lock"):
		return PackageManagerBun
	case hasFile("pnpm-lock.yaml"):
		return PackageManagerPnpm
	case hasFile("yarn.lock"):
		return PackageManagerYarn
	case hasFile("package.json"):
		return PackageManagerNpm
	case hasFile("composer.json"):
		return PackageManagerComposer
	default:
		return PackageManagerNone
	}
}

// applyDefaults fills in zero-value fields with sensible defaults.
func applyDefaults(raw *rawConfig, pm PackageManagerKind) Config {
	cfg := Config{}

	// Git
	cfg.Git.DefaultBranch = stringOr(raw.Git.DefaultBranch, "main")
	cfg.Git.DevelopmentBranch = stringOr(raw.Git.DevelopmentBranch, cfg.Git.DefaultBranch)
	cfg.Git.TagPrefix = stringOr(raw.Git.TagPrefix, "v")
	cfg.Git.Remote = stringOr(raw.Git.Remote, "origin")
	cfg.Git.ReleaseCommitTemplate = stringOr(raw.Git.ReleaseCommitTemplate, "chore(release): {{tag}}")
	cfg.Git.ReleaseBranchTemplate = raw.Git.ReleaseBranchTemplate

	// Versioning
	cfg.Versioning.Scheme = stringOr(raw.Versioning.Scheme, "semver")
	cfg.Versioning.PlaceholderExclude = raw.Versioning.PlaceholderExclude

	// SCM
	cfg.SCM.Provider = raw.SCM.Provider
	cfg.SCM.Host = raw.SCM.Host
	cfg.SCM.TokenEnv = stringOr(raw.SCM.TokenEnv, defaultSCMTokenEnv(raw.SCM.Provider))

	// PM
	cfg.PackageManager.Provider = raw.PackageManager.Provider
	cfg.PackageManager.Host = raw.PackageManager.Host
	cfg.PackageManager.TokenEnv = raw.PackageManager.TokenEnv

	// Tracker
	cfg.Tracker.Provider = raw.Tracker.Provider
	cfg.Tracker.Host = raw.Tracker.Host
	cfg.Tracker.TokenEnv = stringOr(raw.Tracker.TokenEnv, defaultTrackerTokenEnv(raw.Tracker.Provider))
	cfg.Tracker.EmailEnv = raw.Tracker.EmailEnv
	cfg.Tracker.ProjectKey = raw.Tracker.ProjectKey

	// Changelog
	cfg.Changelog.Enabled = boolOr(raw.Changelog.Enabled, true)
	cfg.Changelog.Path = stringOr(raw.Changelog.Path, "CHANGELOG.md")

	// Tasks
	cfg.Tasks.Test = raw.Tasks.Test
	cfg.Tasks.Build = raw.Tasks.Build
	cfg.Tasks.SecretScanning = boolOr(raw.Tasks.SecretScanning, true)

	// Hooks
	cfg.Hooks.BeforeRelease = raw.Hooks.BeforeRelease
	cfg.Hooks.AfterRelease = raw.Hooks.AfterRelease

	return cfg
}

func defaultTrackerTokenEnv(provider string) string {
	switch provider {
	case "github":
		return "GITHUB_TOKEN"
	case "gitea":
		return "GITEA_TOKEN"
	case "forgejo":
		return "FORGEJO_TOKEN"
	case "codeberg":
		return "CODEBERG_TOKEN"
	case "jira":
		return "JIRA_TOKEN"
	case "openproject":
		return "OPENPROJECT_TOKEN"
	default:
		return ""
	}
}

func defaultSCMTokenEnv(provider string) string {
	switch provider {
	case "github":
		return "GITHUB_TOKEN"
	case "gitlab":
		return "GITLAB_TOKEN"
	case "gitea":
		return "GITEA_TOKEN"
	case "forgejo":
		return "FORGEJO_TOKEN"
	case "codeberg":
		return "CODEBERG_TOKEN"
	default:
		return ""
	}
}

// validateTokens checks that each configured tokenEnv variable is set in the environment.
func validateTokens(cfg *Config) error {
	check := func(envVar, component string) error {
		if envVar == "" {
			return nil
		}
		if os.Getenv(envVar) == "" {
			return fmt.Errorf("env var %q (required for %s) is not set — add it to your .env file", envVar, component)
		}
		return nil
	}

	if err := check(cfg.SCM.TokenEnv, "scm."+cfg.SCM.Provider); err != nil {
		return err
	}
	if err := check(cfg.PackageManager.TokenEnv, "pm"); err != nil {
		return err
	}
	if err := check(cfg.Tracker.TokenEnv, "tracker."+cfg.Tracker.Provider); err != nil {
		return err
	}
	return nil
}

// mergeRaw overlays non-zero fields from overlay onto a copy of base.
func mergeRaw(base, overlay *rawConfig) *rawConfig {
	m := *base

	if overlay.Git.DefaultBranch != "" {
		m.Git.DefaultBranch = overlay.Git.DefaultBranch
	}
	if overlay.Git.DevelopmentBranch != "" {
		m.Git.DevelopmentBranch = overlay.Git.DevelopmentBranch
	}
	if overlay.Git.TagPrefix != "" {
		m.Git.TagPrefix = overlay.Git.TagPrefix
	}
	if overlay.Git.Remote != "" {
		m.Git.Remote = overlay.Git.Remote
	}
	if overlay.Git.ReleaseCommitTemplate != "" {
		m.Git.ReleaseCommitTemplate = overlay.Git.ReleaseCommitTemplate
	}
	if overlay.Git.ReleaseBranchTemplate != "" {
		m.Git.ReleaseBranchTemplate = overlay.Git.ReleaseBranchTemplate
	}

	if overlay.Versioning.Scheme != "" {
		m.Versioning.Scheme = overlay.Versioning.Scheme
	}
	if len(overlay.Versioning.PlaceholderExclude) > 0 {
		m.Versioning.PlaceholderExclude = overlay.Versioning.PlaceholderExclude
	}

	if overlay.SCM.Provider != "" {
		m.SCM.Provider = overlay.SCM.Provider
	}
	if overlay.SCM.Host != "" {
		m.SCM.Host = overlay.SCM.Host
	}
	if overlay.SCM.TokenEnv != "" {
		m.SCM.TokenEnv = overlay.SCM.TokenEnv
	}

	if overlay.PackageManager.Provider != "" {
		m.PackageManager.Provider = overlay.PackageManager.Provider
	}
	if overlay.PackageManager.Host != "" {
		m.PackageManager.Host = overlay.PackageManager.Host
	}
	if overlay.PackageManager.TokenEnv != "" {
		m.PackageManager.TokenEnv = overlay.PackageManager.TokenEnv
	}

	if overlay.Tracker.Provider != "" {
		m.Tracker.Provider = overlay.Tracker.Provider
	}
	if overlay.Tracker.Host != "" {
		m.Tracker.Host = overlay.Tracker.Host
	}
	if overlay.Tracker.TokenEnv != "" {
		m.Tracker.TokenEnv = overlay.Tracker.TokenEnv
	}
	if overlay.Tracker.EmailEnv != "" {
		m.Tracker.EmailEnv = overlay.Tracker.EmailEnv
	}
	if overlay.Tracker.ProjectKey != "" {
		m.Tracker.ProjectKey = overlay.Tracker.ProjectKey
	}

	if overlay.Changelog.Enabled != nil {
		m.Changelog.Enabled = overlay.Changelog.Enabled
	}
	if overlay.Changelog.Path != "" {
		m.Changelog.Path = overlay.Changelog.Path
	}

	if len(overlay.Tasks.Test) > 0 {
		m.Tasks.Test = overlay.Tasks.Test
	}
	if len(overlay.Tasks.Build) > 0 {
		m.Tasks.Build = overlay.Tasks.Build
	}
	if overlay.Tasks.SecretScanning != nil {
		m.Tasks.SecretScanning = overlay.Tasks.SecretScanning
	}

	if overlay.Hooks.BeforeRelease != "" {
		m.Hooks.BeforeRelease = overlay.Hooks.BeforeRelease
	}
	if overlay.Hooks.AfterRelease != "" {
		m.Hooks.AfterRelease = overlay.Hooks.AfterRelease
	}

	return &m
}

func stringOr(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

func boolOr(b *bool, fallback bool) bool {
	if b != nil {
		return *b
	}
	return fallback
}

// --- raw deserialization types ---

type rawConfig struct {
	Git        rawGitConfig        `json:"git"`
	Versioning rawVersioningConfig `json:"versioning"`
	SCM        rawSCMConfig        `json:"scm"`
	PackageManager rawPackageManagerConfig `json:"pm"`
	Tracker    rawTrackerConfig    `json:"tracker"`
	Changelog  rawChangelogConfig  `json:"changelog"`
	Tasks      rawTasksConfig      `json:"tasks"`
	Hooks      rawHooksConfig      `json:"hooks"`
}

type rawTrackerConfig struct {
	Provider   string `json:"provider"`
	Host       string `json:"host"`
	TokenEnv   string `json:"tokenEnv"`
	EmailEnv   string `json:"emailEnv"`
	ProjectKey string `json:"projectKey"`
}

type rawGitConfig struct {
	DefaultBranch         string `json:"defaultBranch"`
	DevelopmentBranch     string `json:"developmentBranch"`
	TagPrefix             string `json:"tagPrefix"`
	Remote                string `json:"remote"`
	ReleaseCommitTemplate string `json:"releaseCommitTemplate"`
	ReleaseBranchTemplate string `json:"releaseBranchTemplate"`
}

type rawVersioningConfig struct {
	Scheme             string   `json:"scheme"`
	PlaceholderExclude []string `json:"placeholderExclude"`
}

type rawSCMConfig struct {
	Provider string `json:"provider"`
	Host     string `json:"host"`
	TokenEnv string `json:"tokenEnv"`
}

type rawPackageManagerConfig struct {
	Provider string `json:"provider"`
	Host     string `json:"host"`
	TokenEnv string `json:"tokenEnv"`
}

type rawChangelogConfig struct {
	Enabled *bool  `json:"enabled"` // pointer to distinguish absent from false
	Path    string `json:"path"`
}

type rawTasksConfig struct {
	Test           []TaskRef `json:"test"`
	Build          []TaskRef `json:"build"`
	SecretScanning *bool     `json:"secretScanning"` // pointer: distinguish absent from false
}

type rawHooksConfig struct {
	BeforeRelease string `json:"beforeRelease"`
	AfterRelease  string `json:"afterRelease"`
}
