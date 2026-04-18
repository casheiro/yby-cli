package cloud

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"gopkg.in/yaml.v3"
)

// providerRegistry armazena as factories dos providers concretos.
// Cada implementação (aws.go, azure.go, gcp.go) registra-se via RegisterProvider.
var providerRegistry []func(runner shared.Runner) CloudProvider

// RegisterProvider registra uma factory de provider. Deve ser chamado em init()
// pelas implementações concretas (aws.go, azure.go, gcp.go).
func RegisterProvider(factory func(runner shared.Runner) CloudProvider) {
	providerRegistry = append(providerRegistry, factory)
}

// execCommandPatterns mapeia trechos do exec.command para nomes canônicos de providers.
var execCommandPatterns = []struct {
	fragment     string
	providerName string
}{
	{"aws-iam-authenticator", "aws"},
	{"aws", "aws"},
	{"kubelogin", "azure"},
	{"az", "azure"},
	{"gke-gcloud-auth-plugin", "gcp"},
	{"gcloud", "gcp"},
}

// kubeconfig é a representação mínima do arquivo kubeconfig necessária para detecção.
type kubeconfig struct {
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			Exec *struct {
				Command string `yaml:"command"`
			} `yaml:"exec"`
		} `yaml:"user"`
	} `yaml:"users"`
}

// Detect parseia o kubeconfig ativo para identificar providers cloud em uso,
// verifica se os CLIs correspondentes estão instalados, e retorna os providers disponíveis.
//
// A detecção é puramente local: leitura de arquivo + LookPath. Sem chamadas de rede.
func Detect(ctx context.Context, runner shared.Runner) []CloudProvider {
	detected := detectFromKubeconfig()
	return resolveProviders(ctx, runner, detected)
}

// detectFromKubeconfig lê o kubeconfig ativo e retorna os nomes canônicos dos providers
// encontrados nos campos exec.command de todos os usuários.
func detectFromKubeconfig() map[string]struct{} {
	kubeconfigPath := activeKubeconfigPath()
	if kubeconfigPath == "" {
		return nil
	}

	data, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return nil
	}

	var kc kubeconfig
	if err := yaml.Unmarshal(data, &kc); err != nil {
		return nil
	}

	providers := make(map[string]struct{})
	for _, u := range kc.Users {
		if u.User.Exec == nil {
			continue
		}
		cmd := u.User.Exec.Command
		if cmd == "" {
			continue
		}
		// Usa apenas o nome base do comando (remove caminhos absolutos)
		base := filepath.Base(cmd)
		if name := matchCommand(base); name != "" {
			providers[name] = struct{}{}
		}
	}
	return providers
}

// matchCommand retorna o nome canônico do provider para um dado nome de comando,
// ou string vazia se nenhum padrão for reconhecido.
func matchCommand(cmd string) string {
	lower := strings.ToLower(cmd)
	for _, p := range execCommandPatterns {
		if lower == p.fragment {
			return p.providerName
		}
	}
	return ""
}

// GetProvider retorna uma instância do provider com o nome especificado.
// Retorna nil se nenhum provider registrado corresponder ao nome.
func GetProvider(runner shared.Runner, name string) CloudProvider {
	for _, factory := range providerRegistry {
		p := factory(runner)
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// resolveProviders cria instâncias dos providers registrados cujo nome aparece
// em detected E cujo CLI está instalado (runner.LookPath).
func resolveProviders(ctx context.Context, runner shared.Runner, detected map[string]struct{}) []CloudProvider {
	var result []CloudProvider

	seen := make(map[string]struct{})
	for _, factory := range providerRegistry {
		p := factory(runner)
		name := p.Name()

		if _, already := seen[name]; already {
			continue
		}

		_, inKubeconfig := detected[name]
		available := p.IsAvailable(ctx)

		if inKubeconfig || available {
			seen[name] = struct{}{}
			result = append(result, p)
		}
	}
	return result
}

// activeKubeconfigPath retorna o caminho do kubeconfig ativo, respeitando a
// variável KUBECONFIG. Quando KUBECONFIG contém múltiplos caminhos (separados
// por ":"), usa o primeiro arquivo existente.
func activeKubeconfigPath() string {
	if env := os.Getenv("KUBECONFIG"); env != "" {
		for _, p := range filepath.SplitList(env) {
			if p != "" {
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	defaultPath := filepath.Join(home, ".kube", "config")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	return ""
}
