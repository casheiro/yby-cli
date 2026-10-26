// Package guarda impede que o visual volte a ser montado à mão fora do pkg/ui: varre os
// arquivos .go do repositório inteiro (inclusive os módulos próprios, como plugins/viz e
// plugins/synapstor) atrás de emojis de status em textos e de cores do lipgloss montadas à mão
// (lipgloss.Color, AdaptiveColor, CompleteColor, CompleteAdaptiveColor e ANSIColor em chamada,
// conversão, literal composto ou var com valor, com apelido ou com ponto no import, no v1 e no
// v2). A varredura é sintática: uma string solta passada a um parâmetro do tipo lipgloss.Color
// vira cor sem que a guarda veja; para isso seria preciso checar tipos.
//
// O teste do pacote roda a varredura na raiz do repositório, então ela entra no task test e
// no pipeline (.github/workflows/feature-pipeline.yaml) sem passo extra.
//
// Exceções, todas de propósito:
//   - pkg/ui/: é o dono da paleta e dos glifos (e os testes de largura usam emojis);
//   - arquivos *_test.go: vários testes listam os emojis antigos para conferir que sumiram;
//   - pastas testdata/: goldens e fixtures;
//   - vendor/ e pastas ocultas (.git, .yby, .claude): código que não é nosso ou não é código;
//   - comentários: documentação pode citar um glifo ou mostrar o que não fazer;
//   - os emojis de permitidos, um por um, com o motivo ao lado.
//
// Templates de scaffold (.tmpl) e a documentação (.md) não são .go e ficam de fora.
package guarda

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// caminhosLipgloss são os imports do lipgloss (sem o sufixo de versão /vN) cujas cores só o
// pkg/ui pode montar: o v1 e os caminhos do v2.
var caminhosLipgloss = map[string]bool{
	"github.com/charmbracelet/lipgloss": true,
	"charm.land/lipgloss":               true,
}

// tiposDeCor são os nomes do lipgloss que montam uma cor. Acusados em chamada, conversão,
// literal composto, var com valor e alias; só o tipo de um campo, parâmetro ou var sem valor
// passa (posicoesSoDeTipo).
var tiposDeCor = map[string]bool{
	"Color": true, "AdaptiveColor": true, "CompleteColor": true, "CompleteAdaptiveColor": true, "ANSIColor": true,
}

const usarNoLugarDaCor = "use os tokens de cor do pkg/ui (ui.Tinta, ui.TintaSuave, ui.Ok, ui.Aviso, ui.Erro) ou um componente pronto"

// ehLipgloss diz se caminho é o pacote raiz do lipgloss, em qualquer versão. Subpacotes
// (lipgloss/table, lipgloss/list) não contam: não montam cor.
func ehLipgloss(caminho string) bool {
	if i := strings.LastIndex(caminho, "/v"); i > 0 {
		if _, err := strconv.Atoi(caminho[i+2:]); err == nil {
			caminho = caminho[:i]
		}
	}
	return caminhosLipgloss[caminho]
}

// permitido é um emoji aceito num arquivo só, porque não é saída de terminal.
type permitido struct {
	arquivo, emoji, motivo string
}

// permitidos é a lista curta de exceções pontuais. Cada entrada vale só para aquele emoji
// naquele arquivo; qualquer outro emoji no mesmo arquivo continua acusado. Hoje vazia: o
// único caso, o título do CLI-Reference.md da wiki, saiu na T035.
var permitidos []permitido

// Achado é um ponto do código que a guarda recusa.
type Achado struct {
	Arquivo string // caminho relativo à raiz varrida, com barras normais
	Linha   int
	Motivo  string // o que foi encontrado e o que usar no lugar
}

// String devolve "arquivo:linha: motivo", o formato das mensagens de compilador.
func (a Achado) String() string {
	return fmt.Sprintf("%s:%d: %s", a.Arquivo, a.Linha, a.Motivo)
}

// Varrer percorre os .go a partir de raiz e devolve os achados em ordem de arquivo e linha.
func Varrer(raiz string) ([]Achado, error) {
	achados, _, err := varrer(raiz)
	return achados, err
}

// varrer é o Varrer que também devolve os arquivos conferidos (relativos à raiz, com barras
// normais), para o teste provar que a varredura chega aos módulos próprios.
func varrer(raiz string) (achados []Achado, conferidos []string, err error) {
	err = filepath.WalkDir(raiz, func(caminho string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(raiz, caminho)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && pastaIgnorada(rel, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		doArquivo, err := varrerArquivo(caminho, rel)
		if err != nil {
			return err
		}
		achados = append(achados, doArquivo...)
		conferidos = append(conferidos, rel)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.SliceStable(achados, func(i, j int) bool {
		if achados[i].Arquivo != achados[j].Arquivo {
			return achados[i].Arquivo < achados[j].Arquivo
		}
		return achados[i].Linha < achados[j].Linha
	})
	return achados, conferidos, nil
}

// pastaIgnorada diz se a pasta (rel, relativa à raiz) fica fora da varredura.
func pastaIgnorada(rel, nome string) bool {
	return rel == "pkg/ui" || nome == "testdata" || nome == "vendor" || nome == "node_modules" ||
		strings.HasPrefix(nome, ".")
}

// varrerArquivo confere um arquivo: cores do lipgloss montadas à mão e emojis de status
// nos textos literais. O parser ignora build tags, então os arquivos k8s/aws/azure/gcp entram.
func varrerArquivo(caminho, rel string) ([]Achado, error) {
	fonte, err := os.ReadFile(caminho)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	arq, err := parser.ParseFile(fset, caminho, fonte, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("guarda: não consegui ler %s: %w", rel, err)
	}
	var achados []Achado
	nomesLipgloss := map[string]bool{}
	for _, imp := range arq.Imports {
		if c, _ := strconv.Unquote(imp.Path.Value); !ehLipgloss(c) {
			continue
		}
		switch {
		case imp.Name == nil:
			nomesLipgloss["lipgloss"] = true
		case imp.Name.Name == ".":
			// Com ponto, Color("#fff") vira um nome solto que a guarda não distingue de um
			// local; por isso o próprio import é recusado.
			achados = append(achados, Achado{
				Arquivo: rel,
				Linha:   fset.Position(imp.Pos()).Line,
				Motivo:  "import com ponto do lipgloss fora do pkg/ui esconde as cores montadas à mão; importe com nome e " + usarNoLugarDaCor,
			})
		case imp.Name.Name != "_":
			nomesLipgloss[imp.Name.Name] = true
		}
	}

	soTipo := posicoesSoDeTipo(arq)
	ast.Inspect(arq, func(n ast.Node) bool {
		switch no := n.(type) {
		case *ast.SelectorExpr:
			id, ok := no.X.(*ast.Ident)
			if !ok || !nomesLipgloss[id.Name] || !tiposDeCor[no.Sel.Name] || soTipo[no] {
				return true
			}
			achados = append(achados, Achado{
				Arquivo: rel,
				Linha:   fset.Position(no.Pos()).Line,
				Motivo:  fmt.Sprintf("lipgloss.%s fora do pkg/ui monta cor à mão; %s", no.Sel.Name, usarNoLugarDaCor),
			})
		case *ast.BasicLit:
			if no.Kind != token.STRING && no.Kind != token.CHAR {
				return true
			}
			// Sem as aspas e com os escapes resolvidos, "✅" também conta.
			texto := no.Value
			if v, err := strconv.Unquote(no.Value); err == nil {
				texto = v
			}
			if e := primeiroEmoji(texto, emojisPermitidos(rel)); e != "" {
				achados = append(achados, Achado{
					Arquivo: rel,
					Linha:   fset.Position(no.Pos()).Line,
					Motivo:  fmt.Sprintf("emoji de status %q num texto; use as saídas do pkg/ui (ui.Check, ui.Steps, ui.RenderError) ou os glifos de ui.G() (Feito ✓, Falha ✕, Alerta ▲, Andamento ◈)", e),
				})
			}
		}
		return true
	})
	return achados, nil
}

// posicoesSoDeTipo marca os seletores que só dão o tipo de algo sem montar cor: campos de
// struct, parâmetros e resultados (o catálogo guarda os tokens do pkg/ui num campo
// lipgloss.CompleteAdaptiveColor), var sem valor e asserção de tipo. Conversão, chamada,
// literal composto, var com valor e alias de tipo continuam acusados.
func posicoesSoDeTipo(arq *ast.File) map[*ast.SelectorExpr]bool {
	marcados := map[*ast.SelectorExpr]bool{}
	marcar := func(tipo ast.Expr) {
		if tipo == nil {
			return
		}
		ast.Inspect(tipo, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				marcados[sel] = true
			}
			return true
		})
	}
	ast.Inspect(arq, func(n ast.Node) bool {
		switch no := n.(type) {
		case *ast.Field:
			marcar(no.Type)
		case *ast.ValueSpec:
			if len(no.Values) == 0 {
				marcar(no.Type)
			}
		case *ast.TypeAssertExpr:
			marcar(no.Type)
		}
		return true
	})
	return marcados
}

// emojisPermitidos devolve os emojis que permitidos aceita no arquivo rel.
func emojisPermitidos(rel string) map[string]bool {
	aceitos := map[string]bool{}
	for _, p := range permitidos {
		if p.arquivo == rel {
			aceitos[p.emoji] = true
		}
	}
	return aceitos
}

// primeiroEmoji devolve o primeiro emoji do texto (com o seletor de variação, se vier
// logo depois) que não esteja em aceitos, ou "" se não houver.
func primeiroEmoji(s string, aceitos map[string]bool) string {
	rs := []rune(s)
	for i, r := range rs {
		if !ehEmoji(r) {
			continue
		}
		e := string(r)
		switch {
		case r == 0xFE0F && i > 0:
			e = string(rs[i-1 : i+1])
		case i+1 < len(rs) && rs[i+1] == 0xFE0F:
			e = string(rs[i : i+2])
		}
		if !aceitos[e] && !aceitos[string(r)] {
			return e
		}
	}
	return ""
}

// ehEmoji diz se r é um emoji (ou um símbolo de status que faz as vezes de um). Os glifos do
// pkg/ui ficam de fora: ✓ (U+2713) e ✕ (U+2715) estão no bloco Dingbats, os demais (◆ ◇ ◈
// ▲ ● ○ ▸ ━ ┄ ─ › … —) em blocos que a guarda não olha.
func ehEmoji(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF: // pictogramas, emoticons, transporte, bandeiras
		return true
	case r >= 0x2600 && r <= 0x26FF: // símbolos diversos: ⚠ ⚙ ⛔ ☑ ⚡
		return true
	case r >= 0x2700 && r <= 0x27BF: // dingbats: ✅ ❌ ✨ ✔ ✖ ❗ ❓ ➡
		return r != 0x2713 && r != 0x2715
	case r == 0x231A, r == 0x231B, r >= 0x23E9 && r <= 0x23FA: // ⌚ ⌛ ⏩ ⏳ ⏸
		return true
	case r == 0x2139, r == 0x203C, r == 0x2049: // ℹ ‼ ⁉
		return true
	case r >= 0x2B05 && r <= 0x2B07, r == 0x2B1B, r == 0x2B1C, r == 0x2B50, r == 0x2B55: // ⬅ ⬛ ⭐ ⭕
		return true
	case r == 0xFE0F: // seletor de apresentação emoji
		return true
	}
	return false
}
