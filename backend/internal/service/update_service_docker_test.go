package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDockerBlocksBinaryReplacementBeforeAnyIO(t *testing.T) {
	s := &UpdateService{deploymentMode: "docker"}
	// Nil dependencies ensure blocked calls cannot reach release downloads or cache.
	for name, call := range map[string]func() error{
		"update":           func() error { return s.PerformUpdate(context.Background()) },
		"rollback backup":  s.Rollback,
		"rollback version": func() error { return s.RollbackToVersion(context.Background(), "0.2.8") },
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorContains(t, call(), "Docker")
		})
	}
}
