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

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/scanner"
)

// SynapstorResponse defines the expected JSON output from the AI
type SynapstorResponse struct {
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Summary  string `json:"summary"`
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
	return a.saveResponse(ctx, respJSON)
}

// Study scans code and generates documentation
func (a *Agent) Study(ctx context.Context, query string) (*Resultado, error) {
	et := a.etapas()
	et.Iniciar("procurar no código")
	results, err := scanner.Scan(a.RootDir, query)
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
	sb.WriteString(fmt.Sprintf("User Query: Please document the logic related to '%s'.\n\nRelevant Code Files:\n", query))

	// Limit tokens crudely
	totalChars := 0
	limit := 100000 // approx 25k tokens

	for _, r := range results {
		content := fmt.Sprintf("--- FILE: %s ---\n%s\n\n", r.Path, r.Content)
		if totalChars+len(content) > limit {
			sb.WriteString(fmt.Sprintf("--- FILE: %s ---\n(Truncated due to context limit)\n\n", r.Path))
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
	return a.saveResponse(ctx, respJSON)
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

// validateResponse valida os campos obrigatórios da resposta da IA.
func validateResponse(resp *SynapstorResponse) error {
	if strings.TrimSpace(resp.Title) == "" {
		return fmt.Errorf("campo 'title' está vazio")
	}
	if strings.TrimSpace(resp.Filename) == "" {
		return fmt.Errorf("campo 'filename' está vazio")
	}
	if strings.TrimSpace(resp.Content) == "" {
		return fmt.Errorf("campo 'content' está vazio")
	}

	resp.Filename = nomeDoUKI(resp.Filename, resp.Title)
	return nil
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

func (a *Agent) saveResponse(ctx context.Context, respJSON string) (*Resultado, error) {
	et := a.etapas()
	et.Iniciar("conferir a resposta")
	cleanJSON := limparJSON(respJSON)

	var uki SynapstorResponse
	if err := json.Unmarshal([]byte(cleanJSON), &uki); err != nil {
		et.Falhar("")
		return nil, respostaInvalida(err, respJSON)
	}

	corrigida := false
	// Validar resposta
	if err := validateResponse(&uki); err != nil {
		// Tentar retry com prompt corretivo (1x)
		slog.Debug("resposta da IA incompleta; pedindo correção", "erro", err)
		correctionPrompt := fmt.Sprintf(
			"A resposta anterior teve o seguinte problema de validação: %v\n"+
				"Corrija e reenvie no formato JSON correto com os campos: title, filename (formato UKI-TIMESTAMP-SLUG.md), content, summary.\n"+
				"Resposta original:\n%s", err, cleanJSON)

		retryJSON, retryErr := a.Provider.Completion(ctx, prompts.Get("synapstor.capture"), correctionPrompt)
		if retryErr != nil {
			et.Falhar("")
			return nil, erroDaIA(retryErr, a.Provider)
		}

		uki = SynapstorResponse{}
		if err := json.Unmarshal([]byte(limparJSON(retryJSON)), &uki); err != nil {
			et.Falhar("")
			return nil, respostaInvalida(err, retryJSON)
		}

		// Validar novamente (sem retry desta vez)
		if err := validateResponse(&uki); err != nil {
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
	nome, err := gravarSemSobrescrever(synapstorDir, filepath.Base(uki.Filename), []byte(uki.Content))
	if err != nil {
		et.Falhar("")
		return nil, yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para gravar o UKI").
			WithContext("arquivo", filepath.Join(synapstorDir, nome))
	}
	et.Concluir("")

	return &Resultado{
		Titulo:    uki.Title,
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
