package cmd

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/ai"
	"github.com/casheiro/yby-cli/v5/pkg/cloud"
	"github.com/casheiro/yby-cli/v5/pkg/confianca"
	ybyerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/plugin"
	"github.com/casheiro/yby-cli/v5/pkg/services/logs"
	"github.com/casheiro/yby-cli/v5/pkg/services/secrets"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/casheiro/yby-cli/v5/pkg/testutil"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Goldens das saídas para pessoas dos comandos migrados para a carta do terminal
// (secret, env, plugin, chart, logs, gen-docs, uki e cloud): sucesso e falha principal,
// com glifos Unicode e ASCII, sem cor, e, para as listagens e um erro, também no tema
// escuro com cor (_escuro.golden). Ficam em testdata/pessoas/.

// modoGolden é um jeito de renderizar a saída conferida por golden.
type modoGolden struct {
	nome, sufixo  string
	ascii, escuro bool
}

var (
	modosSemCor = []modoGolden{{"unicode", "", false, false}, {"ascii", "_ascii", true, false}}
	modoEscuro  = modoGolden{"escuro", "_escuro", false, true}
)

// goldenNosDoisModos roda gerar com glifos Unicode e com ASCII, sem cor, e confere
// testdata/pessoas/<nome>.golden e <nome>_ascii.golden.
func goldenNosDoisModos(t *testing.T, nome string, gerar func(t *testing.T) string) {
	t.Helper()
	goldenNosModos(t, nome, modosSemCor, gerar)
}

// goldenNosTresModos faz o mesmo que goldenNosDoisModos e confere também
// <nome>_escuro.golden, no tema escuro com cor TrueColor (o ANSI das células coloridas
// das listagens e da linha de erro).
func goldenNosTresModos(t *testing.T, nome string, gerar func(t *testing.T) string) {
	t.Helper()
	goldenNosModos(t, nome, append(modosSemCor[:len(modosSemCor):len(modosSemCor)], modoEscuro), gerar)
}

// goldenNosModos roda gerar em cada modo. gerar roda num subteste próprio para que os
// Cleanup dele (inclusive o t.Chdir para a pasta do projeto) terminem antes da
// conferência, que lê testdata/ a partir da pasta do pacote.
func goldenNosModos(t *testing.T, nome string, modos []modoGolden, gerar func(t *testing.T) string) {
	t.Helper()
	for _, modo := range modos {
		t.Run(modo.nome, func(t *testing.T) {
			usarSaidaTexto(t, modo.ascii)
			if modo.escuro {
				// usarSaidaTexto restaura o tema e o perfil de cor no Cleanup
				ui.Configurar(ui.Opcoes{Tema: ui.TemaEscuro})
				lipgloss.SetColorProfile(termenv.TrueColor)
			}
			var saida string
			t.Run("gerar", func(t *testing.T) { saida = gerar(t) })
			conferirSemRestos(t, saida)
			conferirGolden(t, "pessoas/"+nome+modo.sufixo, saida)
		})
	}
}

// executar roda o RunE de c com a saída (e o stderr do comando, como no terminal) num
// buffer e, se ele devolver erro, acrescenta o erro como o root o escreve
// (handleExecutionError no formato text). Caminhos da pasta raiz trocam por <raiz> e as
// barras invertidas por barras, para o golden valer em qualquer sistema.
func executar(t *testing.T, c *cobra.Command, raiz string, args ...string) string {
	t.Helper()
	b := saidaDoComando(t, c)
	c.SetErr(b)
	t.Cleanup(func() { c.SetErr(nil) })
	formatoAnterior, saidaErroAnterior := logFormatEfetivo, saidaErro
	logFormatEfetivo, saidaErro = "text", b
	t.Cleanup(func() { logFormatEfetivo, saidaErro = formatoAnterior, saidaErroAnterior })

	if err := c.RunE(c, args); err != nil {
		handleExecutionError(err)
	}
	home, _ := os.UserHomeDir()
	return normalizarCaminhos(b.String(), raiz, home)
}

// reCaminhoNormalizado acha os caminhos que começam por <raiz> ou <home>.
var reCaminhoNormalizado = regexp.MustCompile(`<(raiz|home)>[^\s]*`)

// normalizarCaminhos troca raiz por <raiz> e home por <home> e, nesses caminhos, as
// barras invertidas por barras (o resto do texto, como um regex, fica como está).
func normalizarCaminhos(s, raiz, home string) string {
	if raiz != "" {
		s = strings.ReplaceAll(s, raiz, "<raiz>")
	}
	if home != "" {
		s = strings.ReplaceAll(s, home, "<home>")
	}
	return reCaminhoNormalizado.ReplaceAllStringFunc(s, func(c string) string { return strings.ReplaceAll(c, `\`, "/") })
}

// usarProjeto cria uma pasta de projeto (com .yby e, se manifesto não for vazio, o
// environments.yaml), entra nela e aponta HOME para outra pasta temporária.
func usarProjeto(t *testing.T, manifesto string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".yby"), 0o755))
	if manifesto != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".yby", "environments.yaml"), []byte(manifesto), 0o644))
	}
	usarHomeTemporaria(t)
	t.Chdir(dir)
	return dir
}

// fakeSecretsService implementa secrets.Service com respostas fixas.
type fakeSecretsService struct {
	segredo, chave, chavePublica string
	err                          error
}

func (f *fakeSecretsService) GenerateWebhook(_ context.Context, o secrets.Options) (string, error) {
	if o.SecretVal != "" {
		return o.SecretVal, f.err
	}
	return f.segredo, f.err
}
func (f *fakeSecretsService) GenerateMinIO(context.Context, secrets.Options) (string, error) {
	return "", f.err
}
func (f *fakeSecretsService) CreateGitHubToken(context.Context, secrets.Options) error { return f.err }
func (f *fakeSecretsService) BackupKeys(context.Context, secrets.Options) (string, error) {
	return f.chave, f.err
}
func (f *fakeSecretsService) RestoreKeys(context.Context, secrets.Options) error { return f.err }
func (f *fakeSecretsService) EncryptWithSOPS(context.Context, string, []byte, string) error {
	return f.err
}
func (f *fakeSecretsService) GenerateAgeKey(context.Context, string) (string, error) {
	return f.chavePublica, f.err
}
func (f *fakeSecretsService) GenerateSecretYAML(context.Context, string, string, string, string) ([]byte, error) {
	return []byte("kind: Secret\n"), f.err
}
func (f *fakeSecretsService) SealWithKubeseal(context.Context, []byte, string) error { return f.err }
func (f *fakeSecretsService) RotateKeys(context.Context) error                       { return f.err }

func usarSecretsFalso(t *testing.T, f *fakeSecretsService) {
	t.Helper()
	anterior := newSecretsService
	newSecretsService = func(shared.Runner, shared.Filesystem) secrets.Service { return f }
	t.Cleanup(func() { newSecretsService = anterior })
}

var errFalhaKubectl = stderrors.New("error: the server doesn't have a resource type \"secrets\"")

func TestSaidasSecret_Golden(t *testing.T) {
	t.Setenv("WEBHOOK_SECRET", "")
	casos := []struct {
		nome string
		cmd  *cobra.Command
		args []string
		svc  *fakeSecretsService
		prep func(t *testing.T)
	}{
		{"secret_webhook_gerado", webhookSecretCmd, []string{"github"}, &fakeSecretsService{segredo: "3f9c2a7d41b8e6f0"}, nil},
		{"secret_webhook_falha", webhookSecretCmd, []string{"github"}, &fakeSecretsService{err: errFalhaKubectl}, nil},
		{"secret_minio", minioSecretCmd, nil, &fakeSecretsService{}, nil},
		{"secret_minio_falha", minioSecretCmd, nil, &fakeSecretsService{err: errFalhaKubectl}, nil},
		{"secret_github_token", githubTokenSecretCmd, []string{"ghp_x"}, &fakeSecretsService{}, nil},
		{"secret_github_token_falha", githubTokenSecretCmd, []string{"ghp_x"}, &fakeSecretsService{err: errFalhaKubectl}, nil},
		{"secret_backup", backupKeysCmd, nil, &fakeSecretsService{chave: "sealed-secrets-key9xk2p"}, nil},
		// como o serviço real: a chave ativa não existe e o erro já traz o hint
		{"secret_backup_falha", backupKeysCmd, nil, &fakeSecretsService{err: ybyerrors.New(ybyerrors.ErrCodeExec, "chave não encontrada").
			WithHint("confira se o Sealed Secrets está no cluster e tem chave ativa: kubectl get pods -n sealed-secrets")}, nil},
		{"secret_restore", restoreKeysCmd, nil, &fakeSecretsService{}, nil},
		{"secret_restore_falha", restoreKeysCmd, nil, &fakeSecretsService{err: errFalhaKubectl}, nil},
		{"secret_rotate", rotateKeysCmd, nil, &fakeSecretsService{}, func(t *testing.T) {
			require.NoError(t, rotateKeysCmd.Flags().Set("schedule", "0 0 1 */3 *"))
			t.Cleanup(func() { _ = rotateKeysCmd.Flags().Set("schedule", "") })
		}},
		{"secret_rotate_falha", rotateKeysCmd, nil, &fakeSecretsService{err: errFalhaKubectl}, nil},
		{"secret_init_sops", initSOPSCmd, nil, &fakeSecretsService{chavePublica: "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"}, nil},
		{"secret_init_sops_falha", initSOPSCmd, nil, &fakeSecretsService{err: stderrors.New("exec: \"age-keygen\": executable file not found in $PATH")}, nil},
		{"secret_init_eso", initESOCmd, nil, &fakeSecretsService{}, func(t *testing.T) {
			initESOBackend = "vault"
			t.Cleanup(func() { initESOBackend = "" })
		}},
		{"secret_init_eso_sem_backend", initESOCmd, nil, &fakeSecretsService{}, nil},
		{"secret_init_eso_backend_invalido", initESOCmd, nil, &fakeSecretsService{}, func(t *testing.T) {
			initESOBackend = "foo"
			t.Cleanup(func() { initESOBackend = "" })
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			goldenNosDoisModos(t, c.nome, func(t *testing.T) string {
				raiz := usarProjeto(t, "")
				usarSecretsFalso(t, c.svc)
				if c.prep != nil {
					c.prep(t)
				}
				return executar(t, c.cmd, raiz, c.args...)
			})
		})
	}
}

func TestSaidasSeal_Golden(t *testing.T) {
	for _, c := range []struct {
		nome, estrategia string
		svc              *fakeSecretsService
	}{
		{"seal_sealed_secrets", "sealed-secrets", &fakeSecretsService{}},
		{"seal_sops", "sops", &fakeSecretsService{}},
		{"seal_falha", "sealed-secrets", &fakeSecretsService{err: errFalhaKubectl}},
	} {
		t.Run(c.nome, func(t *testing.T) {
			goldenNosDoisModos(t, c.nome, func(t *testing.T) string {
				raiz := usarProjeto(t, "")
				usarSecretsFalso(t, c.svc)
				antLook, antPrompt, antPath, antEstrategia := lookPath, sealPrompt, sealPathPrompt, sealStrategy
				t.Cleanup(func() {
					lookPath, sealPrompt, sealPathPrompt, sealStrategy = antLook, antPrompt, antPath, antEstrategia
				})
				lookPath = func(f string) (string, error) { return "/usr/bin/" + f, nil }
				sealPrompt = func() (string, string, string, string, error) { return "db", "payments", "password", "s3nh4", nil }
				sealPathPrompt = func(padrao string) (string, error) { return padrao, nil }
				sealStrategy = c.estrategia
				return executar(t, sealCmd, raiz)
			})
		})
	}
}

const manifestoGolden = `current: local
environments:
  local:
    type: local
    description: "cluster k3d na máquina"
    values: "config/values-local.yaml"
  prod:
    type: eks
    description: "produção"
    values: "config/values-prod.yaml"
    url: "https://prod.example.com"
    kube_context: "prod-eks"
    namespace: "payments"
    cloud:
      provider: aws
      region: us-east-1
      cluster: prod-eks
      profile: producao
  staging:
    type: remote
    description: "homologação"
    values: "config/values-staging.yaml"
    kube_context: "staging"
`

// fakeCloudProvider implementa cloud.CloudProvider com respostas fixas.
type fakeCloudProvider struct {
	nome, versao string
	indisponivel bool
	clusters     []cloud.ClusterInfo
	status       *cloud.CredentialStatus
	errListar    error
	errValidar   error
	errRefresh   error
	errKube      error
}

func (f *fakeCloudProvider) Name() string                     { return f.nome }
func (f *fakeCloudProvider) IsAvailable(context.Context) bool { return !f.indisponivel }
func (f *fakeCloudProvider) CLIVersion(context.Context) (string, error) {
	if f.versao == "" {
		return "", stderrors.New("sem versão")
	}
	return f.versao, nil
}
func (f *fakeCloudProvider) ListClusters(context.Context, cloud.ListOptions) ([]cloud.ClusterInfo, error) {
	return f.clusters, f.errListar
}
func (f *fakeCloudProvider) ConfigureKubeconfig(context.Context, cloud.ClusterInfo) error {
	return f.errKube
}
func (f *fakeCloudProvider) ValidateCredentials(context.Context) (*cloud.CredentialStatus, error) {
	return f.status, f.errValidar
}
func (f *fakeCloudProvider) RefreshToken(context.Context, cloud.ClusterInfo) error {
	return f.errRefresh
}

func usarProvidersFalsos(t *testing.T, providers ...*fakeCloudProvider) {
	t.Helper()
	antDetect, antGet, antAudit := detectarProvidersCloud, obterProviderCloud, auditoriaCloud
	t.Cleanup(func() { detectarProvidersCloud, obterProviderCloud, auditoriaCloud = antDetect, antGet, antAudit })
	audit := filepath.Join(t.TempDir(), "audit.log")
	auditoriaCloud = func() *cloud.AuditLogger { return cloud.NewAuditLoggerWithPath(audit) }
	detectarProvidersCloud = func(context.Context, shared.Runner) []cloud.CloudProvider {
		lista := make([]cloud.CloudProvider, len(providers))
		for i, p := range providers {
			lista[i] = p
		}
		return lista
	}
	obterProviderCloud = func(_ shared.Runner, nome string) cloud.CloudProvider {
		for _, p := range providers {
			if p.nome == nome {
				return p
			}
		}
		return nil
	}
}

func expiraEm(d time.Duration) *time.Time {
	t := quandoGolden.Add(d)
	return &t
}

func awsAutenticado() *fakeCloudProvider {
	return &fakeCloudProvider{
		nome:   "aws",
		versao: "aws-cli/2.15.0",
		status: &cloud.CredentialStatus{Authenticated: true, Identity: "arn:aws:iam::123456789012:user/ana", Method: "sso", ExpiresAt: expiraEm(90 * time.Minute)},
		clusters: []cloud.ClusterInfo{
			{Name: "prod-eks", Region: "us-east-1", Provider: "aws", Version: "1.29", Status: "ACTIVE"},
			{Name: "dev-eks", Region: "us-east-1", Provider: "aws", Status: "CREATING"},
		},
	}
}

func TestSaidasEnv_Golden(t *testing.T) {
	casos := []struct {
		nome      string
		manifesto string
		cmd       *cobra.Command
		args      []string
		prep      func(t *testing.T)
	}{
		{"env_list", manifestoGolden, envListCmd, nil, nil},
		{"env_list_vazio", "environments: {}\n", envListCmd, nil, nil},
		{"env_use", manifestoGolden, envUseCmd, []string{"staging"}, func(t *testing.T) {
			ant := newEnvRunner
			t.Cleanup(func() { newEnvRunner = ant })
			newEnvRunner = func() shared.Runner { return &testutil.MockRunner{} }
		}},
		{"env_use_kubectl_falha", manifestoGolden, envUseCmd, []string{"staging"}, func(t *testing.T) {
			ant := newEnvRunner
			t.Cleanup(func() { newEnvRunner = ant })
			newEnvRunner = func() shared.Runner {
				return &testutil.MockRunner{RunFunc: func(context.Context, string, ...string) error {
					return stderrors.New("no context exists with the name: \"staging\"")
				}}
			}
		}},
		{"env_use_inexistente", manifestoGolden, envUseCmd, []string{"qa"}, nil},
		{"env_show", manifestoGolden, envShowCmd, nil, nil},
		{"env_show_cloud", strings.Replace(manifestoGolden, "current: local", "current: prod", 1), envShowCmd, nil, func(t *testing.T) {
			usarProvidersFalsos(t, awsAutenticado())
		}},
		{"env_show_cloud_expirado", strings.Replace(manifestoGolden, "current: local", "current: prod", 1), envShowCmd, nil, func(t *testing.T) {
			aws := awsAutenticado()
			aws.status.ExpiresAt = expiraEm(-12 * time.Minute)
			usarProvidersFalsos(t, aws)
		}},
		{"env_create", manifestoGolden, envCreateCmd, []string{"qa"}, nil},
		{"env_create_cloud", manifestoGolden, envCreateCmd, []string{"qa"}, func(t *testing.T) {
			usarProvidersFalsos(t, awsAutenticado())
			for f, v := range map[string]string{"cloud-provider": "aws", "cloud-region": "us-east-1", "cloud-cluster": "qa-eks"} {
				require.NoError(t, envCreateCmd.Flags().Set(f, v))
				f := f
				t.Cleanup(func() { _ = envCreateCmd.Flags().Set(f, "") })
			}
		}},
		{"env_create_sem_projeto", "", envCreateCmd, []string{"qa"}, nil},
		{"env_check", manifestoGolden, envCheckCmd, nil, nil},
		{"env_list_sem_projeto", "", envListCmd, nil, nil},
		{"env_show_sem_projeto", "", envShowCmd, nil, nil},
		{"env_check_sem_projeto", "", envCheckCmd, nil, nil},
	}
	// a listagem e um erro também no tema escuro com cor
	comEscuro := map[string]bool{"env_list": true, "env_use_inexistente": true}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			golden := goldenNosDoisModos
			if comEscuro[c.nome] {
				golden = goldenNosTresModos
			}
			golden(t, c.nome, func(t *testing.T) string {
				raiz := usarProjeto(t, c.manifesto)
				if c.manifesto == "" {
					// sem projeto: nem .yby
					require.NoError(t, os.RemoveAll(filepath.Join(raiz, ".yby")))
				}
				if c.manifesto != "" {
					// values do local existe; os demais, não (avisos de integridade)
					require.NoError(t, os.MkdirAll(filepath.Join(raiz, "config"), 0o755))
					require.NoError(t, os.WriteFile(filepath.Join(raiz, "config", "values-local.yaml"), nil, 0o644))
				}
				if c.prep != nil {
					c.prep(t)
				}
				ant := agora
				agora = func() time.Time { return quandoGolden }
				t.Cleanup(func() { agora = ant })
				// fora do projeto, o manifesto procurado depende de onde fica a pasta temporária
				// (a busca sobe as pastas e pode achar o ~/.yby de quem roda o teste)
				return reManifestoProcurado.ReplaceAllString(executar(t, c.cmd, raiz, c.args...), "(<manifesto>)")
			})
		})
	}
}

// reManifestoProcurado acha o caminho do environments.yaml citado na causa de um erro.
var reManifestoProcurado = regexp.MustCompile(`\([^()]*environments\.yaml\)`)

func TestSaidasEnvDump_Golden(t *testing.T) {
	goldenNosDoisModos(t, "env_dump", func(t *testing.T) string {
		var b bytes.Buffer
		renderContextDump(&b, []string{"PATH=/usr/bin", "YBY_GIT_REPOURL=https://github.com/casheiro/infra", "YBY_ENV=staging", "HOME=/home/ana"})
		return b.String()
	})
	goldenNosDoisModos(t, "env_dump_vazio", func(t *testing.T) string {
		var b bytes.Buffer
		renderContextDump(&b, []string{"PATH=/usr/bin"})
		return b.String()
	})
}

func TestSaidasPlugin_Golden(t *testing.T) {
	goldenNosTresModos(t, "plugin_list", func(t *testing.T) string {
		var b bytes.Buffer
		renderPluginList(&b, []plugin.PluginManifest{
			{Name: "sentinel", Version: "1.0.0", Hooks: []string{"manifest", "command"}},
			{Name: "atlas", Version: "v1.2.0", Hooks: []string{"manifest", "context", "command"}},
		})
		return b.String()
	})
	goldenNosDoisModos(t, "plugin_list_vazio", func(t *testing.T) string {
		var b bytes.Buffer
		renderPluginList(&b, nil)
		return b.String()
	})
	goldenNosDoisModos(t, "plugin_trust_nao_encontrado", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		usarPluginsFalsos(t, &fakePluginManager{})
		return executar(t, pluginTrustCmd, raiz, "bard")
	})
	goldenNosDoisModos(t, "plugin_trust", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		usarPluginsFalsos(t, &fakePluginManager{plugins: []plugin.LoadedPlugin{binarioDePlugin(t, raiz, "bard")}})
		return executar(t, pluginTrustCmd, raiz, "bard")
	})
	goldenNosDoisModos(t, "plugin_untrust", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		require.NoError(t, plugin.TrustPlugin(binarioDePlugin(t, raiz, "bard").Path))
		return executar(t, pluginUntrustCmd, raiz, "bard")
	})
	goldenNosDoisModos(t, "plugin_untrust_nao_confiavel", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		return executar(t, pluginUntrustCmd, raiz, "bard")
	})

	// ferramenta do Bard no projeto: a confiança é do arquivo, sem plugin nenhum
	ferramentaDoProjeto := func(t *testing.T, raiz string) string {
		t.Helper()
		dir := filepath.Join(raiz, ".yby", "tools")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "backup.yaml"), []byte("name: backup\ncommand: velero backup create x\n"), 0o644))
		return ".yby/tools/backup.yaml"
	}
	goldenNosDoisModos(t, "plugin_trust_arquivo", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		arquivo := ferramentaDoProjeto(t, raiz)
		saida := executar(t, pluginTrustCmd, raiz, arquivo)
		dados, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(arquivo)))
		require.NoError(t, err)
		require.True(t, confianca.Confiavel(arquivo, dados), "o arquivo deveria ficar confiável")
		return saida
	})
	goldenNosDoisModos(t, "plugin_trust_arquivo_inexistente", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		return executar(t, pluginTrustCmd, raiz, ".yby/tools/nao-existe.yaml")
	})
	goldenNosDoisModos(t, "plugin_untrust_arquivo", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		arquivo := ferramentaDoProjeto(t, raiz)
		require.NoError(t, confianca.Confiar(arquivo))
		saida := executar(t, pluginUntrustCmd, raiz, arquivo)
		dados, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(arquivo)))
		require.NoError(t, err)
		require.False(t, confianca.Confiavel(arquivo, dados), "o arquivo deveria sair da confiança")
		return saida
	})
	goldenNosDoisModos(t, "plugin_untrust_arquivo_nao_confiavel", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		return executar(t, pluginUntrustCmd, raiz, ferramentaDoProjeto(t, raiz))
	})

	// install pelo Manager real, de um arquivo local para a HOME temporária
	goldenNosDoisModos(t, "plugin_install", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		return executar(t, pluginInstallCmd, raiz, binarioDePlugin(t, raiz, "demo").Path)
	})
	goldenNosDoisModos(t, "plugin_install_origem_inexistente", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		return executar(t, pluginInstallCmd, raiz, "nao-existe")
	})
	goldenNosDoisModos(t, "plugin_install_sem_confianca", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		// instala o binário sem registrar o checksum, como quando TrustPlugin falha
		usarPluginsFalsos(t, &fakePluginManager{instalar: func(origem string) error {
			home, _ := os.UserHomeDir()
			dir := filepath.Join(home, ".yby", "plugins")
			require.NoError(t, os.MkdirAll(dir, 0o755))
			return os.WriteFile(filepath.Join(dir, "yby-plugin-"+origem), []byte("binário"), 0o755)
		}})
		return executar(t, pluginInstallCmd, raiz, "bard")
	})

	goldenNosDoisModos(t, "plugin_remove", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		usarPluginsFalsos(t, &fakePluginManager{plugins: []plugin.LoadedPlugin{binarioDePlugin(t, raiz, "bard")}})
		return executar(t, pluginRemoveCmd, raiz, "bard")
	})
	goldenNosDoisModos(t, "plugin_remove_nao_encontrado", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		usarPluginsFalsos(t, &fakePluginManager{})
		return executar(t, pluginRemoveCmd, raiz, "bard")
	})

	goldenNosDoisModos(t, "plugin_update", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		usarPluginsFalsos(t, &fakePluginManager{plugins: []plugin.LoadedPlugin{binarioDePlugin(t, raiz, "bard")}})
		return executar(t, pluginUpdateCmd, raiz)
	})
	goldenNosDoisModos(t, "plugin_update_vazio", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		usarPluginsFalsos(t, &fakePluginManager{})
		return executar(t, pluginUpdateCmd, raiz)
	})
	goldenNosTresModos(t, "plugin_update_com_falhas", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		usarPluginsFalsos(t, &fakePluginManager{
			plugins: []plugin.LoadedPlugin{binarioDePlugin(t, raiz, "atlas"), binarioDePlugin(t, raiz, "bard")},
			errAtualizar: map[string]error{
				"atlas": ybyerrors.Wrap(stderrors.New("404 Not Found"), ybyerrors.ErrCodePlugin, "falha ao baixar a release do plugin atlas"),
			},
		})
		return executar(t, pluginUpdateCmd, raiz, "atlas", "bard", "naoexiste")
	})
}

// fakePluginManager implementa gerenciadorPlugins sem executar binário nenhum.
type fakePluginManager struct {
	plugins      []plugin.LoadedPlugin
	instalar     func(origem string) error
	errAtualizar map[string]error
}

func (f *fakePluginManager) Discover() error { return nil }
func (f *fakePluginManager) ListPlugins() []plugin.PluginManifest {
	var lista []plugin.PluginManifest
	for _, p := range f.plugins {
		lista = append(lista, p.Manifest)
	}
	return lista
}
func (f *fakePluginManager) GetPlugin(nome string) (*plugin.LoadedPlugin, bool) {
	for _, p := range f.plugins {
		if p.Manifest.Name == nome {
			return &p, true
		}
	}
	return nil, false
}
func (f *fakePluginManager) Install(origem, _ string, _ bool) error {
	if f.instalar != nil {
		return f.instalar(origem)
	}
	return nil
}
func (f *fakePluginManager) Remove(nome string) error {
	if _, ok := f.GetPlugin(nome); !ok {
		return ybyerrors.New(ybyerrors.ErrCodePluginNotFound, "plugin '"+nome+"' não encontrado")
	}
	return nil
}
func (f *fakePluginManager) Update(nome string) error {
	if err := f.errAtualizar[nome]; err != nil {
		return err
	}
	return f.Remove(nome)
}

func usarPluginsFalsos(t *testing.T, f *fakePluginManager) {
	t.Helper()
	ant := newPluginManager
	t.Cleanup(func() { newPluginManager = ant })
	newPluginManager = func() gerenciadorPlugins { return f }
}

// binarioDePlugin grava em raiz um arquivo yby-plugin-<nome> e devolve o plugin carregado
// que aponta para ele.
func binarioDePlugin(t *testing.T, raiz, nome string) plugin.LoadedPlugin {
	t.Helper()
	caminho := filepath.Join(raiz, "yby-plugin-"+nome)
	require.NoError(t, os.WriteFile(caminho, []byte("binário de "+nome), 0o755))
	return plugin.LoadedPlugin{Manifest: plugin.PluginManifest{Name: nome, Version: "1.0.0"}, Path: caminho}
}

func TestSaidasChart_Golden(t *testing.T) {
	goldenNosDoisModos(t, "chart_create", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		return executar(t, chartCreateCmd, raiz, "api")
	})
	goldenNosDoisModos(t, "chart_create_existente", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		require.NoError(t, os.MkdirAll(filepath.Join(raiz, "charts", "api"), 0o755))
		return executar(t, chartCreateCmd, raiz, "api")
	})
}

func TestSaidasGenDocs_Golden(t *testing.T) {
	goldenNosDoisModos(t, "gen_docs", func(t *testing.T) string {
		raiz := t.TempDir()
		return executar(t, genDocsCmd, raiz, raiz)
	})
	goldenNosDoisModos(t, "gen_docs_destino_invalido", func(t *testing.T) string {
		raiz := t.TempDir()
		// o destino é um arquivo, então a pasta de saída não pode ser criada
		destino := filepath.Join(raiz, "wiki")
		require.NoError(t, os.WriteFile(destino, nil, 0o644))
		// a causa é a mensagem do sistema operacional, que muda com ele
		return reCausa.ReplaceAllString(executar(t, genDocsCmd, raiz, destino), "${1}<erro do sistema>")
	})
}

// reCausa acha o texto da linha "causa" de um erro escrito pelo root.
var reCausa = regexp.MustCompile(`(?m)^(  causa\s+).*$`)

func usarLogsFalso(t *testing.T, svc *mockLogsService) {
	t.Helper()
	antSvc, antLook, antTTY := newLogsService, lookPath, isTTY
	t.Cleanup(func() { newLogsService, lookPath, isTTY = antSvc, antLook, antTTY })
	newLogsService = func(shared.Runner) logs.Service { return svc }
	lookPath = func(f string) (string, error) { return "/usr/bin/" + f, nil }
	isTTY = func() bool { return true }
}

func TestSaidasLogs_Golden(t *testing.T) {
	goldenNosDoisModos(t, "logs", func(t *testing.T) string {
		usarLogsFalso(t, &mockLogsService{
			pods:      []string{"api-7f9c-x2k"},
			namespace: "payments",
			logOutput: "2026-10-24T09:30:01Z INFO servidor no ar :8080\n2026-10-24T09:30:02Z WARN cache frio\n",
		})
		return executar(t, logsCmd, "", "api-7f9c-x2k")
	})
	goldenNosDoisModos(t, "logs_sem_pods", func(t *testing.T) string {
		usarLogsFalso(t, &mockLogsService{})
		return executar(t, logsCmd, "")
	})
	goldenNosDoisModos(t, "logs_sem_kubectl", func(t *testing.T) string {
		usarLogsFalso(t, &mockLogsService{})
		lookPath = func(string) (string, error) { return "", stderrors.New("não encontrado") }
		return executar(t, logsCmd, "", "api")
	})
}

func usarIAFalsa(t *testing.T, p ai.Provider) {
	t.Helper()
	ant := getAIProvider
	t.Cleanup(func() { getAIProvider = ant })
	getAIProvider = func(context.Context, string) ai.Provider { return p }
}

func TestSaidasUki_Golden(t *testing.T) {
	goldenNosDoisModos(t, "uki_capture", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		usarIAFalsa(t, &mockAIProvider{name: "ollama-local", blueprint: &ai.GovernanceBlueprint{Files: []ai.GeneratedFile{
			{Path: ".synapstor/.uki/retencao-de-logs.md", Content: "# Retenção"},
			{Path: "../fora.md", Content: "x"},
			{Path: ".synapstor/.personas/auditor.md", Content: "# Auditor"},
		}}})
		return executar(t, captureCmd, raiz, "política de retenção de logs de 30 dias")
	})
	goldenNosDoisModos(t, "uki_capture_sem_provedor", func(t *testing.T) string {
		raiz := usarProjeto(t, "")
		ant := getAIProvider
		t.Cleanup(func() { getAIProvider = ant })
		getAIProvider = func(context.Context, string) ai.Provider { return nil }
		return executar(t, captureCmd, raiz, "política de retenção")
	})
}

func TestSaidasCloud_Golden(t *testing.T) {
	gcpFalho := &fakeCloudProvider{nome: "gcp", errListar: stderrors.New("gcloud: projeto não configurado"),
		errValidar: stderrors.New("gcloud auth login necessário"), errRefresh: stderrors.New("gcloud auth login necessário")}
	casos := []struct {
		nome      string
		cmd       *cobra.Command
		args      []string
		providers []*fakeCloudProvider
		prep      func(t *testing.T)
	}{
		{"cloud_list", cloudListCmd, nil, []*fakeCloudProvider{awsAutenticado(), gcpFalho}, nil},
		{"cloud_list_sem_cli", cloudListCmd, nil, nil, nil},
		{"cloud_list_provider_invalido", cloudListCmd, nil, nil, func(t *testing.T) {
			require.NoError(t, cloudListCmd.Flags().Set("provider", "oracle"))
			t.Cleanup(func() { _ = cloudListCmd.Flags().Set("provider", "") })
		}},
		{"cloud_status", cloudStatusCmd, nil, []*fakeCloudProvider{awsAutenticado(), gcpFalho}, nil},
		{"cloud_status_sem_cli", cloudStatusCmd, nil, nil, nil},
		{"cloud_refresh", cloudRefreshCmd, nil, []*fakeCloudProvider{awsAutenticado(), gcpFalho}, nil},
		{"cloud_refresh_sem_cli_do_provider", cloudRefreshCmd, nil, []*fakeCloudProvider{{nome: "azure", indisponivel: true}}, func(t *testing.T) {
			require.NoError(t, cloudRefreshCmd.Flags().Set("provider", "azure"))
			t.Cleanup(func() { _ = cloudRefreshCmd.Flags().Set("provider", "") })
		}},
		{"cloud_connect", cloudConnectCmd, nil, []*fakeCloudProvider{awsAutenticado()}, func(t *testing.T) {
			for f, v := range map[string]string{"provider": "aws", "region": "us-east-1", "cluster": "prod-eks", "env-name": "prod"} {
				require.NoError(t, cloudConnectCmd.Flags().Set(f, v))
				f := f
				t.Cleanup(func() { _ = cloudConnectCmd.Flags().Set(f, "") })
			}
		}},
		{"cloud_connect_sem_login", cloudConnectCmd, nil, []*fakeCloudProvider{gcpFalho}, func(t *testing.T) {
			require.NoError(t, cloudConnectCmd.Flags().Set("provider", "gcp"))
			t.Cleanup(func() { _ = cloudConnectCmd.Flags().Set("provider", "") })
		}},
		{"cloud_connect_cluster_inexistente", cloudConnectCmd, nil, []*fakeCloudProvider{awsAutenticado()}, func(t *testing.T) {
			for f, v := range map[string]string{"provider": "aws", "cluster": "qa-eks"} {
				require.NoError(t, cloudConnectCmd.Flags().Set(f, v))
				f := f
				t.Cleanup(func() { _ = cloudConnectCmd.Flags().Set(f, "") })
			}
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			golden := goldenNosDoisModos
			if c.nome == "cloud_list" {
				golden = goldenNosTresModos
			}
			golden(t, c.nome, func(t *testing.T) string {
				raiz := usarProjeto(t, "environments: {}\n")
				require.NoError(t, os.MkdirAll(filepath.Join(raiz, "config"), 0o755))
				usarProvidersFalsos(t, c.providers...)
				ant := agora
				agora = func() time.Time { return quandoGolden }
				t.Cleanup(func() { agora = ant })
				if c.prep != nil {
					c.prep(t)
				}
				return executar(t, c.cmd, raiz, c.args...)
			})
		})
	}
}

func TestSaidasCloudAudit_Golden(t *testing.T) {
	goldenNosTresModos(t, "cloud_audit", func(t *testing.T) string {
		home := usarHomeTemporaria(t)
		require.NoError(t, os.WriteFile(filepath.Join(home, ".yby", "audit.log"), []byte(eventosAuditGolden), 0o600))
		return executar(t, cloudAuditCmd, "")
	})
	goldenNosDoisModos(t, "cloud_audit_vazio", func(t *testing.T) string {
		usarHomeTemporaria(t)
		return executar(t, cloudAuditCmd, "")
	})
	goldenNosDoisModos(t, "cloud_audit_data_invalida", func(t *testing.T) string {
		usarHomeTemporaria(t)
		require.NoError(t, cloudAuditCmd.Flags().Set("since", "01/04/2026"))
		t.Cleanup(func() { _ = cloudAuditCmd.Flags().Set("since", "") })
		return executar(t, cloudAuditCmd, "")
	})
}

// cloud list escreve os providers que falharam em stderr: quem filtra a lista por pipe
// recebe só a tabela.
func TestCloudList_FalhasDeProviderEmStderr(t *testing.T) {
	usarSaidaTexto(t, false)
	usarProjeto(t, "environments: {}\n")
	usarProvidersFalsos(t, awsAutenticado(), &fakeCloudProvider{nome: "gcp", errListar: stderrors.New("gcloud: projeto não configurado")})
	var out, errOut bytes.Buffer
	cloudListCmd.SetOut(&out)
	cloudListCmd.SetErr(&errOut)
	t.Cleanup(func() { cloudListCmd.SetOut(nil); cloudListCmd.SetErr(nil) })

	require.NoError(t, runCloudList(cloudListCmd, nil))
	require.Contains(t, out.String(), "prod-eks")
	require.NotContains(t, out.String(), "gcp")
	require.Contains(t, errOut.String(), "▲ gcp")
	require.Contains(t, errOut.String(), "gcloud: projeto não configurado")
}

// O export do audit e o telemetry export sem dados avisam em stderr e deixam a saída
// (stdout) vazia, para não misturar texto com os dados.
func TestExportSemDados_AvisoForaDaSaida(t *testing.T) {
	usarSaidaTexto(t, false)
	usarHomeTemporaria(t)
	for _, c := range []struct {
		cmd  *cobra.Command
		prep func()
	}{
		{cloudAuditCmd, func() { _ = cloudAuditCmd.Flags().Set("export", "json") }},
		{telemetryExportCmd, func() {}},
	} {
		c.prep()
		var out, errOut bytes.Buffer
		c.cmd.SetOut(&out)
		c.cmd.SetErr(&errOut)
		require.NoError(t, c.cmd.RunE(c.cmd, nil))
		c.cmd.SetOut(nil)
		c.cmd.SetErr(nil)
		if out.Len() != 0 {
			t.Errorf("%s: stdout deveria ficar vazio, veio %q", c.cmd.Name(), out.String())
		}
		if !strings.Contains(errOut.String(), "○ nenhum") {
			t.Errorf("%s: esperava o aviso em stderr, veio %q", c.cmd.Name(), errOut.String())
		}
	}
	_ = cloudAuditCmd.Flags().Set("export", "")
}

// Os comandos não usam mais os estilos antigos (o cmd/style.go saiu na T028; o painel do
// cloud, o último, foi redesenhado na T029).
func TestSaidasMigradas_SemEstilosAntigos(t *testing.T) {
	antigos := []string{"titleStyle", "headerStyle", "stepStyle", "checkStyle", "crossStyle",
		"warningStyle", "itemStyle", "grayStyle", "lipgloss.Color(", "lipgloss.NewStyle("}
	arquivos, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, a := range arquivos {
		if strings.HasSuffix(a, "_test.go") {
			continue
		}
		bruto, err := os.ReadFile(a)
		require.NoError(t, err)
		for _, antigo := range antigos {
			if strings.Contains(string(bruto), antigo) {
				t.Errorf("%s ainda usa %s", a, antigo)
			}
		}
	}
}

func TestCaminhoExibido(t *testing.T) {
	raiz := t.TempDir()
	anterior := pastaAtual
	t.Cleanup(func() { pastaAtual = anterior })
	pastaAtual = func() (string, error) { return raiz, nil }

	assert.Equal(t, "bootstrap/secret-store-vault.yaml", caminhoExibido(filepath.Join(raiz, "bootstrap", "secret-store-vault.yaml")))
	fora := filepath.Join(filepath.Dir(raiz), "outro", "x.yaml")
	assert.Equal(t, fora, caminhoExibido(fora), "fora da pasta atual, o caminho fica como veio")

	pastaAtual = func() (string, error) { return "", os.ErrNotExist }
	assert.Equal(t, "a/b.yaml", caminhoExibido("a/b.yaml"))
}
