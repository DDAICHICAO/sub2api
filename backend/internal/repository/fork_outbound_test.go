package repository

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForkOutboundHeadersAtNetworkBoundary(t *testing.T) {
	received := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received <- r.Header.Clone(); w.WriteHeader(200) }))
	defer server.Close()
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("X-Sub2API-Grok-Client-Tool-Cache", "true")
	req.Header["x-sub2api-debug"] = []string{"internal"}
	req.Header.Set("User-Agent", "sub2api-legacy/1")
	req.Header.Set("Authorization", "Bearer test-only")
	req.Header.Set("X-Request-Id", "retain-me")
	resp, err := NewHTTPUpstream(nil).Do(req, "", 1, 1)
	require.NoError(t, err)
	defer resp.Body.Close()
	headers := <-received
	require.Empty(t, headers.Get("X-Sub2API-Grok-Client-Tool-Cache"))
	require.Empty(t, headers.Get("X-Sub2API-Debug"))
	require.NotContains(t, headers.Get("User-Agent"), "sub2api")
	require.Equal(t, "Bearer test-only", headers.Get("Authorization"))
	require.Equal(t, "retain-me", headers.Get("X-Request-Id"))
}
