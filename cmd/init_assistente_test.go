package cmd

import (
	"bytes"
	"context"
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/casheiro/yby-cli/v5/pkg/ai"
	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/plugin"
	"github.com/casheiro/yby-cli/v5/pkg/scaffold"
	"github.com/casheiro/yby-cli/v5/pkg/services/validate"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
)

// ========================================================
// Dublês do init: plugins, IA e helm
// ========================================================

// pluginsFalsos implementa gerenciadorPluginsInit sem executar binário nenhum.
type pluginsFalsos struct {
	plugins       []plugin.PluginManifest
	errDescoberta error
	errHook       error
}

func (p *pluginsFalsos) Discover() error                      { return p.errDescoberta }
func (p *pluginsFalsos) ListPlugins() []plugin.PluginManifest { return p.plugins }
func (p *pluginsFalsos) ExecuteContextHook(context.Context, *scaffold.BlueprintContext) error {
	return p.errHook
}
func (p *pluginsFalsos) GetAssets() []string { return nil }

// validadorFalso implementa validate.Service sem chamar o helm.
type validadorFalso struct {
	relatorio *validate.ValidateReport
	err       error
}

func (v *validadorFalso) Run(_ context.Context, charts []string, _ string) (*validate.ValidateReport, error) {
	if v.relatorio == nil && v.err == nil {
		return &validate.ValidateReport{Success: true}, nil
	}
	return v.relatorio, v.err
}

// isolarInit deixa o init sem efeitos fora do diretório do teste: home vazia (sem plugins
// nem overrides do usuário), plugins falsos, helm falso e IA que falha o teste se chamada.
func isolarInit(t *testing.T) *pluginsFalsos {
	t.Helper()
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("YBY_AI_PROVIDER", "nenhum")

	plugins := &pluginsFalsos{}
	pm, ia, val := novoGerenciadorPluginsInit, obterProvedorIAInit, novoValidadorInit
	t.Cleanup(func() { novoGerenciadorPluginsInit, obterProvedorIAInit, novoValidadorInit = pm, ia, val })
	novoGerenciadorPluginsInit = func() gerenciadorPluginsInit { return plugins }
	obterProvedorIAInit = func(context.Context, string) ai.Provider {
		t.Fatal("o init não pode chamar a IA de verdade nos testes")
		return nil
	}
	novoValidadorInit = func() validate.Service { return &validadorFalso{} }
	// o validador é falso: o helm da máquina não muda o relatório
	busca := lookPath
	t.Cleanup(func() { lookPath = busca })
	lookPath = func(nome string) (string, error) {
		if nome == "helm" {
			return "helm", nil
		}
		return busca(nome)
	}
	return plugins
}

// noDiretorio roda f com o diretório de trabalho em dir e volta ao anterior.
func noDiretorio(t *testing.T, dir string, f func()) {
	t.Helper()
	anterior, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(anterior) }()
	f()
}

// arquivosEm lista o que existe sob dir (vazio quando nada foi gravado).
func arquivosEm(t *testing.T, dir string) []string {
	t.Helper()
	var nomes []string
	_ = filepath.WalkDir(dir, func(p string, _ fs.DirEntry, _ error) error {
		if p != dir {
			rel, _ := filepath.Rel(dir, p)
			nomes = append(nomes, filepath.ToSlash(rel))
		}
		return nil
	})
	return nomes
}

// opcoesInit devolve as opções do init com os padrões das flags.
func opcoesInit() *InitOptions {
	return &InitOptions{
		GitBranch: "main", Domain: "yby.local", Email: "admin@yby.local", Environment: "dev",
		IncludeCI: true, SecretsStrategy: "external-secrets",
	}
}

// ========================================================
// Condutor do assistente (sem terminal)
// ========================================================

// executarCmdInit roda cmd e devolve as mensagens que ele produz, abrindo tea.Batch e
// tea.Sequence. Comandos que demoram (o cursor piscando) são descartados.
func executarCmdInit(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-ch:
	case <-time.After(30 * time.Millisecond):
		return nil
	}
	if msg == nil {
		return nil
	}
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		var msgs []tea.Msg
		for i := 0; i < v.Len(); i++ {
			c, _ := v.Index(i).Interface().(tea.Cmd)
			msgs = append(msgs, executarCmdInit(c)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

// entregarInit entrega msg ao modelo junto com o que os comandos devolvidos produzirem.
func entregarInit(m tea.Model, fila ...tea.Msg) {
	for i := 0; len(fila) > 0 && i < 1000; i++ {
		msg := fila[0]
		fila = fila[1:]
		_, cmd := m.Update(msg)
		fila = append(fila, executarCmdInit(cmd)...)
	}
}

// Mensagens de roteiro, tratadas pelo condutor e não entregues ao assistente.
type (
	// capturar guarda a view atual com um título.
	capturar struct{ nome string }
	// ateOFim tecla ↵ até o assistente terminar.
	ateOFim struct{}
)

var (
	enter      = tea.KeyMsg{Type: tea.KeyEnter}
	voltar     = tea.KeyMsg{Type: tea.KeyShiftTab}
	esc        = tea.KeyMsg{Type: tea.KeyEsc}
	baixo      = tea.KeyMsg{Type: tea.KeyDown}
	apagar     = tea.KeyMsg{Type: tea.KeyCtrlU}
	espaco     = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	sim        = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	nao        = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	digitar    = func(s string) tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
	larguraFix = tea.WindowSizeMsg{Width: 64, Height: 40}
)

// conducao registra o que o condutor viu: o assistente, a etapa depois de cada tecla e
// as views capturadas.
type conducao struct {
	assistente *ui.Assistente
	etapas     []int
	views      strings.Builder
}

// conduzirAssistente troca executarAssistente por um condutor que entrega o roteiro ao
// modelo, sem terminal.
func conduzirAssistente(t *testing.T, roteiro ...tea.Msg) *conducao {
	t.Helper()
	anterior := executarAssistente
	t.Cleanup(func() { executarAssistente = anterior })
	c := &conducao{}
	executarAssistente = func(a *ui.Assistente) error {
		c.assistente = a
		entregarInit(a, append(executarCmdInit(a.Init()), larguraFix)...)
		for _, passo := range roteiro {
			switch p := passo.(type) {
			case capturar:
				c.views.WriteString("== " + p.nome + "\n" + a.View())
				continue
			case ateOFim:
				for i := 0; i < 40 && !a.Concluido() && !a.Cancelado(); i++ {
					entregarInit(a, enter)
				}
			default:
				entregarInit(a, passo)
			}
			c.etapas = append(c.etapas, a.Etapa())
		}
		return nil
	}
	return c
}

// ========================================================
// buildContext com o assistente
// ========================================================

func TestInitAssistente_PadroesComEnter(t *testing.T) {
	noDiretorio(t, t.TempDir(), func() {
		c := conduzirAssistente(t, ateOFim{})
		o := opcoesInit()
		ctx, err := buildContext(o)
		require.NoError(t, err)

		assert.Equal(t, []string{"projeto", "ambientes", "cluster", "módulos", "ia", "revisão"}, c.assistente.Marcos())
		assert.Equal(t, []string{"destino", "repositorio", "nome", "sobrescrever", "topologia", "workflow",
			"dominio", "email", "modulos", "devcontainer", "ia", "provedor", "descricao", "gerar"}, c.assistente.Chaves())
		assert.Equal(t, ".", o.TargetDir)
		assert.Equal(t, "standard", ctx.Topology)
		assert.Equal(t, "gitflow", ctx.WorkflowPattern)
		assert.Equal(t, "yby-project", ctx.ProjectName)
		assert.Equal(t, "", ctx.GitRepoURL)
		assert.Equal(t, "yby.local", ctx.Domain)
		assert.Equal(t, "admin@yby.local", ctx.Email)
		assert.True(t, ctx.EnableDevContainer, "DevContainer vem marcado")
		assert.False(t, ctx.EnableKepler || ctx.EnableMinio || ctx.EnableKEDA || ctx.EnableMetricsServer)
		assert.Equal(t, "", o.AIProvider, "auto vira vazio")
		assert.Equal(t, []string{"local", "prod"}, ctx.Environments)
		assert.Equal(t, "local", ctx.Environment)
	})
}

func TestInitAssistente_RespostasDigitadas(t *testing.T) {
	noDiretorio(t, t.TempDir(), func() {
		conduzirAssistente(t,
			apagar, digitar("infra"), enter, // destino
			digitar("https://github.com/acme/Pay-Gateway.git"), enter, // repositório
			enter,        // nome: o derivado do repositório
			baixo, enter, // topologia complete
			baixo, enter, // workflow trunkbased
			apagar, digitar("acme.io"), enter, // domínio
			apagar, digitar("ops@acme.io"), enter, // email
			espaco, baixo, baixo, espaco, enter, // módulos: Kepler e KEDA
			nao,                 // devcontainer
			sim,                 // ia
			baixo, baixo, enter, // provedor gemini
			digitar("gateway de pagamento"), enter,
			enter, // revisão: gerar
		)
		o := opcoesInit()
		ctx, err := buildContext(o)
		require.NoError(t, err)

		assert.Equal(t, "infra", o.TargetDir)
		assert.Equal(t, "https://github.com/acme/Pay-Gateway.git", ctx.GitRepoURL)
		assert.Equal(t, "https://github.com/acme/Pay-Gateway.git", ctx.GitRepo, "o repositório digitado vai para o repoURL dos templates")
		assert.Equal(t, "acme", ctx.GithubOrg)
		assert.Equal(t, "pay-gateway", ctx.ProjectName, "nome derivado do repositório e sanitizado")
		assert.Equal(t, "complete", ctx.Topology)
		assert.Equal(t, "trunkbased", ctx.WorkflowPattern)
		assert.Equal(t, "acme.io", ctx.Domain)
		assert.Equal(t, "ops@acme.io", ctx.Email)
		assert.True(t, ctx.EnableKepler)
		assert.True(t, ctx.EnableKEDA)
		assert.False(t, ctx.EnableMinio)
		assert.False(t, ctx.EnableMetricsServer)
		assert.False(t, ctx.EnableDevContainer)
		assert.Equal(t, "gemini", o.AIProvider)
		assert.Equal(t, "gateway de pagamento", o.Description)
		assert.Equal(t, []string{"local", "dev", "staging", "prod"}, ctx.Environments)
		assert.Equal(t, "dev", ctx.Environment)
	})
}

func TestInitAssistente_IADesligadaPulaDetalhes(t *testing.T) {
	noDiretorio(t, t.TempDir(), func() {
		c := conduzirAssistente(t,
			enter, enter, enter, // projeto
			enter, enter, // ambientes
			enter, enter, // cluster
			enter, enter, // módulos
			nao, // ia
		)
		o := opcoesInit()
		_, err := buildContext(o)
		require.Error(t, err, "o roteiro para na revisão sem confirmar")
		ultima := c.etapas[len(c.etapas)-1]
		assert.Equal(t, 5, ultima, "com a IA desligada, vai direto à revisão")
		assert.Equal(t, "", o.AIProvider)
		assert.Equal(t, "", o.Description)
	})
}

func TestInitAssistente_SoPerguntaOQueFalta(t *testing.T) {
	casos := []struct {
		nome      string
		ajustar   func(o *InitOptions)
		ausentes  []string
		conferir  func(t *testing.T, o *InitOptions, ctx *scaffold.BlueprintContext)
		semMarcos []string
	}{
		{"offline", func(o *InitOptions) { o.Offline = true }, []string{"repositorio", "ia", "provedor", "descricao"},
			func(t *testing.T, o *InitOptions, ctx *scaffold.BlueprintContext) {
				assert.Equal(t, repoServidorGitLocal, ctx.GitRepoURL)
				assert.Equal(t, "", ctx.GitRepo, "no offline o bootstrap aponta o repoURL para o git-server")
				assert.Equal(t, "repo", ctx.ProjectName, "o nome sai do repositório do servidor git local")
			}, []string{"ia"}},
		{"git repo por flag", func(o *InitOptions) { o.GitRepo = "https://github.com/org/meu-repo.git" }, []string{"repositorio"},
			func(t *testing.T, o *InitOptions, ctx *scaffold.BlueprintContext) {
				assert.Equal(t, "meu-repo", ctx.ProjectName)
			}, nil},
		{"domínio e email por flag", func(o *InitOptions) { o.Domain, o.Email = "custom.example.com", "admin@custom.example.com" },
			[]string{"dominio", "email"},
			func(t *testing.T, o *InitOptions, ctx *scaffold.BlueprintContext) {
				assert.Equal(t, "custom.example.com", ctx.Domain)
				assert.Equal(t, "admin@custom.example.com", ctx.Email)
			}, []string{"cluster"}},
		{"devcontainer por flag", func(o *InitOptions) { o.IncludeDevContainer = true }, []string{"devcontainer"},
			func(t *testing.T, o *InitOptions, ctx *scaffold.BlueprintContext) {
				assert.True(t, ctx.EnableDevContainer)
			}, nil},
		{"provedor e descrição por flag", func(o *InitOptions) { o.AIProvider, o.Description = "gemini", "já descrito" },
			[]string{"provedor", "descricao"},
			func(t *testing.T, o *InitOptions, ctx *scaffold.BlueprintContext) {
				assert.Equal(t, "gemini", o.AIProvider)
				assert.Equal(t, "já descrito", o.Description)
			}, nil},
		{"destino e topologia por flag", func(o *InitOptions) { o.TargetDir, o.Topology = "infra", "single" },
			[]string{"destino", "topologia"},
			func(t *testing.T, o *InitOptions, ctx *scaffold.BlueprintContext) {
				assert.Equal(t, "infra", o.TargetDir)
				assert.Equal(t, "single", ctx.Topology)
				assert.Equal(t, "gitflow", ctx.WorkflowPattern)
			}, nil},
		{"módulos por flag vêm marcados", func(o *InitOptions) { o.EnableMinio, o.EnableMetricsServer = true, true }, nil,
			func(t *testing.T, o *InitOptions, ctx *scaffold.BlueprintContext) {
				assert.True(t, ctx.EnableMinio)
				assert.True(t, ctx.EnableMetricsServer)
				assert.False(t, ctx.EnableKepler)
			}, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			noDiretorio(t, t.TempDir(), func() {
				cond := conduzirAssistente(t, ateOFim{})
				o := opcoesInit()
				c.ajustar(o)
				ctx, err := buildContext(o)
				require.NoError(t, err)
				for _, chave := range c.ausentes {
					assert.NotContains(t, cond.assistente.Chaves(), chave, "não deveria perguntar %s", chave)
				}
				for _, marco := range c.semMarcos {
					assert.NotContains(t, cond.assistente.Marcos(), marco)
				}
				c.conferir(t, o, ctx)
			})
		})
	}
}

func TestInitAssistente_VoltarUmaEtapa(t *testing.T) {
	noDiretorio(t, t.TempDir(), func() {
		c := conduzirAssistente(t,
			enter, enter, // destino e repositório
			digitar("primeiro"), enter, // nome → ambientes
			voltar, // de volta ao nome
			apagar, digitar("segundo"), enter,
			ateOFim{},
		)
		o := opcoesInit()
		ctx, err := buildContext(o)
		require.NoError(t, err)
		assert.Equal(t, []int{0, 0, 0, 1, 0, 0, 0, 1}, c.etapas[:8], "⇧⇥ no primeiro campo de ambientes volta a projeto")
		assert.Equal(t, "segundo", ctx.ProjectName)
	})
}

func TestInitAssistente_ValidaOsCampos(t *testing.T) {
	noDiretorio(t, t.TempDir(), func() {
		c := conduzirAssistente(t,
			enter, digitar("ftp://errado"), enter, // repositório inválido: não avança
			capturar{"repositorio invalido"},
			apagar, enter, // vazio é aceito
			digitar("Minha Loja!"), enter, // nome vira slug: minha-loja
			enter, enter, // ambientes
			apagar, digitar("com espaço"), enter, // domínio inválido: não avança
			capturar{"dominio invalido"},
			apagar, digitar("acme.io"), enter,
			apagar, digitar("sem-arroba"), enter, // email inválido: não avança
			capturar{"email invalido"},
			apagar, enter, // vazio é aceito, como no ValidateContext
			ateOFim{},
		)
		ctx, err := buildContext(opcoesInit())
		require.NoError(t, err)
		views := c.views.String()
		assert.Contains(t, views, "URL do repositório Git inválida")
		assert.Contains(t, views, "domínio não pode conter espaços")
		assert.Contains(t, views, "email inválido")
		assert.NotContains(t, views, "[ERR_VALIDATION]", "o campo mostra só a mensagem")
		assert.Equal(t, "minha-loja", ctx.ProjectName)
		assert.Equal(t, "acme.io", ctx.Domain)
		assert.Equal(t, "", ctx.Email)
	})
}

func TestInitAssistente_CancelarNoMeioNaoGravaNada(t *testing.T) {
	casos := map[string][]tea.Msg{
		"esc em ambientes":     {enter, enter, enter, esc},
		"ctrl+c em módulos":    {enter, enter, enter, enter, enter, enter, enter, tea.KeyMsg{Type: tea.KeyCtrlC}},
		"cancelar na revisão":  {enter, enter, enter, enter, enter, enter, enter, enter, enter, enter, enter, enter, nao},
		"roteiro para no meio": {enter, enter},
	}
	for nome, roteiro := range casos {
		t.Run(nome, func(t *testing.T) {
			isolarInit(t)
			dir := t.TempDir()
			conduzirAssistente(t, roteiro...)
			usarSaidaTexto(t, true)
			var saida bytes.Buffer
			var err error
			noDiretorio(t, dir, func() { err = executarInit(&saida, opcoesInit()) })

			require.NoError(t, err, "cancelar é saída voluntária, não erro")
			assert.Empty(t, arquivosEm(t, dir), "cancelar não pode gravar nada")
			assert.Equal(t, "  o init cancelado            nenhum arquivo foi gravado\n", saida.String(),
				"só a linha neutra: nada do relatório é escrito antes do fim do assistente")
		})
	}
}

func TestInitAssistente_SemTerminalNaoTrava(t *testing.T) {
	isolarInit(t)
	anterior := stdinTerminal
	stdinTerminal = func() bool { return false }
	t.Cleanup(func() { stdinTerminal = anterior })

	dir := t.TempDir()
	var err error
	noDiretorio(t, dir, func() { err = executarInit(&bytes.Buffer{}, opcoesInit()) })

	var ye *errors.YbyError
	require.True(t, stderrors.As(err, &ye), "erro deveria ser YbyError: %v", err)
	assert.Equal(t, errors.ErrCodeValidation, ye.Code)
	assert.Contains(t, ye.Message, "terminal interativo")
	assert.Contains(t, ye.Hint, "--non-interactive")
	assert.Contains(t, ye.Hint, "--topology")
	assert.Empty(t, arquivosEm(t, dir))
}

func TestInitAssistente_SobrescreverProjetoExistente(t *testing.T) {
	existente := func(t *testing.T) string {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".yby"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".yby", "blueprint.yaml"), []byte("x"), 0o644))
		return dir
	}

	t.Run("sim pula a confirmação de depois", func(t *testing.T) {
		dir := existente(t)
		c := conduzirAssistente(t, enter, enter, enter, sim, ateOFim{})
		o := opcoesInit()
		noDiretorio(t, dir, func() {
			_, err := buildContext(o)
			require.NoError(t, err)
		})
		assert.Contains(t, c.assistente.Chaves(), "sobrescrever")
		assert.True(t, o.sobrescritaConfirmada)
	})

	t.Run("não para na hora sem gravar", func(t *testing.T) {
		isolarInit(t)
		dir := existente(t)
		c := conduzirAssistente(t, enter, enter, enter, nao)
		anterior := askConfirm
		askConfirm = func(string, bool) (bool, error) {
			t.Fatal("o assistente já perguntou; não deveria perguntar de novo")
			return false, nil
		}
		t.Cleanup(func() { askConfirm = anterior })
		var saida bytes.Buffer
		var err error
		noDiretorio(t, dir, func() { err = executarInit(&saida, opcoesInit()) })
		require.NoError(t, err)
		assert.True(t, c.assistente.Concluido(), "com Não, o assistente termina sem passar pelas outras etapas")
		assert.Equal(t, []int{0, 0, 0, 0}, c.etapas, "não chega a ambientes")
		assert.Contains(t, saida.String(), "o projeto existente não foi sobrescrito")
		assert.Equal(t, []string{".yby", ".yby/blueprint.yaml"}, arquivosEm(t, dir))
	})

	t.Run("voltar e trocar o destino segue o assistente", func(t *testing.T) {
		dir := existente(t)
		c := conduzirAssistente(t, enter, enter, enter, voltar, voltar, voltar,
			apagar, digitar("novo"), enter, enter, enter, ateOFim{})
		o := opcoesInit()
		noDiretorio(t, dir, func() {
			_, err := buildContext(o)
			require.NoError(t, err)
		})
		assert.Equal(t, "novo", o.TargetDir)
		assert.False(t, o.sobrescritaConfirmada)
		assert.Contains(t, c.etapas, 5, "o destino novo não tem projeto: o assistente chega à revisão")
	})
}

// ========================================================
// Confirmações fora do assistente (topologia e workflow por flag, --update)
// ========================================================

func TestInit_ConfirmacoesForaDoAssistente(t *testing.T) {
	existente := func(t *testing.T) string {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".yby"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".yby", "blueprint.yaml"), []byte("x"), 0o644))
		return dir
	}
	responder := func(t *testing.T, ok bool, err error) {
		anterior := askConfirm
		askConfirm = func(string, bool) (bool, error) { return ok, err }
		t.Cleanup(func() { askConfirm = anterior })
	}
	porFlag := func() *InitOptions {
		o := opcoesInit()
		o.Topology, o.Workflow, o.ProjectName = "single", "essential", "loja"
		return o
	}

	t.Run("sem terminal o projeto existente pede --force", func(t *testing.T) {
		isolarInit(t)
		dir := existente(t)
		responder(t, false, erroSemTerminal())
		var err error
		noDiretorio(t, dir, func() { err = executarInit(&bytes.Buffer{}, porFlag()) })

		var ye *errors.YbyError
		require.True(t, stderrors.As(err, &ye), "erro deveria ser YbyError: %v", err)
		assert.Equal(t, "falha ao ler a confirmação", ye.Message)
		assert.Contains(t, ye.Hint, "--force")
		assert.NotContains(t, err.Error(), "cancelada pelo usuário")
		assert.Equal(t, []string{".yby", ".yby/blueprint.yaml"}, arquivosEm(t, dir))
	})

	for nome, errResposta := range map[string]error{"recusar": nil, "ctrl+c": huh.ErrUserAborted} {
		t.Run(nome+" não sobrescreve e não é erro", func(t *testing.T) {
			usarSaidaTexto(t, true)
			isolarInit(t)
			dir := existente(t)
			responder(t, false, errResposta)
			var saida bytes.Buffer
			var err error
			noDiretorio(t, dir, func() { err = executarInit(&saida, porFlag()) })
			require.NoError(t, err)
			assert.Equal(t, "  o init cancelado            o projeto existente não foi sobrescrito\n", saida.String())
			assert.Equal(t, []string{".yby", ".yby/blueprint.yaml"}, arquivosEm(t, dir))
		})
	}

	// prepararUpdate gera um projeto e muda o scaffold, para o --update ter o que aplicar
	prepararUpdate := func(t *testing.T) (string, *InitOptions) {
		isolarInit(t)
		usarScaffoldFalso(t)
		dir := t.TempDir()
		o := porFlag()
		o.NonInteractive = true
		noDiretorio(t, dir, func() { require.NoError(t, executarInit(&bytes.Buffer{}, o)) })
		aplicarScaffoldInit = func(d string, ctx *scaffold.BlueprintContext, fsys fs.FS) error {
			if err := scaffoldFalso(d, ctx, fsys); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(d, "novo.yaml"), []byte("a: 1\n"), 0o644)
		}
		u := *o
		u.NonInteractive, u.Update = false, true
		return dir, &u
	}

	t.Run("sem terminal o --update pede --non-interactive", func(t *testing.T) {
		dir, u := prepararUpdate(t)
		responder(t, false, erroSemTerminal())
		var err error
		noDiretorio(t, dir, func() { err = executarInit(&bytes.Buffer{}, u) })
		var ye *errors.YbyError
		require.True(t, stderrors.As(err, &ye), "erro deveria ser YbyError: %v", err)
		assert.Contains(t, ye.Hint, "--non-interactive")
		assert.NoFileExists(t, filepath.Join(dir, "novo.yaml"))
	})

	t.Run("recusar o plano do --update não é erro", func(t *testing.T) {
		usarSaidaTexto(t, true)
		dir, u := prepararUpdate(t)
		responder(t, false, nil)
		var saida bytes.Buffer
		var err error
		noDiretorio(t, dir, func() { err = executarInit(&saida, u) })
		require.NoError(t, err)
		assert.Regexp(t, `o merge +cancelado +nenhum arquivo foi alterado\n$`, saida.String())
		assert.NotContains(t, saida.String(), "RESUMO")
		assert.NoFileExists(t, filepath.Join(dir, "novo.yaml"))
	})
}

// ========================================================
// Revisão: mostra o que vai ser gerado
// ========================================================

func TestInitAssistente_RevisaoMostraOQueVaiSerGerado(t *testing.T) {
	casos := []struct {
		nome      string
		ajustar   func(o *InitOptions, r *respostasInit)
		contem    []string
		naoContem []string
	}{
		{"ambientes do --environments", func(o *InitOptions, r *respostasInit) { o.Environments = "local,dev,hom,prod" },
			[]string{"standard · local · dev · hom · prod"}, nil},
		{"--offline garante o local", func(o *InitOptions, r *respostasInit) { o.Offline, o.Environments = true, "dev,prod" },
			[]string{"standard · local · dev · prod", "ia            desligada"}, nil},
		{"topologia padrão", func(o *InitOptions, r *respostasInit) {},
			[]string{"standard · local · prod"}, nil},
		{"ia ligada sem descrição não roda", func(o *InitOptions, r *respostasInit) {},
			[]string{"ligada · sem descrição, a IA não roda"}, nil},
		{"ia ligada com descrição", func(o *InitOptions, r *respostasInit) { r.Descricao = "loja virtual" },
			[]string{"ia            ligada\n"}, []string{"não roda"}},
		{"ia desligada", func(o *InitOptions, r *respostasInit) { r.IA, r.Descricao = false, "ignorada" },
			[]string{"ia            desligada"}, nil},
		{"descrição por flag", func(o *InitOptions, r *respostasInit) { o.Description = "já descrito" },
			[]string{"ia            ligada\n"}, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			o := opcoesInit()
			ctx := &scaffold.BlueprintContext{ProjectName: "yby-project", Domain: o.Domain, Email: o.Email}
			noDiretorio(t, t.TempDir(), func() {
				a := novoAssistenteInit(o, ctx)
				c.ajustar(o, a.respostas)
				a.perguntas = perguntasPara(o, ctx)
				resumo := a.resumo() + "\n"
				for _, s := range c.contem {
					assert.Contains(t, resumo, s)
				}
				for _, s := range c.naoContem {
					assert.NotContains(t, resumo, s)
				}
			})
		})
	}
}

// ========================================================
// Golden do assistente: a view de cada etapa
// ========================================================

// usarTemaEscuroComCor deixa as saídas no tema escuro em truecolor, com glifos Unicode.
func usarTemaEscuroComCor(t *testing.T) {
	t.Helper()
	usarSaidaTexto(t, false)
	escuro := lipgloss.HasDarkBackground()
	t.Cleanup(func() { lipgloss.SetHasDarkBackground(escuro) })
	ui.Configurar(ui.Opcoes{Tema: ui.TemaEscuro})
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
}

func TestInitAssistente_Golden(t *testing.T) {
	roteiro := []tea.Msg{
		capturar{"projeto"},
		enter, digitar("https://github.com/acme/loja.git"), enter,
		capturar{"projeto (nome)"},
		enter,
		capturar{"ambientes"},
		enter, enter,
		capturar{"cluster"},
		enter, enter,
		capturar{"módulos"},
		espaco, enter, enter,
		capturar{"ia"},
		enter,
		capturar{"ia (detalhes)"},
		enter, digitar("loja virtual"), enter,
		capturar{"revisão"},
	}
	for _, c := range []struct {
		nome string
		tema func(t *testing.T)
	}{
		{"escuro", usarTemaEscuroComCor},
		{"ascii", func(t *testing.T) { usarSaidaTexto(t, true) }},
	} {
		t.Run(c.nome, func(t *testing.T) {
			c.tema(t)
			cond := conduzirAssistente(t, roteiro...)
			noDiretorio(t, t.TempDir(), func() { _, _ = buildContext(opcoesInit()) })
			conferirGolden(t, "init_assistente_"+c.nome, cond.views.String())
			if c.nome == "ascii" {
				conferirSemRestos(t, cond.views.String())
			}
		})
	}
}

func TestInitAssistente_GoldenSobrescrever(t *testing.T) {
	usarSaidaTexto(t, true)
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".yby"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".yby", "blueprint.yaml"), []byte("x"), 0o644))
	cond := conduzirAssistente(t, enter, enter, capturar{"sobrescrever"}, ateOFim{})
	o := opcoesInit()
	o.Topology = "single"
	o.Domain, o.Email, o.Offline = "acme.io", "ops@acme.io", true
	noDiretorio(t, dir, func() { _, _ = buildContext(o) })
	conferirGolden(t, "init_assistente_sobrescrever_ascii", cond.views.String())
}

// ========================================================
// Relatório do init (saída para pessoas)
// ========================================================

// scaffoldFalso grava só o necessário para o init seguir (charts para a validação).
func scaffoldFalso(targetDir string, _ *scaffold.BlueprintContext, _ fs.FS) error {
	for _, c := range []string{"charts/system", "charts/bootstrap"} {
		if err := os.MkdirAll(filepath.Join(targetDir, c), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(targetDir, c, "Chart.yaml"), []byte("name: x\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func usarScaffoldFalso(t *testing.T) {
	t.Helper()
	anterior := aplicarScaffoldInit
	aplicarScaffoldInit = scaffoldFalso
	t.Cleanup(func() { aplicarScaffoldInit = anterior })
}

func TestInitSaida_Golden(t *testing.T) {
	casos := []struct {
		nome    string
		ajustar func(t *testing.T, o *InitOptions, p *pluginsFalsos)
	}{
		{"init_saida_ok", func(t *testing.T, o *InitOptions, p *pluginsFalsos) {
			p.plugins = []plugin.PluginManifest{{Name: "atlas"}, {Name: "bard"}}
		}},
		{"init_saida_avisos", func(t *testing.T, o *InitOptions, p *pluginsFalsos) {
			o.Environment = "hom"
			p.errHook = stderrors.New("plugin atlas não respondeu")
			o.Description = "gateway de pagamento"
			obterProvedorIAInit = func(context.Context, string) ai.Provider {
				return &mockAIProvider{name: "ollama", blueprint: &ai.GovernanceBlueprint{
					Domain: "Fintech", RiskLevel: "alto",
					Files: []ai.GeneratedFile{{Path: ".synapstor/governanca.md", Content: "# governança\n"}},
				}}
			}
			novoValidadorInit = func() validate.Service {
				return &validadorFalso{relatorio: &validate.ValidateReport{Charts: []validate.ChartResult{
					{Chart: "infra/charts/system", Error: "lint falhou:\n  values.yaml inválido"},
				}}}
			}
		}},
		{"init_saida_ia_sem_provedor", func(t *testing.T, o *InitOptions, p *pluginsFalsos) {
			p.errDescoberta = stderrors.New("diretório de plugins ilegível")
			o.Description = "gateway"
			obterProvedorIAInit = func(context.Context, string) ai.Provider { return nil }
			novoValidadorInit = func() validate.Service {
				return &validadorFalso{err: stderrors.New("helm não encontrado")}
			}
		}},
		{"init_saida_ia_falhou", func(t *testing.T, o *InitOptions, p *pluginsFalsos) {
			o.Description = "gateway"
			obterProvedorIAInit = func(context.Context, string) ai.Provider {
				return &mockAIProvider{name: "gemini", err: stderrors.New("cota esgotada")}
			}
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarSaidaTexto(t, false)
			plugins := isolarInit(t)
			usarScaffoldFalso(t)
			o := opcoesInit()
			o.NonInteractive, o.TargetDir, o.ProjectName = true, "infra", "loja"
			o.Topology, o.Workflow = "standard", "gitflow"
			c.ajustar(t, o, plugins)

			var saida bytes.Buffer
			noDiretorio(t, t.TempDir(), func() { require.NoError(t, executarInit(&saida, o)) })
			conferirGolden(t, c.nome, saida.String())
			conferirSemRestos(t, saida.String())
		})
	}
}

func TestInitSaida_UpdateEProjetoExistente(t *testing.T) {
	usarSaidaTexto(t, true)
	isolarInit(t)
	usarScaffoldFalso(t)
	dir := t.TempDir()
	o := opcoesInit()
	o.NonInteractive, o.ProjectName, o.Topology, o.Workflow = true, "loja", "single", "essential"

	var primeira, sobrescrita, primeiroUpdate, semMudanca, update bytes.Buffer
	noDiretorio(t, dir, func() {
		require.NoError(t, executarInit(&primeira, o))
		require.NoError(t, os.WriteFile(filepath.Join(".yby", "blueprint.yaml"), []byte("x"), 0o644))

		// sem --force, o modo não interativo recusa
		err := executarInit(&bytes.Buffer{}, o)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "já tem um projeto Yby")

		f := *o
		f.Force = true
		require.NoError(t, executarInit(&sobrescrita, &f))

		// o primeiro update grava os hashes no manifest; o segundo não tem o que mudar
		u := *o
		u.Update = true
		require.NoError(t, executarInit(&primeiroUpdate, &u))
		require.NoError(t, executarInit(&semMudanca, &u))

		// um arquivo novo no scaffold vira "novos" no plano
		aplicarScaffoldInit = func(d string, ctx *scaffold.BlueprintContext, fsys fs.FS) error {
			if err := scaffoldFalso(d, ctx, fsys); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(d, "novo.yaml"), []byte("a: 1\n"), 0o644)
		}
		require.NoError(t, executarInit(&update, &u))
	})
	assert.Contains(t, sobrescrita.String(), "já existia")
	assert.Contains(t, semMudanca.String(), "o projeto já está atualizado")
	assert.Contains(t, update.String(), "novos")
	assert.Contains(t, update.String(), "scaffold atualizado")
	conferirGolden(t, "init_saida_update_ascii", update.String())
	for _, s := range []string{primeira.String(), sobrescrita.String(), primeiroUpdate.String(), semMudanca.String(), update.String()} {
		conferirSemRestos(t, s)
	}
}

func TestInit_UpdateEForceExclusivos(t *testing.T) {
	o := opcoesInit()
	o.Update, o.Force = true, true
	err := executarInit(&bytes.Buffer{}, o)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutuamente exclusivos")
}
