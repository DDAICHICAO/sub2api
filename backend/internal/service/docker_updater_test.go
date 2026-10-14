package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type updaterTransport func(*http.Request) (*http.Response, error)

func (f updaterTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDockerUpdaterSubmitOnlyVersionAndReadDurableStatus(t *testing.T) {
	client := &DockerUpdater{client: &http.Client{Transport: updaterTransport(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "host-updater", r.URL.Host)
		w := httptest.NewRecorder()
		if r.Method == http.MethodPost {
			require.Equal(t, "/update", r.URL.Path)
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, map[string]string{"version": "0.2.18"}, body)
			w.WriteHeader(http.StatusAccepted)
		} else {
			require.Equal(t, "/status", r.URL.Path)
		}
		_, err := w.WriteString(`{"available":true,"active":true,"stage":"pulling","job_id":"same-job","version":"0.2.18"}`)
		require.NoError(t, err)
		return w.Result(), nil
	})}}
	job, err := client.Start(context.Background(), "0.2.18")
	require.NoError(t, err)
	status, err := client.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, job, status)
}

func TestDockerUpdaterUnavailableDoesNotUseNetworkFallback(t *testing.T) {
	_, err := NewDockerUpdater(t.TempDir() + "/missing.sock").Status(context.Background())
	require.ErrorContains(t, err, "host updater unavailable")
}

func TestDockerUpdaterRejectsConflictAndInvalidJSON(t *testing.T) {
	for _, code := range []int{http.StatusConflict, http.StatusOK} {
		client := &DockerUpdater{client: &http.Client{Transport: updaterTransport(func(_ *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			w.WriteHeader(code)
			_, err := w.WriteString("not JSON")
			require.NoError(t, err)
			return w.Result(), nil
		})}}
		_, err := client.Status(context.Background())
		require.Error(t, err)
	}
}
