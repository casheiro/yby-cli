package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const maxFileSize = 1 << 20 // 1MB
const maxResults = 50

// ScanResult holds the path and content (optional) of a file
type ScanResult struct {
	Path    string
	Content string
}

// palavrasVazias são os termos de uma pergunta que não ajudam a achar código.
var palavrasVazias = map[string]bool{
	"a": true, "o": true, "as": true, "os": true, "um": true, "uma": true, "de": true, "do": true,
	"da": true, "dos": true, "das": true, "em": true, "no": true, "na": true, "nos": true, "nas": true,
	"e": true, "ou": true, "que": true, "com": true, "para": true, "por": true, "se": true,
	"como": true, "qual": true, "quais": true, "funciona": true, "funcionam": true, "sobre": true,
	"the": true, "of": true, "and": true, "how": true, "does": true, "work": true, "works": true,
}

// ignores são as pastas e os arquivos que nunca entram na busca.
var ignores = map[string]bool{
	".git":              true,
	"node_modules":      true,
	"vendor":            true,
	"dist":              true,
	".synapstor":        true,
	"testdata":          true,
	"__snapshots__":     true,
	"go.sum":            true,
	"package-lock.json": true,
}

// Termos separa a pergunta em termos de busca, em minúsculas e sem palavras vazias. Se só
// sobrarem palavras vazias, a pergunta inteira vira o único termo.
func Termos(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	campos := strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.' && r != '/'
	})
	visto := map[string]bool{}
	var termos []string
	for _, c := range campos {
		c = strings.Trim(c, "-./")
		if c == "" || palavrasVazias[c] || visto[c] {
			continue
		}
		visto[c] = true
		termos = append(termos, c)
	}
	if len(termos) == 0 {
		return []string{query}
	}
	return termos
}

// ehTesteOuGolden diz se o arquivo é de teste ou golden, que não explica como o código funciona.
func ehTesteOuGolden(rel string) bool {
	base := strings.ToLower(filepath.Base(rel))
	if strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, ".golden") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
		return true
	}
	return false
}

// ignorado diz se algum trecho do caminho (com barras) cai nas pastas e arquivos ignorados,
// é oculto, ou é um teste/golden.
func ignorado(rel string) bool {
	for _, parte := range strings.Split(rel, "/") {
		if ignores[parte] || strings.HasPrefix(parte, ".") {
			return true
		}
	}
	return ehTesteOuGolden(rel)
}

// listarArquivos devolve os caminhos relativos (com barras) dos arquivos candidatos: os que o
// git conhece (versionados e novos, sem os do .gitignore) quando a pasta está num repositório,
// ou o que o WalkDir encontra.
func listarArquivos(ctx context.Context, root string) ([]string, error) {
	if info, err := os.Stat(root); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, fmt.Errorf("%s não é uma pasta", root)
	}

	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if saida, err := cmd.Output(); err == nil {
		var arquivos []string
		for _, p := range strings.Split(string(saida), "\x00") {
			if p != "" {
				arquivos = append(arquivos, filepath.ToSlash(p))
			}
		}
		return arquivos, nil
	}

	var arquivos []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if ignores[d.Name()] || (strings.HasPrefix(d.Name(), ".") && d.Name() != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr == nil {
			arquivos = append(arquivos, filepath.ToSlash(rel))
		}
		return nil
	})
	return arquivos, err
}

// pontuar dá a nota do arquivo para os termos: o nome do arquivo vale mais que o caminho, que
// vale mais que o conteúdo.
func pontuar(rel, conteudo string, termos []string) int {
	nome := strings.ToLower(filepath.Base(rel))
	caminho := strings.ToLower(rel)
	corpo := strings.ToLower(conteudo)
	total := 0
	for _, t := range termos {
		switch {
		case strings.Contains(nome, t):
			total += 100
		case strings.Contains(caminho, t):
			total += 10
		}
		if strings.Contains(corpo, t) {
			total++
		}
	}
	return total
}

// Scan percorre o diretório e retorna os arquivos que correspondem à pergunta. A pergunta vira
// termos (sem palavras vazias); cada arquivo ganha uma nota (nome > caminho > conteúdo), os
// resultados saem do melhor para o pior e só então são cortados em maxResults. Arquivos
// ignorados pelo git, de teste e goldens não entram. Sem pergunta, vale tudo.
func Scan(ctx context.Context, root string, query string) ([]ScanResult, error) {
	termos := Termos(query)

	caminhos, err := listarArquivos(ctx, root)
	if err != nil {
		return nil, err
	}

	type pontuado struct {
		r    ScanResult
		nota int
	}
	var achados []pontuado
	for _, rel := range caminhos {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if ignorado(rel) {
			continue
		}
		caminho := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(caminho)
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileSize {
			continue
		}
		dados, err := os.ReadFile(caminho)
		if err != nil || !isText(dados) {
			continue
		}
		nota := 0
		if len(termos) > 0 {
			nota = pontuar(rel, string(dados), termos)
			if nota == 0 {
				continue
			}
		}
		achados = append(achados, pontuado{ScanResult{Path: rel, Content: string(dados)}, nota})
		// sem pergunta não há o que ordenar: basta chegar ao limite
		if len(termos) == 0 && len(achados) >= maxResults {
			break
		}
	}

	sort.SliceStable(achados, func(i, j int) bool { return achados[i].nota > achados[j].nota })
	if len(achados) > maxResults {
		achados = achados[:maxResults]
	}
	var results []ScanResult
	for _, a := range achados {
		results = append(results, a.r)
	}
	return results, nil
}

func isText(data []byte) bool {
	// Verificação simples: procurar null bytes
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}
