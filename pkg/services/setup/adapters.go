package setup

import (
	"context"
	"runtime"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// SystemToolChecker implementa ToolChecker usando shared.Runner.LookPath.
type SystemToolChecker struct {
	Runner shared.Runner
}

// IsInstalled verifica se a ferramenta está disponível no PATH do sistema.
func (c *SystemToolChecker) IsInstalled(tool string) (string, error) {
	return c.Runner.LookPath(tool)
}

// SystemPackageManager implementa PackageManager usando shared.Runner.
type SystemPackageManager struct {
	Runner shared.Runner
	// GOOS permite injetar o sistema operacional para testes.
	// Se vazio, usa runtime.GOOS.
	GOOS string
}

func (p *SystemPackageManager) goos() string {
	if p.GOOS != "" {
		return p.GOOS
	}
	return runtime.GOOS
}

// Detect identifica o gerenciador de pacotes disponível no sistema.
// Verifica na ordem: brew > apt-get > snap (os dois últimos apenas em Linux).
func (p *SystemPackageManager) Detect() string {
	if _, err := p.Runner.LookPath("brew"); err == nil {
		return "brew"
	}
	if p.goos() == "linux" {
		if _, err := p.Runner.LookPath("apt-get"); err == nil {
			return "apt"
		}
		if _, err := p.Runner.LookPath("snap"); err == nil {
			return "snap"
		}
	}
	return ""
}

// Install instala uma ferramenta usando o gerenciador de pacotes especificado.
func (p *SystemPackageManager) Install(ctx context.Context, tool, manager string) ([]byte, error) {
	switch manager {
	case "brew":
		return p.Runner.RunCombinedOutput(ctx, "brew", "install", tool)
	case "apt":
		return p.Runner.RunCombinedOutput(ctx, "sudo", "apt-get", "install", "-y", tool)
	case "snap":
		return p.Runner.RunCombinedOutput(ctx, "sudo", "snap", "install", tool)
	default:
		return p.Runner.RunCombinedOutput(ctx, "echo", "noop")
	}
}
