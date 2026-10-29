package cmd

import (
	"bytes"
	gocontext "context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/cloud"
	"github.com/casheiro/yby-cli/v5/pkg/testutil"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Testes dos achados do checkpoint da fase 4 (capturas do yby cloud dashboard com os CLIs
// falsos de captura/cloud_falso, que imitam a saída dos CLIs reais).

// saidaSSOVencidoAWS é o que o AWS CLI v2 escreve no stderr com o SSO vencido (sai com 255).
const saidaSSOVencidoAWS = "\nError when retrieving token from sso: Token has expired and refresh failed\n"

// clustersDaCaptura são os das capturas: versões e regiões reais de cada provider.
func clustersDaCaptura() []clusterPainel {
	em := func(d time.Duration) time.Time { return agoraPainel.Add(d) }
	return []clusterPainel{
		{info: cloud.ClusterInfo{Provider: "aws", Name: "prod-core", Region: "us-east-1", Version: "1.30", Status: "ACTIVE"},
			autenticado: true, conta: "482193759310", papel: "PlatformAdmin", metodo: "IAM", expiraEm: em(46 * time.Minute)},
		{info: cloud.ClusterInfo{Provider: "aws", Name: "staging", Region: "us-east-1", Version: "1.29", Status: "ACTIVE"},
			autenticado: true, conta: "482193759310", papel: "PlatformAdmin", metodo: "IAM", expiraEm: em(46 * time.Minute)},
		{info: cloud.ClusterInfo{Provider: "azure", Name: "aks-dados", Region: "brazilsouth", Version: "1.30.4", Status: "Stopped"},
			autenticado: true, conta: "7f3c2a91-4b6e-4d1a-9c55-2e8f0b6d4a13", usuario: "neto@casheiro.onmicrosoft.com", metodo: "CLI do Azure", expiraEm: em(2 * time.Minute)},
		{info: cloud.ClusterInfo{Provider: "azure", Name: "aks-web", Region: "eastus2", Version: "1.31.1", Status: "Running"},
			autenticado: true, conta: "7f3c2a91-4b6e-4d1a-9c55-2e8f0b6d4a13", usuario: "neto@casheiro.onmicrosoft.com", metodo: "CLI do Azure", expiraEm: em(2 * time.Minute)},
		{info: cloud.ClusterInfo{Provider: "gcp", Name: "gke-ml", Region: "southamerica-east1", Version: "1.30.5-gke.1014001", Status: "RUNNING", ProjectID: "ml-plataforma"},
			autenticado: true, conta: "ml-plataforma", papel: "neto@ml-plataforma.iam.gserviceaccount.com", metodo: "CLI do gcloud", expiraEm: em(-12 * time.Minute)},
	}
}

func fontesDaCaptura() *fontesFalsas {
	return &fontesFalsas{leitura: leituraPainel{provedores: 3, clusters: clustersDaCaptura()}}
}

// linhaDaTabela é a linha (sem ANSI) da tabela que começa com o provider e o nome.
func linhaDaTabela(t *testing.T, tela, provider, nome string) string {
	t.Helper()
	for _, l := range strings.Split(ansi.Strip(tela), "\n") {
		campos := strings.Fields(l)
		if len(campos) >= 2 && campos[0] == provider && campos[1] == nome {
			return l
		}
	}
	t.Fatalf("linha de %s/%s não está na tela:\n%s", provider, nome, ansi.Strip(tela))
	return ""
}

// Achado 2: com espaço, REGIÃO e K8S mostram o texto inteiro; sem espaço, a versão encurta
// sem o sufixo do provider, nunca "1.30.…".
func TestPainelCloud_RegiaoEVersaoInteirasComEspaco(t *testing.T) {
	usarSaidaTexto(t, false)
	m := painelCarregado(t, fontesDaCaptura(), 160, 34)
	linha := linhaDaTabela(t, m.View(), "gcp", "gke-ml")
	assert.Contains(t, linha, "southamerica-east1")
	assert.Contains(t, linha, "1.30.5-gke.1014001")

	for _, largura := range []int{60, 70, 80, 100, 120} {
		m := painelCarregado(t, fontesDaCaptura(), largura, 24)
		tela := ansi.Strip(m.View())
		assert.NotContains(t, tela, "1.3"+ui.G().Reticencias, "%d colunas", largura)
		assert.NotContains(t, tela, "1.30."+ui.G().Reticencias, "%d colunas", largura)
	}
}

func TestVersaoQueCabe(t *testing.T) {
	assert.Equal(t, "1.30.5-gke.1014001", versaoQueCabe("1.30.5-gke.1014001", 18))
	assert.Equal(t, "1.30.5", versaoQueCabe("1.30.5-gke.1014001", 17))
	assert.Equal(t, "1.30.5", versaoQueCabe("1.30.5-gke.1014001", 6))
	assert.Equal(t, "1.30", versaoQueCabe("1.30.5-gke.1014001", 5))
	assert.Equal(t, "1.30", versaoQueCabe("1.30.4", 4))
	assert.Equal(t, "1.31.1", versaoQueCabe("1.31.1", 6))
	assert.Equal(t, "v1.29.3", versaoQueCabe("v1.29.3+k3s1", 7))
}

// Achado 12a: a busca não muda as larguras das colunas (a TOKEN não pula de lugar).
func TestPainelCloud_BuscaNaoMudaAsColunas(t *testing.T) {
	usarSaidaTexto(t, false)
	m := painelCarregado(t, fontesDaCaptura(), 160, 34)
	cabecalho := func(m painelCloud) string {
		for _, l := range strings.Split(ansi.Strip(m.View()), "\n") {
			if strings.HasPrefix(l, "CLOUD") {
				return strings.TrimRight(l, " ")
			}
		}
		t.Fatal("sem cabeçalho")
		return ""
	}
	antes := cabecalho(m)
	m = teclar(t, m, letra("/"), letra("a"), letra("k"), letra("s"))
	assert.Equal(t, antes, cabecalho(m))
	m = teclar(t, m, letra("-"), letra("w"))
	assert.Equal(t, antes, cabecalho(m))
	assert.Equal(t, "aks-web", m.visiveis()[0].info.Name)
	assert.Contains(t, linhaDaTabela(t, m.View(), "azure", "aks-web"), "eastus2")
}

// Achado 3: no Azure, a conta é a assinatura (mascarada) e a identidade é o usuário.
func TestPainelCloud_AzureComAssinaturaEUsuario(t *testing.T) {
	usarSaidaTexto(t, false)
	expira := agoraPainel.Add(time.Hour)
	cred := &cloud.CredentialStatus{Authenticated: true, Identity: "neto@casheiro.onmicrosoft.com",
		Account: "7f3c2a91-4b6e-4d1a-9c55-2e8f0b6d4a13", Method: "az-cli", ExpiresAt: &expira}
	c := novoClusterPainel("azure", cloud.ClusterInfo{Name: "aks-web", Region: "eastus2", Status: "Running"}, cred, nil)
	assert.Equal(t, "7f3c2a91-4b6e-4d1a-9c55-2e8f0b6d4a13", c.conta)
	assert.Equal(t, "neto@casheiro.onmicrosoft.com", c.usuario)
	assert.Empty(t, c.papel)

	m := painelCarregado(t, fontesDaCaptura(), 160, 34)
	assert.Equal(t, 3, m.contas(), "conta AWS, assinatura Azure e projeto GCP")
	assert.Contains(t, ansi.Strip(m.contexto()), "5 clusters · 3 contas")
	m = teclar(t, m, baixo, baixo) // aks-dados
	tela := ansi.Strip(m.View())
	assert.Contains(t, tela, "conta     7f3c·…·4a13")
	assert.Contains(t, tela, "usuário   neto@casheiro.onmicrosoft.com")
	assert.NotContains(t, tela, "papel     neto@")
}

// Achado 4: a saída real do AWS CLI com o SSO vencido chega à frase da falha (com a dica do
// aws sso login), ao toast do t (sem comando nem código) e ao audit log ("SSO vencido").
func TestPainelCloud_SSOVencidoDoCLIAteAFraseOToastEAAuditoria(t *testing.T) {
	usarSaidaTexto(t, false)
	runner := &testutil.MockRunner{
		RunStdinOutputFunc: func(gocontext.Context, string, string, ...string) ([]byte, error) {
			return []byte(saidaSSOVencidoAWS), stderrors.New("exit status 255")
		},
	}
	aws := cloud.GetProvider(runner, "aws")
	require.NotNil(t, aws)
	ctx := gocontext.Background()

	// a leitura: a aws não responde
	_, errListar := aws.ListClusters(ctx, cloud.ListOptions{})
	require.Error(t, errListar)
	f := novasFontesDeExemplo()
	m := painelCarregado(t, f, 160, 34)
	var semAWS []clusterPainel
	for _, c := range clustersDeExemplo() {
		if c.info.Provider != "aws" {
			semAWS = append(semAWS, c)
		}
	}
	m, _ = enviar(t, m, leituraPainelMsg{leitura: leituraPainel{provedores: 3, clusters: semAWS, falhas: map[string]error{"aws": errListar}}})
	assert.Equal(t, "✕ aws sem resposta: SSO vencido (rode aws sso login) · r tenta de novo",
		ansi.Strip(m.mensagem()))

	// o t: o toast tem o motivo, sem "aws eks get-token: exit status 255"
	errRenovar := aws.RefreshToken(ctx, cloud.ClusterInfo{Name: "staging", Region: "us-east-1"})
	require.Error(t, errRenovar)
	f.erroRenova = errRenovar
	m = teclar(t, painelCarregado(t, f, 140, 30), baixo)
	m, cmd := enviar(t, m, letra("t"))
	m, _ = enviar(t, m, cmd())
	assert.Equal(t, "✕ não deu para renovar o token de staging · Error when retrieving token from sso: Token has expired and refresh failed",
		ansi.Strip(m.mensagem()))

	// o audit log guarda o motivo, e o detalhe o resume como "SSO vencido"
	log := cloud.NewAuditLoggerWithPath(filepath.Join(t.TempDir(), "audit.log"))
	registrarRenovacao(log, clustersDeExemplo()[1], agoraPainel, errRenovar)
	eventos, err := log.ReadEvents(time.Time{})
	require.NoError(t, err)
	require.Len(t, eventos, 1)
	assert.Contains(t, eventos[0].Error, "Token has expired and refresh failed")
	assert.Equal(t, "SSO vencido", descricaoEvento(eventos[0]))
}

// Sem saída do CLI, a nota diz o código, e não o comando inteiro.
func TestResumoDoErro_SemSaidaDoCLI(t *testing.T) {
	assert.Equal(t, "o CLI saiu com o código 255", resumoDoErro(stderrors.New("aws eks get-token: exit status 255")))
	assert.Equal(t, "Token has expired", resumoDoErro(stderrors.New("aws eks get-token: exit status 255: Token has expired")))
	assert.Equal(t, "sem rede", resumoDoErro(stderrors.New("sem rede")))
}

// Achado 5: o cluster parado vence o aviso de token no STATUS, e o detalhe tem o status.
func TestPainelCloud_ClusterParadoVenceOAvisoDoToken(t *testing.T) {
	usarSaidaTexto(t, false)
	c := clustersDaCaptura()[2] // aks-dados parado, token com 2 min
	assert.Equal(t, estadoNaoAtivo, c.estadoEm(agoraPainel))
	assert.Equal(t, "▲ parado", ansi.Strip(c.celulaStatus(agoraPainel)))
	assert.Equal(t, "▲ parado", ansi.Strip(c.statusNoDetalhe(agoraPainel)))
	assert.Equal(t, "✓ ativo", ansi.Strip(clustersDaCaptura()[3].statusNoDetalhe(agoraPainel)))

	m := teclar(t, painelCarregado(t, fontesDaCaptura(), 160, 34), baixo, baixo)
	tela := ansi.Strip(m.View())
	assert.Contains(t, linhaDaTabela(t, tela, "azure", "aks-dados"), "▲ parado")
	assert.Contains(t, tela, "status    ▲ parado")
	// a frase fala do que dá para resolver: o token que vence (t renova), não o cluster parado
	assert.Contains(t, ansi.Strip(m.mensagem()), "gke-ml sem token")
	assert.Equal(t, "✓ 2 prontos   ▲ 1 token expira em breve   ✕ 1 token expirado   ▲ 1 cluster não ativo", ansi.Strip(m.linhaResumo()))
}

// Achado 6: r com um provider falhando dá aviso, e não "✓ clusters recarregados".
func TestPainelCloud_RecargaParcialAvisa(t *testing.T) {
	usarSaidaTexto(t, false)
	f := novasFontesDeExemplo()
	m := painelCarregado(t, f, 140, 30)
	var semAWS []clusterPainel
	for _, c := range clustersDeExemplo() {
		if c.info.Provider != "aws" {
			semAWS = append(semAWS, c)
		}
	}
	f.leitura = leituraPainel{provedores: 3, clusters: semAWS, falhas: map[string]error{"aws": stderrors.New("Token has expired")}}
	m, cmd := enviar(t, m, letra("r"))
	m, _ = enviar(t, m, cmd())
	assert.Equal(t, "▲ aws não respondeu; os outros clusters foram recarregados", ansi.Strip(m.mensagem()))
	assert.NotContains(t, m.mensagem(), "✓")
}

// Achado 7: as linhas da leitura anterior saem apagadas, sem a cota, e o detalhe diz de
// quando são os dados.
func TestPainelCloud_LinhaSemRespostaApagada(t *testing.T) {
	usarSaidaTexto(t, false)
	lipgloss.SetColorProfile(termenv.TrueColor) // o usarSaidaTexto restaura
	f := fontesDaCaptura()
	m := painelCarregado(t, f, 160, 34)
	var semAWS []clusterPainel
	for _, c := range clustersDaCaptura() {
		if c.info.Provider != "aws" {
			semAWS = append(semAWS, c)
		}
	}
	m.agora = func() time.Time { return agoraPainel.Add(5 * time.Minute) }
	m, _ = enviar(t, m, leituraPainelMsg{leitura: leituraPainel{provedores: 3, clusters: semAWS, falhas: map[string]error{"aws": stderrors.New("x")}}})
	prod := m.clusters[0]
	require.True(t, prod.desatualizado)
	agora := m.agora()
	s := ui.S()
	assert.Equal(t, s.TintaSuave.Render("41 min"), prod.celulaToken(agora, 12), "sem a cota, só o tempo apagado")
	assert.Equal(t, s.TintaSuave.Render("válido até 12:53 · faltam 41 min"), prod.tokenNoDetalhe(agora))
	assert.Equal(t, "✕ sem resposta · dados de 12:07", ansi.Strip(prod.statusNoDetalhe(agora)))
	_, linhas := m.tabela(m.clusters, 160)
	for _, cel := range linhas[0].Celulas[:4] {
		assert.True(t, strings.HasPrefix(cel, strings.TrimSuffix(s.TintaSuave.Render("x"), "x\x1b[0m")), "célula apagada: %q", cel)
	}
	require.Equal(t, 0, m.cursor, "prod-core, da leitura anterior, selecionado")
	assert.Contains(t, ansi.Strip(m.View()), "dados de 12:07")
}

// Achado 8: abaixo de 120 colunas, o detalhe por cima fica uma linha abaixo do resumo.
func TestPainelCloud_LinhaEmBrancoAntesDoDetalhePorCima(t *testing.T) {
	usarSaidaTexto(t, false)
	for _, tam := range [][2]int{{60, 20}, {80, 16}, {100, 30}} {
		m := teclar(t, painelCarregado(t, novasFontesDeExemplo(), tam[0], tam[1]), baixo, enter)
		require.True(t, m.porCima())
		linhas := strings.Split(ansi.Strip(m.View()), "\n")
		assert.Empty(t, strings.TrimSpace(linhas[3]), "%v: a linha depois do resumo", tam)
		assert.True(t, strings.HasPrefix(linhas[4], "┌ EKS"), "%v: %q", tam, linhas[4])
		conferirTela(t, m.View(), tam[0], tam[1])
	}
}

// Achado 9: o resumo nunca corta um grupo no meio; passa à forma curta e tira os grupos
// menos urgentes.
func TestPainelCloud_ResumoSemCorteNoMeio(t *testing.T) {
	usarSaidaTexto(t, false)
	m := painelCarregado(t, fontesDaCaptura(), 60, 16)
	clusters := clustersDaCaptura()
	clusters[2].info.Status = "Running" // dois tokens expiram em breve
	m, _ = enviar(t, m, leituraPainelMsg{leitura: leituraPainel{provedores: 3, clusters: clusters}})
	assert.Equal(t, "✓ 2 prontos   ▲ 2 expiram logo   ✕ 1 expirado", ansi.Strip(m.linhaResumo()))

	m.largura = 32
	assert.Equal(t, "▲ 2 expiram logo   ✕ 1 expirado", ansi.Strip(m.linhaResumo()))
	m.largura = 20
	assert.Equal(t, "✕ 1 expirado", ansi.Strip(m.linhaResumo()))
	m.largura = 160
	assert.Equal(t, "✓ 2 prontos   ▲ 2 tokens expiram em breve   ✕ 1 token expirado", ansi.Strip(m.linhaResumo()))
}

// Achado 10: no ASCII, o masthead, o título do quadro e a linha do token não têm "·".
func TestPainelCloud_ASCIISemPontoMedio(t *testing.T) {
	usarSaidaTexto(t, true)
	m := teclar(t, painelCarregado(t, fontesDaCaptura(), 160, 34), baixo, baixo)
	linhas := strings.Split(ansi.Strip(m.View()), "\n")
	assert.Contains(t, linhas[0], "5 clusters / 3 contas")
	var titulo, token, conta string
	for _, l := range linhas {
		switch {
		case strings.Contains(l, "AKS"):
			titulo = l
		case strings.Contains(l, "token     "):
			token = l
		case strings.Contains(l, "conta     "):
			conta = l
		}
	}
	assert.Contains(t, titulo, "AKS / aks-dados")
	assert.Contains(t, token, "/ faltam 2 min")
	assert.Contains(t, conta, "7f3c...4a13")
	for _, l := range []string{linhas[0], titulo, token, conta} {
		assert.NotContains(t, l, "·", l)
	}
}

// Achado 11: as teclas dos avisos da linha de estado em negrito.
func TestPainelCloud_TeclasEmNegritoNosAvisos(t *testing.T) {
	usarSaidaTexto(t, false)
	lipgloss.SetColorProfile(termenv.TrueColor)
	s := ui.S()
	m := teclar(t, painelCarregado(t, novasFontesDeExemplo(), 140, 30), letra("R"))
	assert.Contains(t, m.mensagem(), s.Negrito.Render("r"))
	assert.Contains(t, m.mensagem(), s.Negrito.Render("t"))
	assert.Equal(t, "○ recarregar agora é r; t renova o token", ansi.Strip(m.mensagem()))

	m = teclar(t, painelCarregado(t, novasFontesDeExemplo(), 100, 30), baixo, enter, letra("q"))
	assert.Contains(t, m.mensagem(), s.Negrito.Render("q"))
	assert.Contains(t, m.mensagem(), s.Negrito.Render("esc"))
	assert.Equal(t, "○ q sai só da lista; esc volta", ansi.Strip(m.mensagem()))
}

// Achado 12b: um t que falha relê a auditoria, e o registro da falha aparece já.
func TestPainelCloud_TQueFalhaReleAAuditoria(t *testing.T) {
	usarSaidaTexto(t, false)
	f := novasFontesDeExemplo()
	f.erroRenova = stderrors.New("aws eks get-token: exit status 255: Error when retrieving token from sso: Token has expired")
	novo := cloud.CloudAuditEvent{Timestamp: agoraPainel, Action: "refresh", Provider: "aws", Cluster: "staging",
		Error: f.erroRenova.Error()}
	fontes := f.fontes()
	fontes.auditoria = func() []cloud.CloudAuditEvent { return append(auditoriaDeExemplo(), novo) }
	m := novoPainelCloud(fontes, func() time.Time { return agoraPainel })
	m, _ = enviar(t, m, tea.WindowSizeMsg{Width: 140, Height: 30})
	m, _ = enviar(t, m, leituraPainelMsg{leitura: f.leitura})
	m = teclar(t, m, baixo)
	m, cmd := enviar(t, m, letra("t"))
	m, cmd = enviar(t, m, cmd())
	require.NotNil(t, cmd)
	var achou bool
	for _, msg := range executarLote(cmd) {
		if a, ok := msg.(auditoriaMsg); ok {
			achou = true
			m, _ = enviar(t, m, a)
		}
	}
	require.True(t, achou, "o t que falhou não releu a auditoria")
	assert.Len(t, m.auditoria, len(auditoriaDeExemplo())+1)
	assert.False(t, m.lendo, "sem ler as clouds de novo")
}

// executarLote roda um tea.Cmd (e os de um tea.Batch) e devolve as mensagens.
func executarLote(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	lote, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var saida []tea.Msg
	for _, c := range lote {
		saida = append(saida, executarLote(c)...)
	}
	return saida
}

// Achado 1: a chave do cluster e o yby cloud connect levam a região.
func TestPainelCloud_RegiaoNaChaveENoConnect(t *testing.T) {
	c := clustersDaCaptura()[1]
	assert.Equal(t, "aws/us-east-1/staging", c.chave())
	assert.Equal(t, "yby cloud connect --provider aws --region us-east-1 --cluster staging", comandoConectar(c))
}

// Achado 12c: com o stdout num pipe e o stderr num terminal, o erro sai com cor.
func TestCorDoErroPeloStderr(t *testing.T) {
	usarSaidaTexto(t, false)
	antSaida, antStdout, antStderr, antPerfil := saidaErro, stdoutTerminal, stderrTerminal, perfilDoStderr
	t.Cleanup(func() {
		saidaErro, stdoutTerminal, stderrTerminal, perfilDoStderr = antSaida, antStdout, antStderr, antPerfil
	})
	saidaErro = os.Stderr
	stdoutTerminal = func() bool { return false }
	stderrTerminal = func() bool { return true }
	perfilDoStderr = func() termenv.Profile { return termenv.TrueColor }
	corDoErroPeloStderr()
	assert.Equal(t, termenv.TrueColor, lipgloss.ColorProfile())

	// com NO_COLOR, ou o stderr fora do terminal, nada muda
	lipgloss.SetColorProfile(termenv.Ascii)
	stderrTerminal = func() bool { return false }
	corDoErroPeloStderr()
	assert.Equal(t, termenv.Ascii, lipgloss.ColorProfile())
	stderrTerminal = func() bool { return true }
	ui.Configurar(ui.Opcoes{SemCor: true})
	lipgloss.SetColorProfile(termenv.Ascii)
	corDoErroPeloStderr()
	assert.Equal(t, termenv.Ascii, lipgloss.ColorProfile())

	// o erro escrito em outro lugar (os testes) não mexe na cor
	ui.Configurar(ui.Opcoes{})
	saidaErro = &bytes.Buffer{}
	corDoErroPeloStderr()
	assert.Equal(t, termenv.Ascii, lipgloss.ColorProfile())
}

// Testes das revisões do checkpoint (emenda).

// semRespostaDaAWS é o painel depois de uma leitura em que a aws falhou com err.
func semRespostaDaAWS(t *testing.T, m painelCloud, err error) painelCloud {
	t.Helper()
	var semAWS []clusterPainel
	for _, c := range clustersDaCaptura() {
		if c.info.Provider != "aws" {
			semAWS = append(semAWS, c)
		}
	}
	m.agora = func() time.Time { return agoraPainel.Add(5 * time.Minute) }
	m, _ = enviar(t, m, leituraPainelMsg{leitura: leituraPainel{provedores: 3, clusters: semAWS, falhas: map[string]error{"aws": err}}})
	return m
}

// O token vencido vence o "parado" (conta em "token expirado" e vira "sem token: t renova");
// o parado vence só o aviso.
func TestPainelCloud_TokenVencidoVenceOParado(t *testing.T) {
	usarSaidaTexto(t, false)
	c := clustersDaCaptura()[2] // aks-dados, Stopped
	c.expiraEm = agoraPainel.Add(-5 * time.Minute)
	assert.Equal(t, estadoExpirado, c.estadoEm(agoraPainel))
	c.expiraEm = agoraPainel.Add(2 * time.Minute)
	assert.Equal(t, estadoNaoAtivo, c.estadoEm(agoraPainel))
}

// Com a REGIÃO cortada, a K8S vai a major.minor antes; o patch só fica com a região inteira.
func TestPainelCloud_VersaoMajorMinorAntesDeCortarARegiao(t *testing.T) {
	usarSaidaTexto(t, false)
	campos := strings.Fields(linhaDaTabela(t, painelCarregado(t, fontesDaCaptura(), 72, 20).View(), "gcp", "gke-ml"))
	assert.Equal(t, "southamerica-east1", campos[2])
	assert.Equal(t, "1.30", campos[3])
	campos = strings.Fields(linhaDaTabela(t, painelCarregado(t, fontesDaCaptura(), 140, 18).View(), "gcp", "gke-ml"))
	assert.Equal(t, "southamerica-east1", campos[2])
	assert.Equal(t, "1.30.5", campos[3], "com a região inteira, o patch fica")
}

// O STATUS "✕ sem resposta" não corta: sai antes a REGIÃO (ou a K8S).
func TestPainelCloud_StatusSemRespostaInteiro(t *testing.T) {
	usarSaidaTexto(t, false)
	for _, tam := range [][2]int{{60, 20}, {120, 16}} {
		m := semRespostaDaAWS(t, painelCarregado(t, fontesDaCaptura(), tam[0], tam[1]), stderrors.New("x"))
		assert.Contains(t, linhaDaTabela(t, m.View(), "aws", "prod-core"), "✕ sem resposta", "%v", tam)
	}
}

// No detalhe estreito, a linha do token passa à forma curta em vez de cortar o tempo.
func TestPainelCloud_TokenCurtoNoDetalheEstreito(t *testing.T) {
	usarSaidaTexto(t, false)
	tela := ansi.Strip(painelCarregado(t, fontesDaCaptura(), 120, 16).View())
	assert.Contains(t, tela, "token     ✓ até 12:53 · 46 min")
	assert.NotContains(t, tela, "faltam 46 m…")
	c := clustersDaCaptura()[3] // aks-web, expira em 2 min
	assert.Equal(t, "▲ expira 12:09 · 2 min", ansi.Strip(c.tokenQueCabe(agoraPainel, 30)))
	assert.Equal(t, "▲ expira às 12:09 · faltam 2 min", ansi.Strip(c.tokenQueCabe(agoraPainel, 60)))
}

// A frase do provider sem resposta: causa conhecida em poucas palavras, com a dica; sem
// espaço, sai o r e depois a hora, nunca a dica. Causa desconhecida: o r antes da linha do CLI.
func TestPainelCloud_FraseDaFalhaCabe(t *testing.T) {
	usarSaidaTexto(t, false)
	sso := stderrors.New("aws eks list-clusters: exit status 255: Error when retrieving token from sso: Token has expired and refresh failed")
	for _, largura := range []int{60, 72, 80} {
		m := semRespostaDaAWS(t, painelCarregado(t, fontesDaCaptura(), largura, 20), sso)
		linhas := strings.Split(ansi.Strip(m.View()), "\n")
		estado := linhas[len(linhas)-2]
		assert.Contains(t, estado, "SSO vencido (rode aws sso login)", "%d", largura)
		assert.NotContains(t, estado, "…", "%d", largura)
	}
	m := semRespostaDaAWS(t, painelCarregado(t, fontesDaCaptura(), 80, 20), sso)
	assert.Contains(t, ansi.Strip(m.View()), "(rode aws sso login) · r tenta de novo")

	outro := stderrors.New("aws eks list-clusters: exit status 254: Could not connect to the endpoint URL: https://eks.us-east-1.amazonaws.com/clusters")
	m = semRespostaDaAWS(t, painelCarregado(t, fontesDaCaptura(), 100, 30), outro)
	assert.True(t, strings.HasPrefix(ansi.Strip(m.mensagem()), "✕ aws sem resposta · r tenta de novo · Could not connect"), ansi.Strip(m.mensagem()))
	assert.Contains(t, ansi.Strip(m.View()), "atualizado 12:12:12", "a frase longa do CLI não tira a hora")
}

// A partir de 120 colunas com pouca altura, o detalhe ao lado fica uma linha abaixo do
// resumo quando isso não lhe custa conteúdo; se custar (a auditoria), ele sobe.
func TestPainelCloud_DetalheAoLadoAbaixoDoResumo(t *testing.T) {
	usarSaidaTexto(t, false)
	m := teclar(t, painelCarregado(t, novasFontesDeExemplo(), 140, 18), baixo)
	require.Equal(t, layoutAoLado, m.layout())
	linhas := strings.Split(ansi.Strip(m.View()), "\n")
	assert.NotContains(t, linhas[3], "┌", "a linha depois do resumo")
	assert.Contains(t, linhas[4], "┌ EKS · staging")
	assert.Contains(t, ansi.Strip(m.View()), "SSO vencido")

	m = teclar(t, painelCarregado(t, novasFontesDeExemplo(), 120, 16), baixo)
	tela := ansi.Strip(m.View())
	assert.Contains(t, tela, "11:55  ✕ renova  SSO vencido", "a auditoria fica, com o quadro colado no resumo")
	assert.NotContains(t, tela, "status    ✓ ativo", "a linha status que só repete a tabela sai antes")
}

// A ajuda no ASCII também não tem "·".
func TestPainelCloud_AjudaASCIISemPontoMedio(t *testing.T) {
	usarSaidaTexto(t, true)
	tela := ansi.Strip(teclar(t, painelCarregado(t, fontesDaCaptura(), 100, 30), letra("?")).View())
	assert.Contains(t, tela, "TECLAS / CLOUD")
	assert.Contains(t, tela, "LISTA / CLUSTERS")
	assert.NotContains(t, tela, "·")
}

// A 72×20 o detalhe por cima traz a auditoria: a linha em branco entre IDENTIDADE e
// AUDITORIA RECENTE sai antes da seção inteira, e o aviso de rolagem da tabela fica.
func TestPainelCloud_DetalhePorCimaComAuditoriaA72x20(t *testing.T) {
	usarSaidaTexto(t, false)
	m := teclar(t, painelCarregado(t, novasFontesDeExemplo(), 72, 20), baixo, enter)
	require.Equal(t, layoutPorCima, m.layout())
	tela := ansi.Strip(m.View())
	assert.Contains(t, tela, "AUDITORIA RECENTE")
	assert.Contains(t, tela, "11:55  ✕ renova")
	assert.Contains(t, tela, "↓ 2 clusters abaixo")
	conferirTela(t, m.View(), 72, 20)
}
