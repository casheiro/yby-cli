package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/logger"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// atualizarGolden regrava os arquivos golden: go test ./plugins/atlas/ -run <Teste> -update
var atualizarGolden = flag.Bool("update", false, "regrava os arquivos golden em testdata/")

// dirTestdata é o caminho absoluto de testdata: os testes do diagram mudam a pasta atual.
var dirTestdata string

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	dirTestdata = filepath.Join(wd, "testdata")
	// A home dos testes (e dos binários que eles rodam) é temporária: o
	// ~/.yby/config.yaml de verdade não muda o visual nem o log.
	// Com a home trocada o go não acharia o cache de compilação nem o de módulos
	// e o `go build` do helperBuildAtlas partiria do zero: fixa os dois antes.
	if saida, err := exec.Command("go", "env", "GOCACHE", "GOMODCACHE").Output(); err == nil {
		linhas := strings.Fields(string(saida))
		if len(linhas) == 2 {
			os.Setenv("GOCACHE", linhas[0])
			os.Setenv("GOMODCACHE", linhas[1])
		}
	}
	dir, err := os.MkdirTemp("", "atlas-home-")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", dir)
	os.Setenv("USERPROFILE", dir)
	for _, k := range []string{"YBY_THEME", "YBY_ASCII", "NO_COLOR", "YBY_LOG_LEVEL", "YBY_LOG_FORMAT", "YBY_PLUGIN_REQUEST"} {
		os.Unsetenv(k)
	}
	codigo := m.Run()
	os.RemoveAll(dir)
	if binAtlasDir != "" {
		os.RemoveAll(binAtlasDir)
	}
	os.Exit(codigo)
}

// conferirGolden compara obtido com testdata/<nome>; com -update, regrava o arquivo.
func conferirGolden(t *testing.T, nome, obtido string) {
	t.Helper()
	caminho := filepath.Join(dirTestdata, nome)
	if *atualizarGolden {
		if err := os.MkdirAll(dirTestdata, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(obtido), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	esperado, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("golden %s ausente (rode com -update): %v", caminho, err)
	}
	if string(esperado) != obtido {
		t.Errorf("saída diferente do golden %s\n--- esperado ---\n%s\n--- obtido ---\n%s", caminho, esperado, obtido)
	}
}

// relogioDePasso devolve um relógio que avança 1,5 s a cada leitura: a coluna de tempo
// das etapas entra nas goldens sempre igual.
func relogioDePasso() func() time.Time {
	var mu sync.Mutex
	agora := time.Date(2026, 10, 25, 9, 30, 0, 0, time.UTC)
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		agora = agora.Add(1500 * time.Millisecond)
		return agora
	}
}

// usarVisual fixa o modo visual e o perfil de cor, e os restaura no fim do teste.
func usarVisual(t *testing.T, o ui.Opcoes, perfil termenv.Profile) {
	t.Helper()
	perfilAnterior, opcoesAnteriores := lipgloss.ColorProfile(), ui.Atual()
	escuroAnterior := lipgloss.HasDarkBackground()
	t.Cleanup(func() {
		ui.Configurar(opcoesAnteriores)
		lipgloss.SetColorProfile(perfilAnterior)
		lipgloss.SetHasDarkBackground(escuroAnterior)
	})
	ui.Configurar(o)
	lipgloss.SetColorProfile(perfil)
}

// provedorFalso responde sempre o mesmo texto e conta as chamadas; nunca há IA de verdade.
type provedorFalso struct {
	ai.Provider
	nome     string
	resposta string
	err      error
	chamadas int
	// aviso, se não vazio, é logado com slog.Warn a cada Completion.
	aviso string
}

func (p *provedorFalso) Name() string { return p.nome }
func (p *provedorFalso) Completion(context.Context, string, string) (string, error) {
	p.chamadas++
	if p.aviso != "" {
		slog.Warn(p.aviso)
	}
	return p.resposta, p.err
}

// usarDependencias troca a IA por falsos, fixa o relógio, a largura (64) e a detecção de
// terminal (fora dele), e restaura tudo no fim.
func usarDependencias(t *testing.T, provs ...ai.Provider) {
	t.Helper()
	p, r, te, l := provedoresDeIA, relogio, terminalEm, larguraSaida
	t.Cleanup(func() { provedoresDeIA, relogio, terminalEm, larguraSaida = p, r, te, l })
	provedoresDeIA = func(context.Context) []ai.Provider { return provs }
	relogio = relogioDePasso()
	terminalEm = func(io.Writer) bool { return false }
	larguraSaida = func(io.Writer) int { return larguraPadrao }
}

// projetoComChart cria, numa pasta temporária que vira a atual, um chart com uma
// dependência e um Deployment, como o do teste do binário.
func projetoComChart(t *testing.T) string {
	t.Helper()
	raiz := t.TempDir()
	templates := filepath.Join(raiz, "charts", "api", "templates")
	if err := os.MkdirAll(templates, 0o755); err != nil {
		t.Fatal(err)
	}
	chart := "apiVersion: v2\nname: api\nversion: 1.0.0\ndependencies:\n  - name: redis\n    version: 17.0.0\n    repository: https://charts.bitnami.com/bitnami\n"
	if err := os.WriteFile(filepath.Join(raiz, "charts", "api", "Chart.yaml"), []byte(chart), 0o644); err != nil {
		t.Fatal(err)
	}
	deploy := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api-server\n"
	if err := os.WriteFile(filepath.Join(templates, "deployment.yaml"), []byte(deploy), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(raiz)
	return raiz
}

// capturarLog liga o slog do plugin como o main faz (configurarLog, nível info da home
// isolada) e manda o log para o buffer devolvido em vez do stderr do processo de teste;
// restaura tudo no fim. Assim um slog.Warn no caminho feliz aparece no teste.
func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	anterior := slog.Default()
	var log bytes.Buffer
	configurarLog()
	restaurar := logger.Desviar(&log)
	t.Cleanup(func() {
		restaurar()
		slog.SetDefault(anterior)
	})
	return &log
}

// rodar chama o hook command como o core o chamaria e devolve stdout e stderr.
func rodar(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var saida, erros bytes.Buffer
	err := handleCommand(args, &saida, &erros)
	return saida.String(), erros.String(), err
}

func TestAjuda_Golden(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	saida, _, err := rodar(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "ajuda.golden", saida)

	saida, _, err = rodar(t, "diagram", "--help")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "ajuda-diagram.golden", saida)
}

func TestDiagram_GoldenSemIA(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	prov := &provedorFalso{nome: "falso", resposta: "flowchart TD\n  a --> b"}
	usarDependencias(t, prov)
	raiz := projetoComChart(t)
	log := capturarLog(t)

	saida, erros, err := rodar(t, "diagram", "--detail", "full", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	if erros != "" || log.Len() > 0 {
		t.Errorf("nada no stderr nem no log fora do terminal, obtido %q e %q", erros, log.String())
	}
	conferirGolden(t, "diagram.golden", saida)
	if prov.chamadas != 0 {
		t.Errorf("com --no-ai a IA não é chamada, houve %d chamadas", prov.chamadas)
	}

	// o .mmd continua o mesmo de sempre: o diagrama programático
	mmd, err := os.ReadFile(filepath.Join(raiz, ".yby", "atlas-diagram.mmd"))
	if err != nil {
		t.Fatalf("diagrama não gravado: %v", err)
	}
	conferirGolden(t, "atlas-diagram-full.mmd", string(mmd))
}

func TestDiagram_GoldenComIA(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	falho := &provedorFalso{nome: "ollama", err: errors.New("fora do ar")}
	bom := &provedorFalso{nome: "gemini-cli", resposta: "```mermaid\nflowchart TD\n  api[API] --> redis[(Redis)]\n```"}
	usarDependencias(t, falho, bom)
	raiz := projetoComChart(t)
	log := capturarLog(t)

	saida, erros, err := rodar(t, "diagram")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "diagram-ia.golden", saida)
	// a falha do primeiro provider fica no log de depuração, não na tela
	if erros != "" || log.Len() > 0 {
		t.Errorf("nada no stderr nem no log no nível info, obtido %q e %q", erros, log.String())
	}
	mmd, err := os.ReadFile(filepath.Join(raiz, ".yby", "atlas-diagram.mmd"))
	if err != nil || string(mmd) != "flowchart TD\n  api[API] --> redis[(Redis)]" {
		t.Errorf("o .mmd é o diagrama refinado, obtido %q (%v)", mmd, err)
	}
	if falho.chamadas != 1 || bom.chamadas != 1 {
		t.Errorf("cascata de providers: %d e %d chamadas", falho.chamadas, bom.chamadas)
	}
}

func TestDiagram_IAIndisponivelPula(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	casos := []struct {
		nome   string
		provs  []ai.Provider
		etapa  string
		motivo string
	}{
		{"sem provider", nil, "○ refinar com a IA   sem IA disponível", ""},
		{"resposta sem Mermaid", []ai.Provider{
			&provedorFalso{nome: "ollama", err: errors.New("fora do ar")},
			&provedorFalso{nome: "gemini-cli", resposta: "não sei"},
		}, "○ refinar com a IA   a resposta veio sem diagrama", "ficou o diagrama montado a partir dos arquivos"},
		{"todos falharam", []ai.Provider{
			&provedorFalso{nome: "ollama", err: errors.New("fora do ar")},
		}, "○ refinar com a IA   a IA não respondeu", "ficou o diagrama montado a partir dos arquivos"},
	}
	for _, c := range casos {
		usarDependencias(t, c.provs...)
		projetoComChart(t)
		log := capturarLog(t)
		saida, erros, err := rodar(t, "diagram", "-d", "full")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(saida, c.etapa) || !strings.Contains(saida, c.motivo) {
			t.Errorf("%s: a etapa diz o que houve com a IA (%q):\n%s", c.nome, c.etapa, saida)
		}
		if c.motivo == "" && strings.Contains(saida, "ficou o diagrama") {
			t.Errorf("%s: sem IA, a etapa basta:\n%s", c.nome, saida)
		}
		// o motivo já está na etapa; o detalhe fica no log de depuração
		if erros != "" || log.Len() > 0 {
			t.Errorf("%s: nada no stderr nem no log no nível info, obtido %q e %q", c.nome, erros, log.String())
		}
	}
}

func TestDiagram_AtlasYamlIlegivelAvisaUmaVez(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	usarDependencias(t)
	raiz := projetoComChart(t)
	if err := os.MkdirAll(filepath.Join(raiz, ".yby"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, ".yby", "atlas.yaml"), []byte("ignores: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	log := capturarLog(t)
	saida, erros, err := rodar(t, "diagram", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(saida, "não deu para ler .yby/atlas.yaml"); n != 1 {
		t.Errorf("o aviso do atlas.yaml sai uma vez, nas etapas; obtido %d:\n%s", n, saida)
	}
	if erros != "" || log.Len() > 0 {
		t.Errorf("nada no stderr nem no log no nível info, obtido %q e %q", erros, log.String())
	}
}

func TestDiagram_NoTerminalRedesenhaEDesviaOLog(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	prov := &provedorFalso{nome: "gemini-cli", aviso: "a IA demorou para responder", resposta: "flowchart TD\n  api[API] --> redis[(Redis)]"}
	usarDependencias(t, prov)
	projetoComChart(t)
	log := capturarLog(t)
	terminalEm = func(io.Writer) bool { return true }

	saida, erros, err := rodar(t, "diagram")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "\x1b[1A\r\x1b[J") {
		t.Errorf("no terminal, a etapa atual é redesenhada no lugar:\n%q", saida)
	}
	for _, etapa := range []string{"✓ procurar a infraestrutura", "✓ montar o diagrama", "✓ refinar com a IA", "✓ gravar o diagrama"} {
		if n := strings.Count(saida, etapa); n != 1 {
			t.Errorf("%q sai uma vez só, de vez, obtido %d:\n%q", etapa, n, saida)
		}
	}
	// o log passa pelo escritor das etapas e vai para o stderr, fora do trecho redesenhado
	if !strings.Contains(erros, "WARN") || !strings.Contains(erros, "a IA demorou para responder") {
		t.Errorf("o aviso do log vai para o stderr pelo slog do yby:\n%q", erros)
	}
	if strings.Contains(saida, "demorou") || log.Len() > 0 {
		t.Errorf("o log não cai no meio das etapas nem fora do escritor delas:\n%q\n%q", saida, log.String())
	}
}

func TestDiagram_SemInfra(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	usarDependencias(t)
	t.Chdir(t.TempDir())
	saida, _, err := rodar(t, "diagram")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "nenhuma topologia de infraestrutura neste projeto") || !strings.Contains(saida, "o Atlas procura") {
		t.Errorf("estado vazio fora do formato:\n%s", saida)
	}
	if _, err := os.Stat(filepath.Join(".yby", "atlas-diagram.mmd")); !errors.Is(err, os.ErrNotExist) {
		t.Error("sem infraestrutura, nenhum diagrama é gravado")
	}
}

func TestDiagram_FlagsInvalidas(t *testing.T) {
	usarDependencias(t)
	for _, args := range [][]string{
		{"diagram", "--detail", "tudo"},
		{"diagram", "--detail"},
		{"diagram", "--formato", "c4"},
	} {
		_, _, err := rodar(t, args...)
		var ye *yerrors.YbyError
		if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || ye.GetHint() == "" {
			t.Errorf("%v: esperado ERR_VALIDATION com hint, obtido %v", args, err)
		}
	}
}

func TestSairComErro_FormatoDoUI(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	_, _, err := rodar(t, "inventado")
	var b bytes.Buffer
	if codigo := sairComErro(&b, err); codigo != 1 {
		t.Errorf("código de saída %d, esperado 1", codigo)
	}
	if !strings.Contains(b.String(), "ERR_VALIDATION") || !strings.Contains(b.String(), "faça") {
		t.Errorf("erro fora do formato do ui.WriteError:\n%s", b.String())
	}
}
