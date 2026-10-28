package ai

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
)

// Troca de modelo dentro de um provider (o /modelo do Bard): a lista dos modelos de cada
// provider e a criação do provider com o modelo escolhido, sem variável de ambiente nem
// config. Nenhum modelo é validado chamando a IA: cada provider decide pela regra dele.

// ModelCatalog são os modelos de um provider para quem escolhe um.
type ModelCatalog struct {
	Provider string   // nome curto, como em ai.provider ("claude-cli")
	Default  string   // modelo usado sem escolha; vazio quando o próprio CLI decide
	Models   []string // os modelos conhecidos, com o padrão primeiro
	Local    bool     // a lista veio do próprio provider (o /api/tags do Ollama)
	Warning  string   // por que a lista ficou curta (o Ollama não respondeu)
	// Embeddings são os modelos baixados que só geram embeddings (o nomic-embed-text do
	// Ollama): ficam fora de Models, porque não conversam.
	Embeddings []string
}

// modelSpec é o que a factory sabe dos modelos de um provider.
type modelSpec struct {
	defaultModel func() string           // nil quando o CLI escolhe o modelo
	known        []string                // os modelos que o código já cita
	accepts      func(model string) bool // nomes fora da lista que o provider aceita
}

// geminiModels são os modelos atuais do Gemini que o código já conhece (tokencount.go).
var geminiModels = []string{"gemini-2.5-flash", "gemini-2.5-pro", "gemini-2.0-flash"}

// openAIModelPattern aceita os nomes de modelo de chat da OpenAI: gpt-* e a série o1, o3...
var openAIModelPattern = regexp.MustCompile(`^(gpt-[a-z0-9.-]+|o[0-9][a-z0-9.-]*)$`)

// modelSpecs liga cada provider aos modelos dele. O Ollama fica de fora: a lista dele é
// a dos modelos baixados (listOllamaModels). Providers com build tag se registram por
// registerModelSpec.
var modelSpecs = map[string]modelSpec{
	"claude-cli": {
		// Sem config o CLI escolhe o modelo (padrão vazio); com ai.model, é ele que roda.
		defaultModel: func() string { return getConfiguredModel("claude-cli") },
		known:        []string{"sonnet", "opus", "haiku"},
		accepts:      func(model string) bool { return strings.HasPrefix(model, "claude-") },
	},
	"gemini-cli": {
		defaultModel: func() string { return getConfiguredModel("gemini-cli") },
		known:        geminiModels,
		accepts:      func(model string) bool { return strings.HasPrefix(model, "gemini-") },
	},
	"gemini": {
		defaultModel: defaultGeminiModel,
		known:        geminiModels,
		accepts:      func(model string) bool { return strings.HasPrefix(model, "gemini-") },
	},
	"openai": {
		defaultModel: defaultOpenAIModel,
		known:        []string{"gpt-4o-mini", "gpt-4o", "gpt-4-turbo"},
		accepts:      openAIModelPattern.MatchString,
	},
}

// registerModelSpec registra os modelos de um provider com build tag (ex: bedrock).
func registerModelSpec(name string, spec modelSpec) {
	modelSpecs[name] = spec
}

// IsKnownProvider diz se o nome é de um provider que a factory sabe criar, disponível ou
// não neste ambiente (bedrock conta mesmo sem a build tag aws).
func IsKnownProvider(name string) bool {
	for _, p := range defaultPriority {
		if p == name {
			return true
		}
	}
	_, ok := registeredProviders[name]
	return ok
}

// Avisos da lista do Ollama: fora do ar não há o que escolher, e a lista vazia (ou só com
// modelos de embeddings, que não conversam) pede um ollama pull.
const (
	avisoOllamaFora         = "o Ollama não respondeu"
	avisoOllamaVazio        = "nenhum modelo baixado no Ollama"
	avisoOllamaSoEmbeddings = "o Ollama só tem modelos de embeddings"
)

// ollamaListTimeout limita a leitura dos modelos locais: o Ollama fora do ar não pode
// prender a tela de quem lista.
const ollamaListTimeout = 1500 * time.Millisecond

// ListModels devolve os modelos de um provider: a lista conhecida e curta, com o modelo
// configurado (ou o padrão) primeiro; para o Ollama, os modelos baixados nele. Um
// provider sem lista devolve só o nome, e quem escolhe digita o modelo.
func ListModels(ctx context.Context, provider string) ModelCatalog {
	if provider == "ollama" {
		return listOllamaModels(ctx, NewOllamaProvider())
	}
	cat := ModelCatalog{Provider: provider}
	spec, ok := modelSpecs[provider]
	if !ok {
		return cat
	}
	if spec.defaultModel != nil {
		cat.Default = spec.defaultModel()
	}
	cat.Models = juntarModelos(cat.Default, spec.known)
	return cat
}

// listOllamaModels lê os modelos baixados no Ollama. Sem resposta, a lista fica só com o
// modelo atual e o aviso.
func listOllamaModels(ctx context.Context, p *OllamaProvider) ModelCatalog {
	cat := ModelCatalog{Provider: "ollama"}
	ctx, cancel := context.WithTimeout(ctx, ollamaListTimeout)
	defer cancel()
	var baixados []ollamaModelo
	var err error
	if p.IsAvailable(ctx) {
		baixados, err = p.localModels(ctx)
	}
	// Só os modelos de conversa entram na lista: um de embeddings não responde ao Bard.
	locais := nomesDeConversa(baixados)
	for _, m := range baixados {
		if m.deEmbeddings() {
			cat.Embeddings = append(cat.Embeddings, m.Name)
		}
	}
	switch {
	case p.BaseURL == "" || err != nil:
		cat.Warning = avisoOllamaFora
	case len(baixados) == 0:
		cat.Warning = avisoOllamaVazio
	case len(locais) == 0:
		cat.Warning = avisoOllamaSoEmbeddings
	default:
		cat.Local = true
	}
	// Sem modelo no config, o Ollama usa o primeiro de conversa baixado (resolveModel).
	cat.Default = p.Model
	if !p.modelConfigured && len(locais) > 0 {
		cat.Default = locais[0]
	}
	cat.Models = juntarModelos(cat.Default, locais)
	return cat
}

// juntarModelos põe o padrão na frente da lista, sem repetir.
func juntarModelos(padrao string, lista []string) []string {
	var out []string
	visto := map[string]bool{}
	for _, m := range append([]string{padrao}, lista...) {
		if m == "" || visto[m] {
			continue
		}
		visto[m] = true
		out = append(out, m)
	}
	return out
}

// GetProviderWithModel cria o provider pelo nome com o modelo escolhido, pela mesma cadeia
// de decorators do GetProvider, sem ler YBY_AI_MODEL nem gravar no config. Modelo vazio
// usa o do config ou o padrão. O modelo passa pela regra do provider antes: um nome que
// ele não conhece volta como ERR_VALIDATION, sem chamar a IA. Devolve também o modelo
// como o provider o conhece ("llama3" vira "llama3:latest", "Opus" vira "opus").
func GetProviderWithModel(ctx context.Context, provider, model string) (Provider, string, error) {
	model = strings.TrimSpace(model)
	if model != "" {
		canonico, err := validarModelo(ctx, provider, model)
		if err != nil {
			return nil, "", err
		}
		model = canonico
	}
	if p := createProviderWithModel(ctx, provider, model); p != nil {
		return p, model, nil
	}
	return nil, "", errors.New(errors.ErrCodeValidation, "o provider "+provider+" não está disponível").
		WithContext("provider", provider).
		WithHint("Rode 'yby doctor' para ver os providers de IA deste ambiente")
}

// validarModelo confere o modelo pela regra do provider e devolve o nome como o provider
// o conhece (a lista ignora maiúsculas: "Opus" vira "opus").
func validarModelo(ctx context.Context, provider, model string) (string, error) {
	var conhecidos []string
	aceita := func(string) bool { return false }
	switch spec, ok := modelSpecs[provider]; {
	case provider == "ollama":
		cat := ListModels(ctx, provider)
		if err := erroListaOllama(cat, model); err != nil {
			return "", err
		}
		for _, m := range cat.Embeddings {
			if strings.EqualFold(m, model) || strings.EqualFold(m, model+":latest") {
				return "", errors.New(errors.ErrCodeValidation, "o "+m+" é um modelo de embeddings e não conversa").
					WithContext("provider", provider).WithContext("model", model).
					WithHint("Escolha um modelo de conversa: " + strings.Join(cat.Models, ", "))
			}
		}
		conhecidos = cat.Models
		// "llama3" é o "llama3:latest" que o /api/tags devolve.
		for _, m := range conhecidos {
			if strings.EqualFold(m, model+":latest") {
				return m, nil
			}
		}
	case !ok:
		return "", errors.New(errors.ErrCodeValidation, "o provider "+provider+" não aceita escolha de modelo").
			WithContext("provider", provider)
	default:
		conhecidos = spec.known
		if spec.defaultModel != nil {
			conhecidos = juntarModelos(spec.defaultModel(), spec.known)
		}
		if spec.accepts != nil {
			aceita = spec.accepts
		}
	}
	for _, m := range conhecidos {
		if strings.EqualFold(m, model) {
			return m, nil
		}
	}
	if aceita(model) {
		return model, nil
	}
	hint := "Escolha um dos modelos da lista do provider"
	if len(conhecidos) > 0 {
		hint = "Modelos conhecidos do " + provider + ": " + strings.Join(conhecidos, ", ")
	}
	return "", errors.New(errors.ErrCodeValidation, "o "+provider+" não tem o modelo "+model).
		WithContext("provider", provider).WithContext("model", model).
		WithHint(hint)
}

// erroListaOllama é o motivo de não dar para escolher um modelo do Ollama: fora do ar não
// há como conferir o nome, e sem modelo baixado não há o que escolher. Nil com a lista lida.
func erroListaOllama(cat ModelCatalog, model string) error {
	switch {
	case cat.Local:
		return nil
	case cat.Warning == avisoOllamaVazio:
		return errors.New(errors.ErrCodeValidation, avisoOllamaVazio).
			WithContext("provider", "ollama").WithContext("model", model).
			WithHint("Baixe um modelo com 'ollama pull <modelo>'")
	case cat.Warning == avisoOllamaSoEmbeddings:
		return errors.New(errors.ErrCodeValidation, avisoOllamaSoEmbeddings).
			WithContext("provider", "ollama").WithContext("model", model).
			WithHint("Baixe um modelo de conversa com 'ollama pull llama3.2'")
	default:
		return errors.New(errors.ErrCodeValidation, avisoOllamaFora).
			WithContext("provider", "ollama").WithContext("model", model).
			WithHint("Confira se o Ollama está rodando (ollama list)")
	}
}
