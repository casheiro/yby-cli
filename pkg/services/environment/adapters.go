package environment

import (
	"context"
	"strings"

	"github.com/casheiro/yby-cli/pkg/mirror"
	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// K3dClusterManager implements ClusterManager for k3d
type K3dClusterManager struct {
	Runner shared.Runner
}

func (k *K3dClusterManager) Exists(ctx context.Context, name string) (bool, error) {
	out, err := k.Runner.RunCombinedOutput(ctx, "k3d", "cluster", "list", name)
	if err != nil {
		return false, nil // Assume it doesn't exist if command fails or name not in list
	}
	return strings.Contains(string(out), name), nil
}

func (k *K3dClusterManager) Create(ctx context.Context, name string, configFile string) error {
	args := []string{"cluster", "create", name}
	if configFile != "" {
		args = append(args, "--config", configFile)
	}
	return k.Runner.Run(ctx, "k3d", args...)
}

func (k *K3dClusterManager) Start(ctx context.Context, name string) error {
	return k.Runner.Run(ctx, "k3d", "cluster", "start", name)
}

// GitMirrorAdapter adapts pkg/mirror.MirrorManager to MirrorService interface
type GitMirrorAdapter struct {
	manager *mirror.MirrorManager
}

func NewGitMirrorAdapter(localPath string, runner shared.Runner) *GitMirrorAdapter {
	return &GitMirrorAdapter{
		manager: mirror.NewManager(localPath, runner),
	}
}

func (a *GitMirrorAdapter) EnsureGitServer() error {
	return a.manager.EnsureGitServer()
}

func (a *GitMirrorAdapter) SetupTunnel(ctx context.Context) error {
	return a.manager.SetupTunnel(ctx)
}

func (a *GitMirrorAdapter) Sync() error {
	return a.manager.Sync()
}

func (a *GitMirrorAdapter) StartSyncLoop(ctx context.Context) {
	a.manager.StartSyncLoop(ctx)
}
