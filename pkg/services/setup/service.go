package setup

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// ToolInfo descreve uma ferramenta necessária para o setup.
type ToolInfo struct {
	Name        string
	Cmd         string
	InstallHelp string
}

// ToolStatus representa o estado de instalação de uma ferramenta.
type ToolStatus struct {
	Name      string
	Installed bool
	Path      string
}

// InstallResult representa o resultado da tentativa de instalação de uma ferramenta.
type InstallResult struct {
	Tool    string
	Success bool
	Output  string
	Error   error
}

// SetupResult agrupa o resultado da verificação de ferramentas.
type SetupResult struct {
	Tools   []ToolStatus
	Missing []string
}

// Service define o contrato do serviço de setup.
type Service interface {
	// CheckTools verifica as ferramentas necessárias para o perfil informado.
	CheckTools(profile string) (*SetupResult, error)

	// InstallMissing tenta instalar as ferramentas faltantes.
	InstallMissing(ctx context.Context, tools []string) []InstallResult

	// ConfigureDirenv configura o direnv no diretório de trabalho informado.
	ConfigureDirenv(workDir string) error
}

// allTools contém todas as ferramentas suportadas pelo setup.
var allTools = map[string]ToolInfo{
	"kubectl": {Name: "kubectl", Cmd: "kubectl", InstallHelp: "https://kubernetes.io/docs/tasks/tools/"},
	"helm":    {Name: "helm", Cmd: "helm", InstallHelp: "https://helm.sh/docs/intro/install/"},
	"k3d":     {Name: "k3d", Cmd: "k3d", InstallHelp: "https://k3d.io/v5.4.6/#installation"},
	"direnv":  {Name: "direnv", Cmd: "direnv", InstallHelp: "https://direnv.net/docs/installation.html"},
}

// profileTools define quais ferramentas cada perfil requer.
var profileTools = map[string][]string{
	"dev":    {"kubectl", "helm", "k3d", "direnv"},
	"server": {"kubectl", "helm"},
}

type setupService struct {
	checker ToolChecker
	pkg     PackageManager
	runner  shared.Runner
	fs      shared.Filesystem
}

// NewService cria uma nova instância do serviço de setup.
func NewService(checker ToolChecker, pkg PackageManager, runner shared.Runner, fs shared.Filesystem) Service {
	return &setupService{
		checker: checker,
		pkg:     pkg,
		runner:  runner,
		fs:      fs,
	}
}

// CheckTools verifica as ferramentas necessárias para o perfil informado.
func (s *setupService) CheckTools(profile string) (*SetupResult, error) {
	toolNames, ok := profileTools[profile]
	if !ok {
		return nil, fmt.Errorf("perfil desconhecido: %s", profile)
	}

	result := &SetupResult{}
	for _, name := range toolNames {
		info := allTools[name]
		path, err := s.checker.IsInstalled(info.Cmd)
		status := ToolStatus{
			Name:      info.Name,
			Installed: err == nil,
			Path:      path,
		}
		result.Tools = append(result.Tools, status)
		if err != nil {
			result.Missing = append(result.Missing, info.Name)
		}
	}
	return result, nil
}

// InstallMissing tenta instalar as ferramentas faltantes usando o gerenciador de pacotes detectado.
func (s *setupService) InstallMissing(ctx context.Context, tools []string) []InstallResult {
	manager := s.pkg.Detect()
	results := make([]InstallResult, 0, len(tools))

	if manager == "" {
		for _, tool := range tools {
			results = append(results, InstallResult{
				Tool:    tool,
				Success: false,
				Output:  "nenhum gerenciador de pacotes suportado encontrado (brew, apt, snap)",
			})
		}
		return results
	}

	for _, tool := range tools {
		out, err := s.pkg.Install(ctx, tool, manager)
		results = append(results, InstallResult{
			Tool:    tool,
			Success: err == nil,
			Output:  string(out),
			Error:   err,
		})
	}
	return results
}

// envrcContent é o conteúdo padrão do arquivo .envrc.
const envrcContent = "export KUBECONFIG=$(pwd)/.kube/config\necho \"☸️  Ambiente configurado: KUBECONFIG=./.kube/config\""

// ConfigureDirenv configura o direnv no diretório de trabalho informado.
func (s *setupService) ConfigureDirenv(workDir string) error {
	envrcPath := filepath.Join(workDir, ".envrc")

	// Cria .envrc se não existe
	if _, err := s.fs.Stat(envrcPath); err != nil {
		if writeErr := s.fs.WriteFile(envrcPath, []byte(envrcContent), 0600); writeErr != nil {
			return writeErr
		}
	}

	// Executa direnv allow
	return s.runner.Run(context.Background(), "direnv", "allow")
}
