//go:build integration

package scm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// giteaFixture holds the resolved connection details for a running Gitea/Forgejo container
// with a pre-created user, repo, and tag ready for release creation.
type giteaFixture struct {
	baseURL string
	token   string
	owner   string
	repo    string
	tag     string
}

// startGiteaLike starts a Gitea-compatible container (Gitea or Forgejo), creates an admin
// user and API token, then initialises a repo with a tag so the release adapter has
// something to publish against.
func startGiteaLike(ctx context.Context, t *testing.T, image, adminBinary string) giteaFixture {
	t.Helper()
	tc.SkipIfProviderIsNotHealthy(t)

	ctr, err := tc.Run(ctx, image,
		tc.WithExposedPorts("3000/tcp"),
		tc.WithEnv(map[string]string{
			"GITEA__security__INSTALL_LOCK": "true",
			"GITEA__database__DB_TYPE":      "sqlite3",
		}),
		tc.WithWaitStrategy(
			wait.ForHTTP("/api/healthz").
				WithPort("3000/tcp").
				WithStartupTimeout(120*time.Second),
		),
	)
	require.NoError(t, err)
	tc.CleanupContainer(t, ctr)

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, "3000/tcp")
	require.NoError(t, err)
	baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())

	tc.RequireContainerExec(ctx, t, ctr, []string{
		adminBinary, "admin", "user", "create",
		"--username", "testadmin",
		"--password", "TestPass1234!",
		"--email", "admin@test.local",
		"--admin",
		"--must-change-password=false",
	})

	token := giteaCreateToken(t, baseURL, "testadmin", "TestPass1234!")
	giteaCreateRepo(t, baseURL, token, "test-repo")
	giteaCreateTag(t, baseURL, token, "testadmin", "test-repo", "v1.0.0")

	return giteaFixture{
		baseURL: baseURL,
		token:   token,
		owner:   "testadmin",
		repo:    "test-repo",
		tag:     "v1.0.0",
	}
}

func giteaCreateToken(t *testing.T, baseURL, user, pass string) string {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"name":   "integration-test",
		"scopes": []string{"write:repository", "issue"},
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/users/%s/tokens", baseURL, user),
		bytes.NewReader(payload))
	require.NoError(t, err)
	req.SetBasicAuth(user, pass)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create token: %s", body)

	var result struct {
		SHA1 string `json:"sha1"`
	}
	require.NoError(t, json.Unmarshal(body, &result))
	require.NotEmpty(t, result.SHA1, "expected token value in sha1 field")
	return result.SHA1
}

func giteaCreateRepo(t *testing.T, baseURL, token, name string) {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"name":           name,
		"auto_init":      true,
		"default_branch": "main",
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/user/repos", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create repo: %s", body)
}

func giteaCreateTag(t *testing.T, baseURL, token, owner, repo, tag string) {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"tag_name": tag})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/repos/%s/%s/tags", baseURL, owner, repo),
		bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create tag: %s", body)
}

func TestGiteaAdapter_CreateRelease(t *testing.T) {
	ctx := context.Background()
	f := startGiteaLike(ctx, t, "gitea/gitea:latest", "gitea")

	provider, err := New("gitea", Config{
		Host:  f.baseURL,
		Token: f.token,
		Owner: f.owner,
		Repo:  f.repo,
	})
	require.NoError(t, err)

	releaseURL, err := provider.CreateRelease(f.tag, "Release v1.0.0", "Initial release.")
	require.NoError(t, err)
	assert.Contains(t, releaseURL, f.tag)
	assert.Contains(t, releaseURL, f.repo)
}

func TestForgejoAdapter_CreateRelease(t *testing.T) {
	ctx := context.Background()
	f := startGiteaLike(ctx, t, "codeberg.org/forgejo/forgejo:latest", "forgejo")

	provider, err := New("forgejo", Config{
		Host:  f.baseURL,
		Token: f.token,
		Owner: f.owner,
		Repo:  f.repo,
	})
	require.NoError(t, err)

	releaseURL, err := provider.CreateRelease(f.tag, "Release v1.0.0", "Initial release.")
	require.NoError(t, err)
	assert.Contains(t, releaseURL, f.tag)
	assert.Contains(t, releaseURL, f.repo)
}
