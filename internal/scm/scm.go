package scm

import (
	"fmt"

	"github.com/mxstzdev/releasar-cli/internal/log"
)

// Provider creates releases on a hosted SCM platform.
type Provider interface {
	CreateRelease(tag, name, body string) (url string, err error)
}

// Config holds resolved parameters for an SCM adapter.
type Config struct {
	Host  string // base API URL; adapter uses provider default if empty
	Token string // resolved API token
	Owner string // repository owner, parsed from remote URL
	Repo  string // repository name, parsed from remote URL
}

// New returns a Provider for the given provider name.
func New(provider string, cfg Config, log *log.Channel) (Provider, error) {
	switch provider {
	case "github":
		return newGitHub(cfg, log), nil
	case "gitlab":
		return newGitLab(cfg, log), nil
	case "codeberg":
		return newGitea(cfg, codebergBaseURL, log)
	case "gitea":
		return newGitea(cfg, giteaDefaultBaseURL, log)
	case "forgejo":
		return newGitea(cfg, "", log)
	default:
		return nil, fmt.Errorf("unknown SCM provider %q", provider)
	}
}
