package tracker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/mxstzdev/releasar-cli/internal/log"
)

// openProject is the OpenProject issue tracker adapter using versions and work packages.
// Auth: Bearer token.
// Required permissions: View work packages, Edit work packages, Manage versions.
type openProject struct {
	baseURL    string
	token      string
	projectKey string // numeric project ID
	http       *http.Client
	log        *log.Channel
}

var openProjectRefPattern = regexp.MustCompile(`^#(\d+)$`)

func newOpenProject(cfg Config, log *log.Channel) (*openProject, error) {
	if cfg.ProjectKey == "" {
		return nil, fmt.Errorf("tracker.projectKey is required for OpenProject (numeric project ID)")
	}
	host := cfg.Host
	if host == "" {
		return nil, fmt.Errorf("tracker.host is required for OpenProject (e.g. https://openproject.example.com)")
	}
	host = strings.TrimRight(host, "/")
	return &openProject{
		baseURL:    host,
		token:      cfg.Token,
		projectKey: cfg.ProjectKey,
		http:       &http.Client{},
		log:        log,
	}, nil
}

func (o *openProject) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+o.token)
	req.Header.Set("Content-Type", "application/json")
}

// ListVersions returns open versions for the configured OpenProject project.
func (o *openProject) ListVersions() ([]Version, error) {
	endpoint := fmt.Sprintf("%s/api/v3/projects/%s/versions?filters=%%5B%%7B%%22status%%22%%3A%%7B%%22operator%%22%%3A%%22%%3D%%22%%2C%%22values%%22%%3A%%5B%%22open%%22%%5D%%7D%%7D%%5D",
		o.baseURL, o.projectKey)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building versions request: %w", err)
	}
	o.setHeaders(req)

	resp, err := o.http.Do(req)
	if err != nil {
		o.log.Error("OpenProject versions request failed", map[string]any{"endpoint": endpoint, "error": err})
		return nil, fmt.Errorf("fetching versions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading versions response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		o.log.Error("OpenProject API error", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
		return nil, fmt.Errorf("OpenProject API returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Embedded struct {
			Elements []struct {
				ID     int    `json:"id"`
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"elements"`
		} `json:"_embedded"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing versions response: %w", err)
	}

	elements := result.Embedded.Elements
	versions := make([]Version, len(elements))
	for i, v := range elements {
		versions[i] = Version{
			ID:   fmt.Sprintf("%d", v.ID),
			Name: v.Name,
			Open: v.Status == "open",
		}
	}
	o.log.Debug("OpenProject versions listed", map[string]any{"endpoint": endpoint, "count": len(versions)})
	return versions, nil
}

// CreateVersion creates an OpenProject version and returns its numeric ID as a string.
func (o *openProject) CreateVersion(name, description string) (string, error) {
	payload := struct {
		Name        string `json:"name"`
		Description struct {
			Format string `json:"format"`
			Raw    string `json:"raw"`
		} `json:"description"`
	}{Name: name}
	payload.Description.Format = "plain"
	payload.Description.Raw = description

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling version payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v3/projects/%s/versions", o.baseURL, o.projectKey)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("building version request: %w", err)
	}
	o.setHeaders(req)

	resp, err := o.http.Do(req)
	if err != nil {
		o.log.Error("OpenProject version request failed", map[string]any{"endpoint": endpoint, "error": err})
		return "", fmt.Errorf("creating version: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading version response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		o.log.Error("OpenProject API error", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
		return "", fmt.Errorf("OpenProject API returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing version response: %w", err)
	}
	o.log.Debug("OpenProject version created", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
	return fmt.Sprintf("%d", result.ID), nil
}

// ResolveRefs fetches all numeric work package refs in a single filtered GET.
// Non-numeric refs are silently skipped.
func (o *openProject) ResolveRefs(refs []string) ([]ResolvedRef, error) {
	var ids []string
	for _, ref := range refs {
		m := openProjectRefPattern.FindStringSubmatch(ref)
		if m == nil {
			continue
		}
		ids = append(ids, m[1])
	}
	if len(ids) == 0 {
		return nil, nil
	}

	// Build filter JSON: [{"id":{"operator":"=","values":["1","2","3"]}}]
	filterJSON, err := json.Marshal([]struct {
		ID struct {
			Operator string   `json:"operator"`
			Values   []string `json:"values"`
		} `json:"id"`
	}{{ID: struct {
		Operator string   `json:"operator"`
		Values   []string `json:"values"`
	}{Operator: "=", Values: ids}}})
	if err != nil {
		return nil, fmt.Errorf("building filter: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v3/work_packages?filters=%s&pageSize=%d",
		o.baseURL, filterJSON, len(ids)+1)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building work packages request: %w", err)
	}
	o.setHeaders(req)

	resp, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching work packages: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading work packages response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenProject API returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Embedded struct {
			Elements []struct {
				ID      int    `json:"id"`
				Subject string `json:"subject"`
				Links   struct {
					Self   struct{ Href string } `json:"self"`
					Status struct{ Title string } `json:"status"`
				} `json:"_links"`
			} `json:"elements"`
		} `json:"_embedded"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing work packages response: %w", err)
	}

	resolved := make([]ResolvedRef, 0, len(result.Embedded.Elements))
	for _, wp := range result.Embedded.Elements {
		resolved = append(resolved, ResolvedRef{
			Ref:   fmt.Sprintf("#%d", wp.ID),
			Title: wp.Subject,
			State: wp.Links.Status.Title,
			URL:   o.baseURL + wp.Links.Self.Href,
		})
	}
	o.log.Debug("OpenProject refs resolved", map[string]any{"count": len(resolved)})
	return resolved, nil
}

// AssignTickets sets the given version on each work package.
// OpenProject requires a GET before each PATCH to read the lockVersion.
// All refs are attempted; a combined error is returned for any that failed.
func (o *openProject) AssignTickets(refs []string, versionID string) error {
	versionHref := fmt.Sprintf("/api/v3/versions/%s", versionID)
	var errs []string

	for _, ref := range refs {
		m := openProjectRefPattern.FindStringSubmatch(ref)
		if m == nil {
			continue
		}
		wpID := m[1]

		// GET to read lockVersion (required for optimistic concurrency).
		getURL := fmt.Sprintf("%s/api/v3/work_packages/%s", o.baseURL, wpID)
		getReq, err := http.NewRequest(http.MethodGet, getURL, nil)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: building GET request: %v", ref, err))
			continue
		}
		o.setHeaders(getReq)

		getResp, err := o.http.Do(getReq)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: GET: %v", ref, err))
			continue
		}
		getBody, _ := io.ReadAll(getResp.Body)
		getResp.Body.Close()

		if getResp.StatusCode == http.StatusNotFound {
			continue
		}
		if getResp.StatusCode != http.StatusOK {
			errs = append(errs, fmt.Sprintf("%s: GET returned %d", ref, getResp.StatusCode))
			continue
		}

		var wp struct {
			LockVersion int `json:"lockVersion"`
		}
		if err := json.Unmarshal(getBody, &wp); err != nil {
			errs = append(errs, fmt.Sprintf("%s: parsing GET response: %v", ref, err))
			continue
		}

		// PATCH to assign the version.
		patchPayload := struct {
			LockVersion int `json:"lockVersion"`
			Links       struct {
				Version struct {
					Href string `json:"href"`
				} `json:"version"`
			} `json:"_links"`
		}{LockVersion: wp.LockVersion}
		patchPayload.Links.Version.Href = versionHref

		patchData, err := json.Marshal(patchPayload)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: marshalling PATCH payload: %v", ref, err))
			continue
		}

		patchURL := fmt.Sprintf("%s/api/v3/work_packages/%s", o.baseURL, wpID)
		patchReq, err := http.NewRequest(http.MethodPatch, patchURL, bytes.NewReader(patchData))
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: building PATCH request: %v", ref, err))
			continue
		}
		o.setHeaders(patchReq)

		patchResp, err := o.http.Do(patchReq)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: PATCH: %v", ref, err))
			continue
		}
		patchBody, _ := io.ReadAll(patchResp.Body)
		patchResp.Body.Close()

		if patchResp.StatusCode != http.StatusOK {
			errs = append(errs, fmt.Sprintf("%s: PATCH returned %d: %s", ref, patchResp.StatusCode, patchBody))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("assigning tickets: %s", strings.Join(errs, "; "))
	}
	o.log.Debug("OpenProject tickets assigned", map[string]any{"count": len(refs), "version": versionID})
	return nil
}

// CloseVersion sets the OpenProject version status to closed.
func (o *openProject) CloseVersion(versionID string) error {
	payload := struct {
		Status string `json:"status"`
	}{Status: "closed"}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling close payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v3/versions/%s", o.baseURL, versionID)
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building close request: %w", err)
	}
	o.setHeaders(req)

	resp, err := o.http.Do(req)
	if err != nil {
		o.log.Error("OpenProject version close request failed", map[string]any{"endpoint": endpoint, "error": err})
		return fmt.Errorf("closing version: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		o.log.Error("OpenProject API error", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
		return fmt.Errorf("OpenProject API returned %d: %s", resp.StatusCode, body)
	}
	o.log.Debug("OpenProject version closed", map[string]any{"endpoint": endpoint, "status": resp.StatusCode})
	return nil
}
