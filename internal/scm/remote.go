package scm

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseRemoteURL extracts the owner and repository name from a git remote URL.
// Supports SSH (git@host:owner/repo.git) and HTTPS (https://host/owner/repo[.git]) forms.
func ParseRemoteURL(rawURL string) (owner, repo string, err error) {
	rawURL = strings.TrimSpace(rawURL)

	// SSH form: git@github.com:owner/repo.git
	if strings.HasPrefix(rawURL, "git@") {
		// Strip the "git@host:" prefix.
		idx := strings.Index(rawURL, ":")
		if idx == -1 {
			return "", "", fmt.Errorf("cannot parse SSH remote URL %q: missing colon separator", rawURL)
		}
		path := rawURL[idx+1:]
		return splitOwnerRepo(path)
	}

	// HTTPS form: https://host/owner/repo[.git]
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("cannot parse remote URL %q: %w", rawURL, err)
	}
	return splitOwnerRepo(strings.TrimPrefix(u.Path, "/"))
}

func splitOwnerRepo(path string) (owner, repo string, err error) {
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("cannot extract owner/repo from path %q", path)
	}
	return parts[0], parts[1], nil
}
