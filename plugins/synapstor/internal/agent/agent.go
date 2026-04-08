package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/scanner"
	"github.com/charmbracelet/lipgloss"
)

// SynapstorResponse defines the expected JSON output from the AI
type SynapstorResponse struct {
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Summary  string `json:"summary"`
}

// Agent encapsulates the Synapstor logic
type Agent struct {
	Provider ai.Provider
	RootDir  string
}

func NewAgent(provider ai.Provider, rootDir string) *Agent {
	return &Agent{
		Provider: provider,
		RootDir:  rootDir,
	}
}

// Capture processes raw text input and creates a UKI
func (a *Agent) Capture(input string) error {
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).Render("🧠 Synapstor Agent"))
	fmt.Println("Processando input para estruturação...")

	if a.Provider == nil {
		return fmt.Errorf("nenhum provedor de IA configurado")
	}

	// Inject Timestamp to help ID generation
	promptWithContext := fmt.Sprintf("%s\nCurrent Timestamp: %d", prompts.Get("synapstor.capture"), time.Now().Unix())

	respJson, err := a.Provider.Completion(context.Background(), promptWithContext, input)
	if err != nil {
		return fmt.Errorf("falha na IA: %w", err)
	}

	return a.saveResponse(respJson, "Conhecimento Capturado!")
}

// Study scans code and generates documentation
func (a *Agent) Study(query string) error {
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).Render("🧠 Synapstor Agent"))
	fmt.Printf("🔎 Estudando o código sobre: '%s'...\n", query)

	results, err := scanner.Scan(a.RootDir, query)
	if err != nil {
		return fmt.Errorf("erro ao escanear arquivos: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("⚠️  Nenhum arquivo relevante encontrado para este tópico.")
		return nil
	}

	fmt.Printf("📂 %d arquivos relevantes encontrados. Analisando...\n", len(results))

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

	if a.Provider == nil {
		return fmt.Errorf("nenhum provedor de IA configurado")
	}

	respJson, err := a.Provider.Completion(context.Background(), promptWithContext, sb.String())
	if err != nil {
		return fmt.Errorf("falha na IA: %w", err)
	}

	return a.saveResponse(respJson, "Conhecimento Gerado!")
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

	// Normalizar filename para padrão UKI
	if !strings.HasPrefix(resp.Filename, "UKI-") {
		// Gerar nome normalizado
		slug := strings.ToLower(strings.ReplaceAll(resp.Title, " ", "-"))
		if len(slug) > 30 {
			slug = slug[:30]
		}
		resp.Filename = fmt.Sprintf("UKI-%d-%s.md", time.Now().Unix(), slug)
	}

	// Garantir extensão .md
	if !strings.HasSuffix(resp.Filename, ".md") {
		resp.Filename += ".md"
	}

	return nil
}

func (a *Agent) saveResponse(respJson, successTitle string) error {
	// Limpar JSON
	cleanJson := strings.ReplaceAll(respJson, "```json", "")
	cleanJson = strings.ReplaceAll(cleanJson, "```", "")
	cleanJson = strings.TrimSpace(cleanJson)

	var uki SynapstorResponse
	if err := json.Unmarshal([]byte(cleanJson), &uki); err != nil {
		return fmt.Errorf("falha ao parsear resposta da IA: %w\nResp (Raw): %s", err, respJson)
	}

	// Validar resposta
	if err := validateResponse(&uki); err != nil {
		// Tentar retry com prompt corretivo (1x)
		fmt.Printf("⚠️  Resposta inválida: %v. Tentando corrigir...\n", err)
		correctionPrompt := fmt.Sprintf(
			"A resposta anterior teve o seguinte problema de validação: %v\n"+
				"Corrija e reenvie no formato JSON correto com os campos: title, filename (formato UKI-TIMESTAMP-SLUG.md), content, summary.\n"+
				"Resposta original:\n%s", err, cleanJson)

		retryJson, retryErr := a.Provider.Completion(context.Background(), prompts.Get("synapstor.capture"), correctionPrompt)
		if retryErr != nil {
			return fmt.Errorf("falha na correção da IA: %w (erro original: %v)", retryErr, err)
		}

		retryClean := strings.ReplaceAll(retryJson, "```json", "")
		retryClean = strings.ReplaceAll(retryClean, "```", "")
		retryClean = strings.TrimSpace(retryClean)

		if err := json.Unmarshal([]byte(retryClean), &uki); err != nil {
			return fmt.Errorf("falha ao parsear resposta corrigida da IA: %w", err)
		}

		// Validar novamente (sem retry desta vez)
		if err := validateResponse(&uki); err != nil {
			return fmt.Errorf("resposta da IA continua inválida após correção: %w", err)
		}
	}

	// Preparar diretórios e salvar arquivo
	synapstorDir := filepath.Join(a.RootDir, ".synapstor", ".uki")
	if err := os.MkdirAll(synapstorDir, 0755); err != nil {
		return fmt.Errorf("falha ao criar diretório: %w", err)
	}

	filePath := filepath.Join(synapstorDir, uki.Filename)
	if err := os.WriteFile(filePath, []byte(uki.Content), 0644); err != nil {
		return fmt.Errorf("falha ao salvar UKI: %w", err)
	}

	fmt.Printf("\n✅ %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(successTitle))
	fmt.Printf("📂 Arquivo: %s\n", uki.Filename)
	fmt.Printf("📝 Título: %s\n", uki.Title)
	fmt.Println("🔄 Sugestão: Rode 'yby synapstor index' para atualizar o índice do Bard.")
	return nil
}
