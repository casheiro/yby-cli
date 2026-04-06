package setup

import "context"

// ToolChecker verifica se ferramentas estão instaladas no sistema.
type ToolChecker interface {
	// IsInstalled verifica se a ferramenta está disponível no PATH.
	// Retorna o caminho completo do binário ou erro caso não encontrada.
	IsInstalled(tool string) (path string, err error)
}

// PackageManager abstrai a detecção e uso de gerenciadores de pacotes do sistema.
type PackageManager interface {
	// Detect identifica o gerenciador de pacotes disponível.
	// Retorna "brew", "apt", "snap" ou "" caso nenhum seja encontrado.
	Detect() string

	// Install instala uma ferramenta usando o gerenciador de pacotes especificado.
	Install(ctx context.Context, tool, manager string) (output []byte, err error)
}
