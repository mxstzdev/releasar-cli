package scm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mxstzdev/releasar-cli/internal/log"
)

const githubDefaultBaseURL = "https://api.github.com"

// gitHub is the GitHub SCM adapter.
// Required token scopes: "repo" for private repositories, "public_repo" for public ones.
type gitHub struct {
	baseURL string
	token   string
	owner   string
	repo    string
	http    *http.Client
	log     *log.Channel
}

func newGitHub(cfg Config, log *log.Channel) *gitHub {
	baseURL := githubDefaultBaseURL
	if cfg.Host != "" {
		baseURL = cfg.Host
	}
	return &gitHub{
		baseURL: baseURL,
		token:   cfg.Token,
		owner:   cfg.Owner,
		repo:    cfg.Repo,
		http:    &http.Client{},
		log:     log,
	}
}

// CreateRelease publishes a GitHub release against the given tag.
func (g *gitHub) CreateRelease(tag, name, body string) (string, error) {
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

	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases", g.baseURL, g.owner, g.repo)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("building release request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.http.Do(req)
	if err != nil {
		g.log.Error("GitHub release request failed", map[string]any{"endpoint": endpoint, "error": err})
		return "", fmt.Errorf("sending release request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading release response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		g.log.Error("GitHub API error", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
		return "", fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing release response: %w", err)
	}

	g.log.Debug("GitHub release created", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
	return result.HTMLURL, nil
}
