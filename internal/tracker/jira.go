package tracker

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/mxstzdev/releasar-cli/internal/log"
)

// jira is the Jira Cloud issue tracker adapter.
// Auth: Basic base64(email:token).
// Required scopes: read:jira-work, write:jira-work.
type jira struct {
	baseURL    string
	authHeader string
	projectKey string
	http       *http.Client
	log        *log.Channel
}

var jiraRefPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*-\d+$`)

func newJira(cfg Config, log *log.Channel) (*jira, error) {
	if cfg.ProjectKey == "" {
		return nil, fmt.Errorf("tracker.projectKey is required for Jira")
	}
	if cfg.Email == "" {
		return nil, fmt.Errorf("tracker email (tracker.emailEnv) is required for Jira Basic auth")
	}
	host := cfg.Host
	if host == "" {
		return nil, fmt.Errorf("tracker.host is required for Jira (e.g. https://mycompany.atlassian.net)")
	}
	host = strings.TrimRight(host, "/")

	creds := base64.StdEncoding.EncodeToString([]byte(cfg.Email + ":" + cfg.Token))
	return &jira{
		baseURL:    host,
		authHeader: "Basic " + creds,
		projectKey: cfg.ProjectKey,
		http:       &http.Client{},
		log:        log,
	}, nil
}

func (j *jira) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", j.authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

// ListVersions returns unreleased versions for the configured Jira project.
func (j *jira) ListVersions() ([]Version, error) {
	endpoint := fmt.Sprintf("%s/rest/api/3/project/%s/version?status=unreleased&orderBy=-releaseDate&maxResults=50", j.baseURL, j.projectKey)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building versions request: %w", err)
	}
	j.setHeaders(req)

	resp, err := j.http.Do(req)
	if err != nil {
		j.log.Error("Jira versions request failed", map[string]any{"endpoint": endpoint, "error": err})
		return nil, fmt.Errorf("fetching versions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading versions response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		j.log.Error("Jira API error", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
		return nil, fmt.Errorf("Jira API returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Values []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Released bool   `json:"released"`
		} `json:"values"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing versions response: %w", err)
	}

	versions := make([]Version, len(result.Values))
	for i, v := range result.Values {
		versions[i] = Version{
			ID:   v.ID,
			Name: v.Name,
			Open: !v.Released,
		}
	}
	j.log.Debug("Jira versions listed", map[string]any{"endpoint": endpoint, "count": len(versions)})
	return versions, nil
}

// CreateVersion creates a Jira version and returns its ID.
func (j *jira) CreateVersion(name, description string) (string, error) {
	payload := struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Project     string `json:"project"`
	}{Name: name, Description: description, Project: j.projectKey}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling version payload: %w", err)
	}

	endpoint := j.baseURL + "/rest/api/3/version"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("building version request: %w", err)
	}
	j.setHeaders(req)

	resp, err := j.http.Do(req)
	if err != nil {
		j.log.Error("Jira version request failed", map[string]any{"endpoint": endpoint, "error": err})
		return "", fmt.Errorf("creating version: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading version response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		j.log.Error("Jira API error", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
		return "", fmt.Errorf("Jira API returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing version response: %w", err)
	}
	j.log.Debug("Jira version created", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
	return result.ID, nil
}

// ResolveRefs looks up Jira-style refs (e.g. PROJ-123) in a single JQL query.
// Numeric-only refs (#123) are silently skipped.
func (j *jira) ResolveRefs(refs []string) ([]ResolvedRef, error) {
	var keys []string
	for _, ref := range refs {
		if jiraRefPattern.MatchString(ref) {
			keys = append(keys, ref)
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}

	jql := fmt.Sprintf("issueKey in (%s)", strings.Join(keys, ","))
	payload := struct {
		JQL    string   `json:"jql"`
		Fields []string `json:"fields"`
	}{
		JQL:    jql,
		Fields: []string{"summary", "status", "self"},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshalling search payload: %w", err)
	}

	endpoint := j.baseURL + "/rest/api/3/search"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("building search request: %w", err)
	}
	j.setHeaders(req)

	resp, err := j.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searching issues: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading search response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jira API returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Issues []struct {
			Key  string `json:"key"`
			Self string `json:"self"`
			Fields struct {
				Summary string `json:"summary"`
				Status  struct {
					Name string `json:"name"`
				} `json:"status"`
			} `json:"fields"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}

	resolved := make([]ResolvedRef, len(result.Issues))
	for i, issue := range result.Issues {
		resolved[i] = ResolvedRef{
			Ref:   issue.Key,
			Title: issue.Fields.Summary,
			State: issue.Fields.Status.Name,
			URL:   issue.Self,
		}
	}
	j.log.Debug("Jira refs resolved", map[string]any{"count": len(resolved)})
	return resolved, nil
}

// AssignTickets adds the given version to the fixVersions of each Jira issue.
// All refs are attempted; a combined error is returned for any that failed.
func (j *jira) AssignTickets(refs []string, versionID string) error {
	payload := struct {
		Update struct {
			FixVersions []struct {
				Add struct {
					ID string `json:"id"`
				} `json:"add"`
			} `json:"fixVersions"`
		} `json:"update"`
	}{}
	payload.Update.FixVersions = []struct {
		Add struct {
			ID string `json:"id"`
		} `json:"add"`
	}{{Add: struct {
		ID string `json:"id"`
	}{ID: versionID}}}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling assignment payload: %w", err)
	}

	var errs []string
	for _, ref := range refs {
		if !jiraRefPattern.MatchString(ref) {
			continue
		}
		endpoint := fmt.Sprintf("%s/rest/api/3/issue/%s", j.baseURL, ref)
		req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(data))
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: building request: %v", ref, err))
			continue
		}
		j.setHeaders(req)

		resp, err := j.http.Do(req)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ref, err))
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Jira returns 204 No Content on success
		if resp.StatusCode != http.StatusNoContent {
			errs = append(errs, fmt.Sprintf("%s: Jira API returned %d: %s", ref, resp.StatusCode, body))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("assigning tickets: %s", strings.Join(errs, "; "))
	}
	j.log.Debug("Jira tickets assigned", map[string]any{"count": len(refs), "version": versionID})
	return nil
}

// CloseVersion marks the Jira version as released.
func (j *jira) CloseVersion(versionID string) error {
	payload := struct {
		Released bool `json:"released"`
	}{Released: true}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling close payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/rest/api/3/version/%s", j.baseURL, versionID)
	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building close request: %w", err)
	}
	j.setHeaders(req)

	resp, err := j.http.Do(req)
	if err != nil {
		j.log.Error("Jira version close request failed", map[string]any{"endpoint": endpoint, "error": err})
		return fmt.Errorf("closing version: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		j.log.Error("Jira API error", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
		return fmt.Errorf("Jira API returned %d: %s", resp.StatusCode, body)
	}
	j.log.Debug("Jira version closed", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
	return nil
}
