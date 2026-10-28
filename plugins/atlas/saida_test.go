package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/ai"
	yerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/logger"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/casheiro/yby-cli/v5/plugins/atlas/discovery"
	"github.com/casheiro/yby-cli/v5/plugins/atlas/discovery/analyzers"
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
	// travar faz o Completion esperar o contexto acabar, como uma IA que não responde.
	travar bool
}

func (p *provedorFalso) Name() string { return p.nome }
func (p *provedorFalso) Completion(ctx context.Context, _, _ string) (string, error) {
	p.chamadas++
	if p.aviso != "" {
		slog.Warn(p.aviso)
	}
	if p.travar {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return p.resposta, p.err
}

// refinadoDoChart é um Mermaid válido para o projetoComChart: usa os IDs do rascunho.
const refinadoDoChart = "flowchart TD\n  subgraph app[\"Aplicação\"]\n    HelmChart_api[\"API\"] -->|implanta| Deployment_api_server[\"api-server\"]\n  end\n  HelmChart_api -->|depende de| HelmRepository_redis[(Redis)]"

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
	bom := &provedorFalso{nome: "gemini-cli", resposta: "```mermaid\n" + refinadoDoChart + "\n```"}
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
	if err != nil || string(mmd) != refinadoDoChart {
		t.Errorf("o .mmd é o diagrama refinado, obtido %q (%v)", mmd, err)
	}
	// o programático fica ao lado, para comparar com o da IA
	raw, err := os.ReadFile(filepath.Join(raiz, ".yby", "atlas-diagram.raw.mmd"))
	if err != nil || !strings.Contains(string(raw), "HelmChart_api -->|deploys| Deployment_api_server") {
		t.Errorf("o .raw.mmd é o diagrama programático, obtido %q (%v)", raw, err)
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
		saida, erros, err := rodar(t, "diagram", "-d", "overview")
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
	prov := &provedorFalso{nome: "gemini-cli", aviso: "a IA demorou para responder", resposta: refinadoDoChart}
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

// No overview, acima do teto, a etapa conta os nós desenhados e o resumo diz
// quantos recursos foram agrupados.
func TestDiagram_OverviewComTetoContaOsNosDesenhados(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	usarDependencias(t)
	raiz := t.TempDir()
	var manifesto strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&manifesto, "---\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app-%02d\n", i)
	}
	if err := os.WriteFile(filepath.Join(raiz, "deploys.yaml"), []byte(manifesto.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(raiz)

	saida, _, err := rodar(t, "diagram", "--no-ai")
	if err != nil {
		t.Fatal(err)
	}
	for _, trecho := range []string{"1 nó · 0 relações", "30 de 30 no diagrama · 30 agrupados por tipo"} {
		if !strings.Contains(saida, trecho) {
			t.Errorf("faltou %q na saída:\n%s", trecho, saida)
		}
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

func TestDiagram_FullNaoPassaPelaIA(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	prov := &provedorFalso{nome: "falso", resposta: "flowchart TD\n  a --> b"}
	usarDependencias(t, prov)
	raiz := projetoComChart(t)
	saida, _, err := rodar(t, "diagram", "-d", "full")
	if err != nil {
		t.Fatal(err)
	}
	if prov.chamadas != 0 {
		t.Errorf("o full mostra todos os recursos e não passa pela IA, houve %d chamadas", prov.chamadas)
	}
	if !strings.Contains(saida, "○ refinar com a IA") {
		t.Errorf("a etapa da IA é pulada com o motivo:\n%s", saida)
	}
	mmd, err := os.ReadFile(filepath.Join(raiz, ".yby", "atlas-diagram.mmd"))
	if err != nil || strings.Contains(string(mmd), "a --> b") {
		t.Errorf("o .mmd é o programático, obtido %q (%v)", mmd, err)
	}
}

func TestDiagram_EstatisticasSaoDoQueFoiGravado(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	usarDependencias(t, &provedorFalso{nome: "falso", resposta: "flowchart TD\n  HelmChart_api[API] --> Deployment_api_server[api-server]"})
	projetoComChart(t)
	saida, _, err := rodar(t, "diagram")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "2 nós de 3 recursos") || strings.Contains(saida, "3 de 3") {
		t.Errorf("recursos reflete os 2 nós gravados:\n%s", saida)
	}
	if !strings.Contains(saida, "relações      1") {
		t.Errorf("relações reflete a 1 aresta gravada:\n%s", saida)
	}
}

// Resposta da IA que não é um Mermaid que confere com o rascunho não é gravada: fica o
// programático, e a etapa diz o motivo.
func TestDiagram_MermaidInvalidoDaIAFicaOProgramatico(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	casos := []struct{ nome, resposta, etapa string }{
		{"prosa com a palavra flowchart", "Aqui está o flowchart:\nA-->B", "a resposta veio sem diagrama"},
		{"subgraph sem end", "flowchart TD\n subgraph x\n HelmChart_api[API] --> Deployment_api_server[api]", "o diagrama da IA não confere"},
		{"nós fora do rascunho", "```mermaid\nflowchart TD\n  api[API] --> redis[(Redis)]\n```", "o diagrama da IA não confere"},
		{"aresta para nó não declarado", "flowchart TD\n  HelmChart_api[API] --> Deployment_api_server", "o diagrama da IA não confere"},
	}
	for _, c := range casos {
		usarDependencias(t, &provedorFalso{nome: "gemini-cli", resposta: c.resposta})
		raiz := projetoComChart(t)
		saida, _, err := rodar(t, "diagram")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(saida, "○ refinar com a IA   "+c.etapa) || !strings.Contains(saida, "ficou o diagrama montado a partir dos arquivos") {
			t.Errorf("%s: a etapa diz que ficou o programático (%q):\n%s", c.nome, c.etapa, saida)
		}
		mmd, err := os.ReadFile(filepath.Join(raiz, ".yby", "atlas-diagram.mmd"))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(filepath.Join(raiz, ".yby", "atlas-diagram.raw.mmd"))
		if strings.Contains(string(mmd), "A-->B") || strings.Contains(string(mmd), "redis[(Redis)]") || string(mmd) != string(raw) {
			t.Errorf("%s: o .mmd é o programático, obtido %q", c.nome, mmd)
		}
	}
}

// A IA tem prazo: um provider que não responde não trava o diagram.
func TestDiagram_IAComPrazo(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	travado := &provedorFalso{nome: "ollama", travar: true}
	depois := &provedorFalso{nome: "gemini-cli", resposta: refinadoDoChart}
	usarDependencias(t, travado, depois)
	anterior := prazoDaIA
	t.Cleanup(func() { prazoDaIA = anterior })
	prazoDaIA = 50 * time.Millisecond
	raiz := projetoComChart(t)

	type resultado struct {
		saida string
		err   error
	}
	fim := make(chan resultado, 1)
	go func() {
		saida, _, err := rodar(t, "diagram")
		fim <- resultado{saida, err}
	}()
	select {
	case r := <-fim:
		if r.err != nil {
			t.Fatal(r.err)
		}
		if !strings.Contains(r.saida, "○ refinar com a IA   a IA passou do prazo") || !strings.Contains(r.saida, "ficou o diagrama montado a partir dos arquivos") {
			t.Errorf("a etapa diz que a IA passou do prazo:\n%s", r.saida)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("o diagram travou esperando a IA")
	}
	if depois.chamadas != 0 {
		t.Errorf("depois do prazo, nenhum outro provider é chamado: %d chamadas", depois.chamadas)
	}
	if _, err := os.Stat(filepath.Join(raiz, ".yby", "atlas-diagram.mmd")); err != nil {
		t.Errorf("o programático é gravado: %v", err)
	}
}

func TestResumoDoInventario_GrandeEhResumido(t *testing.T) {
	bp := &discovery.InfraBlueprint{}
	for i := 0; i < 40; i++ {
		bp.Resources = append(bp.Resources, analyzers.InfraResource{Kind: "Deployment", Name: fmt.Sprintf("app-%02d", i), Namespace: "loja"})
	}
	bp.Resources = append(bp.Resources, analyzers.InfraResource{Kind: "Service", Name: "api"})
	resumo := buildResourceSummary(bp)
	if !strings.Contains(resumo, "Deployment (40): loja/app-00") || !strings.Contains(resumo, "e mais 30") {
		t.Errorf("um kind grande mostra os primeiros nomes e a contagem do resto:\n%s", resumo)
	}
	if strings.Contains(resumo, "app-10") {
		t.Errorf("os nomes além do limite não vão no prompt:\n%s", resumo)
	}
	if !strings.Contains(resumo, "Service (1): api\n") {
		t.Errorf("um kind pequeno vai inteiro:\n%s", resumo)
	}
}
