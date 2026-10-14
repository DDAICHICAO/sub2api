package admin

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type systemDockerUpdater interface {
	Status(context.Context) (*service.DockerUpdateStatus, error)
	Start(context.Context, string) (*service.DockerUpdateStatus, error)
}

func (h *SystemHandler) DockerUpdateStatus(c *gin.Context) {
	status, err := h.dockerUpdater.Status(c.Request.Context())
	if err != nil {
		response.Success(c, service.DockerUpdateStatus{Available: false, Stage: "unavailable"})
		return
	}
	response.Success(c, status)
}

// StartDockerUpdate only selects the server-verified latest release. Clients
// cannot supply image names, commands, Compose files or an arbitrary version.
func (h *SystemHandler) StartDockerUpdate(c *gin.Context) {
	operationID := buildSystemOperationID(c, "docker-update")
	executeAdminIdempotentJSON(c, "admin.system.docker-update", gin.H{"operation_id": operationID}, service.DefaultSystemOperationIdempotencyTTL(), func(ctx context.Context) (any, error) {
		_, release, err := h.acquireSystemLock(ctx, operationID)
		if err != nil {
			return nil, err
		}
		success := false
		defer func() { release("", success) }()
		info, err := h.updateSvc.CheckUpdate(ctx, true)
		if err != nil {
			return nil, err
		}
		if info.DeploymentMode != "docker" {
			return nil, service.ErrDockerImageUpdateRequired
		}
		if !info.HasUpdate {
			return nil, service.ErrNoUpdateAvailable
		}
		job, err := h.dockerUpdater.Start(ctx, info.LatestVersion)
		if err != nil {
			return nil, err
		}
		success = true
		return job, nil
	})
}
