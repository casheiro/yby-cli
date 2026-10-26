package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// IndexManifest rastreia o estado de indexação de cada arquivo.
type IndexManifest struct {
	Files map[string]IndexedFile `json:"files"`
}

// IndexedFile registra o hash e timestamp de um arquivo indexado.
type IndexedFile struct {
	SHA256    string    `json:"sha256"`
	IndexedAt time.Time `json:"indexed_at"`
	// Chunks é quantos documentos (trechos) o arquivo tem no índice; a soma confere com
	// o tamanho da coleção. Manifest antigo, sem o campo, não bate e refaz o índice.
	Chunks int `json:"chunks"`
}

// manifestFile mora dentro da pasta do índice (que o .gitignore ignora): manifest e índice
// andam juntos, e um clone sem índice não herda um manifest que mandaria pular tudo por
// hash. O chromem ignora arquivos soltos na raiz da pasta persistente.
const manifestFile = ".synapstor/.index/manifest.json"

// manifestAntigo é onde versões anteriores guardavam o manifest, fora do índice e sem
// ser ignorado pelo git. Não é mais lido: o manifest antigo conta como vazio, o que
// refaz o índice uma vez, e é apagado na primeira gravação do novo.
const manifestAntigo = ".synapstor/.index_manifest.json"

// Indexer manages the knowledge ingestion pipeline
type Indexer struct {
	Provider    ai.Provider
	RootDir     string
	FullReindex bool
	// Etapas recebe o andamento (procurar documentos, tirar os removidos, gerar
	// embeddings); o indexador não escreve na tela.
	Etapas shared.Etapas
	// lerArquivo lê os documentos; nil é os.ReadFile. Os testes trocam por uma leitura que
	// falha, porque tirar a permissão (chmod 000) não vale no Windows.
	lerArquivo func(string) ([]byte, error)
}

func NewIndexer(provider ai.Provider, rootDir string) *Indexer {
	return &Indexer{
		Provider: provider,
		RootDir:  rootDir,
		Etapas:   shared.EtapasMudas{},
	}
}

func (i *Indexer) ler(caminho string) ([]byte, error) {
	if i.lerArquivo == nil {
		return os.ReadFile(caminho)
	}
	return i.lerArquivo(caminho)
}

func (i *Indexer) etapas() shared.Etapas {
	if i.Etapas == nil {
		return shared.EtapasMudas{}
	}
	return i.Etapas
}

func plural(n int, singular, plur string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plur)
}

// loadManifest carrega o manifest de indexação do disco.
func (i *Indexer) loadManifest() *IndexManifest {
	data, err := os.ReadFile(filepath.Join(i.RootDir, manifestFile))
	if err != nil {
		return &IndexManifest{Files: make(map[string]IndexedFile)}
	}
	var m IndexManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return &IndexManifest{Files: make(map[string]IndexedFile)}
	}
	if m.Files == nil {
		m.Files = make(map[string]IndexedFile)
	}
	return &m
}

// saveManifest persiste o manifest de indexação no disco.
func (i *Indexer) saveManifest(m *IndexManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(filepath.Join(i.RootDir, manifestFile))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(i.RootDir, manifestFile), data, 0644); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(i.RootDir, manifestAntigo))
	return nil
}

// tirarDoIndice apaga do vector store os documentos dos arquivos removidos. Devolve
// quantos saíram e quais ficaram por causa de erro; o detalhe do erro vai para o log.
func (i *Indexer) tirarDoIndice(ctx context.Context, storePath string, arquivos []string) (int, []string) {
	vs, err := ai.NewVectorStore(ctx, storePath, i.Provider)
	if err != nil {
		slog.Debug("não deu para abrir o índice para tirar os removidos", "erro", err)
		return 0, arquivos
	}
	removidos := 0
	var falhas []string
	for _, relPath := range arquivos {
		if err := vs.DeleteByMetadata(ctx, map[string]string{"source": relPath}); err != nil {
			slog.Debug("não deu para tirar o arquivo do índice", "arquivo", relPath, "erro", err)
			falhas = append(falhas, relPath)
			continue
		}
		removidos++
	}
	return removidos, falhas
}

// refazerSeNecessario apaga o índice e diz se apagou. Apaga quando: o --full foi pedido;
// ele foi feito com outros embeddings que não os do provider de agora (ou com um modelo
// que não ficou registrado); ou o número de documentos não bate com o manifest (a soma
// dos trechos de cada arquivo), o que denuncia órfãos sem entrada no manifest ou arquivos
// que o manifest dá como indexados e não estão no índice.
// Sem conseguir abrir o índice, só o --full decide: apaga a pasta; sem ele, as etapas
// seguintes dizem o que houve.
func (i *Indexer) refazerSeNecessario(ctx context.Context, storePath string, esperados int) (bool, error) {
	vs, err := ai.NewVectorStore(ctx, storePath, i.Provider)
	if err != nil {
		slog.Debug("não deu para abrir o índice para conferir o modelo", "erro", err)
		if !i.FullReindex {
			return false, nil
		}
		// Com --full o índice que não abre (arquivo truncado, coleção sem metadados) é
		// apagado inteiro: é o único conserto possível.
		et := i.etapas()
		et.Iniciar("apagar o índice antigo")
		if rerr := esvaziarIndice(storePath); rerr != nil {
			et.Falhar("")
			return false, fmt.Errorf("não deu para apagar o índice antigo: %w", rerr)
		}
		et.Concluir("--full")
		et.Avisar("o índice não abria; apagado e refeito do zero")
		return true, nil
	}
	motivo, aviso := "", ""
	err = vs.Conferir(ctx)
	var outro *ai.IndiceDeOutroModeloError
	switch {
	case errors.As(err, &outro):
		motivo = "outros embeddings"
		aviso = "índice refeito do zero: era de " + outro.FeitoCom() + ", agora " + outro.Atual.String()
	case err != nil:
		return false, err
	case i.FullReindex:
		motivo = "--full"
	case vs.Count() != esperados:
		motivo = "índice fora do manifest"
		aviso = "o índice não bate com o manifest (" + plural(vs.Count(), "documento", "documentos") + " para " + plural(esperados, "esperado", "esperados") + "); refeito do zero"
	default:
		return false, nil
	}
	et := i.etapas()
	et.Iniciar("apagar o índice antigo")
	if err := vs.Recriar(); err != nil {
		et.Falhar("")
		return false, fmt.Errorf("não deu para apagar o índice antigo: %w", err)
	}
	et.Concluir(motivo)
	if aviso != "" {
		et.Avisar(aviso)
	}
	return true, nil
}

// hashDoConteudo calcula o hash SHA-256 do conteúdo já lido de um arquivo.
func hashDoConteudo(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// listaCurta nomeia o arquivo quando é um só e conta quando são vários.
func listaCurta(caminhos []string) string {
	if len(caminhos) == 1 {
		return caminhos[0]
	}
	return plural(len(caminhos), "arquivo", "arquivos") + " (" + caminhos[0] + " e outros)"
}

// IndexReport contém métricas coletadas durante o processo de indexação.
type IndexReport struct {
	FilesScanned int
	FilesSkipped int
	FilesRemoved int
	// FilesIndexed conta os arquivos novos ou alterados que entraram no índice.
	FilesIndexed int
	// FilesUnreadable conta os arquivos que existem e não deu para ler; não são tratados
	// como apagados.
	FilesUnreadable   int
	ChunksGenerated   int
	EmbeddingsCreated int
	Duration          time.Duration
}

// Run executa o pipeline de indexação e retorna um relatório com métricas.
func (i *Indexer) Run(ctx context.Context) (*IndexReport, error) {
	start := time.Now()
	report := &IndexReport{}
	et := i.etapas()

	// 1. Coletar arquivos
	et.Iniciar("procurar documentos")
	files, err := i.scanFiles()
	if err != nil {
		et.Falhar("")
		return nil, err
	}

	report.FilesScanned = len(files)
	et.Concluir(plural(len(files), "arquivo", "arquivos"))

	// Sem documentos e sem nada indexado antes, não há o que fazer. Com o manifest
	// cheio, segue para tirar do índice os documentos que foram apagados.
	if len(files) == 0 && len(i.loadManifest().Files) == 0 {
		report.Duration = time.Since(start)
		return report, nil
	}

	// 2. Travar o índice: dois index ao mesmo tempo gravariam o manifest e a coleção sem
	// coordenação. O manifest é lido depois da trava, já com o que o outro index gravou.
	storePath := filepath.Join(i.RootDir, ".synapstor", ".index")
	soltar, err := travar(storePath)
	if err != nil {
		return nil, err
	}
	defer soltar()
	manifest := i.loadManifest()

	// 2.5. --full, índice de outros embeddings ou fora do manifest: apaga e refaz tudo.
	refeito, err := i.refazerSeNecessario(ctx, storePath, documentosNoManifest(manifest))
	if err != nil {
		return nil, err
	}
	fullReindex := i.FullReindex
	if refeito {
		// O manifest vazio vai para o disco já: se a indexação parar no meio, a próxima
		// não pula por hash o que saiu do índice.
		manifest = &IndexManifest{Files: make(map[string]IndexedFile)}
		if err := i.saveManifest(manifest); err != nil {
			return nil, fmt.Errorf("erro ao salvar manifest de indexação: %w", err)
		}
		fullReindex = true
	}
	newManifest := &IndexManifest{Files: make(map[string]IndexedFile)}

	// 3. Processar arquivos -> chunks
	var allChunks []string
	var allMetadatas []map[string]string
	var allIDs []string
	var ilegiveis []string
	// reindexados são os arquivos novos ou alterados: os trechos antigos deles saem do
	// índice antes dos novos entrarem, senão um arquivo que encolhe deixa trechos órfãos.
	var reindexados []string

	for _, path := range files {
		rel, _ := filepath.Rel(i.RootDir, path)
		relPath := filepath.ToSlash(rel)
		content, err := i.ler(path)
		if errors.Is(err, os.ErrNotExist) {
			// apagado depois da busca: sai do índice como os outros removidos
			continue
		}
		if err != nil {
			// O arquivo existe e não abriu (sem permissão, preso por outro programa): não
			// foi apagado. A entrada antiga fica no manifest, e o documento, no índice.
			slog.Debug("não deu para ler o arquivo", "arquivo", relPath, "erro", err)
			if existing, ok := manifest.Files[relPath]; ok {
				newManifest.Files[relPath] = existing
			}
			ilegiveis = append(ilegiveis, relPath)
			continue
		}
		hash := hashDoConteudo(content)

		// Verificar se já indexado e não mudou
		if !fullReindex {
			if existing, ok := manifest.Files[relPath]; ok && existing.SHA256 == hash {
				newManifest.Files[relPath] = existing
				report.FilesSkipped++
				continue
			}
		}

		// Um vetor para o arquivo todo diluiria o assunto e estouraria o limite do modelo
		// local: o documento vira um trecho por seção H2 (ID "caminho#n").
		baseName := filepath.Base(path)
		docContent := string(content)

		titulo := ""
		for _, line := range strings.Split(docContent, "\n") {
			if strings.HasPrefix(line, "# ") {
				titulo = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				break
			}
		}

		trechos := dividirEmTrechos(titulo, docContent)
		for n, tr := range trechos {
			meta := map[string]string{
				"source":   relPath,
				"filename": baseName,
				"chunk":    strconv.Itoa(n),
			}
			if titulo != "" {
				meta["title"] = titulo
			}
			if tr.Secao != "" {
				meta["section"] = tr.Secao
			}
			allChunks = append(allChunks, tr.Texto)
			allMetadatas = append(allMetadatas, meta)
			allIDs = append(allIDs, relPath+"#"+strconv.Itoa(n))
		}
		reindexados = append(reindexados, relPath)

		newManifest.Files[relPath] = IndexedFile{
			SHA256:    hash,
			IndexedAt: time.Now(),
			Chunks:    len(trechos),
		}
	}

	report.ChunksGenerated = len(allChunks)
	report.FilesIndexed = len(reindexados)
	report.FilesUnreadable = len(ilegiveis)
	if len(ilegiveis) > 0 {
		et.Avisar("não deu para ler " + listaCurta(ilegiveis) + "; a próxima indexação tenta de novo")
	}

	// 3.5. Detectar arquivos removidos (presentes no manifest antigo, ausentes no novo)
	var removedFiles []string
	for relPath := range manifest.Files {
		if _, exists := newManifest.Files[relPath]; !exists {
			removedFiles = append(removedFiles, relPath)
		}
	}
	if len(removedFiles) > 0 {
		et.Iniciar("tirar os removidos do índice")
		removidos, falhas := i.tirarDoIndice(ctx, storePath, removedFiles)
		// Os que não saíram do índice ficam no manifest, para a próxima indexação
		// tentar de novo.
		for _, relPath := range falhas {
			newManifest.Files[relPath] = manifest.Files[relPath]
		}
		report.FilesRemoved = removidos
		et.Concluir(plural(removidos, "arquivo", "arquivos"))
		if len(falhas) > 0 {
			et.Avisar("não deu para tirar do índice " + plural(len(falhas), "arquivo apagado", "arquivos apagados") + "; a próxima indexação tenta de novo")
		}
	}

	// 4. Se não há chunks novos, salvar manifest e retornar (arquivo que ficou vazio
	// também perde os trechos antigos)
	if len(allChunks) == 0 {
		if len(reindexados) > 0 {
			_, falhas := i.tirarDoIndice(ctx, storePath, reindexados)
			for _, relPath := range falhas {
				// o trecho antigo continua no índice: o manifest antigo faz tentar de novo
				if antigo, ok := manifest.Files[relPath]; ok {
					newManifest.Files[relPath] = antigo
				} else {
					delete(newManifest.Files, relPath)
				}
			}
		}
		if err := i.saveManifest(newManifest); err != nil {
			return nil, fmt.Errorf("erro ao salvar manifest de indexação: %w", err)
		}
		report.Duration = time.Since(start)
		return report, nil
	}

	// 5. Inicializar Vector Store
	et.Iniciar("gerar embeddings")
	vs, err := ai.NewVectorStore(ctx, storePath, i.Provider)
	if err != nil {
		et.Falhar("")
		return nil, fmt.Errorf("falha ao inicializar vector store: %w", err)
	}

	for _, relPath := range reindexados {
		if err := vs.DeleteByMetadata(ctx, map[string]string{"source": relPath}); err != nil {
			et.Falhar("")
			return nil, fmt.Errorf("não deu para tirar os trechos antigos de %s do índice: %w", relPath, err)
		}
	}

	vs.AoAvancar = func(feitos, total int) { et.Avancar(float64(feitos) / float64(total)) }

	// 6. Indexar
	if err := vs.AddDocuments(ctx, allChunks, allMetadatas, allIDs); err != nil {
		et.Falhar("")
		return nil, err
	}

	report.EmbeddingsCreated = len(allChunks)

	// 7. Salvar manifest atualizado
	if err := i.saveManifest(newManifest); err != nil {
		et.Falhar("")
		return nil, fmt.Errorf("erro ao salvar manifest de indexação: %w", err)
	}
	et.Concluir(plural(len(allChunks), "trecho", "trechos"))

	report.Duration = time.Since(start)
	return report, nil
}

func (i *Indexer) scanFiles() ([]string, error) {
	candidates := []string{}

	// Targets: .synapstor/.uki and docs/wiki
	targets := []string{
		filepath.Join(i.RootDir, ".synapstor", ".uki"),
		filepath.Join(i.RootDir, "docs", "wiki"),
	}

	for _, dir := range targets {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			name := strings.ToLower(info.Name())
			if !info.IsDir() && strings.HasSuffix(name, ".md") && name != "index.md" && name != "_footer.md" && name != "_sidebar.md" {
				candidates = append(candidates, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return candidates, nil
}

// documentosNoManifest soma os trechos que o manifest diz ter no índice.
func documentosNoManifest(m *IndexManifest) int {
	total := 0
	for _, f := range m.Files {
		total += f.Chunks
	}
	return total
}
