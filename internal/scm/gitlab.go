package scm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const gitlabDefaultBaseURL = "https://gitlab.com"

// gitLab is the GitLab SCM adapter.
// Works for gitlab.com (auto-detected by default host) and self-hosted GitLab instances
// (requires host in config).
// Required token permission: "api" scope or "Maintainer" role with release creation access.
type gitLab struct {
	baseURL string
	token   string
	owner   string
	repo    string
	http    *http.Client
}

func newGitLab(cfg Config) *gitLab {
	baseURL := gitlabDefaultBaseURL
	if cfg.Host != "" {
		baseURL = cfg.Host
	}
	return &gitLab{
		baseURL: baseURL,
		token:   cfg.Token,
		owner:   cfg.Owner,
		repo:    cfg.Repo,
		http:    &http.Client{},
	}
}

// CreateRelease publishes a GitLab release against the given tag.
func (g *gitLab) CreateRelease(tag, name, body string) (string, error) {
	payload := struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}{
		TagName:     tag,
		Name:        name,
		Description: body,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling release payload: %w", err)
	}

	// GitLab identifies projects by URL-encoded "namespace/repo" path.
	projectID := url.PathEscape(g.owner + "/" + g.repo)
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/releases", g.baseURL, projectID)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("building release request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", g.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending release request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading release response: %w", err)
	}

	// GitLab Create Release returns 200, not 201.
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitLab API returned %d: %s", resp.StatusCode, respBody)
	}

	// Construct the web URL from known parts; _links.self is the API URL, not the web URL.
	releaseURL := fmt.Sprintf("%s/%s/%s/-/releases/%s", g.baseURL, g.owner, g.repo, url.PathEscape(tag))
	return releaseURL, nil
}
