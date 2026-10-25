package cmd

import (
	gocontext "context"
	stderrors "errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/cloud"
	ybycontext "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// agoraPainel é o relógio fixo dos testes do painel (o "atualizado 12:07:12" da prancha).
var agoraPainel = time.Date(2026, 10, 25, 12, 7, 12, 0, time.UTC)

// clustersDeExemplo são os da prancha CloudPainel: prod-core com 11 min, staging vencido
// há 12 min, aks-dados com 3 min e gke-ml com 52 min.
func clustersDeExemplo() []clusterPainel {
	em := func(d time.Duration) time.Time { return agoraPainel.Add(d) }
	return []clusterPainel{
		{
			info:        cloud.ClusterInfo{Provider: "gcp", Name: "gke-ml", Region: "us-central1", Version: "1.31", Status: "RUNNING", ProjectID: "ml-plataforma"},
			autenticado: true, conta: "ml-plataforma", papel: "yby@ml-plataforma.iam.gserviceaccount.com", metodo: "CLI do gcloud",
			expiraEm: em(52 * time.Minute),
		},
		{
			info:        cloud.ClusterInfo{Provider: "aws", Name: "staging", Region: "us-east-1", Version: "1.31", Status: "ACTIVE"},
			autenticado: true, conta: "482100009310", papel: "yby-staging-admin", metodo: "SSO · Identity Center", sso: true,
			ambiente: "staging", expiraEm: em(-12 * time.Minute),
		},
		{
			info:        cloud.ClusterInfo{Provider: "aws", Name: "prod-core", Region: "us-east-1", Version: "1.31", Status: "ACTIVE"},
			autenticado: true, conta: "482100009310", papel: "yby-prod-admin", metodo: "SSO · Identity Center", sso: true,
			expiraEm: em(11 * time.Minute),
		},
		{
			info:        cloud.ClusterInfo{Provider: "azure", Name: "aks-dados", Region: "brazilsouth", Version: "1.30", Status: "Running"},
			autenticado: true, conta: "9f3c2a71-5b8e-4d0a-a1c2-7e4b0d3f6a10", papel: "ana@empresa.com.br", metodo: "CLI do Azure",
			expiraEm: em(3 * time.Minute),
		},
	}
}

// auditoriaDeExemplo: quatro registros de staging (o mais antigo não aparece) e um de
// prod-core (que não é de staging).
func auditoriaDeExemplo() []cloud.CloudAuditEvent {
	hora := func(h, m int) time.Time { return time.Date(2026, 10, 25, h, m, 0, 0, time.UTC) }
	return []cloud.CloudAuditEvent{
		{Timestamp: hora(9, 12), Action: "authenticate", Provider: "aws", Method: "SSO · Identity Center", Success: true},
		{Timestamp: hora(8, 1), Action: "authenticate", Provider: "aws", Method: "perfil antigo", Success: true},
		{Timestamp: hora(11, 55), Action: "refresh", Provider: "aws", Cluster: "staging", Success: false,
			Error: "aws eks get-token: exit status 255: Error when retrieving token from sso: Token has expired and refresh failed"},
		{Timestamp: hora(10, 55), Action: "refresh", Provider: "aws", Cluster: "staging", Method: "SSO", Success: true},
		{Timestamp: hora(12, 0), Action: "refresh", Provider: "aws", Cluster: "prod-core", Success: true},
	}
}

// fontesFalsas lê os clusters de exemplo e registra as renovações.
type fontesFalsas struct {
	leitura    leituraPainel
	renovados  []string
	erroRenova error
}

func (f *fontesFalsas) fontes() fontesPainel {
	return fontesPainel{
		ler: func(gocontext.Context) leituraPainel { return f.leitura },
		renovar: func(_ gocontext.Context, c clusterPainel) error {
			f.renovados = append(f.renovados, c.info.Name)
			return f.erroRenova
		},
	}
}

func novasFontesDeExemplo() *fontesFalsas {
	return &fontesFalsas{leitura: leituraPainel{provedores: 3, clusters: clustersDeExemplo(), auditoria: auditoriaDeExemplo()}}
}

func enviar(t *testing.T, m painelCloud, msg tea.Msg) (painelCloud, tea.Cmd) {
	t.Helper()
	nm, cmd := m.Update(msg)
	p, ok := nm.(painelCloud)
	require.True(t, ok)
	return p, cmd
}

func teclar(t *testing.T, m painelCloud, teclas ...tea.KeyMsg) painelCloud {
	t.Helper()
	for _, k := range teclas {
		m, _ = enviar(t, m, k)
	}
	return m
}

func letra(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// baixo e enter vêm de init_assistente_test.go.
var escape = tea.KeyMsg{Type: tea.KeyEsc}

// painelCarregado é o painel com a leitura de exemplo já recebida, na janela dada.
func painelCarregado(t *testing.T, f *fontesFalsas, largura, altura int) painelCloud {
	t.Helper()
	m := novoPainelCloud(f.fontes(), func() time.Time { return agoraPainel.Add(-12 * time.Second) })
	m, _ = enviar(t, m, tea.WindowSizeMsg{Width: largura, Height: altura})
	m, _ = enviar(t, m, leituraPainelMsg{leitura: f.leitura})
	m.agora = func() time.Time { return agoraPainel } // 12 s depois da leitura: próxima em 18 s
	return m
}

func ehQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func conferirTela(t *testing.T, tela string, largura, altura int) {
	t.Helper()
	linhas := strings.Split(tela, "\n")
	require.Len(t, linhas, altura)
	for i, l := range linhas {
		assert.Equal(t, largura, lipgloss.Width(l), "linha %d: %q", i, l)
	}
}

func TestPainelCloud_Golden(t *testing.T) {
	casos := []struct {
		nome            string
		largura, altura int
		teclas          []tea.KeyMsg
	}{
		// staging selecionado (a segunda linha), como na prancha: detalhe abaixo da tabela.
		{"cloud_painel_140", 140, 30, []tea.KeyMsg{baixo}},
		// pouca altura: o detalhe vai para o lado da tabela.
		{"cloud_painel_140_baixo", 140, 18, []tea.KeyMsg{baixo}},
		// abaixo de 120 colunas: só a tabela; o ↵ abre o detalhe por cima.
		{"cloud_painel_100", 100, 30, []tea.KeyMsg{baixo}},
		{"cloud_painel_100_detalhe", 100, 30, []tea.KeyMsg{baixo, enter}},
		{"cloud_painel_70_detalhe", 70, 30, []tea.KeyMsg{baixo, enter}},
		{"cloud_painel_100_ajuda", 100, 30, []tea.KeyMsg{letra("?")}},
		// as larguras da folha Anatomia: 60, 80, 120 e 160.
		{"cloud_painel_60", 60, 20, []tea.KeyMsg{baixo}},
		{"cloud_painel_60_ajuda", 60, 16, []tea.KeyMsg{letra("?")}},
		{"cloud_painel_80", 80, 24, []tea.KeyMsg{baixo}},
		{"cloud_painel_80_detalhe", 80, 16, []tea.KeyMsg{baixo, enter}},
		{"cloud_painel_80_ajuda", 80, 24, []tea.KeyMsg{letra("?")}},
		{"cloud_painel_120", 120, 30, []tea.KeyMsg{baixo}},
		{"cloud_painel_120_baixo", 120, 16, []tea.KeyMsg{baixo}},
		{"cloud_painel_160", 160, 34, []tea.KeyMsg{baixo}},
		{"cloud_painel_72_detalhe_baixo", 72, 20, []tea.KeyMsg{baixo, enter}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarSaidaTexto(t, false)
			m := teclar(t, painelCarregado(t, novasFontesDeExemplo(), c.largura, c.altura), c.teclas...)
			tela := m.View()
			conferirTela(t, tela, c.largura, c.altura)
			conferirGolden(t, c.nome, tela)
		})
	}
}

func TestPainelCloud_ResumoEContexto(t *testing.T) {
	usarSaidaTexto(t, false)
	m := painelCarregado(t, novasFontesDeExemplo(), 140, 30)
	assert.Equal(t, "✓ 2 prontos   ▲ 1 token expira em breve   ✕ 1 token expirado", m.linhaResumo())
	assert.Equal(t, "4 clusters · 3 contas   ◈ próxima leitura em 18 s", m.contexto())
	// A mensagem é a do problema mais grave: staging sem token.
	assert.Equal(t, "✕ staging sem token: t renova (com o SSO vencido, rode aws sso login antes)", m.mensagem())
	// Ordem por provider e nome, como na prancha.
	var nomes []string
	for _, c := range m.clusters {
		nomes = append(nomes, c.info.Name)
	}
	assert.Equal(t, []string{"prod-core", "staging", "aks-dados", "gke-ml"}, nomes)
}

// A cota do token em função do tempo restante (referência de 1 h, aviso abaixo de 5 min).
func TestPainelCloud_CotaDoToken(t *testing.T) {
	casos := []struct {
		nome       string
		expira     time.Time
		estado     estadoCluster
		status     string
		token      string
		aviso      bool
		noDetalhe  string
		fracao     float64
		temCota    bool
		tempoAviso bool
	}{
		{"59 min", agoraPainel.Add(59 * time.Minute), estadoPronto, "✓ pronto", "━━━━━━━━━━━━  59 min", false,
			"✓ válido até 13:06 · faltam 59 min", 59.0 / 60, true, false},
		{"6 min", agoraPainel.Add(6 * time.Minute), estadoPronto, "✓ pronto", "━┄┄┄┄┄┄┄┄┄┄┄  6 min", false,
			"✓ válido até 12:13 · faltam 6 min", 0.1, true, false},
		{"4 min", agoraPainel.Add(4 * time.Minute), estadoExpiraLogo, "▲ atenção", "━┄┄┄┄┄┄┄┄┄┄┄  4 min", true,
			"▲ expira às 12:11 · faltam 4 min", 4.0 / 60, true, true},
		{"0", agoraPainel, estadoExpirado, "✕ token", "expirou agora", false,
			"✕ expirou às 12:07", 0, false, false},
		{"-12 min", agoraPainel.Add(-12 * time.Minute), estadoExpirado, "✕ token", "expirou há 12 min", false,
			"✕ expirou às 11:55", 0, false, false},
		{"desconhecido", time.Time{}, estadoPronto, "✓ pronto", "—", false,
			"desconhecido", 0, false, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarSaidaTexto(t, false)
			cl := clusterPainel{info: cloud.ClusterInfo{Provider: "aws", Name: "x", Status: "ACTIVE"}, autenticado: true, expiraEm: c.expira}
			v := validadeEm(c.expira, agoraPainel)
			assert.Equal(t, c.aviso, v.aviso)
			assert.InDelta(t, c.fracao, v.fracao, 1e-9)
			assert.Equal(t, c.estado, cl.estadoEm(agoraPainel))
			assert.Equal(t, c.status, ansi.Strip(cl.celulaStatus(agoraPainel)))
			assert.Equal(t, c.token, ansi.Strip(cl.celulaToken(agoraPainel, 12)))
			assert.Equal(t, c.noDetalhe, ansi.Strip(cl.tokenNoDetalhe(agoraPainel)))

			// Em cor, o tempo sai em Aviso só abaixo de 5 min.
			lipgloss.SetColorProfile(termenv.TrueColor)
			celula := cl.celulaToken(agoraPainel, 12)
			if c.temCota {
				assert.Equal(t, c.tempoAviso, strings.Contains(celula, ui.S().Aviso.Render(v.texto())), celula)
			}
		})
	}
}

func TestPainelCloud_TextosDoTempo(t *testing.T) {
	assert.Equal(t, "45 s", textoRestante(45*time.Second))
	assert.Equal(t, "11 min", textoRestante(11*time.Minute+50*time.Second))
	assert.Equal(t, "1 h", textoRestante(time.Hour))
	assert.Equal(t, "2 h 05 min", textoRestante(2*time.Hour+5*time.Minute))
	assert.Equal(t, "expirou há 30 s", textoExpirou(30*time.Second))
	assert.Equal(t, "expirou há 3 h", textoExpirou(3*time.Hour))
	assert.Equal(t, "expirou há 3 dias", textoExpirou(72*time.Hour))
}

func TestPainelCloud_SemLoginENaoAtivo(t *testing.T) {
	usarSaidaTexto(t, false)
	semLogin := clusterPainel{info: cloud.ClusterInfo{Provider: "gcp", Name: "gke-ml", Status: "RUNNING"}}
	assert.Equal(t, estadoSemLogin, semLogin.estadoEm(agoraPainel))
	assert.Equal(t, "✕ sem login", ansi.Strip(semLogin.celulaStatus(agoraPainel)))
	assert.Equal(t, "—", ansi.Strip(semLogin.celulaToken(agoraPainel, 12)))

	criando := clusterPainel{info: cloud.ClusterInfo{Provider: "aws", Name: "novo", Status: "CREATING"}, autenticado: true}
	assert.Equal(t, estadoNaoAtivo, criando.estadoEm(agoraPainel))
	assert.Equal(t, "▲ criando", ansi.Strip(criando.celulaStatus(agoraPainel)))
	parado := clusterPainel{info: cloud.ClusterInfo{Provider: "azure", Name: "aks-dados", Status: "Stopped"}, autenticado: true}
	assert.Equal(t, "▲ parado", ansi.Strip(parado.celulaStatus(agoraPainel)))
	estranho := clusterPainel{info: cloud.ClusterInfo{Provider: "gcp", Name: "x", Status: "WEIRD_STATE"}, autenticado: true}
	assert.Equal(t, "▲ weird_state", ansi.Strip(estranho.celulaStatus(agoraPainel)), "status desconhecido sai como veio")

	f := &fontesFalsas{leitura: leituraPainel{provedores: 2, clusters: []clusterPainel{criando, semLogin}}}
	m := painelCarregado(t, f, 120, 30)
	assert.Equal(t, "✕ 1 sem login   ▲ 1 cluster não ativo", m.linhaResumo())
	assert.Equal(t, "✕ gke-ml sem login: faça o login no CLI do gcp e r recarrega", m.mensagem())

	f = &fontesFalsas{leitura: leituraPainel{provedores: 1, clusters: []clusterPainel{parado}}}
	m = painelCarregado(t, f, 120, 30)
	assert.Equal(t, "▲ aks-dados não está ativo (parado)", m.mensagem())
}

func TestPainelCloud_TeclaTRenovaEmSegundoPlano(t *testing.T) {
	usarSaidaTexto(t, false)
	f := novasFontesDeExemplo()
	m := teclar(t, painelCarregado(t, f, 140, 30), baixo)
	m, cmd := enviar(t, m, letra("t"))
	require.NotNil(t, cmd)
	assert.Equal(t, "◈ renovando o token de staging…", m.mensagem())

	// Segundo t enquanto o primeiro roda: não renova de novo.
	m, cmd2 := enviar(t, m, letra("t"))
	assert.Nil(t, cmd2)
	assert.Contains(t, m.mensagem(), "já está sendo renovado")

	msg := cmd()
	require.Equal(t, []string{"staging"}, f.renovados)
	m, cmd = enviar(t, m, msg)
	assert.Equal(t, "✓ token de staging renovado", m.mensagem())
	assert.True(t, m.lendo, "depois de renovar, lê de novo para trazer a validade nova")
	assert.NotNil(t, cmd)
}

// Uma renovação que termina com uma leitura em voo pede outra leitura no fim dela: a em voo
// pode ter lido a validade de antes da renovação.
func TestPainelCloud_RenovacaoDuranteLeituraPedeOutra(t *testing.T) {
	usarSaidaTexto(t, false)
	f := novasFontesDeExemplo()
	m := teclar(t, painelCarregado(t, f, 140, 30), baixo)
	m, cmdLer := enviar(t, m, letra("r"))
	require.True(t, m.lendo)
	m, cmdRenovar := enviar(t, m, letra("t"))
	m, _ = enviar(t, m, cmdRenovar())
	assert.True(t, m.releitura)
	assert.True(t, m.lendo)

	m, cmd := enviar(t, m, cmdLer())
	assert.False(t, m.releitura)
	assert.True(t, m.lendo, "o fim da leitura em voo dispara outra")
	assert.NotNil(t, cmd)
}

func TestPainelCloud_RenovacaoGravaOMetodoNoAuditLog(t *testing.T) {
	log := cloud.NewAuditLoggerWithPath(filepath.Join(t.TempDir(), "audit.log"))
	c := clustersDeExemplo()[1] // staging, SSO
	registrarRenovacao(log, c, agoraPainel, stderrors.New("Token has expired"))
	registrarRenovacao(log, clustersDeExemplo()[3], agoraPainel, nil) // aks-dados, CLI do Azure
	eventos, err := log.ReadEvents(time.Time{})
	require.NoError(t, err)
	require.Len(t, eventos, 2)
	assert.Equal(t, cloud.CloudAuditEvent{Timestamp: agoraPainel, Action: "refresh", Provider: "aws", Cluster: "staging",
		Region: "us-east-1", Method: "SSO", Error: "Token has expired"}, eventos[0])
	assert.Equal(t, "CLI do Azure", eventos[1].Method)
	assert.True(t, eventos[1].Success)
	assert.Equal(t, "token · SSO", descricaoEvento(cloud.CloudAuditEvent{Action: "refresh", Method: "SSO", Success: true}))
	assert.Equal(t, "credencial vencida", descricaoEvento(eventos[0]))
	assert.Equal(t, "SSO vencido", descricaoEvento(cloud.CloudAuditEvent{Action: "refresh", Error: "Error when retrieving token from sso: Token has expired"}))
	assert.Equal(t, "token não renovado", descricaoEvento(cloud.CloudAuditEvent{Action: "refresh", Error: "exit status 1"}))
}

func TestPainelCloud_FalhaAoRenovarViraToastDeErro(t *testing.T) {
	usarSaidaTexto(t, false)
	f := novasFontesDeExemplo()
	f.erroRenova = stderrors.New("aws eks get-token: exit status 255: Error when retrieving token from sso: Token has expired")
	m := teclar(t, painelCarregado(t, f, 140, 30), baixo)
	m, cmd := enviar(t, m, letra("t"))
	m, _ = enviar(t, m, cmd())
	assert.Equal(t, "✕ não deu para renovar o token de staging · Error when retrieving token from sso: Token has expired", m.mensagem())
	assert.False(t, m.lendo)
}

func TestPainelCloud_RRecarregaERMaiusculoAvisa(t *testing.T) {
	usarSaidaTexto(t, false)
	f := novasFontesDeExemplo()
	m := painelCarregado(t, f, 140, 30)

	m, cmd := enviar(t, m, letra("R"))
	assert.Nil(t, cmd)
	assert.Equal(t, "○ recarregar agora é r; t renova o token", m.mensagem())
	assert.Empty(t, f.renovados, "o R não renova mais")

	m, cmd = enviar(t, m, letra("r"))
	require.NotNil(t, cmd)
	assert.True(t, m.lendo)
	_, cmd2 := enviar(t, m, letra("r"))
	assert.Nil(t, cmd2, "uma leitura por vez")

	m, _ = enviar(t, m, cmd())
	assert.False(t, m.lendo)
	assert.Equal(t, "✓ clusters recarregados", m.mensagem())
}

func TestPainelCloud_EscSoFechaODetalhe(t *testing.T) {
	usarSaidaTexto(t, false)
	m := teclar(t, painelCarregado(t, novasFontesDeExemplo(), 100, 30), baixo, enter)
	require.True(t, m.porCima())

	m, cmd := enviar(t, m, letra("q"))
	assert.False(t, ehQuit(cmd), "com o detalhe por cima, q não sai")
	assert.Equal(t, "○ q sai só da lista; esc volta", m.mensagem())

	m, cmd = enviar(t, m, escape)
	assert.False(t, ehQuit(cmd))
	assert.False(t, m.porCima())
	assert.Equal(t, 1, m.cursor, "a seleção continua em staging")

	_, cmd = enviar(t, m, escape)
	assert.False(t, ehQuit(cmd), "esc no nível de cima não sai")

	_, cmd = enviar(t, m, letra("q"))
	assert.True(t, ehQuit(cmd))
}

func TestPainelCloud_CtrlCSaiDeQualquerLugar(t *testing.T) {
	usarSaidaTexto(t, false)
	ctrlC := tea.KeyMsg{Type: tea.KeyCtrlC}
	base := painelCarregado(t, novasFontesDeExemplo(), 100, 30)
	for nome, m := range map[string]painelCloud{
		"lista":   base,
		"detalhe": teclar(t, base, enter),
		"busca":   teclar(t, base, letra("/")),
		"ajuda":   teclar(t, base, letra("?")),
	} {
		_, cmd := enviar(t, m, ctrlC)
		assert.True(t, ehQuit(cmd), nome)
	}
}

func TestPainelCloud_EnterNaoAbreDetalheAoLado(t *testing.T) {
	usarSaidaTexto(t, false)
	m := teclar(t, painelCarregado(t, novasFontesDeExemplo(), 140, 30), enter)
	assert.False(t, m.detalheAberto)
	assert.Equal(t, layoutAbaixo, m.layout())
}

func TestPainelCloud_BuscaPorNome(t *testing.T) {
	usarSaidaTexto(t, false)
	m := painelCarregado(t, novasFontesDeExemplo(), 140, 30)
	m = teclar(t, m, letra("/"), letra("a"), letra("k"), letra("s"))
	require.True(t, m.campo)
	assert.Len(t, m.visiveis(), 1, "filtra enquanto digita")
	assert.Contains(t, m.mensagem(), "buscar › aks")

	// O q no campo é letra.
	m, cmd := enviar(t, m, letra("q"))
	assert.False(t, ehQuit(cmd))
	assert.Empty(t, m.visiveis())
	m = teclar(t, m, tea.KeyMsg{Type: tea.KeyBackspace}, enter)
	assert.False(t, m.campo)
	assert.Equal(t, "aks", m.busca)
	assert.Contains(t, m.linhaResumo(), "busca: aks · 1 de 4")
	c, _ := m.selecionado()
	assert.Equal(t, "aks-dados", c.info.Name)

	// esc no nível de cima limpa a busca e a seleção fica no mesmo cluster.
	m = teclar(t, m, escape)
	assert.Empty(t, m.busca)
	c, _ = m.selecionado()
	assert.Equal(t, "aks-dados", c.info.Name)

	// esc no campo volta ao termo de antes.
	m = teclar(t, m, letra("/"), letra("x"), escape)
	assert.Empty(t, m.busca)
	assert.Len(t, m.visiveis(), 4)
}

func TestPainelCloud_AltGrDoWindows(t *testing.T) {
	usarSaidaTexto(t, false)
	m := painelCarregado(t, novasFontesDeExemplo(), 140, 30)
	// AltGr manda alt+NUL e depois o símbolo com alt.
	m = teclar(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0}, Alt: true})
	assert.False(t, m.campo)
	m = teclar(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/"), Alt: true})
	assert.True(t, m.campo, "alt+/ do AltGr abre a busca")
	m = teclar(t, m, escape, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?"), Alt: true})
	assert.True(t, m.ajuda, "alt+? do AltGr abre a ajuda")
	m = teclar(t, m, escape)
	assert.False(t, m.ajuda)
}

func TestPainelCloud_Mouse(t *testing.T) {
	usarSaidaTexto(t, false)
	m := painelCarregado(t, novasFontesDeExemplo(), 140, 30)
	roda := func(b tea.MouseButton) tea.MouseMsg {
		return tea.MouseMsg{Button: b, Action: tea.MouseActionPress}
	}
	m, _ = enviar(t, m, roda(tea.MouseButtonWheelDown))
	assert.Equal(t, 1, m.cursor)
	m, _ = enviar(t, m, roda(tea.MouseButtonWheelUp))
	assert.Equal(t, 0, m.cursor)

	// masthead, régua, resumo, linha em branco e cabeçalho: a quarta linha da tabela é y=8.
	m, _ = enviar(t, m, tea.MouseMsg{X: 10, Y: 8, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	c, _ := m.selecionado()
	assert.Equal(t, "gke-ml", c.info.Name)
	assert.Contains(t, strings.Split(ansi.Strip(m.View()), "\n")[8], "gke-ml")

	// Clique fora das linhas não muda a seleção.
	m, _ = enviar(t, m, tea.MouseMsg{X: 10, Y: 2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	assert.Equal(t, 3, m.cursor)
}

func TestPainelCloud_TiqueSoLeDepoisDe30s(t *testing.T) {
	usarSaidaTexto(t, false)
	m := painelCarregado(t, novasFontesDeExemplo(), 140, 30)
	m, _ = enviar(t, m, tiquePainelMsg{})
	assert.False(t, m.lendo, "aos 12 s, o tique só redesenha")

	m.agora = func() time.Time { return agoraPainel.Add(18 * time.Second) }
	assert.Contains(t, m.contexto(), "próxima leitura em 0 s")
	m, cmd := enviar(t, m, tiquePainelMsg{})
	assert.True(t, m.lendo)
	assert.NotNil(t, cmd)
	assert.Contains(t, ansi.Strip(m.contexto()), "lendo as clouds…")
}

func TestPainelCloud_ValidadeAndaComORelogio(t *testing.T) {
	usarSaidaTexto(t, false)
	m := painelCarregado(t, novasFontesDeExemplo(), 140, 30)
	assert.Contains(t, m.linhaResumo(), "▲ 1 token expira em breve")
	// Seis minutos depois, sem nova leitura: prod-core cai para 5 min e aks-dados vence.
	m.agora = func() time.Time { return agoraPainel.Add(7 * time.Minute) }
	assert.Equal(t, "▲ 1 token expira em breve   ✕ 2 tokens expirados   ✓ 1 pronto",
		reordenarResumo(m.linhaResumo()))
}

// reordenarResumo põe o grupo de prontos no fim, para a comparação não depender da ordem.
func reordenarResumo(s string) string {
	partes := strings.Split(s, "   ")
	var prontos, resto []string
	for _, p := range partes {
		if strings.HasPrefix(p, "✓") {
			prontos = append(prontos, p)
		} else {
			resto = append(resto, p)
		}
	}
	return strings.Join(append(resto, prontos...), "   ")
}

func TestPainelCloud_EstadosSemTabela(t *testing.T) {
	usarSaidaTexto(t, false)
	m := novoPainelCloud(novasFontesDeExemplo().fontes(), func() time.Time { return agoraPainel })
	m, _ = enviar(t, m, tea.WindowSizeMsg{Width: 100, Height: 20})
	assert.Contains(t, ansi.Strip(m.View()), "lendo os clusters das clouds")

	semCloud, _ := enviar(t, m, leituraPainelMsg{leitura: leituraPainel{}})
	assert.Contains(t, ansi.Strip(semCloud.View()), "nenhuma cloud no kubeconfig")

	comErro, _ := enviar(t, m, leituraPainelMsg{leitura: leituraPainel{provedores: 1, err: stderrors.New("exit status 254: An error occurred (ExpiredToken)")}})
	tela := ansi.Strip(comErro.View())
	assert.Contains(t, tela, "não deu para ler as clouds")
	assert.Contains(t, tela, "An error occurred (ExpiredToken)")
	assert.Contains(t, tela, "r tentar de novo")

	// Uma falha depois de uma leitura boa mantém os clusters e avisa.
	bom := painelCarregado(t, novasFontesDeExemplo(), 100, 20)
	falhou, _ := enviar(t, bom, leituraPainelMsg{leitura: leituraPainel{err: stderrors.New("timeout")}})
	assert.Len(t, falhou.clusters, 4)
	assert.Equal(t, "▲ a última leitura falhou; os dados são de 12:07:00", ansi.Strip(falhou.mensagem()))
}

func TestPainelCloud_JanelaPequena(t *testing.T) {
	usarSaidaTexto(t, false)
	m := painelCarregado(t, novasFontesDeExemplo(), 50, 12)
	tela := m.View()
	conferirTela(t, tela, 50, 12)
	assert.Contains(t, tela, "Aumente a janela")
}

func TestPainelCloud_ComandoSemTerminal(t *testing.T) {
	anterior := stdinTerminal
	t.Cleanup(func() { stdinTerminal = anterior })
	antSaida := stdoutTerminal
	t.Cleanup(func() { stdoutTerminal = antSaida })
	for nome, c := range map[string]struct{ entrada, saida bool }{
		"entrada fora do terminal": {false, true},
		"saída num pipe":           {true, false},
	} {
		stdinTerminal = func() bool { return c.entrada }
		stdoutTerminal = func() bool { return c.saida }
		err := cloudDashboardCmd.RunE(cloudDashboardCmd, nil)
		require.Error(t, err, nome)
		assert.Contains(t, err.Error(), "terminal interativo", nome)
		var ybyErr *errors.YbyError
		require.True(t, stderrors.As(err, &ybyErr), nome)
		assert.Contains(t, ybyErr.Hint, "yby cloud list", nome)
	}
}

func TestContaEPapelEMetodo(t *testing.T) {
	conta, papel := contaEPapel("aws", "arn:aws:sts::482100009310:assumed-role/AWSReservedSSO_Admin_abc/neto", cloud.ClusterInfo{}, nil)
	assert.Equal(t, "482100009310", conta)
	assert.Equal(t, "AWSReservedSSO_Admin_abc", papel)
	metodo, sso := metodoDeAcesso("aws", "iam", papel, nil)
	assert.Equal(t, "SSO · Identity Center", metodo)
	assert.True(t, sso)

	_, papel = contaEPapel("aws", "arn:aws:iam::482100009310:user/neto", cloud.ClusterInfo{},
		&ybycontext.CloudConfig{RoleARN: "arn:aws:iam::482100009310:role/yby-staging-admin"})
	assert.Equal(t, "yby-staging-admin", papel)
	metodo, _ = metodoDeAcesso("aws", "iam", papel, &ybycontext.CloudConfig{Profile: "dev"})
	assert.Equal(t, "IAM · perfil dev", metodo)
	metodo, sso = metodoDeAcesso("aws", "iam", papel, &ybycontext.CloudConfig{Auth: &ybycontext.AuthConfig{Method: "sso"}})
	assert.Equal(t, "SSO · Identity Center", metodo)
	assert.True(t, sso)

	conta, papel = contaEPapel("gcp", "yby@proj.iam.gserviceaccount.com", cloud.ClusterInfo{ProjectID: "ml-plataforma"}, nil)
	assert.Equal(t, "ml-plataforma", conta)
	assert.Equal(t, "yby@proj.iam.gserviceaccount.com", papel)

	conta, papel = contaEPapel("azure", "ana@empresa.com.br", cloud.ClusterInfo{}, &ybycontext.CloudConfig{Subscription: "9f3c2a71-5b8e"})
	assert.Equal(t, "9f3c2a71-5b8e", conta)
	assert.Equal(t, "ana@empresa.com.br", papel)
	metodo, _ = metodoDeAcesso("azure", "az-cli", papel, nil)
	assert.Equal(t, "CLI do Azure", metodo)
}

func TestMascararConta(t *testing.T) {
	usarSaidaTexto(t, false)
	assert.Equal(t, "4821·…·9310", mascararConta("482100009310"))
	assert.Equal(t, "9f3c·…·6a10", mascararConta("9f3c2a71-5b8e-4d0a-a1c2-7e4b0d3f6a10"))
	assert.Equal(t, "ml-plataforma", mascararConta("ml-plataforma"))
	assert.Equal(t, "ana@empresa.com.br", mascararConta("ana@empresa.com.br"))
	assert.Equal(t, "1234", mascararConta("1234"))
}

func TestNovoClusterPainel_JuntaCredencialEAmbiente(t *testing.T) {
	expira := agoraPainel.Add(time.Hour)
	cred := &cloud.CredentialStatus{Authenticated: true, Identity: "arn:aws:sts::482100009310:assumed-role/yby-staging-admin/neto", Method: "iam", ExpiresAt: &expira}
	amb := map[string]ambienteCloud{"aws/staging": {nome: "staging", cfg: &ybycontext.CloudConfig{Provider: "aws", Cluster: "staging", Auth: &ybycontext.AuthConfig{Method: "sso"}}}}
	c := novoClusterPainel("aws", cloud.ClusterInfo{Name: "staging", Region: "us-east-1"}, cred, amb)
	assert.Equal(t, "aws", c.info.Provider)
	assert.True(t, c.autenticado)
	assert.Equal(t, expira, c.expiraEm)
	assert.Equal(t, "staging", c.ambiente)
	assert.Equal(t, "yby-staging-admin", c.papel)
	assert.Equal(t, "SSO · Identity Center", c.metodo)

	sem := novoClusterPainel("gcp", cloud.ClusterInfo{Name: "gke-ml"}, nil, nil)
	assert.False(t, sem.autenticado)
	assert.True(t, sem.expiraEm.IsZero())
}

// Um provider que falha não some calado: os clusters dele ficam da leitura anterior, como
// "sem resposta", e a linha de estado diz o erro.
func TestPainelCloud_ProviderSemResposta(t *testing.T) {
	usarSaidaTexto(t, false)
	f := novasFontesDeExemplo()
	m := painelCarregado(t, f, 140, 30)

	var semAWS []clusterPainel
	for _, c := range clustersDeExemplo() {
		if c.info.Provider != "aws" {
			semAWS = append(semAWS, c)
		}
	}
	erro := stderrors.New("aws eks list-clusters: exit status 255: Error when retrieving token from sso: Token has expired and refresh failed")
	m, _ = enviar(t, m, leituraPainelMsg{leitura: leituraPainel{provedores: 3, clusters: semAWS, falhas: map[string]error{"aws": erro}}})
	require.Len(t, m.clusters, 4, "os clusters da aws continuam, da leitura anterior")
	for _, c := range m.clusters {
		assert.Equal(t, c.info.Provider == "aws", c.desatualizado, c.info.Name)
	}
	assert.Equal(t, "✕ aws sem resposta   ✓ 1 pronto   ▲ 1 token expira em breve", m.linhaResumo())
	assert.Equal(t, "✕ aws sem resposta: Error when retrieving token from sso: Token has expired and refresh failed (rode aws sso login) · r tenta de novo",
		ansi.Strip(m.mensagem()))
	assert.Equal(t, "✕ sem resposta", ansi.Strip(m.clusters[0].celulaStatus(agoraPainel)))

	// A aws volta: as linhas deixam de estar desatualizadas.
	m, _ = enviar(t, m, leituraPainelMsg{leitura: f.leitura})
	assert.Empty(t, m.falhas)
	for _, c := range m.clusters {
		assert.False(t, c.desatualizado)
	}
}

func TestLerClouds_FalhaDeUmProvider(t *testing.T) {
	gcp := &fakeCloudProvider{nome: "gcp", status: &cloud.CredentialStatus{Authenticated: true, Identity: "yby@ml"},
		clusters: []cloud.ClusterInfo{{Name: "gke-ml", Region: "us-central1", Status: "RUNNING"}}}
	aws := &fakeCloudProvider{nome: "aws", errListar: stderrors.New("Token has expired")}
	usarProvidersFalsos(t, aws, gcp)
	log := cloud.NewAuditLoggerWithPath(filepath.Join(t.TempDir(), "audit.log"))
	l := lerClouds(gocontext.Background(), nil, nil, log, agoraPainel)
	assert.NoError(t, l.err, "um provider respondeu")
	assert.Len(t, l.clusters, 1)
	require.Contains(t, l.falhas, "aws")

	usarProvidersFalsos(t, aws)
	l = lerClouds(gocontext.Background(), nil, nil, log, agoraPainel)
	assert.Error(t, l.err, "nenhum provider respondeu")
}

// O layout a partir de 120 colunas não depende da seleção, e a escolha "abaixo" conta a
// linha do aviso de rolagem.
func TestPainelCloud_LayoutPelaAltura(t *testing.T) {
	usarSaidaTexto(t, false)
	for _, altura := range []int{16, 18, 19, 20, 21, 22, 30} {
		for _, largura := range []int{120, 140, 160} {
			m := painelCarregado(t, novasFontesDeExemplo(), largura, altura)
			lay := m.layout()
			for i := range m.clusters {
				m.cursor = i
				assert.Equal(t, lay, m.layout(), "%dx%d, cursor %d", largura, altura, i)
			}
			if lay == layoutAbaixo {
				assert.GreaterOrEqual(t, m.linhasDeDados(), 3, "%dx%d", largura, altura)
			}
			conferirTela(t, m.View(), largura, altura)
		}
	}
}

// O detalhe nunca sai cortado: em qualquer janela, o quadro fecha a borda de baixo, e a
// linha do token aparece mesmo no aperto.
func TestPainelCloud_DetalheSempreFechado(t *testing.T) {
	usarSaidaTexto(t, false)
	for _, largura := range []int{60, 70, 80, 100, 119, 120, 140, 160} {
		for _, altura := range []int{16, 18, 20, 24, 30} {
			m := teclar(t, painelCarregado(t, novasFontesDeExemplo(), largura, altura), baixo, enter)
			tela := ansi.Strip(m.View())
			conferirTela(t, m.View(), largura, altura)
			abre, fecha := strings.Count(tela, "┌"), strings.Count(tela, "└")
			assert.Equal(t, 1, abre, "%dx%d: um quadro\n%s", largura, altura, tela)
			assert.Equal(t, abre, fecha, "%dx%d: o quadro fecha\n%s", largura, altura, tela)
			assert.Contains(t, tela, "expirou às", "%dx%d: a linha do token\n%s", largura, altura, tela)
		}
	}
	// 70 x 30: o detalhe inteiro.
	m := teclar(t, painelCarregado(t, novasFontesDeExemplo(), 70, 30), baixo, enter)
	tela := ansi.Strip(m.View())
	assert.Contains(t, tela, "09:12")
	assert.Contains(t, tela, "yby cloud audit")
}

// A coluna TOKEN não corta o tempo em telas estreitas: a barra encolhe ou sai.
func TestPainelCloud_TokenLegivelEmTelaEstreita(t *testing.T) {
	usarSaidaTexto(t, false)
	for _, largura := range []int{60, 70, 80} {
		tela := ansi.Strip(painelCarregado(t, novasFontesDeExemplo(), largura, 20).View())
		for _, texto := range []string{"11 min", "expirou há 12 min", "3 min", "52 min"} {
			assert.Contains(t, tela, texto, "%d colunas", largura)
		}
	}
}

// A ajuda mostra as teclas da lista em qualquer largura (empilhadas e com rolagem quando
// não cabem lado a lado).
func TestPainelCloud_AjudaComTodasAsTeclas(t *testing.T) {
	usarSaidaTexto(t, false)
	for _, tam := range [][2]int{{60, 16}, {80, 16}, {80, 30}, {100, 30}} {
		m := teclar(t, painelCarregado(t, novasFontesDeExemplo(), tam[0], tam[1]), letra("?"))
		var vistas []string
		for i := 0; i <= m.maxRolagemAjuda(); i++ {
			conferirTela(t, m.View(), tam[0], tam[1])
			vistas = append(vistas, ansi.Strip(m.View()))
			m = teclar(t, m, baixo)
		}
		tudo := strings.Join(vistas, "\n")
		for _, texto := range []string{"renovar o token do cluster", "buscar por nome do cluster", "sai de qualquer lugar", "abrir o detalhe", "início e fim"} {
			assert.Contains(t, tudo, texto, "%dx%d", tam[0], tam[1])
		}
		for _, l := range strings.Split(tudo, "\n") {
			if strings.HasPrefix(l, "│") {
				assert.NotContains(t, l, "…", "%dx%d: nada cortado no quadro", tam[0], tam[1])
			}
		}
	}
}

// A linha selecionada sai invertida, inclusive depois de rolar, com o nome em negrito.
func TestPainelCloud_SelecaoInvertida(t *testing.T) {
	usarSaidaTexto(t, false)
	lipgloss.SetColorProfile(termenv.TrueColor)
	var muitos []clusterPainel
	for _, sufixo := range strings.Split("abcdefghijkl", "") {
		c := clustersDeExemplo()[2]
		c.info.Name = "eks-" + sufixo
		muitos = append(muitos, c)
	}
	f := &fontesFalsas{leitura: leituraPainel{provedores: 1, clusters: muitos}}
	m := painelCarregado(t, f, 100, 16)
	m = teclar(t, m, letra("G"))
	require.Greater(t, m.rolagem, 0)
	invertidas := 0
	for _, l := range strings.Split(m.View(), "\n") {
		if strings.Contains(l, "\x1b[7") || strings.Contains(l, ";7;") || strings.Contains(l, "[1;7") {
			invertidas++
			assert.Contains(t, l, ui.S().Selecionado.Render("eks-l   "), "o nome selecionado em negrito invertido: %q", l)
		}
	}
	assert.Equal(t, 1, invertidas, "só a linha selecionada é invertida")
}
