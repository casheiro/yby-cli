package analyzers

import (
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// kustomization representa a estrutura de um arquivo kustomization.yaml.
type kustomization struct {
	Resources             []string         `yaml:"resources"`
	Bases                 []string         `yaml:"bases"`
	Components            []string         `yaml:"components"`
	Namespace             string           `yaml:"namespace"`
	Patches               []kustomizePatch `yaml:"patches"`
	PatchesStrategicMerge []string         `yaml:"patchesStrategicMerge"`
	PatchesJSON6902       []kustomizePatch `yaml:"patchesJson6902"`
}

// kustomizePatch é uma entrada de patches/patchesJson6902; aqui só o arquivo importa.
type kustomizePatch struct {
	Path string `yaml:"path"`
}

// kustomizacaoLida é uma kustomization com as referências já resolvidas para
// caminhos relativos à raiz, com '/'.
type kustomizacaoLida struct {
	recurso   InfraResource // o nó Kustomization
	namespace string
	arquivos  []string // manifests listados em resources
	filhas    []string // diretórios locais (resources, bases, components)
	remotas   []string
	patches   []string
}

// PlanoKustomize é o que as kustomizations dizem ao analyzer K8s sobre os
// manifests. As chaves são caminhos relativos à raiz, com '/'.
type PlanoKustomize struct {
	// Patches são os arquivos de patch: alteram recursos, não criam outros.
	Patches map[string]bool
	// Namespaces são os namespaces em que cada manifest é aplicado, um por
	// overlay que o alcança; "" mantém o namespace do próprio manifest.
	Namespaces map[string][]string
	// Donos são os IDs das kustomizations que listam cada manifest.
	Donos map[string][]string
}

// KustomizeAnalyzer descobre recursos e relações em arquivos Kustomize.
// O ID de cada Kustomization é o diretório relativo à raiz, para que as
// referências entre elas ("../../base") resolvam para o mesmo nó.
type KustomizeAnalyzer struct {
	raiz  string
	lidas map[string]*kustomizacaoLida // por diretório relativo
}

// NewKustomizeAnalyzer cria uma nova instância do KustomizeAnalyzer.
func NewKustomizeAnalyzer() *KustomizeAnalyzer {
	return &KustomizeAnalyzer{}
}

// Name retorna o identificador do analyzer.
func (a *KustomizeAnalyzer) Name() string {
	return "kustomize"
}

// Plano lê as kustomizations e devolve o plano para o analyzer K8s, que precisa
// dele antes de extrair as relações. O Analyze seguinte reaproveita a leitura.
func (a *KustomizeAnalyzer) Plano(rootPath string, files []string) *PlanoKustomize {
	a.ler(rootPath, files)

	plano := &PlanoKustomize{
		Patches:    make(map[string]bool),
		Namespaces: make(map[string][]string),
		Donos:      make(map[string][]string),
	}

	referenciadas := make(map[string]bool)
	for _, l := range a.lidas {
		for _, f := range l.filhas {
			referenciadas[f] = true
		}
		for _, f := range l.arquivos {
			plano.Donos[f] = append(plano.Donos[f], l.recurso.ID())
		}
	}

	// O namespace de fora prevalece: o transformer do overlay roda sobre a
	// saída da base. Cada caminho a partir de uma raiz dá um namespace.
	vistos := make(map[string]bool)
	alcancadas := make(map[string]bool)
	var visitar func(dir, ns string)
	visitar = func(dir, ns string) {
		l := a.lidas[dir]
		if l == nil {
			return
		}
		if ns == "" {
			ns = l.namespace
		}
		chave := dir + "\x00" + ns
		if vistos[chave] {
			return
		}
		vistos[chave] = true
		alcancadas[dir] = true
		for _, f := range l.arquivos {
			plano.Namespaces[f] = append(plano.Namespaces[f], ns)
		}
		for _, filha := range l.filhas {
			visitar(filha, ns)
		}
	}
	dirs := a.diretorios()
	for _, dir := range dirs {
		if !referenciadas[dir] {
			visitar(dir, "")
		}
	}
	// Kustomizations só alcançáveis por um ciclo não têm raiz: cada uma vale por si.
	for _, dir := range dirs {
		if !alcancadas[dir] {
			visitar(dir, "")
		}
	}

	for _, l := range a.lidas {
		for _, p := range l.patches {
			// Arquivo listado como recurso em outra kustomization continua recurso.
			if _, recurso := plano.Namespaces[p]; !recurso {
				plano.Patches[p] = true
			}
		}
	}
	for f := range plano.Namespaces {
		plano.Namespaces[f] = ordenarSemRepetir(plano.Namespaces[f])
	}
	for f := range plano.Donos {
		plano.Donos[f] = ordenarSemRepetir(plano.Donos[f])
	}
	return plano
}

// Analyze processa arquivos kustomization.yaml e retorna recursos e relações.
// As ligações aos manifests listados saem do analyzer K8s, pelo Plano.
func (a *KustomizeAnalyzer) Analyze(rootPath string, files []string) (*AnalyzerResult, error) {
	result := &AnalyzerResult{Type: "kustomize"}
	if a.lidas == nil || a.raiz != rootPath {
		a.ler(rootPath, files)
	}

	for _, dir := range a.diretorios() {
		l := a.lidas[dir]
		result.Resources = append(result.Resources, l.recurso)
		for _, ref := range l.remotas {
			// Referência remota — cria recurso KustomizeRemote
			remoteRes := InfraResource{
				Kind:     "KustomizeRemote",
				APIGroup: "kustomize",
				Name:     ref,
				Path:     l.recurso.Path,
			}
			result.Resources = append(result.Resources, remoteRes)
			result.Relations = append(result.Relations, InfraRelation{
				From: l.recurso.ID(),
				To:   remoteRes.ID(),
				Type: "includes",
			})
		}
		for _, filha := range l.filhas {
			result.Relations = append(result.Relations, InfraRelation{
				From: l.recurso.ID(),
				To:   a.idDoDiretorio(filha),
				Type: "includes",
			})
		}
	}

	return result, nil
}

// ler lê as kustomizations de files, indexadas pelo diretório relativo.
func (a *KustomizeAnalyzer) ler(rootPath string, files []string) {
	a.raiz = rootPath
	a.lidas = make(map[string]*kustomizacaoLida)
	for _, f := range filterKustomizeFiles(files) {
		l, err := a.lerArquivo(f)
		if err != nil {
			slog.Warn("erro ao analisar arquivo kustomize", "path", f, "error", err)
			continue
		}
		a.lidas[path.Dir(l.recurso.Path)] = l
	}
}

// diretorios devolve os diretórios das kustomizations lidas, em ordem.
func (a *KustomizeAnalyzer) diretorios() []string {
	dirs := make([]string, 0, len(a.lidas))
	for dir := range a.lidas {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

// filterKustomizeFiles filtra apenas arquivos kustomization válidos.
func filterKustomizeFiles(files []string) []string {
	validNames := map[string]bool{
		"kustomization.yaml": true,
		"kustomization.yml":  true,
	}

	var matched []string
	for _, f := range files {
		base := filepath.Base(f)
		if validNames[strings.ToLower(base)] {
			matched = append(matched, f)
		}
	}
	return matched
}

// lerArquivo lê um único arquivo kustomization.yaml e resolve as referências.
func (a *KustomizeAnalyzer) lerArquivo(filePath string) (*kustomizacaoLida, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var k kustomization
	if err := yaml.Unmarshal(data, &k); err != nil {
		return nil, err
	}

	relPath, _ := relSlash(a.raiz, filePath)
	dir := path.Dir(relPath)

	l := &kustomizacaoLida{
		recurso: InfraResource{
			Kind:     "Kustomization",
			APIGroup: "kustomize",
			Name:     a.nomeDoDiretorio(dir),
			Path:     relPath,
		},
		namespace: k.Namespace,
	}
	// O namespace não entra no ID: as referências chegam só pelo caminho.
	if k.Namespace != "" {
		l.recurso.Metadata = map[string]string{"namespace": k.Namespace}
	}

	// resources, bases e components viram referências "includes"
	refs := make([]string, 0, len(k.Resources)+len(k.Bases)+len(k.Components))
	refs = append(refs, k.Resources...)
	refs = append(refs, k.Bases...)
	refs = append(refs, k.Components...)
	for _, ref := range refs {
		switch {
		case isRemoteRef(ref):
			l.remotas = append(l.remotas, ref)
		case hasYAMLExtension(ref):
			l.arquivos = append(l.arquivos, resolverRef(dir, ref))
		default:
			l.filhas = append(l.filhas, resolverRef(dir, ref))
		}
	}

	for _, p := range append(k.Patches, k.PatchesJSON6902...) {
		if p.Path != "" {
			l.patches = append(l.patches, resolverRef(dir, p.Path))
		}
	}
	// patchesStrategicMerge aceita o patch inline; só os caminhos interessam.
	for _, p := range k.PatchesStrategicMerge {
		if !strings.Contains(p, "\n") && hasYAMLExtension(p) {
			l.patches = append(l.patches, resolverRef(dir, p))
		}
	}

	return l, nil
}

// nomeDoDiretorio dá o nome da Kustomization de dir: o caminho relativo, ou o
// nome da pasta raiz para a kustomization da raiz.
func (a *KustomizeAnalyzer) nomeDoDiretorio(dir string) string {
	if dir != "." {
		return dir
	}
	if abs, err := filepath.Abs(a.raiz); err == nil {
		return filepath.Base(abs)
	}
	return filepath.Base(a.raiz)
}

// idDoDiretorio é o ID da Kustomization de dir, exista ela ou não.
func (a *KustomizeAnalyzer) idDoDiretorio(dir string) string {
	return InfraResource{Kind: "Kustomization", Name: a.nomeDoDiretorio(dir)}.ID()
}

// resolverRef resolve ref, relativa ao diretório dir, para o caminho relativo à raiz.
func resolverRef(dir, ref string) string {
	return path.Join(dir, filepath.ToSlash(strings.TrimSpace(ref)))
}

// ordenarSemRepetir ordena s e tira as repetições.
func ordenarSemRepetir(s []string) []string {
	sort.Strings(s)
	var saida []string
	for _, v := range s {
		if len(saida) == 0 || saida[len(saida)-1] != v {
			saida = append(saida, v)
		}
	}
	return saida
}

// isRemoteRef verifica se a referência é um URL remoto.
func isRemoteRef(ref string) bool {
	return strings.HasPrefix(ref, "http://") ||
		strings.HasPrefix(ref, "https://") ||
		strings.HasPrefix(ref, "git@") ||
		strings.HasPrefix(ref, "git://") ||
		strings.HasPrefix(ref, "ssh://")
}

// hasYAMLExtension verifica se o path termina com extensão YAML.
func hasYAMLExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}
