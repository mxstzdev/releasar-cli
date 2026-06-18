package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads .env files in precedence order and merges them into the
// process environment via os.Setenv. Must be called before Load().
//
// Load order (later entries override earlier):
//  1. {repoRoot}/.env
//  2. {workingDir}/.env  (skipped when equal to repoRoot)
//  3. extraFiles          (from --env-file CLI flag)
//
// Missing files are silently skipped. Unreadable files return an error.
// Warns to stderr if a loaded .env file is not listed in the nearest .gitignore.
func LoadDotEnv(repoRoot, workingDir string, extraFiles ...string) error {
	candidates := []string{repoRoot + "/.env"}
	if workingDir != repoRoot {
		candidates = append(candidates, workingDir+"/.env")
	}
	candidates = append(candidates, extraFiles...)

	for _, path := range candidates {
		if err := loadDotEnvFile(path); err != nil {
			return err
		}
		warnIfNotGitignored(path)
	}
	return nil
}

// loadDotEnvFile loads a single .env file into the process environment.
// Returns nil if the file does not exist.
func loadDotEnvFile(path string) error {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking .env file %s: %w", path, err)
	}
	if err := godotenv.Overload(path); err != nil {
		return fmt.Errorf("loading .env file %s: %w", path, err)
	}
	return nil
}

// warnIfNotGitignored prints a warning to stderr if path is not covered by .gitignore.
// This is a best-effort check; failures are silently ignored.
func warnIfNotGitignored(path string) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	// git check-ignore -q exits 0 if the path is ignored, 1 if not.
	err := exec.Command("git", "check-ignore", "-q", path).Run()
	if err == nil {
		return // file is gitignored
	}
	fmt.Fprintf(os.Stderr, "warning: %s is not listed in .gitignore — add it to avoid committing secrets\n", path)
}
