package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/plugins/synapstor/internal/scanner"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/uki"
	"github.com/casheiro/yby-cli/v5/pkg/ai"
	"github.com/casheiro/yby-cli/v5/pkg/ai/prompts"
	yerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
)

// SynapstorResponse é o JSON que a IA devolve. O frontmatter do UKI sai do ID, do tipo e
// das tags; o status e as datas são do código.
type SynapstorResponse struct {
	Title    string `json:"title"`
	ID       string `json:"id"`
	Tipo     string `json:"tipo"`
	Tags     tags   `json:"tags"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// tags aceita a lista de tags em JSON ou num texto separado por vírgulas, que é como os
// modelos menores às vezes respondem.
type tags []string

func (t *tags) UnmarshalJSON(b []byte) error {
	var lista []string
	if err := json.Unmarshal(b, &lista); err == nil {
		*t = lista
		return nil
	}
	var texto string
	if err := json.Unmarshal(b, &texto); err != nil {
		return err
	}
	*t = strings.Split(texto, ",")
	return nil
}

// Resultado é o UKI gravado por Capture ou Study.
type Resultado struct {
	// Titulo é o título do UKI.
	Titulo string
	// Arquivo é o caminho do UKI, relativo à raiz do projeto.
	Arquivo string
	// Corrigida indica que a primeira resposta da IA veio incompleta e foi corrigida.
	Corrigida bool
}

// ErrSemArquivos é o fim de Study quando nenhum arquivo do projeto fala do tópico.
var ErrSemArquivos = errors.New("nenhum arquivo relevante para o tópico")

// Agent encapsulates the Synapstor logic
type Agent struct {
	Provider ai.Provider
	RootDir  string
	// Etapas recebe o andamento (estruturar com a IA, conferir a resposta, gravar); o
	// agente não escreve na tela.
	Etapas shared.Etapas
	// agora dá a data do criado_em e do atualizado_em (nos testes, uma data fixa).
	agora func() time.Time
}

func NewAgent(provider ai.Provider, rootDir string) *Agent {
	return &Agent{
		Provider: provider,
		RootDir:  rootDir,
		Etapas:   shared.EtapasMudas{},
	}
}

func (a *Agent) etapas() shared.Etapas {
	if a.Etapas == nil {
		return shared.EtapasMudas{}
	}
	return a.Etapas
}

// hoje é a data do dia, para o frontmatter do UKI.
func (a *Agent) hoje() time.Time {
	agora := time.Now
	if a.agora != nil {
		agora = a.agora
	}
	ano, mes, dia := agora().Date()
	return time.Date(ano, mes, dia, 0, 0, 0, 0, time.UTC)
}

// semProvider é o erro de quando nenhum provider de IA responde.
func semProvider() error {
	return yerrors.New(yerrors.ErrCodeConfig, "nenhum provider de IA disponível").
		WithHint("rode ollama serve, exporte OPENAI_API_KEY ou GEMINI_API_KEY, ou defina ai.provider no ~/.yby/config.yaml")
}

// Capture processes raw text input and creates a UKI
func (a *Agent) Capture(ctx context.Context, input string) (*Resultado, error) {
	if a.Provider == nil {
		return nil, semProvider()
	}

	// Inject Timestamp to help ID generation
	promptWithContext := fmt.Sprintf("%s\nCurrent Timestamp: %d", prompts.Get("synapstor.capture"), time.Now().Unix())

	respJSON, err := a.perguntar(ctx, promptWithContext, input)
	if err != nil {
		return nil, err
	}
	return a.saveResponse(ctx, respJSON, "", "synapstor.capture")
}

// Study scans code and generates documentation
func (a *Agent) Study(ctx context.Context, query string) (*Resultado, error) {
	et := a.etapas()
	et.Iniciar("procurar no código")
	results, err := scanner.Scan(ctx, a.RootDir, query)
	if err != nil {
		et.Falhar("")
		return nil, yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para ler os arquivos do projeto").
			WithContext("pasta", a.RootDir)
	}
	if len(results) == 0 {
		et.Concluir("nenhum arquivo")
		return nil, ErrSemArquivos
	}
	et.Concluir(plural(len(results), "arquivo", "arquivos"))

	if a.Provider == nil {
		return nil, semProvider()
	}

	// Construct context from files
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pedido: documente a lógica relacionada a '%s'.\n\nArquivos de código relevantes:\n", query))

	// Limit tokens crudely
	totalChars := 0
	limit := 100000 // approx 25k tokens

	for _, r := range results {
		content := fmt.Sprintf("--- ARQUIVO: %s ---\n%s\n\n", r.Path, r.Content)
		if totalChars+len(content) > limit {
			sb.WriteString(fmt.Sprintf("--- ARQUIVO: %s ---\n(Truncado pelo limite de contexto)\n\n", r.Path))
			continue
		}
		sb.WriteString(content)
		totalChars += len(content)
	}

	promptWithContext := fmt.Sprintf("%s\nCurrent Timestamp: %d", prompts.Get("synapstor.study"), time.Now().Unix())

	respJSON, err := a.perguntar(ctx, promptWithContext, sb.String())
	if err != nil {
		return nil, err
	}
	// o study documenta o código: sem tipo na resposta, o UKI é uma referência
	return a.saveResponse(ctx, respJSON, uki.TipoReferencia, "synapstor.study")
}

// perguntar pede à IA o UKI, na etapa "estruturar com a IA".
func (a *Agent) perguntar(ctx context.Context, system, user string) (string, error) {
	et := a.etapas()
	et.Iniciar("estruturar com a IA")
	resp, err := a.Provider.Completion(ctx, system, user)
	if err != nil {
		et.Falhar("")
		return "", erroDaIA(err, a.Provider)
	}
	et.Concluir(a.Provider.Name())
	return resp, nil
}

// erroDaIA é a falha de uma chamada ao provider, com o nome dele nos detalhes.
func erroDaIA(err error, p ai.Provider) error {
	ye := yerrors.Wrap(err, yerrors.ErrCodeExec, "a IA não respondeu").
		WithHint("confira o provider de IA com yby doctor; para ver o log, rode com YBY_LOG_LEVEL=debug")
	if nome := p.Name(); nome != "" {
		ye = ye.WithContext("provider", nome)
	}
	return ye
}

// validateResponse valida os campos obrigatórios da resposta da IA. O tipo vem do JSON,
// do metadado que a IA tenha escrito no content ou do tipoPadrao, e tem de ser um dos da
// especificação.
func validateResponse(resp *SynapstorResponse, tipoPadrao string) error {
	if strings.TrimSpace(resp.Title) == "" {
		return fmt.Errorf("campo 'title' está vazio")
	}
	if strings.TrimSpace(resp.Filename) == "" {
		return fmt.Errorf("campo 'filename' está vazio")
	}
	if strings.TrimSpace(resp.Content) == "" {
		return fmt.Errorf("campo 'content' está vazio")
	}

	original := strings.TrimSpace(resp.Tipo)
	resp.Tipo = uki.NormalizarTipo(original)
	if resp.Tipo == "" {
		meta, _ := uki.Ler(resp.Content)
		resp.Tipo = meta.Tipo
	}
	if resp.Tipo == "" {
		resp.Tipo = tipoPadrao
	}
	switch {
	case resp.Tipo == "":
		return fmt.Errorf("campo 'tipo' está vazio (use %s)", strings.Join(uki.Tipos, ", "))
	case !uki.TipoValido(resp.Tipo):
		return fmt.Errorf("campo 'tipo' fora da especificação: %q (use %s)", original, strings.Join(uki.Tipos, ", "))
	}

	resp.Filename = nomeDoUKI(resp.Filename, resp.Title)
	return nil
}

// montarUKI é o arquivo do UKI no formato da especificação: o frontmatter (ID, tipo e
// tags da IA, status rascunho e as datas do dia) e o content sem o frontmatter que a IA
// tenha escrito. Sem ID válido, ele sai do título.
func (a *Agent) montarUKI(resp SynapstorResponse) string {
	meta, corpo := uki.Ler(resp.Content)
	id := uki.LimparID(resp.ID)
	if id == "" {
		id = uki.LimparID(meta.ID)
	}
	if id == "" {
		id = uki.LimparID(resp.Title)
	}
	tags := uki.LimparTags(resp.Tags)
	if len(tags) == 0 {
		tags = uki.LimparTags(meta.Tags)
	}
	hoje := a.hoje()
	return uki.Montar(uki.Metadados{
		ID:           id,
		Tipo:         resp.Tipo,
		Status:       uki.StatusRascunho,
		CriadoEm:     hoje,
		AtualizadoEm: hoje,
		Tags:         tags,
	}, corpo)
}

// nomeDoUKI devolve um nome de arquivo seguro no padrão UKI-<n>-<slug>.md. O nome da IA
// vale só se, sem separadores de pasta e caracteres inválidos, ainda começar com UKI-;
// senão o nome sai do título. O resultado nunca tem separador de pasta.
func nomeDoUKI(nome, titulo string) string {
	nome = strings.TrimSpace(limparNome(nome))
	if !strings.HasPrefix(nome, "UKI-") || strings.Trim(nome, ".-") == "" {
		slug := strings.Trim(limparNome(strings.ToLower(strings.Join(strings.Fields(titulo), "-"))), ".-")
		if r := []rune(slug); len(r) > 30 {
			slug = strings.Trim(string(r[:30]), ".-")
		}
		if slug == "" {
			slug = "uki"
		}
		nome = fmt.Sprintf("UKI-%d-%s", time.Now().Unix(), slug)
	}
	if !strings.HasSuffix(nome, ".md") {
		nome += ".md"
	}
	return nome
}

// limparNome troca por '-' os separadores de pasta e os caracteres que não valem em nome
// de arquivo (no Windows ou fora dele) e junta os '-' repetidos.
func limparNome(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || strings.ContainsRune(`/\:*?"<>|`, r) {
			r = '-'
		}
		if r == '-' && strings.HasSuffix(b.String(), "-") {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// gravarSemSobrescrever grava conteudo em pasta/nome sem apagar um arquivo que já existe:
// se o nome estiver ocupado, tenta nome-2.md, nome-3.md e assim por diante. Devolve o nome
// usado.
func gravarSemSobrescrever(pasta, nome string, conteudo []byte) (string, error) {
	base := strings.TrimSuffix(nome, ".md")
	for n := 1; ; n++ {
		candidato := nome
		if n > 1 {
			candidato = fmt.Sprintf("%s-%d.md", base, n)
		}
		f, err := os.OpenFile(filepath.Join(pasta, candidato), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if errors.Is(err, os.ErrExist) && n < 1000 {
			continue
		}
		if err != nil {
			return candidato, err
		}
		_, err = f.Write(conteudo)
		if errFechar := f.Close(); err == nil {
			err = errFechar
		}
		return candidato, err
	}
}

// limparJSON tira as cercas de código que a IA às vezes põe em volta do JSON.
func limparJSON(s string) string {
	s = strings.ReplaceAll(s, "```json", "")
	s = strings.ReplaceAll(s, "```", "")
	return strings.TrimSpace(s)
}

// respostaInvalida é o erro de quando a resposta da IA não forma um UKI; a resposta crua
// e o erro técnico do encoding/json (em inglês) vão para o log de depuração, e a causa na
// tela sai em PT-BR (ai.CausaForaDoFormato).
func respostaInvalida(err error, resposta string) error {
	slog.Debug("resposta da IA fora do formato do UKI", "resposta", resposta)
	return yerrors.Wrap(ai.CausaForaDoFormato(err), yerrors.ErrCodePlugin, "a resposta da IA não veio no formato de um UKI").
		WithHint("rode de novo; se repetir, troque o modelo com YBY_AI_MODEL (com YBY_LOG_LEVEL=debug a resposta sai no log)")
}

// saveResponse confere a resposta da IA (pedindo uma correção, se faltar algo) e grava o
// UKI; tipoPadrao vale quando a resposta não traz o tipo, e promptCorrecao é o prompt do
// próprio comando (capture ou study), usado no pedido de correção.
func (a *Agent) saveResponse(ctx context.Context, respJSON, tipoPadrao, promptCorrecao string) (*Resultado, error) {
	et := a.etapas()
	et.Iniciar("conferir a resposta")
	cleanJSON := limparJSON(respJSON)

	var resp SynapstorResponse
	if err := json.Unmarshal([]byte(cleanJSON), &resp); err != nil {
		et.Falhar("")
		return nil, respostaInvalida(err, respJSON)
	}

	corrigida := false
	// Validar resposta
	if err := validateResponse(&resp, tipoPadrao); err != nil {
		// Tentar retry com prompt corretivo (1x)
		slog.Debug("resposta da IA incompleta; pedindo correção", "erro", err)
		correctionPrompt := fmt.Sprintf(
			"A resposta anterior teve o seguinte problema de validação: %v\n"+
				"Corrija e reenvie no formato JSON correto com os campos: title, id, tipo (%s), tags, "+
				"filename (formato UKI-TIMESTAMP-SLUG.md), content.\n"+
				"Resposta original:\n%s", err, strings.Join(uki.Tipos, ", "), cleanJSON)

		retryJSON, retryErr := a.Provider.Completion(ctx, prompts.Get(promptCorrecao), correctionPrompt)
		if retryErr != nil {
			et.Falhar("")
			return nil, erroDaIA(retryErr, a.Provider)
		}

		resp = SynapstorResponse{}
		if err := json.Unmarshal([]byte(limparJSON(retryJSON)), &resp); err != nil {
			et.Falhar("")
			return nil, respostaInvalida(err, retryJSON)
		}

		// Validar novamente (sem retry desta vez)
		if err := validateResponse(&resp, tipoPadrao); err != nil {
			et.Falhar("")
			return nil, respostaInvalida(err, retryJSON)
		}
		corrigida = true
	}
	if corrigida {
		et.Concluir("corrigida pela IA")
	} else {
		et.Concluir("")
	}

	// Preparar diretórios e salvar arquivo
	et.Iniciar("gravar o UKI")
	synapstorDir := filepath.Join(a.RootDir, ".synapstor", ".uki")
	if err := os.MkdirAll(synapstorDir, 0755); err != nil {
		et.Falhar("")
		return nil, yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para criar a pasta dos UKIs").
			WithContext("pasta", synapstorDir)
	}

	// O nome já vem limpo do validateResponse; o Base é a última garantia de que nada é
	// gravado fora de .synapstor/.uki.
	nome, err := gravarSemSobrescrever(synapstorDir, filepath.Base(resp.Filename), []byte(a.montarUKI(resp)))
	if err != nil {
		et.Falhar("")
		return nil, yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para gravar o UKI").
			WithContext("arquivo", filepath.Join(synapstorDir, nome))
	}
	et.Concluir("")

	return &Resultado{
		Titulo:    resp.Title,
		Arquivo:   filepath.Join(".synapstor", ".uki", nome),
		Corrigida: corrigida,
	}, nil
}

func plural(n int, singular, plur string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plur)
}
