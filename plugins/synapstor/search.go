package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
)

const defaultTopK = 5

// minScore é a pontuação mínima de um resultado da busca; os abaixo dela ficam de fora.
const minScore = 0.35

// searchResult representa um resultado de busca formatável.
type searchResult struct {
	Score   float32
	Title   string
	Path    string
	Preview string
}

// erroTopK é o erro de um --top-k sem número válido.
func erroTopK(valor string) error {
	return yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("o --top-k precisa de um número maior que zero, veio %q", valor)).
		WithHint("use, por exemplo, yby synapstor search \"consulta\" --top-k 10")
}

// parseSearchArgs extrai a query e o valor de --top-k dos argumentos.
// Retorna a query concatenada e o topK (padrão: 5).
func parseSearchArgs(args []string) (string, int, error) {
	topK := defaultTopK
	var queryParts []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--top-k" {
			if i+1 >= len(args) {
				return "", 0, erroTopK("")
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil || v <= 0 {
				return "", 0, erroTopK(args[i+1])
			}
			topK = v
			i++ // pular o valor
			continue
		}
		// formato inline --top-k=N
		if val, ok := strings.CutPrefix(args[i], "--top-k="); ok {
			v, err := strconv.Atoi(val)
			if err != nil || v <= 0 {
				return "", 0, erroTopK(val)
			}
			topK = v
			continue
		}
		queryParts = append(queryParts, args[i])
	}

	return strings.Join(queryParts, " "), topK, nil
}

// indiceVazio é o erro da busca sem índice.
func indiceVazio() error {
	return yerrors.New(yerrors.ErrCodeValidation, "o índice do Synapstor está vazio").
		WithHint("rode yby synapstor index para indexar os UKIs")
}

// runSearch busca a consulta no índice e escreve os resultados em saida.
func runSearch(ctx context.Context, raiz string, args []string, saida, erros io.Writer) error {
	query, topK, err := parseSearchArgs(args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(query) == "" {
		return yerrors.New(yerrors.ErrCodeValidation, "falta a consulta").
			WithHint("use yby synapstor search \"sua consulta\" [--top-k N]")
	}

	provider := provedorDeEmbeddings(ctx)
	if provider == nil {
		return semEmbeddings()
	}

	ui.Title(saida, "synapstor", "search · "+aspas(query))
	fmt.Fprintln(saida)
	prog := novoProgresso(saida)
	parar := animar(prog, erros)
	defer parar()

	prog.Iniciar("buscar no índice")
	storePath := filepath.Join(raiz, ".synapstor", ".index")
	if _, err := os.Stat(storePath); err != nil {
		prog.Falhar("")
		return indiceVazio()
	}
	vs, err := ai.NewVectorStore(ctx, storePath, provider)
	if err != nil {
		prog.Falhar("")
		return yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para abrir o índice do Synapstor").
			WithHint("rode yby synapstor index --full para refazer o índice").
			WithContext("pasta", storePath)
	}
	total := vs.Count()
	if total == 0 {
		prog.Falhar("")
		return indiceVazio()
	}

	docs, err := vs.Search(ctx, query, min(topK, total))
	var outro *ai.IndiceDeOutroModeloError
	if errors.As(err, &outro) {
		prog.Falhar("")
		return yerrors.New(yerrors.ErrCodeValidation, outro.Diferenca()).
			WithHint("rode yby synapstor index --full para refazer o índice com " + outro.Atual.String())
	}
	if err != nil {
		prog.Falhar("")
		return yerrors.Wrap(err, yerrors.ErrCodeExec, "a busca no índice falhou").
			WithHint("confira o provider de embeddings com yby doctor; para ver o log, rode com YBY_LOG_LEVEL=debug").
			WithContext("provider", provider.Name())
	}

	// Filtrar resultados com score mínimo de relevância
	var results []searchResult
	for _, doc := range docs {
		if doc.Score < minScore {
			continue
		}

		title := doc.Metadata["title"]
		if title == "" {
			title = doc.Metadata["filename"]
		}
		if title == "" {
			title = doc.ID
		}
		caminho := doc.Metadata["source"]
		if caminho == "" {
			caminho = doc.ID
		}

		results = append(results, searchResult{
			Score:   doc.Score,
			Title:   title,
			Path:    caminho,
			Preview: truncatePreview(doc.Content, 200),
		})
	}
	prog.Concluir(plural(len(docs), "parecido", "parecidos"))
	parar()

	escreverResultados(saida, results, len(docs)-len(results))
	return nil
}

// escreverResultados escreve a seção RESULTADOS e o resumo da busca: cada resultado em
// três linhas (pontuação e título, caminho e o começo do texto), e fora quantos ficaram
// abaixo de minScore.
func escreverResultados(w io.Writer, results []searchResult, foraDoCorte int) {
	corte := pontuacao(minScore)
	if len(results) == 0 {
		fmt.Fprintln(w)
		ui.Check(w, ui.CheckNeutro, "nenhum UKI parecido com a consulta", "", "")
		frase := ""
		if foraDoCorte > 0 {
			frase = plural(foraDoCorte, "abaixo de "+corte+" ficou de fora", "abaixo de "+corte+" ficaram de fora")
		}
		ui.SummaryCom(w, []ui.ParteResumo{{Texto: "0 resultados", Tipo: ui.CheckNeutro}}, frase)
		return
	}

	s := ui.S()
	// O título, o caminho e o trecho ficam depois de 8 colunas (o recuo e a pontuação) e
	// são cortados para caber na largura.
	const recuo = "        "
	texto := max(larguraSaida(w)-len(recuo), 8)
	ui.Section(w, "resultados")
	for i, r := range results {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, "  "+s.Ok.Render(pontuacao(r.Score))+"  "+s.Tinta.Render(caber(r.Title, texto)))
		fmt.Fprintln(w, recuo+s.TintaSuave.Render(caberPeloFim(filepath.ToSlash(r.Path), texto)))
		if r.Preview == "" {
			continue
		}
		// No máximo duas linhas; o que sobra vira reticências. Uma palavra mais larga que a
		// linha, ou uma largura estreita demais para quebrar, também é cortada.
		linhas := ui.Quebrar(r.Preview, texto)
		if len(linhas) > 2 {
			linhas = linhas[:2]
			linhas[1] = cortar(linhas[1], texto-1) + ui.G().Reticencias
		}
		for _, l := range linhas {
			fmt.Fprintln(w, recuo+s.TintaSuave.Render(caber(l, texto)))
		}
	}
	frase := ""
	if foraDoCorte > 0 {
		frase = plural(foraDoCorte, "abaixo de "+corte+" ficou de fora", "abaixo de "+corte+" ficaram de fora")
	}
	ui.SummaryCom(w, []ui.ParteResumo{{Texto: plural(len(results), "resultado", "resultados"), Tipo: ui.CheckOk}}, frase)
}

// pontuacao formata a pontuação com duas casas e vírgula, como "0,82".
func pontuacao(v float32) string {
	return strings.Replace(fmt.Sprintf("%.2f", v), ".", ",", 1)
}

// truncatePreview corta o conteúdo no limite de caracteres, adicionando "…" se truncado.
func truncatePreview(content string, maxLen int) string {
	// Remover frontmatter YAML (--- ... ---)
	if strings.HasPrefix(content, "---\n") {
		if end := strings.Index(content[4:], "\n---"); end >= 0 {
			content = strings.TrimSpace(content[end+8:])
		}
	}

	// O título (o primeiro # ) já aparece na linha do resultado; das outras linhas saem as
	// marcas de markdown (#, **, ` e as cercas de código), para o trecho ler como texto.
	var linhas []string
	titulo := true
	for _, l := range strings.Split(content, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "```") {
			continue
		}
		if titulo && strings.HasPrefix(l, "# ") {
			titulo = false
			continue
		}
		titulo = false
		l = strings.TrimSpace(strings.TrimLeft(l, "#"))
		l = strings.NewReplacer("**", "", "`", "").Replace(l)
		linhas = append(linhas, l)
	}

	// Remover quebras de linha e espaços repetidos para preview compacto
	content = strings.Join(strings.Fields(strings.Join(linhas, " ")), " ")

	if len([]rune(content)) <= maxLen {
		return content
	}
	return cortar(content, maxLen) + ui.G().Reticencias
}

// cortar devolve no máximo n runas de s, sem espaço sobrando no fim.
func cortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimRight(string(r[:max(n, 0)]), " ")
}

// caber corta s em n runas, com reticências no fim quando cortado.
func caber(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return cortar(s, n-1) + ui.G().Reticencias
}

// caberPeloFim corta o começo de s para caber em n runas, com reticências na frente:
// num caminho, o nome do arquivo é o que mais importa.
func caberPeloFim(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return ui.G().Reticencias + strings.TrimLeft(string(r[len(r)-max(n-1, 0):]), " ")
}

// aspas põe a consulta entre aspas, cortada em 32 runas para caber no título.
func aspas(s string) string {
	if len([]rune(s)) > 32 {
		s = cortar(s, 31) + ui.G().Reticencias
	}
	return "\"" + s + "\""
}
