package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// DockerUpdater only talks to the locally installed host service. The application
// never receives a Docker socket, Compose path, or arbitrary command interface.
type DockerUpdater struct {
	client *http.Client
}

type DockerUpdateStatus struct {
	Available bool   `json:"available"`
	Active    bool   `json:"active"`
	Stage     string `json:"stage"`
	JobID     string `json:"job_id,omitempty"`
	Version   string `json:"version,omitempty"`
}

func NewDockerUpdater(socketPath string) *DockerUpdater {
	return &DockerUpdater{client: &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
			DisableKeepAlives: true,
		},
	}}
}

func (s *DockerUpdater) request(ctx context.Context, method, path string, body []byte) (*DockerUpdateStatus, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://host-updater"+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("host updater unavailable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("host updater rejected request (status %d)", resp.StatusCode)
	}
	var result DockerUpdateStatus
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *DockerUpdater) Status(ctx context.Context) (*DockerUpdateStatus, error) {
	return s.request(ctx, http.MethodGet, "/status", nil)
}

func (s *DockerUpdater) Start(ctx context.Context, version string) (*DockerUpdateStatus, error) {
	body, err := json.Marshal(map[string]string{"version": version})
	if err != nil {
		return nil, err
	}
	return s.request(ctx, http.MethodPost, "/update", body)
}
