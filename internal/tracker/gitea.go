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

	"github.com/mxstzdev/releasar-cli/internal/log"
)

const (
	giteaDefaultBaseURL = "https://gitea.com"
	codebergBaseURL     = "https://codeberg.org"
)

// giteaTracker is the Gitea/Forgejo/Codeberg issue tracker adapter using milestones as versions.
// All three platforms expose an identical API.
// Required token scopes: "issue" (Gitea/Forgejo/Codeberg).
type giteaTracker struct {
	baseURL string
	token   string
	owner   string
	repo    string
	http    *http.Client
	log     *log.Channel
}

var giteaRefPattern = regexp.MustCompile(`^#(\d+)$`)

func newGitea(cfg Config, defaultHost string, log *log.Channel) (*giteaTracker, error) {
	host := cfg.Host
	if host == "" {
		host = defaultHost
	}
	if host == "" {
		return nil, fmt.Errorf("tracker.host is required for Gitea/Forgejo (no public SaaS default)")
	}
	return &giteaTracker{
		baseURL: host,
		token:   cfg.Token,
		owner:   cfg.Owner,
		repo:    cfg.Repo,
		http:    &http.Client{},
		log:     log,
	}, nil
}

func (g *giteaTracker) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Content-Type", "application/json")
}

// ListVersions returns open milestones for the repository.
func (g *giteaTracker) ListVersions() ([]Version, error) {
	endpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/milestones?state=open&limit=50", g.baseURL, g.owner, g.repo)
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
		return nil, fmt.Errorf("Gitea API returned %d: %s", resp.StatusCode, body)
	}

	var milestones []struct {
		ID     int64  `json:"id"`
		Title  string `json:"title"`
		Closed bool   `json:"closed"`
	}
	if err := json.Unmarshal(body, &milestones); err != nil {
		return nil, fmt.Errorf("parsing milestones response: %w", err)
	}

	versions := make([]Version, len(milestones))
	for i, m := range milestones {
		versions[i] = Version{
			ID:   strconv.FormatInt(m.ID, 10),
			Name: m.Title,
			Open: !m.Closed,
		}
	}
	return versions, nil
}

// CreateVersion creates a Gitea milestone and returns its ID as a string.
func (g *giteaTracker) CreateVersion(name, description string) (string, error) {
	payload := struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}{Title: name, Description: description}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling milestone payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/milestones", g.baseURL, g.owner, g.repo)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("building milestone request: %w", err)
	}
	g.setHeaders(req)

	resp, err := g.http.Do(req)
	if err != nil {
		g.log.Error("Gitea milestone request failed", map[string]any{"endpoint": endpoint, "error": err})
		return "", fmt.Errorf("creating milestone: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading milestone response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		g.log.Error("Gitea API error", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
		return "", fmt.Errorf("Gitea API returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing milestone response: %w", err)
	}
	g.log.Debug("Gitea milestone created", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
	return strconv.FormatInt(result.ID, 10), nil
}

// ResolveRefs looks up each numeric ref (#N) and returns its current state.
// Non-numeric refs are silently skipped.
// Gitea has no batch issue lookup endpoint; one request is made per ref.
func (g *giteaTracker) ResolveRefs(refs []string) ([]ResolvedRef, error) {
	var resolved []ResolvedRef
	var errs []string

	for _, ref := range refs {
		m := giteaRefPattern.FindStringSubmatch(ref)
		if m == nil {
			continue
		}
		endpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%s", g.baseURL, g.owner, g.repo, m[1])
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
			errs = append(errs, fmt.Sprintf("%s: Gitea API returned %d", ref, resp.StatusCode))
			continue
		}

		var issue struct {
			Title  string `json:"title"`
			State  string `json:"state"`
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
func (g *giteaTracker) AssignTickets(refs []string, versionID string) error {
	milestoneID, err := strconv.ParseInt(versionID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid milestone ID %q: %w", versionID, err)
	}

	payload := struct {
		Milestone int64 `json:"milestone"`
	}{Milestone: milestoneID}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling assignment payload: %w", err)
	}

	var errs []string
	for _, ref := range refs {
		m := giteaRefPattern.FindStringSubmatch(ref)
		if m == nil {
			continue
		}
		endpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%s", g.baseURL, g.owner, g.repo, m[1])
		req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(data))
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

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			errs = append(errs, fmt.Sprintf("%s: Gitea API returned %d: %s", ref, resp.StatusCode, body))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("assigning tickets: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CloseVersion closes the Gitea milestone identified by its numeric ID.
func (g *giteaTracker) CloseVersion(versionID string) error {
	payload := struct {
		State string `json:"state"`
	}{State: "closed"}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling close payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/milestones/%s", g.baseURL, g.owner, g.repo, versionID)
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building close request: %w", err)
	}
	g.setHeaders(req)

	resp, err := g.http.Do(req)
	if err != nil {
		g.log.Error("Gitea milestone close request failed", map[string]any{"endpoint": endpoint, "error": err})
		return fmt.Errorf("closing milestone: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		g.log.Error("Gitea API error", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
		return fmt.Errorf("Gitea API returned %d: %s", resp.StatusCode, body)
	}
	g.log.Debug("Gitea milestone closed", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
	return nil
}
