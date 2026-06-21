package scm

import "fmt"

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
func New(provider string, cfg Config) (Provider, error) {
	switch provider {
	case "github":
		return newGitHub(cfg), nil
	case "gitlab":
		return newGitLab(cfg), nil
	case "codeberg":
		return newGitea(cfg, codebergBaseURL)
	case "gitea":
		return newGitea(cfg, giteaDefaultBaseURL)
	case "forgejo":
		return newGitea(cfg, "")
	default:
		return nil, fmt.Errorf("unknown SCM provider %q", provider)
	}
}
