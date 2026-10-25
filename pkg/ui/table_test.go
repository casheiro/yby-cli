package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func tableColunas() []Coluna {
	return []Coluna{
		{Titulo: "Nome", Min: 8, Max: 30, Prioridade: 0},
		{Titulo: "Status", Min: 14, Max: 20, Prioridade: 1},
		{Titulo: "Prontos", Min: 7, Max: 7, Prioridade: 3, Direita: true},
		{Titulo: "Idade", Min: 5, Max: 8, Prioridade: 2, Direita: true},
		{Titulo: "Nó", Min: 10, Max: 40, Prioridade: 4},
	}
}

func tableLinhas() []Linha {
	return []Linha{
		{Celulas: []string{"api-gateway-7d9f8b6c5-x2k4m", Status(StatusRodando, "Running"), "1/1", "3d", "ip-10-0-1-23.internal"}},
		{Celulas: []string{"pagamentos-configuração-ção", Status(StatusFalha, "CrashLoopBackOff"), "0/1", "12m", "ip-10-0-2-77.internal"}},
		{Celulas: []string{"fila-mensageria", Status(StatusAlerta, "Pending"), "0/1", "4s", "-"}},
		{Celulas: []string{"migração-banco", Status(StatusConcluido, "Completed"), "0/1", "2d", "ip-10-0-1-23.internal"}, Concluida: true},
		{Celulas: []string{"worker-parado", Status(StatusParado, "Parado"), "0/0", "9d", "-"}},
	}
}

func tableLarguraMax(t *testing.T, s string, largura int) {
	t.Helper()
	for i, l := range strings.Split(s, "\n") {
		if w := lipgloss.Width(l); w > largura {
			t.Errorf("linha %d tem %d células (> %d): %q", i+1, w, largura, ansi.Strip(l))
		}
	}
}

func TestTabela_Golden(t *testing.T) {
	casos := []struct {
		nome    string
		escuro  bool
		perfil  termenv.Profile
		ascii   bool
		largura int
	}{
		{"table_escuro_140", true, termenv.TrueColor, false, 140},
		{"table_claro_140", false, termenv.TrueColor, false, 140},
		{"table_escuro_80", true, termenv.TrueColor, false, 80},
		{"table_claro_40", false, termenv.TrueColor, false, 40},
		{"table_ascii_80", true, termenv.Ascii, true, 80},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarTema(t, c.escuro, c.perfil, c.ascii)
			saida := Tabela(tableColunas(), tableLinhas(), 1, c.largura)
			tableLarguraMax(t, saida, c.largura)
			conferirGolden(t, c.nome, saida)
		})
	}
}

func TestTabela_NenhumaLinhaPassaDaLargura(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	linhas := append(tableLinhas(),
		Linha{Celulas: []string{"日本語のポッド名前テスト", "● 実行中", "1/1", "1日", "ノード"}},
		Linha{Celulas: []string{"emoji-🚀🚀🚀-deploy", Status(StatusRodando, "Rodando 🚀"), "1/1", "1d", "👩‍💻"}},
	)
	for _, n := range []int{1, 5, 12, 20, 30, 40, 60, 80, 100, 140, 200} {
		for sel := -1; sel < len(linhas); sel++ {
			tableLarguraMax(t, Tabela(tableColunas(), linhas, sel, n), n)
		}
	}
}

func TestTabela_ColunasDeMenorPrioridadeSomem(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	largo := ansi.Strip(Tabela(tableColunas(), tableLinhas(), -1, 140))
	for _, titulo := range []string{"NOME", "STATUS", "PRONTOS", "IDADE", "NÓ"} {
		if !strings.Contains(largo, titulo) {
			t.Errorf("largura 140: esperava coluna %s", titulo)
		}
	}
	estreito := ansi.Strip(Tabela(tableColunas(), tableLinhas(), -1, 40))
	if strings.Contains(estreito, "NÓ") || !strings.Contains(estreito, "PRONTOS") {
		t.Errorf("largura 40: só a de menor prioridade deveria sumir:\n%s", estreito)
	}
	menor := ansi.Strip(Tabela(tableColunas(), tableLinhas(), -1, 31))
	if strings.Contains(menor, "PRONTOS") || !strings.Contains(menor, "IDADE") {
		t.Errorf("largura 31: PRONTOS deveria sumir antes de IDADE:\n%s", menor)
	}
	if !strings.Contains(estreito, "NOME") || !strings.Contains(estreito, "STATUS") {
		t.Errorf("largura 40: nome e status deveriam permanecer:\n%s", estreito)
	}
}

func TestTabela_TruncaComReticenciasSemQuebrarUTF8(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	cols := []Coluna{{Titulo: "Nome", Min: 6, Max: 8}}
	saida := ansi.Strip(Tabela(cols, []Linha{
		{Celulas: []string{"configuração"}},
		{Celulas: []string{"日本語のポッド"}},
	}, -1, 80))
	linhas := strings.Split(saida, "\n")
	if !strings.HasPrefix(linhas[1], "configu…") {
		t.Errorf("acentuado truncado errado: %q", linhas[1])
	}
	if strings.ContainsRune(linhas[2], '�') || !strings.Contains(linhas[2], "…") {
		t.Errorf("CJK truncado errado: %q", linhas[2])
	}
	if lipgloss.Width(linhas[2]) > 8 {
		t.Errorf("CJK passou do Max: %d", lipgloss.Width(linhas[2]))
	}
}

func TestTabela_ReticenciasASCII(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	saida := Tabela([]Coluna{{Titulo: "Nome", Min: 4, Max: 6}}, []Linha{{Celulas: []string{"abcdefghij"}}}, -1, 80)
	if !strings.Contains(saida, "abcde"+ASCII.Reticencias) {
		t.Errorf("esperava reticências ASCII: %q", saida)
	}
}

func TestTabela_AlinhamentoADireita(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	saida := ansi.Strip(Tabela([]Coluna{{Titulo: "N", Min: 5, Max: 5, Direita: true}}, []Linha{{Celulas: []string{"42"}}}, -1, 80))
	if !strings.Contains(saida, "   42") {
		t.Errorf("esperava alinhamento à direita: %q", saida)
	}
}

func TestTabela_SelecionadaUsaVideoInversoEPreencheLargura(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	saida := Tabela(tableColunas(), tableLinhas(), 0, 100)
	linhas := strings.Split(saida, "\n")
	if !strings.Contains(linhas[1], "\x1b[1;7m") && !strings.Contains(linhas[1], "\x1b[7;1m") && !strings.Contains(linhas[1], "\x1b[1m\x1b[7m") {
		// aceita qualquer sequência que contenha o código de vídeo inverso (7)
		if !strings.Contains(linhas[1], ";7") && !strings.Contains(linhas[1], "[7") {
			t.Errorf("linha selecionada sem vídeo inverso: %q", linhas[1])
		}
	}
	if w := lipgloss.Width(linhas[1]); w != 100 {
		t.Errorf("linha selecionada deveria preencher 100 células, tem %d", w)
	}
	if strings.Contains(linhas[2], "[7") {
		t.Errorf("linha não selecionada com vídeo inverso: %q", linhas[2])
	}
}

func TestTabela_SelecionadaSemCorAindaDestaca(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	t.Setenv("NO_COLOR", "1")
	Configurar(Opcoes{Tema: TemaEscuro, SemCor: true})
	saida := Tabela(tableColunas(), tableLinhas(), 0, 100)
	if strings.Contains(saida, "38;") {
		t.Errorf("com NO_COLOR não deveria haver cor: %q", saida)
	}
}

func TestTabela_ConcluidaEmTintaSuave(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	saida := Tabela(tableColunas(), tableLinhas(), -1, 140)
	linhas := strings.Split(saida, "\n")
	suave := S().TintaSuave.Render("x")
	prefixo := suave[:strings.Index(suave, "x")]
	if !strings.HasPrefix(linhas[4], prefixo) {
		t.Errorf("linha concluída deveria começar em TintaSuave: %q", linhas[4])
	}
	if strings.HasPrefix(linhas[1], prefixo) {
		t.Errorf("linha normal não deveria ser TintaSuave inteira: %q", linhas[1])
	}
}

func TestTabela_SemColunasLarguraOuLinhas(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	if Tabela(nil, nil, -1, 80) != "" || Tabela(tableColunas(), nil, -1, 0) != "" {
		t.Error("esperava saída vazia")
	}
	saida := Tabela(tableColunas(), nil, -1, 80)
	if strings.Contains(saida, "\n") || !strings.Contains(saida, "NOME") {
		t.Errorf("sem linhas deveria ter só o cabeçalho: %q", saida)
	}
}

func TestTabela_CelulasFaltantesEColunaUnicaLarga(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	tableLarguraMax(t, Tabela(tableColunas(), []Linha{{Celulas: []string{"só-nome"}}}, 0, 60), 60)
	tableLarguraMax(t, Tabela([]Coluna{{Titulo: "Nome", Min: 30}}, []Linha{{Celulas: []string{"x"}}}, -1, 10), 10)
}

func TestStatus_GlifoCorEPalavra(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	casos := []struct {
		tipo  TipoStatus
		glifo string
		texto string
	}{
		{StatusRodando, "●", "Running"},
		{StatusFalha, "✕", "CrashLoopBackOff"},
		{StatusAlerta, "▲", "Pending"},
		{StatusConcluido, "✓", "Completed"},
		{StatusParado, "○", "Parado"},
	}
	vistos := map[string]bool{}
	for _, c := range casos {
		saida := Status(c.tipo, c.texto)
		if ansi.Strip(saida) != c.glifo+" "+c.texto {
			t.Errorf("Status(%d) = %q", c.tipo, ansi.Strip(saida))
		}
		vistos[saida[:strings.Index(saida, c.glifo)]] = true
	}
	// Ok, Erro, Aviso e TintaSuave: ao menos 4 cores distintas.
	if len(vistos) < 4 {
		t.Errorf("esperava cores distintas por tipo, obtive %d", len(vistos))
	}
	usarTema(t, true, termenv.Ascii, true)
	if got := Status(StatusFalha, "erro"); got != "x erro" {
		t.Errorf("ASCII: %q", got)
	}
}

func TestTabela_NomeMarcadoLevaONegritoNaSelecao(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	colunas := []Coluna{{Titulo: "Cloud", Min: 3}, {Titulo: "Cluster", Min: 7, Nome: true}, {Titulo: "Status", Min: 6}}
	linhas := []Linha{{Celulas: []string{"aws", "staging", "pronto"}}}
	saida := Tabela(colunas, linhas, 0, 40)
	linha := strings.Split(saida, "\n")[1]
	if !strings.Contains(linha, S().Selecionado.Render("staging")) {
		t.Errorf("o nome marcado deveria sair em negrito invertido: %q", linha)
	}
	if strings.Contains(linha, S().Selecionado.Render("aws  ")) || !strings.Contains(linha, S().Selecionado.Bold(false).Render("aws  ")) {
		t.Errorf("a primeira coluna, sem Nome, sai sem negrito: %q", linha)
	}
	// Sem nenhuma coluna marcada, a saída é a de sempre (negrito na primeira).
	colunas[1].Nome = false
	antes := Tabela(colunas, linhas, 0, 40)
	if !strings.HasPrefix(strings.Split(antes, "\n")[1], S().Selecionado.Render("aws  ")) {
		t.Errorf("sem Nome, o negrito fica na primeira coluna: %q", antes)
	}
}

func TestLargurasDasColunas_IgualATabela(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	w := LargurasDasColunas(tableColunas(), tableLinhas(), 50)
	if len(w) != len(tableColunas()) {
		t.Fatalf("uma largura por coluna: %v", w)
	}
	cab := strings.Split(Tabela(tableColunas(), tableLinhas(), -1, 50), "\n")[0]
	for i, c := range tableColunas() {
		if (w[i] == 0) == strings.Contains(cab, strings.ToUpper(c.Titulo)) {
			t.Errorf("coluna %s: largura %d não bate com o cabeçalho %q", c.Titulo, w[i], cab)
		}
	}
	if total := sum(w) + espacoColunas*(contar(w)-1); total > 50 {
		t.Errorf("larguras somam %d > 50", total)
	}
	if got := LargurasDasColunas(nil, nil, 50); len(got) != 0 {
		t.Errorf("sem colunas: %v", got)
	}
}

func sum(v []int) int {
	t := 0
	for _, x := range v {
		t += x
	}
	return t
}

func contar(v []int) int {
	n := 0
	for _, x := range v {
		if x > 0 {
			n++
		}
	}
	return n
}
