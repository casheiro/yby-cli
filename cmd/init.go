/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/filesystem"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/services/validate"
	"github.com/casheiro/yby-cli/pkg/templates"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// askConfirm é um atalho mockável para as confirmações fora do assistente (projeto já
// existente quando topologia e workflow vieram por flag, plano do --update).
var askConfirm = func(title string, defaultVal bool) (bool, error) {
	return prompter.Confirm(title, defaultVal)
}

// gerenciadorPluginsInit é o que o init usa do gerenciador de plugins.
type gerenciadorPluginsInit interface {
	Discover() error
	ListPlugins() []plugin.PluginManifest
	ExecuteContextHook(ctx context.Context, bp *scaffold.BlueprintContext) error
	GetAssets() []string
}

// novoGerenciadorPluginsInit, obterProvedorIAInit e novoValidadorInit são trocados nos
// testes: sem plugins do usuário, sem IA de verdade e sem helm. aplicarScaffoldInit
// também, para conferir o relatório sem depender dos templates.
var (
	aplicarScaffoldInit = scaffold.Apply
	// Com a verificação de confiança: o init roda os hooks manifest, context e assets, e
	// plugin não confiável (de um .yby/plugins clonado, por exemplo) não roda nenhum.
	novoGerenciadorPluginsInit = func() gerenciadorPluginsInit {
		pm := plugin.NewManager()
		pm.EnableTrustCheck()
		return pm
	}
	obterProvedorIAInit = ai.GetProvider
	novoValidadorInit   = func() validate.Service {
		return validate.NewService(&validate.RealHelmRunner{Runner: &shared.RealRunner{}})
	}
)

// InitOptions holds the flags for headless mode
type InitOptions struct {
	Topology            string
	Workflow            string
	IncludeDevContainer bool
	IncludeCI           bool

	// Project Details
	TargetDir   string // New Flag
	GitRepo     string
	GitBranch   string
	ProjectName string // New Flag
	Description string // AI Context Flag
	AIProvider  string // explicit provider
	Domain      string
	Email       string
	Environment string

	// Secrets
	SecretsStrategy string

	// Modules
	EnableKepler        bool
	EnableMinio         bool
	EnableKEDA          bool
	EnableMetricsServer bool

	// Environments custom (comma-separated)
	Environments string

	// Enterprise overrides
	ConfigFile string

	// Modes
	Offline        bool
	NonInteractive bool
	Force          bool
	Update         bool

	// sobrescritaConfirmada indica que o assistente já confirmou sobrescrever o projeto
	// existente no destino.
	sobrescritaConfirmada bool
}

var opts InitOptions

func init() {
	rootCmd.AddCommand(initCmd)

	// Bind Flags
	initCmd.Flags().StringVar(&opts.Topology, "topology", "", "Estratégia de Topologia: single, standard, complete")
	initCmd.Flags().StringVar(&opts.Workflow, "workflow", "", "Padrão de Workflow: essential, gitflow, trunkbased")
	initCmd.Flags().BoolVar(&opts.IncludeDevContainer, "include-devcontainer", false, "Gerar configuração .devcontainer")
	initCmd.Flags().BoolVar(&opts.IncludeCI, "include-ci", true, "Habilitar geração de CI/CD")
	initCmd.Flags().BoolVar(&opts.Offline, "offline", false, "Modo offline: pula as verificações do Git remoto e usa os padrões locais")
	initCmd.Flags().BoolVar(&opts.NonInteractive, "non-interactive", false, "Modo Não-Interativo: Falha se argumentos obrigatórios estiverem faltando (Ideal para VPS/CI)")
	initCmd.Flags().BoolVar(&opts.Force, "force", false, "Sobrescrever projeto existente sem confirmação")
	initCmd.Flags().BoolVar(&opts.Update, "update", false, "Atualiza scaffold preservando alterações do usuário")

	initCmd.Flags().StringVarP(&opts.TargetDir, "target-dir", "t", "", "Diretório alvo para inicialização do projeto")
	initCmd.Flags().StringVar(&opts.GitRepo, "git-repo", "", "URL do Repositório Git")
	initCmd.Flags().StringVar(&opts.GitBranch, "git-branch", "main", "Branch principal do git")
	initCmd.Flags().StringVar(&opts.ProjectName, "project-name", "", "Nome do Projeto/Slug (Sobrescreve derivação padrão)")
	initCmd.Flags().StringVar(&opts.Description, "description", "", "Descrição em linguagem natural do projeto (Habilita geração por IA)")
	initCmd.Flags().StringVar(&opts.AIProvider, "ai-provider", "", "Forçar provedor de IA específico (ollama, gemini, openai)")
	initCmd.Flags().StringVar(&opts.Domain, "domain", "yby.local", "Domínio base do cluster")
	initCmd.Flags().StringVar(&opts.Email, "email", "admin@yby.local", "Email do admin")
	initCmd.Flags().StringVar(&opts.Environment, "env", "dev", "Nome do ambiente inicial")
	initCmd.Flags().StringVar(&opts.Environments, "environments", "", "Lista de ambientes separados por vírgula (ex: local,dev,hom,prod). Sobrescreve os ambientes padrão da topologia")
	initCmd.Flags().StringVar(&opts.ConfigFile, "config", "", "Arquivo de configuração enterprise (.yby/overrides.yaml)")

	initCmd.Flags().StringVar(&opts.SecretsStrategy, "secrets-strategy", "external-secrets", "Estratégia de secrets: sealed-secrets, external-secrets, sops")

	initCmd.Flags().BoolVar(&opts.EnableKepler, "enable-kepler", false, "Habilitar módulo Kepler")
	initCmd.Flags().BoolVar(&opts.EnableMinio, "enable-minio", false, "Habilitar módulo MinIO")
	initCmd.Flags().BoolVar(&opts.EnableKEDA, "enable-keda", false, "Habilitar módulo KEDA")
	initCmd.Flags().BoolVar(&opts.EnableMetricsServer, "enable-metrics-server", false, "Habilitar Metrics Server (Observability)")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inicializa um novo projeto Yby (Scaffold)",
	Long: `Gera a estrutura inicial do projeto (Charts, Manifests, Workflows) baseada em padrões.
Suporta execução interativa (Wizard) ou Headless (Flags).`,
	Example: `  # Modo Interativo (Wizard)
  yby init

  # Modo Headless (CI/CD ou Scripts)
  yby init --project-name meu-app --git-repo https://github.com/org/repo.git --topology standard --workflow gitflow --target-dir infra

  # AI-Native Initialization
  yby init --description "A secure payment gateway for crypto assets"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return executarInit(cmd.OutOrStdout(), &opts)
	},
}

// colunasInit são as colunas das linhas do relatório do init (nome e estado).
var colunasInit = ui.Colunas{Nome: 14, Versao: 15}

// avisoNaoConfiaveis é a nota da linha dos plugins não confiáveis do init: o verbo
// concorda com a quantidade e, com um só, o comando já leva o nome dele.
func avisoNaoConfiaveis(nomes []string) string {
	lista := strings.Join(nomes, ", ")
	if len(nomes) == 1 {
		return lista + " · não rodou; rode yby plugin trust " + nomes[0]
	}
	return lista + " · não rodaram; rode yby plugin trust <nome>"
}

// relatorioInit escreve as linhas do init conforme acontecem e conta ok, avisos e erros
// para o resumo do fim.
type relatorioInit struct {
	w                io.Writer
	ok, aviso, falha int
}

func (r *relatorioInit) secao(nome string) { ui.Section(r.w, nome) }

func (r *relatorioInit) linha(tipo ui.TipoCheck, nome, estado, nota string) {
	switch tipo {
	case ui.CheckOk:
		r.ok++
	case ui.CheckAviso:
		r.aviso++
	case ui.CheckErro:
		r.falha++
	}
	ui.CheckCom(r.w, colunasInit, tipo, nome, estado, umaLinha(nota))
}

func (r *relatorioInit) resumo(frase string) { ui.Summary(r.w, r.ok, r.aviso, r.falha, frase) }

// umaLinha junta as linhas de uma mensagem (saída do helm, erro com quebras) numa só.
func umaLinha(s string) string { return strings.Join(strings.Fields(s), " ") }

// caminhoManifest é o caminho do .yby/project.yaml no destino, como os outros caminhos do
// relatório.
func caminhoManifest(targetDir string) string {
	return filepath.ToSlash(filepath.Join(targetDir, ".yby", "project.yaml"))
}

// confirmarInit faz uma pergunta de sim ou não fora do assistente. Recusar ou sair com
// ctrl+c devolve false sem erro (é cancelar); sem resposta (sem terminal, falha na
// leitura) devolve um erro com a dica de como rodar sem perguntar.
func confirmarInit(titulo string, padrao bool, dica string) (bool, error) {
	confirmado, err := askConfirm(titulo, padrao)
	if err == nil {
		return confirmado, nil
	}
	if stderrors.Is(err, huh.ErrUserAborted) {
		return false, nil
	}
	return false, errors.Wrap(err, errors.ErrCodeIO, "falha ao ler a confirmação").WithHint(dica)
}

// executarInit é o corpo do yby init: monta o contexto (flags e, se faltar algo, o
// assistente), gera o scaffold e escreve o relatório em w. Nada é gravado antes de o
// contexto estar completo e confirmado.
func executarInit(w io.Writer, o *InitOptions) error {
	// Validação: --update e --force são mutuamente exclusivos
	if o.Update && o.Force {
		return errors.New(errors.ErrCodeValidation, "--update e --force são mutuamente exclusivos").
			WithHint("use --update para mesclar ou --force para sobrescrever")
	}

	// O init não recebe contexto de quem o chama: os hooks dos plugins, a IA e o helm
	// rodam com este.
	bgCtx := context.Background()

	// 1. Plugins e contexto (flags + assistente)
	pm := novoGerenciadorPluginsInit()
	errPlugins := pm.Discover()

	ctx, errCtx := buildContext(o)
	var cancelado initCancelado
	if stderrors.As(errCtx, &cancelado) {
		// saída voluntária do assistente: não é falha
		ui.Check(w, ui.CheckNeutro, "init cancelado", "", cancelado.motivo)
		return nil
	}
	if errCtx != nil {
		return errCtx
	}

	// Hook: context (enriquece o BlueprintContext)
	errHook := pm.ExecuteContextHook(bgCtx, ctx)

	// 1.5 Detecção de init duplo (pular se --update, que trata isso depois)
	targetDir := resolveTargetDir(o.TargetDir)
	blueprintPath := filepath.Join(targetDir, ".yby", "blueprint.yaml")
	projetoExistente := false
	if !o.Update {
		if _, err := os.Stat(blueprintPath); err == nil {
			projetoExistente = true
			switch {
			case o.Force, o.sobrescritaConfirmada:
			case o.NonInteractive:
				return errors.New(errors.ErrCodeValidation, "o destino já tem um projeto Yby").
					WithHint("use --force para sobrescrever ou --update para atualizar o projeto")
			default:
				confirm, err := confirmarInit("Sobrescrever o projeto Yby existente?", false,
					"use --force para sobrescrever sem perguntar ou --update para atualizar o projeto")
				if err != nil {
					return err
				}
				if !confirm {
					ui.Check(w, ui.CheckNeutro, "init cancelado", "", "o projeto existente não foi sobrescrito")
					return nil
				}
			}
		}
	}

	// 1.6 Carregar manifest anterior como defaults (se existir)
	var existingManifest *scaffold.ProjectManifest
	if m, err := scaffold.LoadProjectManifest(targetDir); err == nil {
		existingManifest = m
		defaults := scaffold.ManifestToContext(existingManifest)
		scaffold.MergeContextDefaults(ctx, defaults)
	}

	ui.Title(w, "init", agora().Format("02/01 15:04"))
	rel := &relatorioInit{w: w}
	rel.secao("projeto")
	rel.linha(ui.CheckOk, "nome", ctx.ProjectName, "")
	rel.linha(ui.CheckOk, "destino", filepath.ToSlash(targetDir), "")
	rel.linha(ui.CheckOk, "topologia", ctx.Topology, strings.Join(ctx.Environments, " · "))
	rel.linha(ui.CheckOk, "workflow", ctx.WorkflowPattern, "")
	switch {
	case o.Environment == "dev" && ctx.Environment != o.Environment:
		// dev é o padrão da flag: a troca é esperada, não pede atenção
		rel.linha(ui.CheckOk, "ambiente", ctx.Environment, "a topologia "+ctx.Topology+" não tem dev")
	case o.Environment != "" && ctx.Environment != o.Environment:
		rel.linha(ui.CheckAviso, "ambiente", ctx.Environment,
			fmt.Sprintf("%s não existe na topologia %s", o.Environment, ctx.Topology))
	default:
		rel.linha(ui.CheckOk, "ambiente", ctx.Environment, "")
	}
	if errPlugins != nil {
		slog.Warn("falha na descoberta de plugins", "erro", errPlugins)
		rel.linha(ui.CheckAviso, "plugins", "não lidos", errPlugins.Error())
	} else {
		// O plugin não confiável não rodou nenhum hook: vai numa linha de aviso à parte.
		var nomes, naoConfiaveis []string
		for _, p := range pm.ListPlugins() {
			if p.NaoConfiavel {
				naoConfiaveis = append(naoConfiaveis, p.Name)
			} else {
				nomes = append(nomes, p.Name)
			}
		}
		if len(nomes) > 0 {
			rel.linha(ui.CheckOk, "plugins", fmt.Sprintf("%d", len(nomes)), strings.Join(nomes, ", "))
		}
		if len(naoConfiaveis) > 0 {
			rel.linha(ui.CheckAviso, "plugins", contagem(len(naoConfiaveis), "não confiável", "não confiáveis"),
				avisoNaoConfiaveis(naoConfiaveis))
		}
	}
	if errHook != nil {
		slog.Warn("falha no hook context dos plugins", "erro", errHook)
		rel.linha(ui.CheckAviso, "plugins", "hook context", errHook.Error())
	}
	if projetoExistente {
		rel.linha(ui.CheckAviso, "projeto", "já existia", "sobrescrevendo")
	}
	if existingManifest != nil {
		rel.linha(ui.CheckOk, "manifest", "anterior", "configurações carregadas como base")
	}

	// 1.7 Fluxo --update: merge inteligente preservando alterações do usuário
	if o.Update {
		if existingManifest == nil {
			return errors.New(errors.ErrCodeValidation,
				"--update requer um projeto já inicializado com manifest (.yby/project.yaml)").
				WithHint("rode yby init sem --update para gerar o projeto")
		}

		return runUpdateFlow(rel, targetDir, ctx, existingManifest, o)
	}

	// 2. Scaffold
	// Camadas: [Core embed] + [assets dos plugins]. O CompositeFS procura do fim para o
	// começo, então os plugins (índices maiores) sobrepõem o core.
	layers := []fs.FS{templates.Assets}
	for _, path := range pm.GetAssets() {
		// o diretório do plugin deve conter a pasta "assets", como o core
		layers = append(layers, os.DirFS(path))
	}
	compositeFS := filesystem.NewCompositeFS(layers...)

	if err := aplicarScaffoldInit(targetDir, ctx, compositeFS); err != nil {
		return errors.Wrap(err, errors.ErrCodeScaffold, "o scaffold falhou").
			WithHint("confira o template indicado na causa e os assets dos plugins")
	}
	rel.secao("arquivos")
	rel.linha(ui.CheckOk, "scaffold", "gerado", "")
	avisarCamposVazios(rel, targetDir, o.Offline)

	// 2.5 Camada de IA: só com descrição (--description ou no assistente)
	if o.Description != "" {
		gerarComIA(rel, bgCtx, o, targetDir)
	}

	// 3. Values de cada ambiente (diferenciados)
	for _, env := range ctx.Environments {
		target := filepath.Join(targetDir, fmt.Sprintf("config/values-%s.yaml", env))
		if _, err := os.Stat(target); os.IsNotExist(err) {
			content := scaffold.RenderEnvironmentValues(ctx, env)
			_ = os.MkdirAll(filepath.Dir(target), 0755)
			_ = os.WriteFile(target, []byte(content), 0644)
			rel.linha(ui.CheckOk, env, "values", filepath.ToSlash(target))
		}
	}

	// 4. Persistir o manifest do projeto (.yby/project.yaml); ele não guarda hashes, então
	// gravá-lo antes da validação não muda o conteúdo
	if err := scaffold.SaveProjectManifest(targetDir, ctx); err != nil {
		rel.linha(ui.CheckAviso, "manifest", "não salvo", err.Error())
	} else {
		rel.linha(ui.CheckOk, "manifest", "salvo", caminhoManifest(targetDir))
	}

	// 5. Validação pós-scaffold (só avisa, não bloqueia)
	validarChartsInit(rel, bgCtx, targetDir)

	rel.resumo("próximo passo: yby env list")
	return nil
}

// avisarCamposVazios mostra os campos críticos que o scaffold deixou vazios (repoURL
// sem --git-repo, por exemplo): o projeto é gerado, mas o Argo CD precisa deles. No
// --offline o repoURL fica vazio de propósito: o bootstrap aponta para o git-server local.
func avisarCamposVazios(rel *relatorioInit, targetDir string, offline bool) {
	vazios := scaffold.CamposCriticosVazios(targetDir)
	campos := make([]string, 0, len(vazios))
	for campo := range vazios {
		if offline && (campo == "repoURL" || campo == "repoName") {
			continue
		}
		campos = append(campos, campo)
	}
	sort.Strings(campos)
	for _, campo := range campos {
		dica := "preencha antes do yby bootstrap"
		if campo == "repoURL" || campo == "repoName" {
			dica = "use --git-repo ou preencha antes do yby bootstrap"
		}
		arquivos := "arquivos"
		if vazios[campo] == 1 {
			arquivos = "arquivo"
		}
		rel.linha(ui.CheckAviso, campo, "vazio", fmt.Sprintf("%d %s · %s", vazios[campo], arquivos, dica))
	}
}

// gerarComIA pede à IA a governança a partir da descrição e grava os arquivos que vierem.
// Sem provedor ou com falha, ficam só os templates.
func gerarComIA(rel *relatorioInit, bgCtx context.Context, o *InitOptions, targetDir string) {
	aiProvider := obterProvedorIAInit(bgCtx, o.AIProvider)
	if aiProvider == nil {
		rel.linha(ui.CheckAviso, "ia", "sem provedor", "suba o Ollama ou defina GEMINI_API_KEY ou OPENAI_API_KEY")
		return
	}
	blueprint, err := generateGovernanceViaCompletion(bgCtx, aiProvider, o.Description)
	if err != nil {
		rel.linha(ui.CheckAviso, "ia", aiProvider.Name(), "falhou, ficam os templates: "+err.Error())
		return
	}
	rel.linha(ui.CheckOk, "ia", aiProvider.Name(), fmt.Sprintf("%s · risco %s", blueprint.Domain, blueprint.RiskLevel))

	for _, f := range blueprint.Files {
		fullPath := filepath.Join(targetDir, f.Path)
		// .github vai para a raiz do repositório Git, não para dentro do targetDir
		if strings.HasPrefix(f.Path, ".github") {
			if gitRoot, err := scaffold.GetGitRoot(); err == nil && gitRoot != "" {
				fullPath = filepath.Join(gitRoot, f.Path)
			} else if targetDir != "." && targetDir != "" {
				wd, _ := os.Getwd()
				fullPath = filepath.Join(wd, f.Path)
			}
		}
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte(f.Content), 0644); err == nil {
			rel.linha(ui.CheckOk, "ia", "gerado", f.Path)
		}
	}
}

// validarChartsInit valida com o helm os charts gerados que existirem. Problemas viram
// avisos: o projeto já está gerado e o yby validate mostra os detalhes.
func validarChartsInit(rel *relatorioInit, bgCtx context.Context, targetDir string) {
	var existingCharts []string
	for _, c := range []string{"charts/system", "charts/bootstrap", "charts/cluster-config"} {
		chart := filepath.Join(targetDir, c)
		if _, err := os.Stat(filepath.Join(chart, "Chart.yaml")); err == nil {
			existingCharts = append(existingCharts, chart)
		}
	}
	if len(existingCharts) == 0 {
		return
	}

	rel.secao("validação")
	if _, err := lookPath("helm"); err != nil {
		rel.linha(ui.CheckAviso, "charts", "não validados", "helm não encontrado · instale e rode yby validate")
		return
	}
	valuesFile := filepath.Join(targetDir, "config/cluster-values.yaml")
	report, err := novoValidadorInit().Run(bgCtx, existingCharts, valuesFile)
	if err == nil && report != nil && report.Success {
		rel.linha(ui.CheckOk, "charts", fmt.Sprintf("%d válidos", len(existingCharts)), "")
		return
	}
	if err != nil {
		rel.linha(ui.CheckAviso, "charts", "não validados", mensagemDoErro(err))
	} else if report != nil {
		for _, cr := range report.Charts {
			if cr.Error != "" {
				rel.linha(ui.CheckAviso, filepath.Base(cr.Chart), "com problema", cr.Error)
			}
		}
	}
	rel.linha(ui.CheckNeutro, "yby validate mostra os detalhes", "", "")
}

func buildContext(flags *InitOptions) (*scaffold.BlueprintContext, error) {
	ctx := &scaffold.BlueprintContext{
		GitRepoURL:          flags.GitRepo,
		GitBranch:           flags.GitBranch,
		Domain:              flags.Domain,
		Email:               flags.Email,
		Environment:         flags.Environment,
		EnableCI:            flags.IncludeCI,
		EnableDevContainer:  flags.IncludeDevContainer,
		EnableKepler:        flags.EnableKepler,
		EnableMinio:         flags.EnableMinio,
		EnableKEDA:          flags.EnableKEDA,
		EnableMetricsServer: flags.EnableMetricsServer,
		SecretsStrategy:     flags.SecretsStrategy,
		Topology:            flags.Topology,
		WorkflowPattern:     flags.Workflow,

		// Template Data
		GitRepo:     flags.GitRepo,
		ProjectName: resolveProjectName(flags),
	}

	// Carregar enterprise overrides
	targetDir := flags.TargetDir
	if targetDir == "" {
		targetDir, _ = os.Getwd()
	}
	overridePaths := scaffold.ResolveOverridePaths(flags.ConfigFile, targetDir)
	overrides, err := scaffold.LoadOverrides(overridePaths...)
	if err != nil {
		return nil, err
	}
	ctx.Overrides = overrides

	// Populate Github details
	if org := extractGithubOrg(flags.GitRepo); org != "" {
		ctx.GithubOrg = org
		ctx.GithubDiscovery = true
	} else {
		// Default to a placeholder if needed, or disable discovery
		// If Discovery is enabled by default in schema, we should provide something?
		// Schema says enabled: true default.
		// If we leave empty, helm might complain OR render empty string.
		// Let's set a default "yby-org" if strictly needed, or trust user fills it.
		// For Zero Config, we try our best.
		if flags.Offline {
			ctx.GithubOrg = "yby-local"
			ctx.GithubDiscovery = false // Disable in offline/local mirror mode usually?
		}
	}

	// Calculate RepoRootPath (for ArgoCD)
	if gitRoot, err := scaffold.GetGitRoot(); err == nil && gitRoot != "" {
		// Determine absolute target path
		targetDir := "."
		if flags.TargetDir != "" {
			targetDir = flags.TargetDir
		}
		absTarget, _ := filepath.Abs(targetDir)

		// Calculate relative path from GitRoot to TargetDir
		if rel, err := filepath.Rel(gitRoot, absTarget); err == nil {
			// If target is same as root, rel is "." -> empty for path joining
			if rel == "." {
				ctx.RepoRootPath = ""
			} else {
				// Ensure forward slashes for ArgoCD/K8s manifests
				ctx.RepoRootPath = filepath.ToSlash(rel)
			}
		}
	}

	// Enrich with AI Context
	inferContext(ctx)

	// If flags are missing, ask via Survey (Interactive Mode)
	// We check strictly if Topology/Workflow are empty implies interaction needed.
	// Or we can ask specifically for what's missing.

	interactive := false
	// Interaction logic:
	// If NonInteractive is TRUE -> User explicitly wants NO prompts. We must VALIDATE required flags.
	// If NonInteractive is FALSE -> We default to interactive if critical flags are missing.

	if flags.NonInteractive {
		// VALIDATION MODE
		missing := validateNonInteractiveFlags(ctx, flags)
		if len(missing) > 0 {
			return nil, errors.New(errors.ErrCodeValidation,
				"faltam flags para o modo --non-interactive: "+strings.Join(missing, ", ")).
				WithHint("passe as flags que faltam ou rode yby init sem --non-interactive")
		}
		interactive = false
	} else {
		// Default behavior: specific flags missing trigger interaction
		if ctx.Topology == "" || ctx.WorkflowPattern == "" {
			interactive = true
		}
	}

	// Assistente: um formulário só, uma etapa por grupo, com o que as flags não trouxeram.
	// Cancelar no meio não grava nada: as respostas só valem depois da revisão.
	if interactive {
		if err := perguntarInit(flags, ctx); err != nil {
			return nil, err
		}
	}

	// Post-Validation / Defaults

	// Sanitizar project name em modo interativo
	if interactive && ctx.ProjectName != "" {
		ctx.ProjectName = scaffold.SanitizeProjectName(ctx.ProjectName)
	}

	// Validar contexto (project-name, domain, email, git-repo, topology, workflow)
	if err := scaffold.ValidateContext(ctx); err != nil {
		return nil, err
	}

	// Calcula lista de ambientes: usa --environments se fornecido, senão defaults da topologia
	if flags.Environments != "" {
		envs := parseEnvironments(flags.Environments)
		if err := scaffold.ValidateEnvironmentNames(envs); err != nil {
			return nil, err
		}
		ctx.Environments = envs
	} else {
		ctx.Environments = environmentsForTopology(ctx.Topology)
	}

	// Modo offline: garante que "local" está presente
	if flags.Offline {
		ctx.Environments = ensureLocalEnvironment(ctx.Environments)
	}

	// Valida se o ambiente atual existe na topologia; o ajuste aparece como aviso no relatório
	if newEnv, valid := validateEnvironment(ctx.Environment, ctx.Environments); !valid {
		ctx.Environment = newEnv
	}

	return ctx, nil
}

func resolveProjectName(flags *InitOptions) string {
	if flags.ProjectName != "" {
		return flags.ProjectName
	}
	return deriveProjectName(flags.GitRepo)
}

func deriveProjectName(repoURL string) string {
	if repoURL == "" {
		return "yby-project"
	}

	// 1. Clean URL
	repoURL = strings.TrimSpace(repoURL)
	repoURL = strings.TrimSuffix(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// 2. Extract last part
	parts := strings.Split(repoURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return "yby-project"
}

func extractGithubOrg(repoURL string) string {
	if repoURL == "" {
		return ""
	}
	repoURL = strings.TrimSpace(repoURL)
	repoURL = strings.TrimSuffix(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Handle git@github.com:org/repo
	if strings.Contains(repoURL, "git@") {
		parts := strings.Split(repoURL, ":")
		if len(parts) == 2 {
			repoURL = parts[1]
		}
	} else {
		// Handle https://github.com/org/repo
		parts := strings.Split(repoURL, "github.com/")
		if len(parts) == 2 {
			repoURL = parts[1]
		}
	}

	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		return parts[0] // Org is the first part
	}
	return ""
}

// resolveTargetDir retorna o diretório alvo para o scaffold.
// Se o TargetDir das opções estiver vazio, retorna ".".
func resolveTargetDir(targetDir string) string {
	if targetDir != "" {
		return targetDir
	}
	return "."
}

// environmentsForTopology retorna a lista de ambientes baseada na topologia informada.
func environmentsForTopology(topology string) []string {
	switch topology {
	case "single":
		return []string{"local"}
	case "standard":
		return []string{"local", "prod"}
	case "complete":
		return []string{"local", "dev", "staging", "prod"}
	default:
		return []string{"local"}
	}
}

// parseEnvironments converte uma string comma-separated em uma lista de ambientes,
// removendo espaços em branco ao redor de cada nome.
func parseEnvironments(raw string) []string {
	parts := strings.Split(raw, ",")
	var envs []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			envs = append(envs, trimmed)
		}
	}
	return envs
}

// ensureLocalEnvironment garante que o ambiente "local" está presente na lista,
// adicionando-o ao início caso não exista. Usado no modo offline.
func ensureLocalEnvironment(envs []string) []string {
	for _, e := range envs {
		if e == "local" {
			return envs
		}
	}
	return append([]string{"local"}, envs...)
}

// validateEnvironment verifica se o ambiente atual é válido para a topologia.
// Se não for, retorna o primeiro ambiente da lista e found=false.
func validateEnvironment(env string, envs []string) (string, bool) {
	for _, e := range envs {
		if e == env {
			return env, true
		}
	}
	if len(envs) > 0 {
		return envs[0], false
	}
	return env, false
}

// mapModuleSelection converte a lista de seleções do prompt MultiSelect
// em flags booleanas de módulos.
func mapModuleSelection(selectedModules []string) (kepler, minio, keda, metricsServer bool) {
	for _, m := range selectedModules {
		if strings.Contains(m, "Kepler") {
			kepler = true
		}
		if strings.Contains(m, "MinIO") {
			minio = true
		}
		if strings.Contains(m, "KEDA") {
			keda = true
		}
		if strings.Contains(m, "Observability") {
			metricsServer = true
		}
	}
	return
}

// validateNonInteractiveFlags valida que os campos obrigatórios estão presentes
// no modo não-interativo. Retorna a lista de flags faltantes.
func validateNonInteractiveFlags(ctx *scaffold.BlueprintContext, flags *InitOptions) []string {
	missing := []string{}
	if ctx.Topology == "" {
		missing = append(missing, "--topology")
	}
	if ctx.WorkflowPattern == "" {
		missing = append(missing, "--workflow")
	}
	if ctx.GitRepoURL == "" && !flags.Offline {
		if ctx.ProjectName == "yby-project" && flags.ProjectName == "" {
			missing = append(missing, "--project-name ou --git-repo")
		}
	}
	return missing
}

// resolveAIFilePath determina o caminho completo para um arquivo gerado por IA.
// Arquivos .github são colocados na raiz do repositório Git, não dentro do targetDir.
func resolveAIFilePath(filePath, targetDir, gitRoot string) string {
	fullPath := filepath.Join(targetDir, filePath)

	if strings.HasPrefix(filePath, ".github") {
		if gitRoot != "" {
			return filepath.Join(gitRoot, filePath)
		}
		if targetDir != "." && targetDir != "" {
			// Sem gitRoot, usa o diretório de trabalho atual
			// (nesta função pura, retornamos apenas baseado no targetDir)
			return filePath // retorna o caminho relativo ao CWD
		}
	}

	return fullPath
}

// inferContext populates AI-related context fields based on heuristics
func inferContext(ctx *scaffold.BlueprintContext) {
	name := strings.ToLower(ctx.ProjectName)

	// Defaults
	ctx.BusinessDomain = "General Purpose"
	ctx.ImpactLevel = "Medium"
	ctx.Archetype = "Cloud-Native Application"

	// Heuristics
	if strings.Contains(name, "bank") || strings.Contains(name, "pay") || strings.Contains(name, "wallet") || strings.Contains(name, "fin") {
		ctx.BusinessDomain = "Fintech / Financial Services"
		ctx.ImpactLevel = "Critical (High Security Requirement)"
	} else if strings.Contains(name, "shop") || strings.Contains(name, "store") || strings.Contains(name, "comm") || strings.Contains(name, "cart") {
		ctx.BusinessDomain = "E-Commerce / Retail"
		ctx.ImpactLevel = "High (Availability Requirement)"
	} else if strings.Contains(name, "data") || strings.Contains(name, "etl") || strings.Contains(name, "flow") || strings.Contains(name, "lake") {
		ctx.BusinessDomain = "Data Engineering"
		ctx.Archetype = "Data Pipeline / Batch Processing"
	} else if strings.Contains(name, "api") || strings.Contains(name, "svc") || strings.Contains(name, "gate") {
		ctx.Archetype = "Backend Microservice"
	}

	if ctx.Topology == "complete" {
		ctx.ImpactLevel += " (Enterprise Topology)"
	}
}

// runUpdateFlow executa o fluxo de --update: gera scaffold em tmpdir, computa merge plan e aplica.
func runUpdateFlow(rel *relatorioInit, targetDir string, ctx *scaffold.BlueprintContext, manifest *scaffold.ProjectManifest, flags *InitOptions) error {
	// 1. Gerar scaffold em diretório temporário
	tmpDir, err := os.MkdirTemp("", "yby-update-*")
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeIO, "falha ao criar a pasta temporária do --update").
			WithHint("confira o espaço e as permissões da pasta temporária do sistema")
	}
	defer os.RemoveAll(tmpDir)

	layers := []fs.FS{templates.Assets}
	compositeFS := filesystem.NewCompositeFS(layers...)

	if err := aplicarScaffoldInit(tmpDir, ctx, compositeFS); err != nil {
		return errors.Wrap(err, errors.ErrCodeScaffold, "o scaffold para comparação falhou").
			WithHint("confira o template indicado na causa")
	}

	// 2. Computar plano de merge
	manifestHashes := manifest.Spec.FileHashes
	if manifestHashes == nil {
		manifestHashes = make(map[string]string)
	}

	plan, err := scaffold.ComputeMergePlan(manifestHashes, targetDir, tmpDir)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeScaffold, "falha ao montar o plano de merge").
			WithHint("confira se os arquivos do projeto podem ser lidos")
	}

	// 3. Exibir resumo do plano
	summary := plan.Summary()
	rel.secao("plano de merge")
	linhas := []struct {
		acao   scaffold.FileAction
		tipo   ui.TipoCheck
		rotulo string
	}{
		{scaffold.ActionNone, ui.CheckNeutro, "sem mudança"},
		{scaffold.ActionUpdate, ui.CheckOk, "a atualizar"},
		{scaffold.ActionPreserve, ui.CheckNeutro, "preservados"},
		{scaffold.ActionConflict, ui.CheckAviso, "em conflito"},
		{scaffold.ActionNew, ui.CheckOk, "novos"},
	}
	for _, l := range linhas {
		if n := summary[l.acao]; n > 0 {
			rotulo := "arquivos"
			if n == 1 {
				rotulo = "arquivo"
			}
			rel.linha(l.tipo, l.rotulo, fmt.Sprintf("%d %s", n, rotulo), "")
		}
	}

	// Se não há nada a fazer, sair cedo
	totalChanges := summary[scaffold.ActionUpdate] + summary[scaffold.ActionConflict] + summary[scaffold.ActionNew]
	if totalChanges == 0 {
		rel.resumo("o projeto já está atualizado")
		return nil
	}

	// 4. Confirmar com o usuário
	if !flags.NonInteractive {
		confirm, err := confirmarInit("Aplicar o plano de merge?", true,
			"use --non-interactive para aplicar o plano sem perguntar")
		if err != nil {
			return err
		}
		if !confirm {
			rel.linha(ui.CheckNeutro, "merge", "cancelado", "nenhum arquivo foi alterado")
			return nil
		}
	}

	// 5. Aplicar plano com resolver de conflitos
	resolver := &scaffold.NonInteractiveResolver{Strategy: "conflict-markers"}
	if err := scaffold.ApplyMergePlan(plan, targetDir, tmpDir, resolver); err != nil {
		return errors.Wrap(err, errors.ErrCodeScaffold, "falha ao aplicar o plano de merge").
			WithHint("confira o arquivo indicado na causa; o git mostra o que já mudou")
	}

	// 6. Salvar manifest com novos hashes
	if newHashes, err := scaffold.ComputeDirHashes(targetDir); err != nil {
		rel.linha(ui.CheckAviso, "manifest", "sem hashes", err.Error())
	} else if err := scaffold.SaveProjectManifest(targetDir, ctx, newHashes); err != nil {
		rel.linha(ui.CheckAviso, "manifest", "não salvo", err.Error())
	} else {
		rel.linha(ui.CheckOk, "manifest", "salvo", caminhoManifest(targetDir))
	}

	rel.resumo("scaffold atualizado")
	return nil
}
