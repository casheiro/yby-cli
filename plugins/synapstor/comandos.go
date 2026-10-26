package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/agent"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/decay"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/exporter"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/indexer"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/quality"
)

// Providers de IA, trocados por falsos nos testes (nunca há IA de verdade neles).
var (
	provedorDeIA         = func(ctx context.Context) ai.Provider { return ai.GetProvider(ctx, "auto") }
	provedorDeEmbeddings = ai.GetEmbeddingProvider
	// analisarDecay é a análise de idade dos UKIs (usa o git do projeto).
	analisarDecay = decay.AnalyzeDecay
)

// pastaDosUKIs é onde ficam os UKIs, relativa à raiz do projeto.
var pastaDosUKIs = filepath.Join(".synapstor", ".uki")

func plural(n int, singular, plur string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plur)
}

// semProvider é o erro de quando nenhum provider de IA responde.
func semProvider() error {
	return yerrors.New(yerrors.ErrCodeConfig, "nenhum provider de IA disponível").
		WithHint("rode ollama serve, exporte OPENAI_API_KEY ou GEMINI_API_KEY, ou defina ai.provider no ~/.yby/config.yaml")
}

// semEmbeddings é o erro de quando nenhum provider gera embeddings.
func semEmbeddings() error {
	return yerrors.New(yerrors.ErrCodeConfig, "nenhum provider de IA com embeddings disponível").
		WithHint("rode ollama serve, ou exporte OPENAI_API_KEY ou GEMINI_API_KEY (os CLIs claude e gemini não geram embeddings)")
}

// semUKIs informa se a pasta dos UKIs não existe ainda.
func semUKIs(raiz string) bool {
	_, err := os.Stat(filepath.Join(raiz, pastaDosUKIs))
	return errors.Is(err, os.ErrNotExist)
}

// escreverSemUKIs é o estado vazio dos comandos que leem os UKIs.
func escreverSemUKIs(w io.Writer) {
	ui.Check(w, ui.CheckNeutro, "nenhum UKI em "+filepath.ToSlash(pastaDosUKIs), "", "")
	ui.Section(w, "próximo passo")
	ui.Passos(w, "yby synapstor capture \"texto\" cria o primeiro UKI")
}

// erroDeLeitura é a falha ao ler os UKIs.
func erroDeLeitura(err error, raiz string) error {
	return yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para ler os UKIs").
		WithContext("pasta", filepath.Join(raiz, pastaDosUKIs))
}

// runCapture estrutura o texto num UKI com a IA.
func runCapture(ctx context.Context, raiz string, args []string, saida, erros io.Writer) error {
	input := strings.TrimSpace(strings.Join(args, " "))
	if input == "" {
		return yerrors.New(yerrors.ErrCodeValidation, "falta o texto a capturar").
			WithHint("use yby synapstor capture \"seu texto\"")
	}
	provider := provedorDeIA(ctx)
	if provider == nil {
		return semProvider()
	}
	ui.Title(saida, "synapstor", "capture")
	fmt.Fprintln(saida)
	return gerarUKI(saida, erros, provider, raiz, func(agt *agent.Agent) (*agent.Resultado, error) {
		return agt.Capture(ctx, input)
	})
}

// runStudy documenta num UKI o código do projeto sobre o tópico.
func runStudy(ctx context.Context, raiz string, args []string, saida, erros io.Writer) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return yerrors.New(yerrors.ErrCodeValidation, "falta o tópico a estudar").
			WithHint("use yby synapstor study \"tópico ou arquivo\"")
	}
	provider := provedorDeIA(ctx)
	if provider == nil {
		return semProvider()
	}
	ui.Title(saida, "synapstor", "study · "+aspas(query))
	fmt.Fprintln(saida)
	err := gerarUKI(saida, erros, provider, raiz, func(agt *agent.Agent) (*agent.Resultado, error) {
		return agt.Study(ctx, query)
	})
	if errors.Is(err, agent.ErrSemArquivos) {
		fmt.Fprintln(saida)
		ui.Check(saida, ui.CheckAviso, "nenhum arquivo do projeto fala de "+aspas(query), "", "")
		ui.Section(saida, "próximo passo")
		ui.Passos(saida, "tente outro termo, ou o nome de um arquivo ou de uma pasta")
		return nil
	}
	return err
}

// gerarUKI roda capture ou study com as etapas na tela e mostra onde o UKI foi gravado.
func gerarUKI(saida, erros io.Writer, provider ai.Provider, raiz string, gerar func(*agent.Agent) (*agent.Resultado, error)) error {
	prog := novoProgresso(saida)
	parar := animar(prog, erros)
	defer parar()

	agt := agent.NewAgent(provider, raiz)
	agt.Etapas = prog
	res, err := gerar(agt)
	parar()
	if err != nil {
		return err
	}

	ui.Section(saida, "uki gravado")
	ui.Campos(saida,
		ui.Campo{Rotulo: "título", Valor: res.Titulo},
		ui.Campo{Rotulo: "arquivo", Valor: filepath.ToSlash(res.Arquivo), Caminho: true},
	)
	ui.Section(saida, "próximo passo")
	ui.Passos(saida, "yby synapstor index põe o UKI na busca e no Bard")
	return nil
}

// runIndex indexa os UKIs com embeddings.
func runIndex(ctx context.Context, raiz string, args []string, saida, erros io.Writer) error {
	fullReindex := false
	for _, a := range args {
		switch {
		case a == "--full":
			fullReindex = true
		case strings.HasPrefix(a, "-"):
			return yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("o index não tem a flag %s", a)).
				WithHint("rode yby synapstor index --help para ver as flags")
		}
	}

	provider := provedorDeEmbeddings(ctx)
	if provider == nil {
		return semEmbeddings()
	}

	modo := "incremental"
	if fullReindex {
		modo = "completo"
	}
	ui.Title(saida, "synapstor", "index · "+modo)
	fmt.Fprintln(saida)
	prog := novoProgresso(saida)
	parar := animar(prog, erros)
	defer parar()

	idx := indexer.NewIndexer(provider, raiz)
	idx.FullReindex = fullReindex
	idx.Etapas = prog

	report, err := idx.Run(ctx)
	parar()
	if errors.Is(err, indexer.ErrIndexEmAndamento) {
		return yerrors.New(yerrors.ErrCodeExec, "outro yby synapstor index está rodando neste projeto").
			WithHint("espere ele terminar e rode de novo")
	}
	if err != nil {
		return yerrors.Wrap(err, yerrors.ErrCodeExec, "a indexação parou").
			WithHint("confira o provider de embeddings com yby doctor; para ver o log, rode com YBY_LOG_LEVEL=debug").
			WithContext("provider", provider.Name())
	}
	escreverIndexacao(saida, report)
	return nil
}

// escreverIndexacao escreve o resumo da indexação: arquivos indexados, removidos, trechos
// gerados e arquivos sem mudança.
func escreverIndexacao(w io.Writer, r *indexer.IndexReport) {
	if r.FilesScanned == 0 && r.FilesRemoved == 0 {
		fmt.Fprintln(w)
		ui.Check(w, ui.CheckNeutro, "nenhum UKI em "+filepath.ToSlash(pastaDosUKIs)+" nem em docs/wiki", "", "")
		ui.Section(w, "próximo passo")
		ui.Passos(w, "yby synapstor capture \"texto\" cria o primeiro UKI")
		return
	}
	indexados := ui.CheckOk
	if r.FilesIndexed == 0 {
		indexados = ui.CheckNeutro
	}
	partes := []ui.ParteResumo{
		{Texto: plural(r.FilesIndexed, "indexado", "indexados"), Tipo: indexados},
		{Texto: plural(r.FilesRemoved, "removido", "removidos"), Tipo: ui.CheckNeutro},
	}
	if r.EmbeddingsCreated > 0 {
		partes = append(partes, ui.ParteResumo{Texto: plural(r.EmbeddingsCreated, "trecho", "trechos"), Tipo: ui.CheckNeutro})
	}
	if r.FilesSkipped > 0 {
		partes = append(partes, ui.ParteResumo{Texto: plural(r.FilesSkipped, "sem mudança", "sem mudança"), Tipo: ui.CheckNeutro})
	}
	ui.SummaryCom(w, partes, "")
	ui.Campos(w, ui.Campo{Rotulo: "índice", Valor: filepath.ToSlash(filepath.Join(".synapstor", ".index")), Caminho: true})
}

// faixaDaNota classifica a nota de um UKI: 70 ou mais é bom, abaixo de 40 é fraco.
func faixaDaNota(nota int) ui.TipoCheck {
	switch {
	case nota >= 70:
		return ui.CheckOk
	case nota >= 40:
		return ui.CheckAviso
	default:
		return ui.CheckErro
	}
}

// listaComE junta itens como "a, b e c".
func listaComE(itens []string) string {
	switch len(itens) {
	case 0:
		return ""
	case 1:
		return itens[0]
	}
	return strings.Join(itens[:len(itens)-1], ", ") + " e " + itens[len(itens)-1]
}

// oQueFalta descreve o que tirou pontos do UKI, como "sem exemplos e links · texto curto".
func oQueFalta(b quality.QualityBreakdown) string {
	var sem []string
	if !b.HasContext {
		sem = append(sem, "contexto")
	}
	if !b.HasExamples {
		sem = append(sem, "exemplos")
	}
	if !b.HasHeaders {
		sem = append(sem, "seções")
	}
	if b.LinkCount == 0 {
		sem = append(sem, "links")
	}
	if !b.HasMetadata {
		sem = append(sem, "metadados")
	}
	var partes []string
	if len(sem) > 0 {
		partes = append(partes, "sem "+listaComE(sem))
	}
	if b.WordCount <= 100 {
		partes = append(partes, "texto curto")
	}
	return strings.Join(partes, " · ")
}

// runQuality dá a nota de cada UKI.
func runQuality(raiz string, saida io.Writer) error {
	if semUKIs(raiz) {
		ui.Title(saida, "synapstor", "quality")
		fmt.Fprintln(saida)
		escreverSemUKIs(saida)
		return nil
	}
	scores, err := quality.ScoreAll(filepath.Join(raiz, pastaDosUKIs))
	if err != nil {
		return erroDeLeitura(err, raiz)
	}
	escreverQualidade(saida, scores)
	return nil
}

// escreverQualidade escreve a nota de cada UKI, com o glifo da faixa e o que faltou, e o
// resumo por faixa com a média.
func escreverQualidade(w io.Writer, scores []quality.QualityScore) {
	ui.Title(w, "synapstor", "quality · "+plural(len(scores), "UKI", "UKIs"))
	if len(scores) == 0 {
		fmt.Fprintln(w)
		escreverSemUKIs(w)
		return
	}
	nomes := make([]string, len(scores))
	for i, s := range scores {
		nomes[i] = s.Title
		if nomes[i] == "" {
			nomes[i] = filepath.Base(s.Path)
		}
	}
	col := ui.ColunasPara(nomes...)
	col.Versao = 9
	ui.Section(w, "notas")
	var bons, medios, fracos, soma int
	for i, s := range scores {
		tipo := faixaDaNota(s.Score)
		switch tipo {
		case ui.CheckOk:
			bons++
		case ui.CheckAviso:
			medios++
		default:
			fracos++
		}
		soma += s.Score
		ui.CheckCom(w, col, tipo, nomes[i], fmt.Sprintf("%d/100", s.Score), oQueFalta(s.Breakdown))
	}
	ui.SummaryCom(w, []ui.ParteResumo{
		{Texto: plural(bons, "bom", "bons"), Tipo: ui.CheckOk},
		{Texto: plural(medios, "a melhorar", "a melhorar"), Tipo: ui.CheckAviso},
		{Texto: plural(fracos, "fraco", "fracos"), Tipo: ui.CheckErro},
	}, fmt.Sprintf("média %d/100", soma/len(scores)))
	ui.Campos(w, ui.Campo{Rotulo: "pasta", Valor: filepath.ToSlash(pastaDosUKIs), Caminho: true})
}

// runDecay aponta os UKIs sem atividade recente no git.
func runDecay(raiz string, saida io.Writer) error {
	if semUKIs(raiz) {
		ui.Title(saida, "synapstor", "decay")
		fmt.Fprintln(saida)
		escreverSemUKIs(saida)
		return nil
	}
	infos, err := analisarDecay(filepath.Join(raiz, pastaDosUKIs), raiz)
	if err != nil {
		return erroDeLeitura(err, raiz)
	}
	escreverDecay(saida, infos)
	return nil
}

// escreverDecay escreve a idade de cada UKI (▲ nos desatualizados) e o resumo.
func escreverDecay(w io.Writer, infos []decay.DecayInfo) {
	ui.Title(w, "synapstor", fmt.Sprintf("decay · limite de %d dias", decay.StaleThresholdDays))
	if len(infos) == 0 {
		fmt.Fprintln(w)
		escreverSemUKIs(w)
		return
	}
	nomes := make([]string, len(infos))
	for i, info := range infos {
		nomes[i] = info.Title
		if nomes[i] == "" {
			nomes[i] = filepath.Base(info.Path)
		}
	}
	col := ui.ColunasPara(nomes...)
	ui.Section(w, "atividade no git")
	desatualizados := 0
	for i, info := range infos {
		idade := plural(info.DaysSinceActivity, "dia", "dias")
		if info.IsStale {
			desatualizados++
			ui.CheckCom(w, col, ui.CheckAviso, nomes[i], idade, "pode estar desatualizado")
			continue
		}
		ui.CheckCom(w, col, ui.CheckOk, nomes[i], idade, "")
	}
	ui.SummaryCom(w, []ui.ParteResumo{
		{Texto: plural(len(infos)-desatualizados, "em dia", "em dia"), Tipo: ui.CheckOk},
		{Texto: plural(desatualizados, "desatualizado", "desatualizados"), Tipo: ui.CheckAviso},
	}, "")
}

// runExport exporta os UKIs no formato pedido.
func runExport(raiz string, args []string, saida, erros io.Writer) error {
	format := "markdown"
	outputDir := ""
	ajuda := "rode yby synapstor export --help para ver as flags"
	for i := 0; i < len(args); i++ {
		a, valor, comIgual := args[i], "", false
		if nome, v, ok := strings.Cut(a, "="); ok && strings.HasPrefix(a, "--") {
			a, valor, comIgual = nome, v, true
		}
		var destino *string
		switch a {
		case "--format":
			destino = &format
		case "--output":
			destino = &outputDir
		default:
			return yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("o export não reconhece %s", args[i])).WithHint(ajuda)
		}
		if !comIgual {
			if i+1 >= len(args) {
				return yerrors.New(yerrors.ErrCodeValidation, "falta o valor de "+a).WithHint(ajuda)
			}
			valor = args[i+1]
			i++
		}
		if valor == "" {
			return yerrors.New(yerrors.ErrCodeValidation, "falta o valor de "+a).WithHint(ajuda)
		}
		*destino = valor
	}
	format = strings.ToLower(format)
	exp, err := exporter.NewExporter(format)
	if err != nil {
		return yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("formato de exportação desconhecido: %s", format)).
			WithHint("use --format docusaurus, obsidian ou markdown")
	}

	mostrado := outputDir
	if outputDir == "" {
		mostrado = filepath.Join(".synapstor", "export", format)
		outputDir = filepath.Join(raiz, mostrado)
	} else if !filepath.IsAbs(outputDir) {
		outputDir = filepath.Join(raiz, outputDir)
	}

	ui.Title(saida, "synapstor", "export · "+format)
	fmt.Fprintln(saida)
	if semUKIs(raiz) {
		escreverSemUKIs(saida)
		return nil
	}

	prog := novoProgresso(saida)
	parar := animar(prog, erros)
	defer parar()

	prog.Iniciar("carregar os UKIs")
	ukis, err := exporter.LoadUKIs(filepath.Join(raiz, pastaDosUKIs))
	if err != nil {
		prog.Falhar("")
		return erroDeLeitura(err, raiz)
	}
	if len(ukis) == 0 {
		prog.Concluir("nenhum UKI")
		parar()
		fmt.Fprintln(saida)
		escreverSemUKIs(saida)
		return nil
	}
	prog.Concluir(plural(len(ukis), "UKI", "UKIs"))

	prog.Iniciar("exportar")
	if err := exp.Export(ukis, outputDir); err != nil {
		prog.Falhar("")
		return yerrors.Wrap(err, yerrors.ErrCodeIO, "a exportação falhou").
			WithContext("destino", outputDir)
	}
	prog.Concluir(format)
	parar()

	ui.Section(saida, "exportação")
	ui.Campos(saida,
		ui.Campo{Rotulo: "formato", Valor: format},
		ui.Campo{Rotulo: "destino", Valor: filepath.ToSlash(mostrado), Caminho: true},
		ui.Campo{Rotulo: "UKIs", Valor: fmt.Sprintf("%d", len(ukis))},
	)
	return nil
}
