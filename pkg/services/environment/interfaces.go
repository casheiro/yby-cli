package environment

import (
	"context"

	"github.com/casheiro/yby-cli/pkg/services/bootstrap"
)

// MirrorService abstracts pkg/mirror.Manager
type MirrorService interface {
	EnsureGitServer() error
	SetupTunnel(ctx context.Context) error
	Sync() error
	StartSyncLoop(ctx context.Context)
}

// ClusterManager abstracts cluster (e.g. k3d) lifecycle operations
type ClusterManager interface {
	Exists(ctx context.Context, name string) (bool, error)
	Create(ctx context.Context, name string, configFile string) error
	Start(ctx context.Context, name string) error
}

// BootstrapService abstracts the bootstrap process
type BootstrapService interface {
	Run(ctx context.Context, opts bootstrap.BootstrapOptions) error
}

// UpOptions defines parameters for environment initialization
type UpOptions struct {
	Root        string
	Environment string
	ClusterName string
}
