//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestForkGeneratedOutboundIdentity(t *testing.T) {
	for name, value := range map[string]string{
		"image instructions": codexImageGenerationBridgeText,
		"spark instructions": codexSparkImageUnsupportedText,
		"todo instructions":  openAICompatClaudeCodeTodoGuardText,
		"python alias":       codexPythonToolAlias,
		"model description":  configuredCodexCustomDescription,
		"voice probe":        defaultGrokTTSTestText,
		"vertex job":         vertexBatchDisplayName(BatchImageInput{BatchID: "test-job"}),
	} {
		t.Run(name, func(t *testing.T) { require.NotContains(t, strings.ToLower(value), "sub2api") })
	}
}

func TestForkDisabledBillingProbeCannotSendRequest(t *testing.T) {
	t.Setenv("UPSTREAM_BILLING_PROBE_DISABLED", "true")
	svc := &UpstreamBillingProbeService{}
	require.NoError(t, svc.RunDue(context.Background()))
	_, err := svc.probeLoadedAccount(context.Background(), &Account{}, 30)
	require.ErrorIs(t, err, ErrUpstreamBillingProbeUnavailable)
}

type forkReleaseClient struct {
	updateServiceGitHubClientStub
	repos []string
}

func (c *forkReleaseClient) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	c.repos = append(c.repos, repo)
	return &GitHubRelease{TagName: "v0.2.6", HTMLURL: "https://github.com/DDAICHICAO/sub2api/releases/tag/v0.2.6"}, nil
}
func (c *forkReleaseClient) FetchRecentReleases(_ context.Context, repo string, _ int) ([]*GitHubRelease, error) {
	c.repos = append(c.repos, repo)
	return nil, nil
}

func TestForkUpdateSourceAndLegacyCacheIsolation(t *testing.T) {
	for _, repo := range []string{"", "Wei-Shaw/sub2api"} {
		t.Run("legacy:"+repo, func(t *testing.T) {
			old, err := json.Marshal(map[string]any{"repository": repo, "latest": "99.0.0", "timestamp": time.Now().Unix(), "release_info": map[string]any{"html_url": "https://github.com/Wei-Shaw/sub2api/releases/tag/v99.0.0"}})
			require.NoError(t, err)
			cache := &updateServiceCacheStub{data: string(old)}
			client := &forkReleaseClient{}
			svc := NewUpdateService(cache, client, "0.2.6", "release")
			info, err := svc.CheckUpdate(context.Background(), false)
			require.NoError(t, err)
			require.Equal(t, "0.2.6", info.LatestVersion)
			require.False(t, info.HasUpdate)
			_, err = svc.ListRollbackVersions(context.Background())
			require.NoError(t, err)
			require.Equal(t, []string{"DDAICHICAO/sub2api", "DDAICHICAO/sub2api"}, client.repos)
			cached, err := svc.CheckUpdate(context.Background(), false)
			require.NoError(t, err)
			require.True(t, cached.Cached)
		})
	}
}
