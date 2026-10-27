package ai

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"unicode/utf8"
)

// GeminiCLIProvider implementa o Provider usando o Gemini CLI (gemini).
type GeminiCLIProvider struct {
	command string
	model   string // vazio deixa o CLI escolher o modelo
}

// NewGeminiCLIProvider cria um provider que usa o Gemini CLI.
func NewGeminiCLIProvider() *GeminiCLIProvider {
	return &GeminiCLIProvider{command: "gemini"}
}

// Name retorna o identificador do provider.
func (p *GeminiCLIProvider) Name() string {
	return "Gemini CLI"
}

// IsAvailable verifica se o CLI gemini está instalado.
func (p *GeminiCLIProvider) IsAvailable(_ context.Context) bool {
	_, err := exec.LookPath(p.command)
	return err == nil
}

// instrucaoStdinGemini acompanha o -p do gemini: o CLI junta o stdin com o texto do -p, e o
// prompt de verdade vai pelo stdin porque argumento tem limite de tamanho.
const instrucaoStdinGemini = "Responda ao pedido recebido."

// args monta os argumentos do gemini: -p e, com modelo escolhido, --model.
func (p *GeminiCLIProvider) args() []string {
	if p.model == "" {
		return []string{"-p", instrucaoStdinGemini}
	}
	return []string{"--model", p.model, "-p", instrucaoStdinGemini}
}

// GenerateGovernance não é suportado pelo CLI.
func (p *GeminiCLIProvider) GenerateGovernance(_ context.Context, _ string) (*GovernanceBlueprint, error) {
	return nil, fmt.Errorf("GenerateGovernance nao suportado pelo %s", p.Name())
}

// Completion executa gemini com o prompt pelo stdin e retorna a resposta.
func (p *GeminiCLIProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var stdout strings.Builder
	stderr, err := executarCLI(ctx, p.command, p.args(), systemPrompt+"\n\n"+userPrompt, &stdout)
	if err != nil {
		return "", erroCLI("gemini cli", "", err, stderr)
	}

	result := cleanGeminiCLIOutput(stdout.String())
	if result == "" {
		return "", fmt.Errorf("resposta vazia do gemini cli")
	}
	return result, nil
}

// linhaStatusGemini diz se a linha é de status do Gemini CLI, não da resposta.
func linhaStatusGemini(line string) bool {
	return strings.HasPrefix(line, "Loaded cached credentials") ||
		strings.HasPrefix(line, "Using ") ||
		strings.HasPrefix(line, "Authenticating")
}

// cleanGeminiCLIOutput remove linhas de log/status que o Gemini CLI imprime no stdout.
func cleanGeminiCLIOutput(output string) string {
	lines := strings.Split(output, "\n")
	var cleaned []string
	for _, line := range lines {
		if linhaStatusGemini(line) {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

// StreamCompletion executa gemini e escreve a saída no writer, sem as linhas de status.
func (p *GeminiCLIProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	filtro := &filtroStatusGemini{out: out}
	stderr, err := executarCLI(ctx, p.command, p.args(), systemPrompt+"\n\n"+userPrompt, filtro)
	if ferr := filtro.fechar(); err == nil {
		err = ferr
	}
	if err != nil {
		return erroCLI("gemini cli", " streaming", err, stderr)
	}
	return nil
}

// maiorPrefixoStatusGemini é o tamanho do maior prefixo de linha de status.
const maiorPrefixoStatusGemini = len("Loaded cached credentials")

// filtroStatusGemini repassa a saída do gemini tirando as linhas de status. Segura o começo
// de cada linha só até dar para decidir se é de status; o resto passa direto. Cada Write de
// entrada vira no máximo um Write de saída (quem consome renderiza a cada Write), e um
// caractere UTF-8 partido no fim espera o restante dele.
type filtroStatusGemini struct {
	out      io.Writer
	inicio   []byte // começo da linha atual, ainda não decidido
	repassar bool   // a linha atual não é de status
	descarte bool   // a linha atual é de status
	saida    []byte // o que já é resposta e ainda não foi escrito
}

func (f *filtroStatusGemini) Write(b []byte) (int, error) {
	for _, c := range b {
		f.byte(c)
	}
	// Não corta um caractere no meio: o pedaço incompleto vai junto com o próximo Write.
	corte := len(f.saida)
	for i := 1; i < utf8.UTFMax && i <= len(f.saida); i++ {
		r := f.saida[len(f.saida)-i]
		if utf8.RuneStart(r) {
			if !utf8.FullRune(f.saida[len(f.saida)-i:]) {
				corte = len(f.saida) - i
			}
			break
		}
	}
	if err := f.soltar(corte); err != nil {
		return 0, err
	}
	return len(b), nil
}

// soltar escreve os n primeiros bytes pendentes de uma vez.
func (f *filtroStatusGemini) soltar(n int) error {
	if n == 0 {
		return nil
	}
	_, err := f.out.Write(f.saida[:n])
	f.saida = append(f.saida[:0], f.saida[n:]...)
	return err
}

func (f *filtroStatusGemini) byte(c byte) {
	if c == '\n' {
		inicio, repassou, descartou := f.inicio, f.repassar, f.descarte
		f.inicio, f.repassar, f.descarte = nil, false, false
		if descartou || (!repassou && linhaStatusGemini(string(inicio))) {
			return
		}
		f.saida = append(append(f.saida, inicio...), '\n')
		return
	}
	switch {
	case f.descarte:
	case f.repassar:
		f.saida = append(f.saida, c)
	default:
		f.inicio = append(f.inicio, c)
		if len(f.inicio) >= maiorPrefixoStatusGemini {
			if linhaStatusGemini(string(f.inicio)) {
				f.descarte = true
			} else {
				f.repassar = true
				f.saida = append(f.saida, f.inicio...)
			}
			f.inicio = nil
		}
	}
}

// fechar solta o fim da saída, que pode ficar sem quebra de linha.
func (f *filtroStatusGemini) fechar() error {
	if !f.descarte && !f.repassar && len(f.inicio) > 0 && !linhaStatusGemini(string(f.inicio)) {
		f.saida = append(f.saida, f.inicio...)
	}
	f.inicio = nil
	return f.soltar(len(f.saida))
}

// EmbedDocuments não é suportado pelo CLI.
func (p *GeminiCLIProvider) EmbedDocuments(_ context.Context, _ []string) ([][]float32, error) {
	return nil, fmt.Errorf("embeddings nao suportados pelo %s", p.Name())
}
