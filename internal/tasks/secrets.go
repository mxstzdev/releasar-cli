package tasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	glconfig "github.com/zricethezav/gitleaks/v8/config"
	"github.com/zricethezav/gitleaks/v8/detect"
	gllogging "github.com/zricethezav/gitleaks/v8/logging"
	"github.com/zricethezav/gitleaks/v8/report"
	"github.com/zricethezav/gitleaks/v8/sources"
)

// RunSecretsScan scans sourceDir for secrets using gitleaks rules.
// It auto-detects .gitleaks.toml in sourceDir; otherwise built-in rules apply.
// Findings are reported by rule ID, file, and line — secret values are never printed.
func RunSecretsScan(sourceDir string, verbose bool) error {
	// Silence gitleaks' own logging so releasar controls all terminal output.
	saved := gllogging.Logger
	gllogging.Logger = gllogging.Logger.Level(zerolog.Disabled)
	defer func() { gllogging.Logger = saved }()

	detector, err := loadDetector(sourceDir)
	if err != nil {
		return fmt.Errorf("initializing secret scanner: %w", err)
	}

	findings, err := detector.DetectSource(context.Background(), &sources.Files{
		Config: &detector.Config,
		Path:   sourceDir,
		Sema:   detector.Sema,
	})
	if err != nil {
		return fmt.Errorf("scanning for secrets: %w", err)
	}

	if len(findings) == 0 {
		return nil
	}

	printFindings(findings)
	return fmt.Errorf("%d potential secret(s) detected — fix or add a gitleaks:allow comment before releasing", len(findings))
}

// printFindings writes finding locations to stderr. Secret values are intentionally omitted.
func printFindings(findings []report.Finding) {
	for _, f := range findings {
		fmt.Fprintf(os.Stderr, "  [%s] %s:%d\n", f.RuleID, f.File, f.StartLine)
	}
}

// loadDetector creates a detector using .gitleaks.toml from sourceDir when present,
// falling back to gitleaks' built-in default rules.
func loadDetector(sourceDir string) (*detect.Detector, error) {
	customCfgPath := filepath.Join(sourceDir, ".gitleaks.toml")
	if _, err := os.Stat(customCfgPath); os.IsNotExist(err) {
		return detect.NewDetectorDefaultConfig()
	}

	v := viper.New()
	v.SetConfigType("toml")
	v.SetConfigFile(customCfgPath)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading .gitleaks.toml: %w", err)
	}

	var vc glconfig.ViperConfig
	if err := v.Unmarshal(&vc); err != nil {
		return nil, fmt.Errorf("parsing .gitleaks.toml: %w", err)
	}

	cfg, err := vc.Translate()
	if err != nil {
		return nil, fmt.Errorf("translating .gitleaks.toml: %w", err)
	}

	return detect.NewDetector(cfg), nil
}
