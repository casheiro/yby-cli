package analyzers

import (
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	// reResource captura blocos "resource "type" "name""
	reResource = regexp.MustCompile(`(?m)^\s*resource\s+"([^"]+)"\s+"([^"]+)"`)
	// reModule captura blocos "module "name""
	reModule = regexp.MustCompile(`(?m)^\s*module\s+"([^"]+)"`)
	// reData captura blocos "data "type" "name""
	reData = regexp.MustCompile(`(?m)^\s*data\s+"([^"]+)"\s+"([^"]+)"`)
	// reModuleSource captura "source = "path"" dentro de blocos module
	reModuleSource = regexp.MustCompile(`(?m)^\s*source\s*=\s*"([^"]+)"`)
	// reReferencia captura cadeias como aws_vpc.main.id, data.x.y.z e module.net.out
	reReferencia = regexp.MustCompile(`[A-Za-z_][\w-]*(?:\.[A-Za-z_][\w-]*)+`)
	// reHeredoc captura a abertura de um heredoc (<<EOT ou <<-EOT) e o marcador
	reHeredoc = regexp.MustCompile(`^<<-?([A-Za-z_][\w-]*)\r?\n`)
)

// TerraformAnalyzer descobre recursos e relações em arquivos Terraform (.tf).
// Usa regex (sem dependência HCL) para extração leve de blocos.
type TerraformAnalyzer struct{}

// NewTerraformAnalyzer cria uma nova instância do TerraformAnalyzer.
func NewTerraformAnalyzer() *TerraformAnalyzer {
	return &TerraformAnalyzer{}
}

// Name retorna o identificador do analyzer.
func (a *TerraformAnalyzer) Name() string {
	return "terraform"
}

// blocoTerraform é um resource, data ou module lido de um arquivo .tf.
type blocoTerraform struct {
	recurso InfraResource
	// endereco é o nome pelo qual o Terraform o referencia no diretório
	// (aws_vpc.main, data.aws_ami.x, module.net)
	endereco string
	// corpo é o conteúdo entre as chaves, sem comentários e sem o texto
	// literal das strings (as interpolações ficam)
	corpo string
	// fonte é o diretório resolvido do source de um module local, relativo à raiz
	fonte string
}

// Analyze processa arquivos .tf e retorna recursos e relações.
// Cada diretório é um módulo do Terraform: os IDs levam o diretório, as
// referências só valem dentro dele e o module com source local liga ao nó do
// diretório resolvido, que liga aos blocos desse diretório.
func (a *TerraformAnalyzer) Analyze(rootPath string, files []string) (*AnalyzerResult, error) {
	result := &AnalyzerResult{Type: "terraform"}

	tfFiles := filterTerraformFiles(files)
	if len(tfFiles) == 0 {
		return result, nil
	}
	sort.Strings(tfFiles)

	// Agrupa os blocos por diretório relativo à raiz
	porDiretorio := make(map[string][]blocoTerraform)
	var diretorios []string
	for _, f := range tfFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			slog.Warn("erro ao ler arquivo terraform", "path", f, "error", err)
			continue
		}
		relPath, err := relSlash(rootPath, f)
		if err != nil {
			continue
		}
		dir := path.Dir(relPath)
		if _, ok := porDiretorio[dir]; !ok {
			diretorios = append(diretorios, dir)
		}
		porDiretorio[dir] = append(porDiretorio[dir], lerBlocosTerraform(string(data), relPath, dir)...)
	}
	sort.Strings(diretorios)

	modulos := make(map[string]InfraResource) // diretório resolvido -> nó do módulo
	for _, dir := range diretorios {
		blocos := porDiretorio[dir]
		enderecos := make(map[string]string, len(blocos))
		for _, b := range blocos {
			result.Resources = append(result.Resources, b.recurso)
			enderecos[b.endereco] = b.recurso.ID()
		}

		for _, b := range blocos {
			if b.fonte != "" {
				modulo, ok := modulos[b.fonte]
				if !ok {
					modulo = InfraResource{
						Kind:     "TerraformModule",
						APIGroup: "terraform",
						Name:     b.fonte,
						Path:     b.fonte,
					}
					modulos[b.fonte] = modulo
				}
				result.Relations = append(result.Relations, InfraRelation{
					From: b.recurso.ID(),
					To:   modulo.ID(),
					Type: "includes",
				})
			}
			for _, alvo := range referenciasTerraform(b.corpo, enderecos) {
				if alvo == b.recurso.ID() {
					continue
				}
				result.Relations = append(result.Relations, InfraRelation{
					From: b.recurso.ID(),
					To:   alvo,
					Type: "references",
				})
			}
		}
	}

	// O nó de cada módulo chamado liga aos blocos do diretório dele (o que
	// está fora da varredura fica só com o nó)
	fontes := make([]string, 0, len(modulos))
	for fonte := range modulos {
		fontes = append(fontes, fonte)
	}
	sort.Strings(fontes)
	for _, fonte := range fontes {
		modulo := modulos[fonte]
		result.Resources = append(result.Resources, modulo)
		for _, b := range porDiretorio[fonte] {
			result.Relations = append(result.Relations, InfraRelation{
				From: modulo.ID(),
				To:   b.recurso.ID(),
				Type: "includes",
			})
		}
	}

	return result, nil
}

// filterTerraformFiles filtra apenas arquivos .tf.
func filterTerraformFiles(files []string) []string {
	var matched []string
	for _, f := range files {
		if strings.ToLower(filepath.Ext(f)) == ".tf" {
			matched = append(matched, f)
		}
	}
	return matched
}

// nomeNoDiretorio prefixa o nome com o diretório, para que recursos de mesmo
// nome em envs/dev e envs/prod não colidam no ID.
func nomeNoDiretorio(dir, nome string) string {
	if dir == "." || dir == "" {
		return nome
	}
	return dir + "/" + nome
}

// lerBlocosTerraform extrai os blocos resource, data e module de um arquivo.
// Os cabeçalhos e o source saem do texto sem comentários; o corpo, das mesmas
// posições no texto sem comentários nem strings literais.
func lerBlocosTerraform(conteudo, relPath, dir string) []blocoTerraform {
	semComentarios := limparHCL(conteudo, false)
	codigo := limparHCL(conteudo, true)

	var blocos []blocoTerraform
	corpo := func(fimCabecalho int) (string, string) {
		ini, fim, ok := blocoEntreChaves(codigo, fimCabecalho)
		if !ok {
			return "", ""
		}
		return codigo[ini:fim], semComentarios[ini:fim]
	}

	for _, m := range reResource.FindAllStringSubmatchIndex(semComentarios, -1) {
		tipo, nome := semComentarios[m[2]:m[3]], semComentarios[m[4]:m[5]]
		endereco := tipo + "." + nome
		c, _ := corpo(m[1])
		blocos = append(blocos, blocoTerraform{
			recurso: InfraResource{
				Kind:     "TerraformResource",
				APIGroup: "terraform",
				Name:     nomeNoDiretorio(dir, endereco),
				Path:     relPath,
				Metadata: map[string]string{"type": tipo, "name": nome},
			},
			endereco: endereco,
			corpo:    c,
		})
	}

	for _, m := range reData.FindAllStringSubmatchIndex(semComentarios, -1) {
		tipo, nome := semComentarios[m[2]:m[3]], semComentarios[m[4]:m[5]]
		c, _ := corpo(m[1])
		blocos = append(blocos, blocoTerraform{
			recurso: InfraResource{
				Kind:     "TerraformData",
				APIGroup: "terraform",
				Name:     nomeNoDiretorio(dir, tipo+"."+nome),
				Path:     relPath,
				Metadata: map[string]string{"type": tipo, "name": nome},
			},
			endereco: "data." + tipo + "." + nome,
			corpo:    c,
		})
	}

	for _, m := range reModule.FindAllStringSubmatchIndex(semComentarios, -1) {
		nome := semComentarios[m[2]:m[3]]
		c, texto := corpo(m[1])
		b := blocoTerraform{
			recurso: InfraResource{
				Kind:     "TerraformModule",
				APIGroup: "terraform",
				Name:     nomeNoDiretorio(dir, "module."+nome),
				Path:     relPath,
			},
			endereco: "module." + nome,
			corpo:    c,
		}
		if s := reModuleSource.FindStringSubmatch(texto); len(s) == 2 {
			b.recurso.Metadata = map[string]string{"source": s[1]}
			// Só o source local aponta para um diretório; registry e git ficam
			// só com o nó da chamada
			if strings.HasPrefix(s[1], "./") || strings.HasPrefix(s[1], "../") {
				b.fonte = path.Join(dir, s[1])
			}
		}
		blocos = append(blocos, b)
	}

	return blocos
}

// referenciasTerraform devolve, em ordem e sem repetir, os IDs dos blocos do
// mesmo diretório que o corpo referencia (aws_vpc.main.id, data.x.y.z,
// module.net.saida). Cadeias que não são blocos conhecidos (var., local.)
// ficam de fora.
func referenciasTerraform(corpo string, enderecos map[string]string) []string {
	var alvos []string
	vistos := make(map[string]bool)
	for _, cadeia := range reReferencia.FindAllString(corpo, -1) {
		partes := strings.Split(cadeia, ".")
		var endereco string
		switch {
		case partes[0] == "data" && len(partes) >= 3:
			endereco = strings.Join(partes[:3], ".")
		default:
			endereco = partes[0] + "." + partes[1]
		}
		id, ok := enderecos[endereco]
		if !ok || vistos[id] {
			continue
		}
		vistos[id] = true
		alvos = append(alvos, id)
	}
	return alvos
}

// blocoEntreChaves devolve as posições do conteúdo entre o primeiro '{' após
// inicio e o '}' correspondente.
func blocoEntreChaves(conteudo string, inicio int) (int, int, bool) {
	abre := strings.IndexByte(conteudo[inicio:], '{')
	if abre == -1 {
		return 0, 0, false
	}
	abre += inicio

	profundidade := 0
	for i := abre; i < len(conteudo); i++ {
		switch conteudo[i] {
		case '{':
			profundidade++
		case '}':
			profundidade--
			if profundidade == 0 {
				return abre + 1, i, true
			}
		}
	}
	return 0, 0, false
}

// trechoHCL marca onde a leitura está: numa string, num heredoc ou numa
// interpolação dentro deles.
type trechoHCL struct {
	tipo byte   // 's' string, 'h' heredoc, 'i' interpolação
	prof int    // chaves abertas dentro da interpolação
	fim  string // marcador que fecha o heredoc
}

// limparHCL troca por espaços os comentários (#, // e /* */) e, com textos,
// também o texto literal de strings e heredocs, deixando as interpolações
// (${...}). As quebras de linha e as posições se mantêm, para que as regex
// usem os mesmos índices nas duas versões e não casem com o que não é código.
func limparHCL(src string, textos bool) string {
	b := []byte(src)
	apagar := func(de, ate int) {
		for k := de; k < ate && k < len(b); k++ {
			if b[k] != '\n' && b[k] != '\r' {
				b[k] = ' '
			}
		}
	}

	var pilha []trechoHCL
	for i := 0; i < len(b); {
		c := b[i]
		var topo *trechoHCL
		if len(pilha) > 0 {
			topo = &pilha[len(pilha)-1]
		}

		// Dentro de string ou heredoc: só a interpolação é código
		if topo != nil && topo.tipo != 'i' {
			if topo.tipo == 'h' && (i == 0 || b[i-1] == '\n') {
				fimLinha := strings.IndexByte(src[i:], '\n')
				if fimLinha == -1 {
					fimLinha = len(src) - i
				}
				if strings.TrimSpace(src[i:i+fimLinha]) == topo.fim {
					pilha = pilha[:len(pilha)-1]
					i += fimLinha
					continue
				}
			}
			switch {
			case (c == '$' || c == '%') && i+1 < len(b) && b[i+1] == '{':
				pilha = append(pilha, trechoHCL{tipo: 'i'})
				i += 2
			case topo.tipo == 's' && c == '\\':
				if textos {
					apagar(i, i+2)
				}
				i += 2
			case topo.tipo == 's' && c == '"':
				pilha = pilha[:len(pilha)-1]
				i++
			default:
				if textos {
					apagar(i, i+1)
				}
				i++
			}
			continue
		}

		// Código: fora de tudo ou dentro de uma interpolação
		resto := src[i:]
		switch {
		case c == '"':
			pilha = append(pilha, trechoHCL{tipo: 's'})
			i++
		case c == '#' || strings.HasPrefix(resto, "//"):
			fimLinha := strings.IndexByte(resto, '\n')
			if fimLinha == -1 {
				fimLinha = len(resto)
			}
			apagar(i, i+fimLinha)
			i += fimLinha
		case strings.HasPrefix(resto, "/*"):
			fim := strings.Index(resto[2:], "*/")
			if fim == -1 {
				fim = len(resto)
			} else {
				fim += 4
			}
			apagar(i, i+fim)
			i += fim
		case topo == nil && strings.HasPrefix(resto, "<<"):
			if m := reHeredoc.FindStringSubmatch(resto); m != nil {
				pilha = append(pilha, trechoHCL{tipo: 'h', fim: m[1]})
				i += len(m[0])
			} else {
				i++
			}
		case topo != nil && c == '{':
			topo.prof++
			i++
		case topo != nil && c == '}':
			if topo.prof == 0 {
				pilha = pilha[:len(pilha)-1]
			} else {
				topo.prof--
			}
			i++
		default:
			i++
		}
	}
	return string(b)
}
