//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type dockerUpdaterStub struct{ versions []string }

func (s *dockerUpdaterStub) Status(context.Context) (*service.DockerUpdateStatus, error) {
	return &service.DockerUpdateStatus{Available: true, Stage: "idle"}, nil
}
func (s *dockerUpdaterStub) Start(_ context.Context, version string) (*service.DockerUpdateStatus, error) {
	s.versions = append(s.versions, version)
	return &service.DockerUpdateStatus{Available: true, Active: true, Stage: "queued", JobID: "durable-job", Version: version}, nil
}

func TestDockerUpdateHandlerOnlySelectsServerRelease(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.SetDefaultIdempotencyCoordinator(nil)
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(nil) })
	for _, hasUpdate := range []bool{true, false} {
		update := &systemHandlerUpdateServiceStub{updateInfo: &service.UpdateInfo{DeploymentMode: "docker", HasUpdate: hasUpdate, LatestVersion: "0.2.18"}}
		lock := service.NewSystemOperationLockService(newMemoryIdempotencyRepoStub(), service.IdempotencyConfig{ProcessingTimeout: time.Second, SystemOperationTTL: time.Minute})
		h := NewSystemHandler(update, lock)
		updater := &dockerUpdaterStub{}
		h.dockerUpdater = updater
		r := gin.New()
		r.POST("/update", h.StartDockerUpdate)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader(`{"version":"evil/image","command":"rm -rf /"}`))
		req.Header.Set("Idempotency-Key", "test-latest")
		r.ServeHTTP(w, req)
		if hasUpdate {
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, []string{"0.2.18"}, updater.versions)
			require.Contains(t, w.Body.String(), "durable-job")
		} else {
			require.Empty(t, updater.versions)
		}
		require.Equal(t, []bool{true}, update.checkForces)
		require.Zero(t, update.performCall, "Docker must never invoke the binary replacement path")
	}
}
