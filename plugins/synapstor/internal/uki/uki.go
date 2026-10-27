// Package uki lê e monta os UKIs no formato da especificação (.synapstor/UKI_SPEC.md):
// a lista dos arquivos, com as subpastas de domínio, e os metadados do frontmatter, que
// também saem dos formatos antigos (o frontmatter em inglês e as linhas em negrito).
package uki

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Os tipos de UKI da especificação.
const (
	TipoRegra      = "regra"
	TipoDecisao    = "decisao"
	TipoPadrao     = "padrao"
	TipoMetrica    = "metrica"
	TipoReferencia = "referencia"
)

// Os status de UKI da especificação: nasce rascunho, vira ativo quando validado e
// obsoleto quando deixa de valer.
const (
	StatusRascunho = "rascunho"
	StatusAtivo    = "ativo"
	StatusObsoleto = "obsoleto"
)

// Tipos são os tipos da especificação, na ordem em que ela os apresenta.
var Tipos = []string{TipoRegra, TipoDecisao, TipoPadrao, TipoMetrica, TipoReferencia}

// sinonimos leva os nomes antigos (em inglês e os do capture antigo) aos da especificação.
var sinonimos = map[string]string{
	"rule": TipoRegra, "decision": TipoDecisao, "adr": TipoDecisao, "pattern": TipoPadrao,
	"metric": TipoMetrica, "reference": TipoReferencia,
	"active": StatusAtivo, "proposal": StatusRascunho, "proposta": StatusRascunho, "draft": StatusRascunho,
	"deprecated": StatusObsoleto,
}

// Metadados são os campos do frontmatter de um UKI e o título do corpo. Tipo e Status
// vêm normalizados para os valores da especificação quando são conhecidos.
type Metadados struct {
	ID           string
	Tipo         string
	Status       string
	CriadoEm     time.Time
	AtualizadoEm time.Time
	Tags         []string
	// Titulo é o texto do primeiro H1 do corpo.
	Titulo string
	// Frontmatter indica que o UKI começa por um bloco ---, mesmo com o YAML quebrado.
	Frontmatter bool
}

// Completos indica que o UKI tem ID, tipo e status.
func (m Metadados) Completos() bool {
	return m.ID != "" && m.Tipo != "" && m.Status != ""
}

// Arquivo é um UKI encontrado por Listar.
type Arquivo struct {
	// Caminho é o caminho do arquivo, a partir da pasta passada a Listar.
	Caminho string
	// Relativo é o caminho dentro da pasta dos UKIs, com /.
	Relativo string
}

// Listar devolve os .md da pasta dos UKIs e das subpastas de domínio, em ordem
// alfabética. As pastas ocultas (como a .obsidian) ficam de fora; a pasta que não dá
// para ler é erro.
func Listar(pasta string) ([]Arquivo, error) {
	var arquivos []Arquivo
	err := filepath.WalkDir(pasta, func(caminho string, d fs.DirEntry, err error) error {
		if err != nil {
			if caminho == pasta {
				return err
			}
			return nil
		}
		if d.IsDir() {
			if caminho != pasta && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(pasta, caminho)
		if err != nil {
			return nil
		}
		arquivos = append(arquivos, Arquivo{Caminho: caminho, Relativo: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao ler diretório de UKIs: %w", err)
	}
	return arquivos, nil
}

var (
	reTitulo  = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	reNegrito = regexp.MustCompile(`(?mi)^\s*\*\*\s*(id|tipo|type|status|tags)\s*(?::\s*\*\*|\*\*\s*:)[ \t]*(.*?)\s*$`)
)

// Ler separa o frontmatter do corpo e devolve os metadados. Sem frontmatter, os
// metadados saem das linhas em negrito do formato antigo (**ID:**, **Tipo:**/**Type:**,
// **Status:**, **Tags:**), que continuam no corpo.
func Ler(conteudo string) (Metadados, string) {
	var meta Metadados
	corpo := conteudo
	if bloco, resto, ok := separarFrontmatter(conteudo); ok {
		meta.Frontmatter = true
		corpo = resto
		var campos map[string]any
		if yaml.Unmarshal([]byte(bloco), &campos) == nil {
			meta.ID = texto(primeiro(campos, "id", "uki_id"))
			meta.Tipo = NormalizarTipo(texto(primeiro(campos, "tipo", "type")))
			meta.Status = NormalizarStatus(texto(campos["status"]))
			meta.CriadoEm = data(primeiro(campos, "criado_em", "created_at"))
			meta.AtualizadoEm = data(primeiro(campos, "atualizado_em", "updated_at"))
			meta.Tags = lista(campos["tags"])
		}
	} else {
		for _, m := range reNegrito.FindAllStringSubmatch(conteudo, -1) {
			valor := strings.TrimSpace(m[2])
			switch strings.ToLower(m[1]) {
			case "id":
				meta.ID = valor
			case "tipo", "type":
				meta.Tipo = NormalizarTipo(valor)
			case "status":
				meta.Status = NormalizarStatus(valor)
			case "tags":
				meta.Tags = lista(valor)
			}
		}
	}
	if m := reTitulo.FindStringSubmatch(corpo); m != nil {
		meta.Titulo = strings.TrimSpace(m[1])
	}
	return meta, corpo
}

// separarFrontmatter devolve o YAML entre a primeira linha --- e a linha --- (ou ...)
// que o fecha, e o corpo depois dele sem as linhas em branco do começo.
func separarFrontmatter(conteudo string) (string, string, bool) {
	resto := strings.TrimPrefix(conteudo, "\uFEFF")
	primeira, resto, ok := cortarLinha(resto)
	if !ok || strings.TrimSpace(primeira) != "---" {
		return "", "", false
	}
	var bloco strings.Builder
	for resto != "" {
		var linha string
		linha, resto, _ = cortarLinha(resto)
		if l := strings.TrimSpace(linha); l == "---" || l == "..." {
			return bloco.String(), strings.TrimLeft(resto, "\r\n"), true
		}
		bloco.WriteString(linha)
		bloco.WriteString("\n")
	}
	return "", "", false
}

// cortarLinha separa a primeira linha (sem o \r\n) do resto do texto.
func cortarLinha(s string) (string, string, bool) {
	linha, resto, ok := strings.Cut(s, "\n")
	return strings.TrimSuffix(linha, "\r"), resto, ok
}

// Montar junta o frontmatter da especificação e o corpo, com uma linha em branco entre
// os dois.
func Montar(m Metadados, corpo string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", m.ID)
	fmt.Fprintf(&b, "tipo: %s\n", m.Tipo)
	fmt.Fprintf(&b, "status: %s\n", m.Status)
	fmt.Fprintf(&b, "criado_em: %s\n", formatarData(m.CriadoEm))
	fmt.Fprintf(&b, "atualizado_em: %s\n", formatarData(m.AtualizadoEm))
	fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(m.Tags, ", "))
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimLeft(corpo, "\r\n"))
	return b.String()
}

func formatarData(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.DateOnly)
}

// NormalizarTipo leva o tipo para o nome da especificação (sem acento e em minúsculas),
// aceitando os nomes em inglês. Um tipo desconhecido volta limpo, do mesmo jeito.
func NormalizarTipo(s string) string {
	return normalizar(s)
}

// NormalizarStatus leva o status para o da especificação: rascunho (proposal, draft),
// ativo (active) ou obsoleto (deprecated).
func NormalizarStatus(s string) string {
	return normalizar(s)
}

func normalizar(s string) string {
	s = semAcento(strings.ToLower(strings.TrimSpace(s)))
	if v, ok := sinonimos[s]; ok {
		return v
	}
	return s
}

// TipoValido indica que o tipo é um dos da especificação.
func TipoValido(tipo string) bool {
	for _, t := range Tipos {
		if tipo == t {
			return true
		}
	}
	return false
}

var acentos = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i",
	"ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ç", "c",
)

func semAcento(s string) string {
	return acentos.Replace(s)
}

// LimparID deixa o ID no formato da especificação (dominio.subdominio.nome-em-kebab-case):
// minúsculas sem acento, com o que não for letra, número, ponto ou hífen trocado por hífen.
func LimparID(s string) string {
	s = semAcento(strings.ToLower(strings.TrimSpace(s)))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
			continue
		}
		if !strings.HasSuffix(b.String(), "-") {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), ".-")
}

// LimparTags deixa as tags em minúsculas, com hífen no lugar dos espaços, sem os
// caracteres que quebram a lista do YAML, sem vazias e sem repetidas.
func LimparTags(tags []string) []string {
	var limpas []string
	vistas := map[string]bool{}
	for _, t := range tags {
		t = strings.Join(strings.Fields(strings.ToLower(t)), "-")
		t = strings.Map(func(r rune) rune {
			if strings.ContainsRune("[]{},:#\"'&*!|>%@`", r) {
				return -1
			}
			return r
		}, t)
		if t == "" || vistas[t] {
			continue
		}
		vistas[t] = true
		limpas = append(limpas, t)
	}
	return limpas
}

// primeiro devolve o valor da primeira chave presente.
func primeiro(campos map[string]any, chaves ...string) any {
	for _, c := range chaves {
		if v, ok := campos[c]; ok && v != nil {
			return v
		}
	}
	return nil
}

// texto é o valor escalar do YAML como texto.
func texto(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case time.Time:
		return x.Format(time.DateOnly)
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

// data lê AAAA-MM-DD (ou data e hora RFC 3339); o que não for data fica zero.
func data(v any) time.Time {
	if t, ok := v.(time.Time); ok {
		return t
	}
	s := texto(v)
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

// lista lê as tags em lista do YAML ou em texto separado por vírgulas.
func lista(v any) []string {
	var itens []string
	switch x := v.(type) {
	case []any:
		for _, i := range x {
			itens = append(itens, texto(i))
		}
	case string:
		itens = strings.Split(strings.Trim(x, "[]"), ",")
	}
	var tags []string
	for _, i := range itens {
		if i = strings.TrimSpace(i); i != "" {
			tags = append(tags, i)
		}
	}
	return tags
}
