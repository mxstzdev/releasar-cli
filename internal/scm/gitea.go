package scm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mxstzdev/releasar-cli/internal/log"
)

const (
	giteaDefaultBaseURL    = "https://gitea.com"
	codebergBaseURL        = "https://codeberg.org"
)

// gitea is the Gitea/Forgejo/Codeberg SCM adapter. All three platforms expose an identical API.
// Gitea SaaS uses https://gitea.com; Codeberg uses https://codeberg.org. Self-hosted Gitea and
// Forgejo instances always require an explicit host URL via scm.host.
// Required token permission: "issue" + "write:repository" scopes (Gitea/Forgejo/Codeberg).
type gitea struct {
	baseURL string
	token   string
	owner   string
	repo    string
	http    *http.Client
	log     *log.Channel
}

func newGitea(cfg Config, defaultHost string, log *log.Channel) (*gitea, error) {
	host := cfg.Host
	if host == "" {
		host = defaultHost
	}
	if host == "" {
		return nil, fmt.Errorf("scm.host is required for Gitea/Forgejo (no public SaaS default)")
	}
	return &gitea{
		baseURL: host,
		token:   cfg.Token,
		owner:   cfg.Owner,
		repo:    cfg.Repo,
		http:    &http.Client{},
		log:     log,
	}, nil
}

// CreateRelease publishes a Gitea/Forgejo release against the given tag.
func (g *gitea) CreateRelease(tag, name, body string) (string, error) {
	payload := struct {
		TagName    string `json:"tag_name"`
		Name       string `json:"name"`
		Body       string `json:"body"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}{
		TagName: tag,
		Name:    name,
		Body:    body,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling release payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/releases", g.baseURL, g.owner, g.repo)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("building release request: %w", err)
	}
	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		g.log.Error("Gitea release request failed", map[string]any{"endpoint": endpoint, "error": err})
		return "", fmt.Errorf("sending release request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading release response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		g.log.Error("Gitea API error", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
		return "", fmt.Errorf("Gitea/Forgejo API returned %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing release response: %w", err)
	}

	g.log.Debug("Gitea release created", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
	return result.HTMLURL, nil
}
