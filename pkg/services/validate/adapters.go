package validate

import (
	"context"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// RealHelmRunner implementa HelmRunner delegando ao shared.Runner.
type RealHelmRunner struct {
	Runner shared.Runner
}

// DependencyBuild executa `helm dependency build` no chartPath informado.
func (r *RealHelmRunner) DependencyBuild(ctx context.Context, chartPath string) error {
	return r.Runner.Run(ctx, "helm", "dependency", "build", chartPath)
}

// Lint executa `helm lint` no chartPath informado.
func (r *RealHelmRunner) Lint(ctx context.Context, chartPath string) error {
	return r.Runner.Run(ctx, "helm", "lint", chartPath)
}

// Template executa `helm template` e retorna a saida combinada (stdout+stderr).
func (r *RealHelmRunner) Template(ctx context.Context, releaseName, chartPath, valuesFile string) ([]byte, error) {
	return r.Runner.RunCombinedOutput(ctx, "helm", "template", releaseName, chartPath, "-f", valuesFile)
}
