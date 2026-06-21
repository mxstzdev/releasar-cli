package tracker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const githubDefaultBaseURL = "https://api.github.com"

// gitHub is the GitHub issue tracker adapter using milestones as versions.
// Required token scopes: "repo" for private repositories, "public_repo" for public ones.
type gitHub struct {
	baseURL string
	token   string
	owner   string
	repo    string
	http    *http.Client
}

var githubRefPattern = regexp.MustCompile(`^#(\d+)$`)

func newGitHub(cfg Config) *gitHub {
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
	}
}

func (g *gitHub) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

// ListVersions returns open milestones for the repository.
func (g *gitHub) ListVersions() ([]Version, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/milestones?state=open&per_page=100", g.baseURL, g.owner, g.repo)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building milestones request: %w", err)
	}
	g.setHeaders(req)

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching milestones: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading milestones response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, body)
	}

	var milestones []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(body, &milestones); err != nil {
		return nil, fmt.Errorf("parsing milestones response: %w", err)
	}

	versions := make([]Version, len(milestones))
	for i, m := range milestones {
		versions[i] = Version{
			ID:   strconv.Itoa(m.Number),
			Name: m.Title,
			Open: m.State == "open",
		}
	}
	return versions, nil
}

// CreateVersion creates a GitHub milestone and returns its number as a string ID.
func (g *gitHub) CreateVersion(name, description string) (string, error) {
	payload := struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}{Title: name, Description: description}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling milestone payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/milestones", g.baseURL, g.owner, g.repo)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("building milestone request: %w", err)
	}
	g.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("creating milestone: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading milestone response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing milestone response: %w", err)
	}
	return strconv.Itoa(result.Number), nil
}

// ResolveRefs looks up each numeric GitHub ref (#N) and returns its current state.
// Non-numeric refs (e.g. JIRA-123) are silently skipped.
// GitHub has no batch issue lookup endpoint; one request is made per ref.
func (g *gitHub) ResolveRefs(refs []string) ([]ResolvedRef, error) {
	var resolved []ResolvedRef
	var errs []string

	for _, ref := range refs {
		m := githubRefPattern.FindStringSubmatch(ref)
		if m == nil {
			continue
		}
		endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%s", g.baseURL, g.owner, g.repo, m[1])
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: building request: %v", ref, err))
			continue
		}
		g.setHeaders(req)

		resp, err := g.http.Do(req)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ref, err))
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			errs = append(errs, fmt.Sprintf("%s: GitHub API returned %d", ref, resp.StatusCode))
			continue
		}

		var issue struct {
			Title   string `json:"title"`
			State   string `json:"state"`
			HTMLURL string `json:"html_url"`
		}
		if err := json.Unmarshal(body, &issue); err != nil {
			errs = append(errs, fmt.Sprintf("%s: parsing response: %v", ref, err))
			continue
		}
		resolved = append(resolved, ResolvedRef{
			Ref:   ref,
			Title: issue.Title,
			State: issue.State,
			URL:   issue.HTMLURL,
		})
	}

	if len(errs) > 0 {
		return resolved, fmt.Errorf("resolving refs: %s", strings.Join(errs, "; "))
	}
	return resolved, nil
}

// AssignTickets sets the given milestone on each numeric issue ref.
// All refs are attempted; a combined error is returned for any that failed.
func (g *gitHub) AssignTickets(refs []string, versionID string) error {
	milestoneNum, err := strconv.Atoi(versionID)
	if err != nil {
		return fmt.Errorf("invalid milestone ID %q: %w", versionID, err)
	}

	payload := struct {
		Milestone int `json:"milestone"`
	}{Milestone: milestoneNum}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling assignment payload: %w", err)
	}

	var errs []string
	for _, ref := range refs {
		m := githubRefPattern.FindStringSubmatch(ref)
		if m == nil {
			continue
		}
		endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%s", g.baseURL, g.owner, g.repo, m[1])
		req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(data))
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: building request: %v", ref, err))
			continue
		}
		g.setHeaders(req)
		req.Header.Set("Content-Type", "application/json")

		resp, err := g.http.Do(req)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ref, err))
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			errs = append(errs, fmt.Sprintf("%s: GitHub API returned %d: %s", ref, resp.StatusCode, body))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("assigning tickets: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CloseVersion closes the GitHub milestone identified by its numeric ID.
func (g *gitHub) CloseVersion(versionID string) error {
	payload := struct {
		State string `json:"state"`
	}{State: "closed"}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling close payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/milestones/%s", g.baseURL, g.owner, g.repo, versionID)
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building close request: %w", err)
	}
	g.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("closing milestone: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, body)
	}
	return nil
}
