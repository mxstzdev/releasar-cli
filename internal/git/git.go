package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mariusvslprts/releasar-cli/internal/log"
)

type Config struct {
	DefaultBranch     string // name of the project default branch
	DevelopmentBranch string // name of the project development branch
	TagPrefix         string // prefix uses for tags, e.g. "myapp/" or "myapp:"
}

type Client struct {
	cfg              Config       // component configuration
	log              *log.Channel // component log channel
	version          string       // git version
	rootDirectory    string       // name of the repository root directory
	workingDirectory string       // name of the project working directory
	defaultBranch    string       // name of the repository default branch
}

type Commit struct {
	Hash      string
	Author    string
	Timestamp time.Time
	Subject   string
	Body      string
}

func New(workingDir string, cfg Config, log *log.Channel) (*Client, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git is not installed or not in PATH: %w", err)
	}

	workingDir, err := filepath.Abs(workingDir)

	if err != nil {
		return nil, fmt.Errorf("resolving working directory: %w", err)
	}

	c := &Client{
		cfg:              cfg,
		log:              log,
		workingDirectory: workingDir,
	}

	if !c.isRepository() {
		return nil, fmt.Errorf("working directory is not inside a git repository")
	}

	version, err := c.gitVersion()
	if err != nil {
		return nil, fmt.Errorf("detecting git version: %w", err)
	}
	c.version = version

	rootDirectory, err := c.repositoryRootDirectory()
	if err != nil {
		return nil, fmt.Errorf("detecting repository root directory: %w", err)
	}
	c.rootDirectory = rootDirectory

	defaultBranch := cfg.DefaultBranch
	if defaultBranch == "" {
		defaultBranch, err = c.repositoryDefaultBranch()
		if err != nil {
			return nil, fmt.Errorf("detecting repository default branch: %w", err)
		}
	}
	c.defaultBranch = defaultBranch

	return c, nil
}

func (c *Client) exec(command string, args ...string) (string, error) {
	dir := c.rootDirectory
	if dir == "" {
		dir = c.workingDirectory
	}

	cmd := exec.Command("git", append([]string{command}, args...)...)
	cmd.Dir = dir

	out, err := cmd.Output()

	if err != nil {
		c.log.Error("Git command failed", map[string]any{
			"command": "git " + command + " " + strings.Join(args, " "),
			"error":   err,
		})

		var exitErr *exec.ExitError

		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("git %s: %s", command, strings.TrimSpace(string(exitErr.Stderr)))
		}

		return "", fmt.Errorf("git %s: %w", command, err)
	}

	c.log.Debug("Git command executed", map[string]any{
		"command": "git " + command + " " + strings.Join(args, " "),
	})

	return strings.TrimSpace(string(out)), nil
}

func (c *Client) gitVersion() (string, error) {
	out, err := c.exec("version")

	if err != nil {
		return "", fmt.Errorf("detecting git version: %w", err)
	}

	return strings.TrimPrefix(out, "git version "), nil
}

func (c *Client) isRepository() bool {
	_, err := c.exec("rev-parse", "--is-inside-work-tree")

	if err != nil {
		return false
	}

	return true
}

func (c *Client) hasRemote() bool {
	out, err := c.exec("remote")

	return err == nil && out != ""
}

func (c *Client) repositoryRootDirectory() (string, error) {
	if c.rootDirectory == "" {
		out, err := c.exec("rev-parse", "--show-toplevel")

		if err != nil {
			return "", fmt.Errorf("detecting repository root: %w", err)
		}

		rootDir, err := filepath.Abs(out)

		if err != nil {
			return "", fmt.Errorf("resolving root directory: %w", err)
		}

		c.rootDirectory = rootDir
	}

	return c.rootDirectory, nil
}

func (c *Client) repositoryDefaultBranch() (string, error) {
	if c.hasRemote() {
		out, err := c.exec("symbolic-ref", "refs/remotes/origin/HEAD")
		if err == nil {
			return strings.TrimPrefix(out, "refs/remotes/origin/"), nil
		}
	}

	out, err := c.exec("config", "--get", "init.defaultBranch")
	if err == nil && out != "" {
		return out, nil
	}

	for _, candidate := range []string{"main", "master"} {
		if _, err := c.exec("rev-parse", "--verify", candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not determine default branch")
}

func (c *Client) Pull(remote, ref string) error {
	_, err := c.exec("pull", remote, ref)
	if err != nil {
		return fmt.Errorf("pulling %s from %s: %w", ref, remote, err)
	}
	return nil
}

func (c *Client) Push(remote, ref string) error {
	_, err := c.exec("push", remote, ref)
	if err != nil {
		return fmt.Errorf("pushing %s to %s: %w", ref, remote, err)
	}
	return nil
}

func (c *Client) CurrentBranch() (string, error) {
	out, err := c.exec("rev-parse", "--abbrev-ref", "HEAD")

	if err != nil {
		return "", fmt.Errorf("detecting current branch: %w", err)
	}

	return out, nil
}

func (c *Client) CreateBranch(name string, base string) error {
	args := []string{"-c", name}

	if base != "" {
		args = append(args, base)
	}

	_, err := c.exec("switch", args...)

	if err != nil {
		return fmt.Errorf("creating new branch %s: %w", name, err)
	}

	return nil
}

func (c *Client) Switch(branch string) error {
	_, err := c.exec("switch", branch)

	if err != nil {
		return fmt.Errorf("switching to branch %s: %w", branch, err)
	}

	return nil
}

func (c *Client) Merge(branch string, args ...string) error {
	_, err := c.exec("merge", append([]string{branch}, args...)...)

	if err != nil {
		return fmt.Errorf("merging branch %s: %w", branch, err)
	}

	return nil
}

// RevParse runs git rev-parse with the given arguments and returns the trimmed output.
func (c *Client) RevParse(args ...string) (string, error) {
	out, err := c.exec("rev-parse", args...)
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// Reset runs git reset with optional flags followed by the ref.
func (c *Client) Reset(ref string, args ...string) error {
	_, err := c.exec("reset", append(args, ref)...)
	if err != nil {
		return fmt.Errorf("resetting to %s: %w", ref, err)
	}
	return nil
}

// Checkout runs git checkout with the given arguments.
func (c *Client) Checkout(args ...string) error {
	_, err := c.exec("checkout", args...)
	if err != nil {
		return fmt.Errorf("checkout %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// DeleteBranch force-deletes a local branch.
func (c *Client) DeleteBranch(name string) error {
	_, err := c.exec("branch", "-D", name)
	if err != nil {
		return fmt.Errorf("deleting branch %s: %w", name, err)
	}
	return nil
}

// DeleteTag deletes a local tag.
func (c *Client) DeleteTag(name string) error {
	_, err := c.exec("tag", "-d", c.tagName(name))
	if err != nil {
		return fmt.Errorf("deleting tag %s: %w", name, err)
	}
	return nil
}

// Remotes returns the list of configured remote names.
func (c *Client) Remotes() ([]string, error) {
	out, err := c.exec("remote")
	if err != nil {
		return nil, fmt.Errorf("listing remotes: %w", err)
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// ConflictedFiles returns the paths of files with unresolved merge conflicts.
func (c *Client) ConflictedFiles() ([]string, error) {
	out, err := c.exec("diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, fmt.Errorf("listing conflicted files: %w", err)
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func (c *Client) Add(files ...string) error {
	_, err := c.exec("add", append([]string{"--"}, files...)...)

	if err != nil {
		return fmt.Errorf("adding files (%s): %w", strings.Join(files, ", "), err)
	}

	return nil
}

func (c *Client) Commit(message string) error {
	_, err := c.exec("commit", "-m", message)

	if err != nil {
		return fmt.Errorf("creating commit: %w", err)
	}

	return nil
}

func (c *Client) tagName(name string) string {
	if strings.HasPrefix(name, c.cfg.TagPrefix) {
		return name
	}
	return c.cfg.TagPrefix + name
}

func (c *Client) LatestTag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", c.tagName("*"))
	cmd.Dir = c.rootDirectory
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 128 {
			return "", nil
		}
		return "", fmt.Errorf("detecting latest tag: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) Tag(name string, message string) error {
	var args []string
	if message != "" {
		args = []string{"-a", c.tagName(name), "-m", message}
	} else {
		args = []string{c.tagName(name)}
	}
	_, err := c.exec("tag", args...)
	if err != nil {
		return fmt.Errorf("creating tag %s: %w", name, err)
	}
	return nil
}

func (c *Client) Log(since string) ([]Commit, error) {
	args := []string{"log", "--first-parent", "--pretty=format:%x00%H\n%an\n%aI\n%s\n%b"}
	if since != "" {
		args = append(args, since+"..HEAD")
	}
	args = append(args, "--", c.workingDirectory)

	cmd := exec.Command("git", args...)
	cmd.Dir = c.rootDirectory
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("reading commit log: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("reading commit log: %w", err)
	}

	return parseLog(out)
}

func parseLog(data []byte) ([]Commit, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	var commits []Commit
	for _, record := range bytes.Split(data, []byte{0}) {
		record = bytes.TrimSpace(record)
		if len(record) == 0 {
			continue
		}
		lines := bytes.SplitN(record, []byte{'\n'}, 5)
		if len(lines) < 4 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(string(lines[2])))
		if err != nil {
			return nil, fmt.Errorf("parsing commit timestamp %q: %w", string(lines[2]), err)
		}
		commit := Commit{
			Hash:      string(bytes.TrimSpace(lines[0])),
			Author:    string(bytes.TrimSpace(lines[1])),
			Timestamp: ts,
			Subject:   string(bytes.TrimSpace(lines[3])),
		}
		if len(lines) == 5 {
			commit.Body = string(bytes.TrimSpace(lines[4]))
		}
		commits = append(commits, commit)
	}
	return commits, nil
}
