package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/gomlx/go-huggingface/tokenizers/hftokenizer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// modeloMultilingue é o caminho no Hub dos arquivos do modelo local, na revisão fixada.
var modeloMultilingue = "/sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2/resolve/e8f8c211226b894fcb81acc59f3b34ba3efd5f42/"

// TestDownloadLocalEmbeddingModel: o modelo local é o multilíngue em int8. Ele não tem
// vocab.txt (o tokenizador é o tokenizer.json), e o ONNX int8 vai para a raiz como
// model.onnx, o único .onnx que o hugot encontra. O tokenizer.json vem por último.
func TestDownloadLocalEmbeddingModel(t *testing.T) {
	var pedidos []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos = append(pedidos, r.URL.Path)
		// Responde com o próprio caminho do arquivo para conferir o destino
		_, _ = w.Write([]byte(strings.TrimPrefix(r.URL.Path, modeloMultilingue)))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), localEmbeddingModelDir)
	require.NoError(t, downloadLocalEmbeddingModel(context.Background(), server.URL, dest))

	assert.Equal(t, []string{
		modeloMultilingue + "config.json",
		modeloMultilingue + "special_tokens_map.json",
		modeloMultilingue + "tokenizer_config.json",
		modeloMultilingue + "onnx/model_qint8_avx512.onnx",
		modeloMultilingue + "tokenizer.json",
	}, pedidos)

	for destino, origem := range map[string]string{
		"config.json":             "config.json",
		"special_tokens_map.json": "special_tokens_map.json",
		"tokenizer_config.json":   "tokenizer_config.json",
		"model.onnx":              "onnx/model_qint8_avx512.onnx",
		"tokenizer.json":          "tokenizer.json",
	} {
		data, err := os.ReadFile(filepath.Join(dest, destino))
		require.NoError(t, err)
		assert.Equal(t, origem, string(data))
	}
	assert.NoFileExists(t, filepath.Join(dest, "vocab.txt"))

	parts, _ := filepath.Glob(filepath.Join(dest, "*.part"))
	assert.Empty(t, parts, "não deve sobrar arquivo temporário")
}

func TestDownloadLocalEmbeddingModel_ErroHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".onnx") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	dest := t.TempDir()
	err := downloadLocalEmbeddingModel(context.Background(), server.URL, dest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "onnx/model_qint8_avx512.onnx")
	assert.Contains(t, err.Error(), "404")

	// Sem tokenizer.json o download é tratado como incompleto e refeito
	assert.NoFileExists(t, filepath.Join(dest, "tokenizer.json"))
	assert.NoFileExists(t, filepath.Join(dest, "model.onnx.part"))
}

// etapasGravadas grava as etapas e os downloads na ordem em que acontecem.
type etapasGravadas struct {
	shared.EtapasMudas
	eventos *[]string
}

func (e etapasGravadas) Iniciar(r string)  { *e.eventos = append(*e.eventos, "iniciar "+r) }
func (e etapasGravadas) Concluir(n string) { *e.eventos = append(*e.eventos, "concluir "+n) }
func (e etapasGravadas) Falhar(n string)   { *e.eventos = append(*e.eventos, "falhar "+n) }

// homeTemporaria põe a home (e o ~/.yby/models) numa pasta temporária e o Hub num
// servidor de teste que grava os pedidos em eventos.
func homeTemporaria(t *testing.T, eventos *[]string, status int) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*eventos = append(*eventos, "GET "+path.Base(r.URL.Path))
		w.WriteHeader(status)
		_, _ = w.Write([]byte("x"))
	}))
	t.Cleanup(server.Close)
	t.Setenv("HF_ENDPOINT", server.URL)
	return home
}

// TestPrepararEmbeddings_EtapaAntesDoDownload: o primeiro uso baixa ~120 MB; antes só
// saía um slog.Info, de dentro da etapa de quem pediu os embeddings. Agora o download
// tem etapa própria, aberta antes do primeiro arquivo, e com o modelo no disco não há etapa.
func TestPrepararEmbeddings_EtapaAntesDoDownload(t *testing.T) {
	var eventos []string
	home := homeTemporaria(t, &eventos, http.StatusOK)
	et := etapasGravadas{eventos: &eventos}
	p := NewLocalEmbeddingProvider()

	require.NoError(t, PrepararEmbeddings(context.Background(), p, et))
	require.NotEmpty(t, eventos)
	assert.Equal(t, "iniciar baixar o modelo de embeddings (~120 MB)", eventos[0])
	assert.Equal(t, "GET config.json", eventos[1])
	assert.Equal(t, "concluir ", eventos[len(eventos)-1])
	assert.FileExists(t, filepath.Join(home, ".yby", "models", localEmbeddingModelDir, "tokenizer.json"))

	// O modelo já está no disco: nada a baixar e nenhuma etapa.
	eventos = nil
	require.NoError(t, PrepararEmbeddings(context.Background(), p, et))
	assert.Empty(t, eventos)
}

func TestPrepararEmbeddings_FalhaNoDownload(t *testing.T) {
	var eventos []string
	homeTemporaria(t, &eventos, http.StatusServiceUnavailable)
	et := etapasGravadas{eventos: &eventos}

	err := PrepararEmbeddings(context.Background(), NewLocalEmbeddingProvider(), et)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
	assert.Equal(t, "falhar ", eventos[len(eventos)-1])
}

// TestPrepararEmbeddings_OutroProvider: quem não baixa modelo (Ollama, APIs) não abre etapa.
func TestPrepararEmbeddings_OutroProvider(t *testing.T) {
	var eventos []string
	require.NoError(t, PrepararEmbeddings(context.Background(), NewOllamaProvider(), etapasGravadas{eventos: &eventos}))
	assert.Empty(t, eventos)
}

// TestLocalEmbedding_IdentidadeENome: a identidade do índice e o nome dizem que o modelo
// é o multilíngue em int8; o "Local" do nome é o que o Bard lê para dizer "embeddings locais".
func TestLocalEmbedding_IdentidadeENome(t *testing.T) {
	p := NewLocalEmbeddingProvider()
	id, err := p.IdentidadeDosEmbeddings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, IdentidadeDoIndice{Provider: "local", Modelo: "paraphrase-multilingual-MiniLM-L12-v2-int8"}, id)
	assert.Contains(t, p.Name(), "Local")
	assert.Contains(t, p.Name(), "multilingual")
}

func TestHuggingFaceEndpoint(t *testing.T) {
	t.Setenv("HF_ENDPOINT", "")
	assert.Equal(t, "https://huggingface.co", huggingFaceEndpoint())

	t.Setenv("HF_ENDPOINT", "https://hf.mirror.local/")
	assert.Equal(t, "https://hf.mirror.local", huggingFaceEndpoint())
}

// TestCortarEmBytes_NaoPartePalavraDeRuna: o corte em bytes caía no meio de um
// caractere de 2 bytes e entregava UTF-8 inválido ao tokenizador.
func TestCortarEmBytes_NaoPartePalavraDeRuna(t *testing.T) {
	texto := strings.Repeat("a", 9) + "çç"
	cortado := cortarEmBytes(texto, 10)
	assert.True(t, utf8.ValidString(cortado), "o corte tem de cair numa divisa de caractere")
	assert.Equal(t, strings.Repeat("a", 9), cortado)

	curto := "ação"
	assert.Equal(t, curto, cortarEmBytes(curto, 10))
}

func TestEncurtarPalavras(t *testing.T) {
	assert.Equal(t, "abc de\nfgh", encurtarPalavras("abcdef de\nfghij", 3))
	assert.Equal(t, "açã x", encurtarPalavras("açãoes x", 3))
	curto := "nada a encurtar aqui"
	assert.Equal(t, curto, encurtarPalavras(curto, 10))
}

// tokenizadorUnigram monta, com o go-huggingface que o hugot usa, um tokenizador Unigram
// com a mesma pré-tokenização e o mesmo pós-processamento do tokenizer.json do modelo
// local, mas com vocabulário só de caracteres: base64 e hex viram um token por caractere,
// como no modelo de verdade (que dá ~1500 tokens para 2000 caracteres de base64).
func tokenizadorUnigram(t *testing.T) func(string) int {
	t.Helper()
	const metaspace = "▁"
	vocab := [][]any{{"<s>", 0.0}, {"<pad>", 0.0}, {"</s>", 0.0}, {"<unk>", 0.0}, {metaspace, -1.0}}
	for _, r := range "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/=:-_.,#`'\"" {
		vocab = append(vocab, []any{string(r), -2.0})
	}
	especial := func(id int, conteudo string) map[string]any {
		return map[string]any{"id": id, "content": conteudo, "special": true, "single_word": false, "lstrip": false, "rstrip": false, "normalized": false}
	}
	tj := map[string]any{
		"version":      "1.0",
		"truncation":   map[string]any{"max_length": 128, "strategy": "LongestFirst", "stride": 0},
		"added_tokens": []any{especial(0, "<s>"), especial(1, "<pad>"), especial(2, "</s>"), especial(3, "<unk>")},
		"normalizer":   nil,
		"pre_tokenizer": map[string]any{"type": "Sequence", "pretokenizers": []any{
			map[string]any{"type": "WhitespaceSplit"},
			map[string]any{"type": "Metaspace", "replacement": metaspace, "add_prefix_space": true},
		}},
		"post_processor": map[string]any{
			"type": "TemplateProcessing",
			"single": []any{
				map[string]any{"SpecialToken": map[string]any{"id": "<s>", "type_id": 0}},
				map[string]any{"Sequence": map[string]any{"id": "A", "type_id": 0}},
				map[string]any{"SpecialToken": map[string]any{"id": "</s>", "type_id": 0}},
			},
			"special_tokens": map[string]any{
				"<s>":  map[string]any{"id": "<s>", "ids": []int{0}, "tokens": []string{"<s>"}},
				"</s>": map[string]any{"id": "</s>", "ids": []int{2}, "tokens": []string{"</s>"}},
			},
		},
		"decoder": map[string]any{"type": "Metaspace", "replacement": metaspace, "add_prefix_space": true},
		"model":   map[string]any{"type": "Unigram", "unk_id": 3, "vocab": vocab},
	}
	conteudo, err := json.Marshal(tj)
	require.NoError(t, err)
	tk, err := hftokenizer.NewFromContent(nil, conteudo)
	require.NoError(t, err)
	// Confere o pós-processamento: o texto vazio é só <s> e </s>.
	require.Equal(t, []int{0, 2}, tk.Encode(""))
	return func(s string) int { return len(tk.Encode(s)) }
}

// trechosDensos são trechos de UKI de infraestrutura que derrubavam o index inteiro com
// o modelo local: o corte era em 2000 bytes, mas base64, hex, PEM e YAML denso passam de
// 512 tokens bem antes disso, e acima de 512 o grafo entra em pânico.
func trechosDensos() map[string]string {
	base64 := strings.Repeat("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0Jn", 12)[:800]
	var linhas []string
	for i := 0; i < len(base64); i += 64 {
		linhas = append(linhas, base64[i:min(i+64, len(base64))])
	}
	hex := strings.Repeat("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", 16)[:1000]
	yaml := strings.Repeat("- name: api\n  image: registry.local/api:v1.2.3\n  ports: [{containerPort: 8080}]\n", 25)
	return map[string]string{
		"kubeconfig": "# Acesso ao cluster\n\n## Kubeconfig\n```yaml\ncertificate-authority-data: " + base64 + "\n```\nO campo acima vem do cluster.",
		"pem":        "## Certificado da CA\n```\n-----BEGIN CERTIFICATE-----\n" + strings.Join(linhas, "\n") + "\n-----END CERTIFICATE-----\n```",
		"hex":        "## Digest\n" + hex,
		"yaml":       "## Containers\n```yaml\n" + yaml + "```",
	}
}

// TestPrepararParaModelo_TrechoDensoCabeNoModelo: nenhum texto preparado passa do limite
// de tokens do modelo, contado com um tokenizador Unigram como o do modelo local.
func TestPrepararParaModelo_TrechoDensoCabeNoModelo(t *testing.T) {
	contar := tokenizadorUnigram(t)
	for nome, texto := range trechosDensos() {
		t.Run(nome, func(t *testing.T) {
			require.Greater(t, contar(cortarEmBytes(texto, 2000)), limiteTokensPadrao,
				"o trecho tem de passar do limite com o corte antigo, em 2000 bytes")
			preparado := prepararParaModelo(texto, limiteTokensPadrao, contar)
			assert.LessOrEqual(t, contar(preparado), limiteTokensPadrao)
			assert.True(t, utf8.ValidString(preparado))
			assert.NotEmpty(t, preparado)
		})
	}
}

// TestPrepararParaModelo_CortaNoMaiorComecoQueCabe: o texto que cabe vai inteiro; o que
// não cabe perde só o fim, numa divisa de caractere, e o que sobra usa o limite todo.
func TestPrepararParaModelo_CortaNoMaiorComecoQueCabe(t *testing.T) {
	// Um token por runa, mais os dois especiais.
	contar := func(s string) int { return utf8.RuneCountInString(s) + 2 }

	curto := "Implantação com Argo CD"
	assert.Equal(t, curto, prepararParaModelo(curto, 512, contar))

	longo := strings.Repeat("ação ", 300)
	preparado := prepararParaModelo(longo, 512, contar)
	assert.Equal(t, 512, contar(preparado))
	assert.True(t, strings.HasPrefix(longo, preparado))
	assert.True(t, utf8.ValidString(preparado))
}

// TestPrepararParaModelo_PalavraEnormeEncurtada: uma palavra sem espaço maior que
// maxRunasPorPalavra fica só com o começo; tokenizar a palavra inteira custa segundos.
func TestPrepararParaModelo_PalavraEnormeEncurtada(t *testing.T) {
	contar := func(s string) int { return utf8.RuneCountInString(s) + 2 }
	texto := "token: " + strings.Repeat("A", 5000) + " fim"
	assert.Equal(t, "token: "+strings.Repeat("A", maxRunasPorPalavra)+" fim", prepararParaModelo(texto, 512, contar))
}
