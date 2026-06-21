package tracker

import "fmt"

// Version represents a named release version or milestone in an issue tracker.
type Version struct {
	ID   string
	Name string
	Open bool // false if already closed/released
}

// ResolvedRef is a ticket reference with its current state in the tracker.
type ResolvedRef struct {
	Ref   string // original ref string, e.g. "#123" or "PROJ-456"
	Title string
	State string // tracker-native state, e.g. "open", "closed", "In Progress"
	URL   string
}

// Tracker manages issue tracker versions and ticket assignments.
type Tracker interface {
	// ListVersions returns open versions/milestones for the configured project.
	ListVersions() ([]Version, error)

	// CreateVersion creates a new milestone/version and returns its tracker-native ID.
	CreateVersion(name, description string) (id string, err error)

	// ResolveRefs looks up each ref and returns its current state in the tracker.
	// Refs that do not match this tracker's format or are not found are omitted.
	// Adapters batch API calls where the platform supports it.
	ResolveRefs(refs []string) ([]ResolvedRef, error)

	// AssignTickets links the given refs to the version identified by versionID.
	// All refs are attempted; a combined error is returned listing any that failed.
	AssignTickets(refs []string, versionID string) error

	// CloseVersion marks the version as released/closed.
	CloseVersion(versionID string) error
}

// Config holds resolved runtime parameters for a tracker adapter.
type Config struct {
	Host       string // base API URL; adapter uses provider default if empty
	Token      string // resolved API token
	Email      string // Jira only: account email for Basic auth
	ProjectKey string // Jira: project key (e.g. "PROJ"); OpenProject: numeric project ID
	Owner      string // GitHub/Gitea: repo owner, inherited from SCM config
	Repo       string // GitHub/Gitea: repo name, inherited from SCM config
}

// New returns a Tracker for the given provider name.
func New(provider string, cfg Config) (Tracker, error) {
	switch provider {
	case "github":
		return newGitHub(cfg), nil
	case "gitea":
		return newGitea(cfg, giteaDefaultBaseURL)
	case "forgejo":
		return newGitea(cfg, "")
	case "codeberg":
		return newGitea(cfg, codebergBaseURL)
	case "jira":
		return newJira(cfg)
	case "openproject":
		return newOpenProject(cfg)
	default:
		return nil, fmt.Errorf("unknown tracker provider %q", provider)
	}
}
