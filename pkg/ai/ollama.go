package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai/prompts"
)

// defaultOllamaModel é o modelo do Ollama sem config; sem ele configurado, o provider usa
// o primeiro modelo baixado (resolveModel).
const defaultOllamaModel = "llama3"

type OllamaProvider struct {
	BaseURL         string
	Model           string
	Endpoints       []string
	modelConfigured bool // true quando o modelo foi definido via config/usuário (pula auto-detect)
}

func NewOllamaProvider() *OllamaProvider {
	// Candidates to probe
	candidates := []string{}

	// 1. Env Var (User Override)
	if env := os.Getenv("OLLAMA_HOST"); env != "" {
		candidates = append(candidates, normalizeOllamaHost(env))
	}

	// 2. Standard Localhost
	candidates = append(candidates, "http://localhost:11434")

	// 3. WSL2 Specifics
	// 3a. User specified hardcoded fallback
	candidates = append(candidates, "http://172.17.224.1:11434")

	// 3b. Try to detect via /etc/resolv.conf (WSL Host)
	if wslHost, err := detectWSLHost(); err == nil && wslHost != "" {
		candidates = append(candidates, fmt.Sprintf("http://%s:11434", wslHost))
	}

	model := defaultOllamaModel
	modelConfigured := false
	if override := getConfiguredModel("ollama"); override != "" {
		model = override
		modelConfigured = true
	}

	return &OllamaProvider{
		BaseURL:         "", // Será resolvido em IsAvailable
		Model:           model,
		Endpoints:       candidates,
		modelConfigured: modelConfigured,
	}
}

func (p *OllamaProvider) Name() string {
	if p.BaseURL != "" {
		return fmt.Sprintf("Ollama (Local @ %s)", p.BaseURL)
	}
	return "Ollama (Local)"
}

func (p *OllamaProvider) IsAvailable(ctx context.Context) bool {
	// If already resolved, just ping
	if p.BaseURL != "" {
		return p.ping(ctx, p.BaseURL)
	}

	// Otherwise, probe all candidates
	for _, endpoint := range p.Endpoints {
		if p.ping(ctx, endpoint) {
			p.BaseURL = endpoint
			return true
		}
	}
	return false
}

func (p *OllamaProvider) ping(ctx context.Context, url string) bool {
	client := http.Client{Timeout: 500 * time.Millisecond} // Fast fail
	req, err := http.NewRequestWithContext(ctx, "GET", url+"/api/tags", nil)
	if err != nil {
		slog.Debug("Endpoint do Ollama inválido", "url", url, "erro", err)
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// normalizeOllamaHost converte OLLAMA_HOST em URL de cliente, seguindo as
// regras do próprio Ollama: sem esquema assume http e porta 11434. Hosts
// curinga como 0.0.0.0 (endereço de bind do servidor) viram localhost,
// pois não servem para conectar.
func normalizeOllamaHost(raw string) string {
	raw = strings.TrimSpace(raw)
	hasScheme := strings.Contains(raw, "://")
	if !hasScheme {
		raw = "http://" + raw
	}

	u, err := neturl.Parse(raw)
	if err != nil {
		return raw
	}

	host := u.Hostname()
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	port := u.Port()
	if port == "" && !hasScheme {
		port = "11434"
	}

	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else if host == "localhost" {
		u.Host = host
	}
	return strings.TrimRight(u.String(), "/")
}

func detectWSLHost() (string, error) {
	// Read /etc/resolv.conf
	content, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return "", err
	}

	// Look for nameserver x.x.x.x
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1], nil
			}
		}
	}
	return "", nil
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream bool   `json:"stream"`
	Format string `json:"format"` // json mode
}

type ollamaTagsResponse struct {
	Models []ollamaModelo `json:"models"`
}

// ollamaModelo é um modelo do /api/tags; os detalhes dizem a família (bert, nomic-bert…).
type ollamaModelo struct {
	Name    string `json:"name"`
	Details struct {
		Family   string   `json:"family"`
		Families []string `json:"families"`
	} `json:"details"`
}

// familiasDeEmbeddings são as famílias do /api/tags que só geram embeddings.
var familiasDeEmbeddings = map[string]bool{"bert": true, "nomic-bert": true}

// deEmbeddings diz se o modelo só gera embeddings (nomic-embed-text, mxbai-embed-large,
// all-minilm…): o nome traz "embed" ou a família é de embeddings. Um modelo assim não
// conversa, e o /api/generate dele falha.
func (m ollamaModelo) deEmbeddings() bool {
	if strings.Contains(strings.ToLower(m.Name), "embed") {
		return true
	}
	if familiasDeEmbeddings[strings.ToLower(m.Details.Family)] {
		return true
	}
	for _, f := range m.Details.Families {
		if familiasDeEmbeddings[strings.ToLower(f)] {
			return true
		}
	}
	return false
}

// nomesDeConversa são os nomes dos modelos que conversam, na ordem do /api/tags.
func nomesDeConversa(modelos []ollamaModelo) []string {
	var nomes []string
	for _, m := range modelos {
		if !m.deEmbeddings() {
			nomes = append(nomes, m.Name)
		}
	}
	return nomes
}

// resolveModel escolhe o modelo de conversa sem config: o primeiro baixado que não é de
// embeddings (o /api/tags pode trazer o nomic-embed-text na frente do llama3.2).
func (p *OllamaProvider) resolveModel(ctx context.Context) error {
	modelos, err := p.localModels(ctx)
	if err != nil {
		return err
	}
	if conversa := nomesDeConversa(modelos); len(conversa) > 0 {
		p.Model = conversa[0]
		return nil
	}
	if len(modelos) > 0 {
		return fmt.Errorf("o Ollama só tem modelos de embeddings (%s); baixe um de conversa com ollama pull llama3.2", modelos[0].Name)
	}
	return fmt.Errorf("nenhum modelo encontrado no Ollama")
}

// modeloDeEmbeddings é o modelo dos embeddings sem config: o primeiro baixado, de qualquer
// tipo, como sempre foi (trocar o modelo mudaria os vetores de um índice já gravado).
func (p *OllamaProvider) modeloDeEmbeddings(ctx context.Context) (string, error) {
	modelos, err := p.localModels(ctx)
	if err != nil {
		return "", err
	}
	if len(modelos) == 0 {
		return "", fmt.Errorf("nenhum modelo encontrado no Ollama")
	}
	return modelos[0].Name, nil
}

// localModels lista os modelos baixados no Ollama (/api/tags), na ordem que ele devolve.
func (p *OllamaProvider) localModels(ctx context.Context) ([]ollamaModelo, error) {
	client := http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", p.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama respondeu %d em /api/tags", resp.StatusCode)
	}

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}
	modelos := make([]ollamaModelo, 0, len(tags.Models))
	for _, m := range tags.Models {
		if m.Name != "" {
			modelos = append(modelos, m)
		}
	}
	return modelos, nil
}

type ollamaResponse struct {
	Response string `json:"response"`
}

func (p *OllamaProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	if !p.modelConfigured {
		if err := p.resolveModel(ctx); err != nil {
			return nil, fmt.Errorf("verificação de modelo ollama falhou: %w", err)
		}
	}

	reqBody := ollamaRequest{
		Model:  p.Model,
		Prompt: fmt.Sprintf("Descrição do Projeto: %s", description),
		System: prompts.Get("governance.system"),
		Stream: false,
		Format: "json",
	}

	jsonBody, _ := json.Marshal(reqBody)
	client := http.Client{Timeout: 300 * time.Second} // Generative tasks take time (CPU inference)

	resp, err := client.Post(p.BaseURL+"/api/generate", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("falha ao chamar ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, NewAPIErrorFromResponse("ollama", resp, body)
	}

	var oResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return nil, fmt.Errorf("falha ao decodificar resposta do ollama: %w", err)
	}

	// Parse the inner JSON generated by LLM
	var blueprint GovernanceBlueprint
	// Clean potential markdown fences if model disobeys
	cleanJSON := strings.TrimPrefix(oResp.Response, "```json")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")

	if err := json.Unmarshal([]byte(cleanJSON), &blueprint); err != nil {
		return nil, fmt.Errorf("falha ao analisar json do blueprint: %w. Bruto: %s", err, oResp.Response)
	}

	return &blueprint, nil
}

func (p *OllamaProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if !p.modelConfigured {
		if err := p.resolveModel(ctx); err != nil {
			return "", fmt.Errorf("verificação de modelo ollama falhou: %w", err)
		}
	}

	reqBody := ollamaRequest{
		Model:  p.Model,
		Prompt: userPrompt,
		System: systemPrompt,
		Stream: false,
	}

	jsonBody, _ := json.Marshal(reqBody)
	client := http.Client{Timeout: 300 * time.Second}

	resp, err := client.Post(p.BaseURL+"/api/generate", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("falha ao chamar ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", NewAPIErrorFromResponse("ollama", resp, body)
	}

	var oResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return "", fmt.Errorf("falha ao decodificar resposta do ollama: %w", err)
	}

	return oResp.Response, nil
}

func (p *OllamaProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	if !p.modelConfigured {
		if err := p.resolveModel(ctx); err != nil {
			return fmt.Errorf("verificação de modelo ollama falhou: %w", err)
		}
	}

	reqBody := ollamaRequest{
		Model:  p.Model,
		Prompt: userPrompt,
		System: systemPrompt,
		Stream: true,
	}

	jsonBody, _ := json.Marshal(reqBody)
	client := http.Client{Timeout: 300 * time.Second}

	req, _ := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/api/generate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("falha ao chamar ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return NewAPIErrorFromResponse("ollama", resp, body)
	}

	decoder := json.NewDecoder(resp.Body)
	for {
		var oResp ollamaResponse
		if err := decoder.Decode(&oResp); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if _, err := io.WriteString(out, oResp.Response); err != nil {
			return err
		}
	}

	return nil
}

// ollamaEmbedBatchSize define o tamanho máximo de batch para /api/embed (Ollama v0.5+).
// Batch de 1 pois documentos inteiros (UKIs) podem exceder o context window do modelo.
const ollamaEmbedBatchSize = 1

type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// structs legadas para fallback em Ollama < v0.5
type ollamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// modeloParaEmbeddings é o modelo que gera os embeddings: o de embeddings do config, o do
// provider quando configurado ou, sem config, o primeiro baixado.
func (p *OllamaProvider) modeloParaEmbeddings(ctx context.Context) (string, error) {
	if embModel := GetEmbeddingModel("ollama"); embModel != "" {
		return embModel, nil
	}
	if p.modelConfigured {
		return p.Model, nil
	}
	return p.modeloDeEmbeddings(ctx)
}

// IdentidadeDosEmbeddings diz que os embeddings vêm do Ollama e com que modelo; sem config,
// o modelo é o primeiro baixado e muda com um ollama pull.
func (p *OllamaProvider) IdentidadeDosEmbeddings(ctx context.Context) (IdentidadeDoIndice, error) {
	modelo, err := p.modeloParaEmbeddings(ctx)
	if err != nil {
		return IdentidadeDoIndice{}, err
	}
	return IdentidadeDoIndice{Provider: "ollama", Modelo: modelo}, nil
}

func (p *OllamaProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	origModel := p.Model
	defer func() { p.Model = origModel }()
	modelo, err := p.modeloParaEmbeddings(ctx)
	if err != nil {
		return nil, err
	}
	p.Model = modelo

	results, err := p.embedBatch(ctx, texts)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// embedBatch usa /api/embed (batch, Ollama v0.5+) com fallback para /api/embeddings (sequencial).
func (p *OllamaProvider) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	client := http.Client{Timeout: 300 * time.Second} // 5min — documentos grandes
	var allResults [][]float32

	for i := 0; i < len(texts); i += ollamaEmbedBatchSize {
		end := i + ollamaEmbedBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		reqBody := ollamaEmbedRequest{
			Model: p.Model,
			Input: batch,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/api/embed", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("falha ao chamar ollama embed (batch %d): %w", i, err)
		}

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			// Ollama antigo não suporta /api/embed, usar fallback sequencial
			slog.Warn("Ollama não suporta batch, usando modo sequencial")
			return p.embedSequential(ctx, texts)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, NewAPIErrorFromResponse("ollama", resp, body)
		}

		var oResp ollamaEmbedResponse
		if err := json.Unmarshal(body, &oResp); err != nil {
			return nil, fmt.Errorf("falha ao decodificar resposta de embed batch %d: %w", i, err)
		}

		if len(oResp.Embeddings) != len(batch) {
			return nil, fmt.Errorf("mismatch no batch %d: enviados %d, recebidos %d", i, len(batch), len(oResp.Embeddings))
		}

		allResults = append(allResults, oResp.Embeddings...)
	}

	return allResults, nil
}

// embedSequential usa /api/embeddings (1 texto por request) para Ollama < v0.5.
func (p *OllamaProvider) embedSequential(ctx context.Context, texts []string) ([][]float32, error) {
	client := http.Client{Timeout: 300 * time.Second}
	results := make([][]float32, len(texts))

	for i, text := range texts {
		reqBody := ollamaEmbeddingRequest{
			Model:  p.Model,
			Prompt: text,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/api/embeddings", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("falha ao chamar ollama embedding [%d]: %w", i, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, NewAPIErrorFromResponse("ollama", resp, body)
		}

		var oResp ollamaEmbeddingResponse
		if err := json.Unmarshal(body, &oResp); err != nil {
			return nil, err
		}
		results[i] = oResp.Embedding
	}
	return results, nil
}
