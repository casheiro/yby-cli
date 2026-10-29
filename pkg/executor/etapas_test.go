package executor

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
)

// progressoTexto devolve um ui.Progresso fora do terminal e com relógio que avança 1,5 s
// por leitura, escrevendo em w. Sem terminal no go test, as saídas já saem sem cor.
func progressoTexto(t *testing.T, w io.Writer, ascii bool) *ui.Progresso {
	t.Helper()
	opcoes := ui.Atual()
	t.Cleanup(func() { ui.Configurar(opcoes) })
	ui.Configurar(ui.Opcoes{ASCII: ascii})
	agora := time.Date(2026, 10, 24, 9, 30, 0, 0, time.UTC)
	return ui.NovoProgressoCom(w, ui.OpcoesProgresso{
		Terminal: func(io.Writer) bool { return false },
		Relogio: func() time.Time {
			agora = agora.Add(1500 * time.Millisecond)
			return agora
		},
		Largura: 64,
	})
}

func TestLocalExecutor_Run_EscreveEtapas(t *testing.T) {
	var b bytes.Buffer
	e := &LocalExecutor{
		Etapas: progressoTexto(t, &b, false),
		Executar: func(script string) ([]byte, error) {
			if script == "falha" {
				return []byte("linha 1\n\nE: pacote curl não encontrado\n"), fmt.Errorf("exit status 100")
			}
			return []byte("ok"), nil
		},
	}

	assert.NoError(t, e.Run("docker", "instala"))
	assert.Error(t, e.Run("sistema e dependências", "falha"))

	esperado := "" +
		"  ✓ docker                                                 1,5 s\n" +
		"  ✕ sistema e dependências                                 1,5 s\n" +
		"    linha 1\n" +
		"    E: pacote curl não encontrado\n"
	assert.Equal(t, esperado, b.String())
	for _, emoji := range []string{"⚙️", "✅", "❌"} {
		assert.NotContains(t, b.String(), emoji)
	}
}

func TestLocalExecutor_Run_ASCII(t *testing.T) {
	var b bytes.Buffer
	e := &LocalExecutor{
		Etapas:   progressoTexto(t, &b, true),
		Executar: func(string) ([]byte, error) { return nil, nil },
	}
	assert.NoError(t, e.Run("firewall (ufw)", ""))
	assert.Equal(t, "  v firewall (ufw)                                         1,5 s\n", b.String())
}

func TestLocalExecutor_EtapasPadrao(t *testing.T) {
	e := &LocalExecutor{}
	assert.NotNil(t, e.etapas(), "sem Etapas, o executor cria um ui.Progresso em os.Stdout")
	assert.Same(t, e.etapas(), e.etapas(), "as etapas padrão são criadas uma vez só")

	s := &SSHExecutor{}
	assert.NotNil(t, s.etapas())
}

func TestLocalExecutor_FetchFile_PermissaoNegadaUsaSudo(t *testing.T) {
	var b bytes.Buffer
	e := &LocalExecutor{
		Etapas:     progressoTexto(t, &b, false),
		LerArquivo: func(string) ([]byte, error) { return nil, fs.ErrPermission },
		LerComSudo: func(path string) ([]byte, error) { return []byte("kubeconfig de " + path), nil },
	}
	dados, err := e.FetchFile("/etc/rancher/k3s/k3s.yaml")
	assert.NoError(t, err)
	assert.Equal(t, "kubeconfig de /etc/rancher/k3s/k3s.yaml", string(dados))
	assert.Equal(t, "    permissão negada em /etc/rancher/k3s/k3s.yaml · lendo com sudo\n", b.String())
}

func TestLocalExecutor_FetchFile_SudoPadrao(t *testing.T) {
	// sem LerComSudo, o executor chama sudo cat; aqui só confere que o caminho é seguido
	// sem pânico (o sudo pode não existir no ambiente de teste)
	e := &LocalExecutor{
		Etapas:     progressoTexto(t, io.Discard, false),
		LerArquivo: func(string) ([]byte, error) { return nil, fs.ErrPermission },
	}
	_, _ = e.FetchFile("/arquivo/de/teste")
}

func TestSSHExecutor_Run_SessaoFalhaMostraEtapa(t *testing.T) {
	var b bytes.Buffer
	e := &SSHExecutor{
		client: &mockSSHClient{newSessionFunc: func() (*ssh.Session, error) {
			return nil, fmt.Errorf("falha ao criar sessão SSH")
		}},
		Etapas: progressoTexto(t, &b, false),
	}
	err := e.Run("k3s", "true")
	assert.Error(t, err)
	assert.Equal(t, "  ✕ k3s                                                    1,5 s\n", b.String())
}

// capturarLog troca o log padrão por um em debug que escreve no buffer devolvido.
func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	anterior := slog.Default()
	t.Cleanup(func() { slog.SetDefault(anterior) })
	var b bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&b, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return &b
}

func TestLocalExecutor_Run_SaidaInteiraNoLog(t *testing.T) {
	log := capturarLog(t)
	var linhas []string
	for i := 1; i <= 20; i++ {
		linhas = append(linhas, fmt.Sprintf("linha %d", i))
	}
	var b bytes.Buffer
	e := &LocalExecutor{
		Etapas: progressoTexto(t, &b, false),
		Executar: func(string) ([]byte, error) {
			return []byte(strings.Join(linhas, "\n")), fmt.Errorf("exit status 1")
		},
	}
	assert.Error(t, e.Run("k3s", "instala"))
	// na tela, só as 12 últimas; no log em debug, a saída inteira
	assert.NotContains(t, b.String(), "linha 8\n")
	assert.Contains(t, b.String(), "linha 9\n")
	assert.Contains(t, log.String(), "saída do script")
	assert.Contains(t, log.String(), "etapa=k3s")
	assert.Contains(t, log.String(), "linha 1\\n")
}

func TestLocalExecutor_FetchFile_SudoPausaORedesenho(t *testing.T) {
	opcoes := ui.Atual()
	t.Cleanup(func() { ui.Configurar(opcoes) })
	ui.Configurar(ui.Opcoes{ASCII: true})
	var b bytes.Buffer
	parado := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	p := ui.NovoProgressoCom(&b, ui.OpcoesProgresso{Terminal: func(io.Writer) bool { return true }, Largura: 64,
		Relogio: func() time.Time { return parado }}, "kubeconfig")
	p.Iniciar("kubeconfig")
	var duranteOSudo string
	e := &LocalExecutor{
		Etapas:     p,
		LerArquivo: func(string) ([]byte, error) { return nil, fs.ErrPermission },
		LerComSudo: func(string) ([]byte, error) {
			p.Tique() // o tique do Animar não redesenha durante o pedido de senha
			duranteOSudo = b.String()
			return []byte("ok"), nil
		},
	}
	_, err := e.FetchFile("/etc/rancher/k3s/k3s.yaml")
	assert.NoError(t, err)
	// antes do sudo, o trecho vivo sai da tela: o pedido de senha fica abaixo do detalhe
	assert.True(t, strings.HasSuffix(duranteOSudo, "lendo com sudo\n  + kubeconfig\n\x1b[1A\r\x1b[J"), "%q", duranteOSudo)
	// depois do sudo, a etapa volta uma vez
	assert.Equal(t, "  + kubeconfig\n", b.String()[len(duranteOSudo):])
}

func TestUltimasLinhas(t *testing.T) {
	var linhas []string
	for i := 1; i <= 20; i++ {
		linhas = append(linhas, fmt.Sprintf("l%d", i))
	}
	got := ultimasLinhas(strings.Join(linhas, "\r\n"), 3)
	assert.Equal(t, "l18\nl19\nl20", got)
	assert.Equal(t, "", ultimasLinhas("\n\n", 3))
}
