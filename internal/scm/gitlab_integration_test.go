//go:build integration

package scm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const gitlabRootPassword = "TestPass1234!"

type gitLabFixture struct {
	baseURL string
	token   string
	owner   string
	repo    string
	tag     string
}

// startGitLabContainer starts a GitLab CE container and returns a fixture with a
// pre-created project and tag. GitLab CE takes 3–5 minutes to fully initialise,
// so the wait timeout is generous.
func startGitLabContainer(ctx context.Context, t *testing.T) gitLabFixture {
	t.Helper()
	tc.SkipIfProviderIsNotHealthy(t)

	ctr, err := tc.Run(ctx, "gitlab/gitlab-ce:latest",
		tc.WithExposedPorts("80/tcp"),
		tc.WithEnv(map[string]string{
			"GITLAB_ROOT_PASSWORD": gitlabRootPassword,
		}),
		tc.WithWaitStrategy(
			wait.ForHTTP("/-/readiness").
				WithPort("80/tcp").
				WithStartupTimeout(10*time.Minute),
		),
	)
	require.NoError(t, err)
	tc.CleanupContainer(t, ctr)

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, "80/tcp")
	require.NoError(t, err)
	baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())

	token := gitlabMintToken(ctx, t, ctr)
	projectID, defaultBranch := gitlabCreateProject(t, baseURL, token, "test-repo")
	gitlabCreateTag(t, baseURL, token, projectID, "v1.0.0", defaultBranch)

	return gitLabFixture{
		baseURL: baseURL,
		token:   token,
		owner:   "root",
		repo:    "test-repo",
		tag:     "v1.0.0",
	}
}

// gitlabMintToken runs a gitlab-rails runner script that creates a personal access
// token for root and prints it prefixed with "TOKEN=" so it can be extracted from
// the combined exec output.
func gitlabMintToken(ctx context.Context, t *testing.T, ctr tc.Container) string {
	t.Helper()

	script := `t = User.find_by_username('root').personal_access_tokens.create!(` +
		`name: 'integration-test', scopes: ['api'], expires_at: 365.days.from_now); ` +
		`STDOUT.puts "TOKEN=#{t.token}"; STDOUT.flush`

	exitCode, reader, err := ctr.Exec(ctx, []string{"gitlab-rails", "runner", script})
	require.NoError(t, err)
	require.Zero(t, exitCode, "gitlab-rails runner exited non-zero")

	output, err := io.ReadAll(reader)
	require.NoError(t, err)

	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "TOKEN=") {
			token := strings.TrimSpace(strings.TrimPrefix(line, "TOKEN="))
			require.NotEmpty(t, token)
			return token
		}
	}
	t.Fatalf("TOKEN= line not found in gitlab-rails runner output:\n%s", output)
	return ""
}

func gitlabCreateProject(t *testing.T, baseURL, token, name string) (projectID int, defaultBranch string) {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"name":                   name,
		"initialize_with_readme": true,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v4/projects", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create project: %s", body)

	var project struct {
		ID            int    `json:"id"`
		DefaultBranch string `json:"default_branch"`
	}
	require.NoError(t, json.Unmarshal(body, &project))
	require.NotZero(t, project.ID)
	return project.ID, project.DefaultBranch
}

func gitlabCreateTag(t *testing.T, baseURL, token string, projectID int, tag, ref string) {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"tag_name": tag,
		"ref":      ref,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v4/projects/%d/repository/tags", baseURL, projectID),
		bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create tag: %s", body)
}

func TestGitLabAdapter_CreateRelease(t *testing.T) {
	ctx := context.Background()
	f := startGitLabContainer(ctx, t)

	provider, err := New("gitlab", Config{
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
