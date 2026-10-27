package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

// homeEPastaIsoladas põe HOME (e o USERPROFILE do Windows) e o diretório atual em pastas
// temporárias: nem o ~/.yby nem o .yby/plugins de verdade entram no teste.
func homeEPastaIsoladas(t *testing.T) (home, cwd string) {
	t.Helper()
	home, cwd = t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Chdir(cwd)
	return home, cwd
}

// binarioFalso cria o arquivo do plugin nome em dir/.yby/plugins, com o nome deste sistema.
func binarioFalso(t *testing.T, dir, nome string) string {
	t.Helper()
	pasta := filepath.Join(dir, ".yby", "plugins")
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		t.Fatal(err)
	}
	arquivo := filepath.Join(pasta, nomeDoBinario(nome, runtime.GOOS))
	if err := os.WriteFile(arquivo, []byte("binário de "+nome+" em "+dir), 0o755); err != nil {
		t.Fatal(err)
	}
	return arquivo
}

// O arquivo do plugin leva .exe no Windows, como o yby plugin install grava.
func TestNomeDoBinario_PorSistema(t *testing.T) {
	casos := []struct{ goos, want string }{
		{"windows", "yby-plugin-sentinel.exe"},
		{"linux", "yby-plugin-sentinel"},
		{"darwin", "yby-plugin-sentinel"},
	}
	for _, c := range casos {
		if got := nomeDoBinario("sentinel", c.goos); got != c.want {
			t.Errorf("%s: %q, esperado %q", c.goos, got, c.want)
		}
	}
}

// O plugin instalado e confiável em ~/.yby/plugins é achado, com .exe no Windows.
func TestDiscoverPluginBinary_InstaladoNaHome(t *testing.T) {
	home, _ := homeEPastaIsoladas(t)
	arquivo := binarioFalso(t, home, "sentinel")
	if err := plugin.TrustPlugin(arquivo); err != nil {
		t.Fatal(err)
	}

	got, err := discoverPluginBinary("sentinel")
	if err != nil {
		t.Fatalf("o sentinel da home deveria ser achado: %v", err)
	}
	if got != arquivo {
		t.Errorf("caminho %q, esperado %q", got, arquivo)
	}
}

// Um ./.yby/plugins sem confiança (um repo clonado que traz o binário) não roda: o Bard
// confere a confiança como o core.
func TestDiscoverPluginBinary_ProjetoSemConfiancaNaoRoda(t *testing.T) {
	_, cwd := homeEPastaIsoladas(t)
	binarioFalso(t, cwd, "sentinel")

	got, err := discoverPluginBinary("sentinel")
	if err == nil {
		t.Fatalf("o binário do projeto sem confiança não pode rodar, foi devolvido %q", got)
	}
	if !errors.Is(err, errNaoConfiavel) {
		t.Errorf("o erro diz que falta a confiança: %v", err)
	}
	if !strings.Contains(err.Error(), "yby plugin trust sentinel") {
		t.Errorf("o erro ensina o trust: %v", err)
	}
	if _, err := executeSentinelScan(t.Context(), map[string]string{}); err == nil {
		t.Error("o sentinel_scan não pode rodar o binário sem confiança")
	}
}

// O binário do projeto confiável roda, como no core; a home vem primeiro.
func TestDiscoverPluginBinary_ProjetoConfiavelEHomePrimeiro(t *testing.T) {
	home, cwd := homeEPastaIsoladas(t)
	doProjeto := binarioFalso(t, cwd, "atlas")
	if err := plugin.TrustPlugin(doProjeto); err != nil {
		t.Fatal(err)
	}
	if got, err := discoverPluginBinary("atlas"); err != nil || got != doProjeto {
		t.Fatalf("o atlas confiável do projeto: %q %v", got, err)
	}

	// Na home, outro binário com o mesmo nome e sem confiança: ele vence e é recusado,
	// como o core, que carrega o da home e confere a confiança dele.
	binarioFalso(t, home, "atlas")
	if _, err := discoverPluginBinary("atlas"); !errors.Is(err, errNaoConfiavel) {
		t.Errorf("o da home vem primeiro e não é confiável: %v", err)
	}
}

// Sem o plugin, o erro diz que ele não está instalado, e os nomes legados (sem o prefixo
// yby-plugin-) não contam, como no core.
func TestDiscoverPluginBinary_Ausente(t *testing.T) {
	home, _ := homeEPastaIsoladas(t)
	legado := filepath.Join(home, ".yby", "plugins", "sentinel")
	if err := os.MkdirAll(filepath.Dir(legado), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legado, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := discoverPluginBinary("sentinel")
	if !errors.Is(err, errPluginAusente) {
		t.Errorf("o sentinel não está instalado: %v", err)
	}
}

// envPluginLento faz o binário de teste rodar como um plugin que dorme 30 s.
const envPluginLento = "YBY_BARD_TOOLS_PLUGIN_LENTO"

func TestMain(m *testing.M) {
	if os.Getenv(envPluginLento) == "1" {
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if arquivo := os.Getenv(envKubectlFalso); arquivo != "" {
		os.Exit(rodarKubectlFalso(arquivo))
	}
	os.Exit(m.Run())
}

// O esc (cancelamento do ctx) mata o plugin em andamento em vez de esperar ele terminar.
func TestInvokePlugin_CancelamentoMataOPlugin(t *testing.T) {
	t.Setenv(envPluginLento, "1")
	ctx, cancelar := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelar()

	inicio := time.Now()
	_, err := invokePlugin(ctx, os.Args[0], plugin.PluginRequest{Hook: "command"})
	if err == nil {
		t.Fatal("esperava erro do plugin cancelado")
	}
	if d := time.Since(inicio); d > 5*time.Second {
		t.Fatalf("o cancelamento não matou o plugin: voltou em %v", d)
	}
}
