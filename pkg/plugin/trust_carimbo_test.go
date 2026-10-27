package plugin

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contarHashes troca o calcularSHA256 por um que conta as leituras do binário inteiro.
func contarHashes(t *testing.T) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	original := calcularSHA256
	calcularSHA256 = func(path string) (string, error) {
		n.Add(1)
		return original(path)
	}
	t.Cleanup(func() { calcularSHA256 = original })
	return &n
}

// descobrir roda a descoberta com a verificação de confiança e devolve o plugin manifesto.
func descobrir(t *testing.T) *LoadedPlugin {
	t.Helper()
	registrarExecucoes(t)
	m := NewManager()
	m.EnableTrustCheck()
	require.NoError(t, m.Discover())
	p, achado := m.GetPlugin("manifesto")
	require.True(t, achado)
	return p
}

// entradaDoRegistro lê do registro a entrada do arquivo nome.
func entradaDoRegistro(t *testing.T, nome string) TrustedPluginEntry {
	t.Helper()
	registro, err := loadTrustRegistry()
	require.NoError(t, err)
	entrada, ok := registro.Plugins[nome]
	require.True(t, ok, "a entrada de %s some do registro", nome)
	return entrada
}

func TestDescoberta_ConfiavelComCarimboNaoLeOBinarioInteiro(t *testing.T) {
	binario := projetoClonado(t, "yby-plugin-manifesto.exe")
	require.NoError(t, TrustPlugin(binario))
	entrada := entradaDoRegistro(t, "yby-plugin-manifesto.exe")
	info, err := os.Stat(binario)
	require.NoError(t, err)
	abs, err := filepath.Abs(binario)
	require.NoError(t, err)
	assert.Equal(t, abs, entrada.Path, "o trust guarda o caminho do binário")
	assert.Equal(t, info.Size(), entrada.Size, "o trust guarda o tamanho")
	assert.True(t, info.ModTime().Equal(entrada.ModTime), "o trust guarda o mtime")

	hashes := contarHashes(t)
	p := descobrir(t)
	assert.False(t, p.Manifest.NaoConfiavel)
	assert.Equal(t, int32(0), hashes.Load(), "com tamanho e mtime iguais, a descoberta não calcula o hash")

	// a verificação do executor continua calculando o hash inteiro
	confiavel, err := IsTrusted(binario)
	require.NoError(t, err)
	assert.True(t, confiavel)
	assert.Equal(t, int32(1), hashes.Load(), "o IsTrusted sempre calcula o hash")

	// mtime diferente, mesmo conteúdo: recalcula, continua confiável e renova o carimbo
	depois := info.ModTime().Add(time.Hour)
	require.NoError(t, os.Chtimes(binario, depois, depois))
	hashes.Store(0)
	assert.False(t, descobrir(t).Manifest.NaoConfiavel)
	assert.Equal(t, int32(1), hashes.Load(), "com o mtime diferente, a descoberta calcula o hash")
	assert.True(t, depois.Equal(entradaDoRegistro(t, "yby-plugin-manifesto.exe").ModTime), "o carimbo novo é gravado")
	descobrir(t)
	assert.Equal(t, int32(1), hashes.Load(), "com o carimbo renovado, a próxima descoberta não lê o binário")

	// conteúdo e tamanho diferentes: recalcula e deixa de ser confiável
	require.NoError(t, os.WriteFile(binario, []byte("binário trocado por outro maior"), 0o755))
	hashes.Store(0)
	assert.True(t, descobrir(t).Manifest.NaoConfiavel, "o binário trocado não é confiável")
	assert.Equal(t, int32(1), hashes.Load(), "com o tamanho diferente, a descoberta calcula o hash")
}

func TestDescoberta_RegistroAntigoSemCarimboGanhaOCarimbo(t *testing.T) {
	binario := projetoClonado(t, "yby-plugin-manifesto.exe")
	require.NoError(t, TrustPlugin(binario))
	// o registro de antes do carimbo: só o hash e a data
	registro, err := loadTrustRegistry()
	require.NoError(t, err)
	antiga := registro.Plugins["yby-plugin-manifesto.exe"]
	registro.Plugins["yby-plugin-manifesto.exe"] = TrustedPluginEntry{SHA256: antiga.SHA256, TrustedAt: antiga.TrustedAt}
	require.NoError(t, saveTrustRegistry(registro))

	hashes := contarHashes(t)
	assert.False(t, descobrir(t).Manifest.NaoConfiavel, "o registro antigo continua valendo")
	assert.Equal(t, int32(1), hashes.Load(), "sem carimbo, a descoberta calcula o hash")
	assert.True(t, entradaDoRegistro(t, "yby-plugin-manifesto.exe").temCarimbo(), "o hash bateu: o registro ganha o carimbo")

	descobrir(t)
	assert.Equal(t, int32(1), hashes.Load(), "com o carimbo, a próxima descoberta não lê o binário")
}

func TestDescoberta_CarimboDeOutroCaminhoFicaComoEsta(t *testing.T) {
	binario := projetoClonado(t, "yby-plugin-manifesto.exe")
	require.NoError(t, TrustPlugin(binario))
	// o mesmo nome de arquivo confiado em outra pasta (~/.yby/plugins, por exemplo)
	registro, err := loadTrustRegistry()
	require.NoError(t, err)
	entrada := registro.Plugins["yby-plugin-manifesto.exe"]
	outro := filepath.Join(t.TempDir(), "yby-plugin-manifesto.exe")
	entrada.Path = outro
	registro.Plugins["yby-plugin-manifesto.exe"] = entrada
	require.NoError(t, saveTrustRegistry(registro))

	hashes := contarHashes(t)
	assert.False(t, descobrir(t).Manifest.NaoConfiavel, "o hash bate: confiável")
	assert.Equal(t, int32(1), hashes.Load(), "o carimbo é de outro caminho: calcula o hash")
	assert.Equal(t, outro, entradaDoRegistro(t, "yby-plugin-manifesto.exe").Path, "o carimbo do outro caminho não é trocado")
}

// semCaixa troca o registroSemCaixa (o Windows) até o fim do teste.
func semCaixa(t *testing.T, valor bool) {
	t.Helper()
	anterior := registroSemCaixa
	registroSemCaixa = valor
	t.Cleanup(func() { registroSemCaixa = anterior })
}

// tirarCarimbo deixa a entrada nome como num registro antigo, só com o hash e a data.
func tirarCarimbo(t *testing.T, nome string) {
	t.Helper()
	registro, err := loadTrustRegistry()
	require.NoError(t, err)
	antiga := registro.Plugins[nome]
	registro.Plugins[nome] = TrustedPluginEntry{SHA256: antiga.SHA256, TrustedAt: antiga.TrustedAt}
	require.NoError(t, saveTrustRegistry(registro))
}

// noMeioDoHash roda outroProcesso depois de a descoberta calcular o hash e antes de ela
// gravar o carimbo, como um yby de outro processo, que não passa pelo trustMu.
func noMeioDoHash(t *testing.T, outroProcesso func()) {
	t.Helper()
	original := calcularSHA256
	calcularSHA256 = func(path string) (string, error) {
		hash, err := original(path)
		outroProcesso()
		return hash, err
	}
	t.Cleanup(func() { calcularSHA256 = original })
}

func TestDescoberta_CarimboNaoDesfazOQueOutroProcessoGravou(t *testing.T) {
	t.Run("untrust no meio do hash não é desfeito", func(t *testing.T) {
		binario := projetoClonado(t, "yby-plugin-manifesto.exe")
		require.NoError(t, TrustPlugin(binario))
		tirarCarimbo(t, "yby-plugin-manifesto.exe")
		noMeioDoHash(t, func() {
			registro, err := loadTrustRegistry()
			require.NoError(t, err)
			delete(registro.Plugins, "yby-plugin-manifesto.exe")
			require.NoError(t, saveTrustRegistry(registro))
		})

		assert.False(t, descobrir(t).Manifest.NaoConfiavel, "a descoberta segue com o resultado do hash")
		registro, err := loadTrustRegistry()
		require.NoError(t, err)
		assert.NotContains(t, registro.Plugins, "yby-plugin-manifesto.exe", "o carimbo não traz de volta a entrada tirada")
		confiavel, err := IsTrusted(binario)
		require.NoError(t, err)
		assert.False(t, confiavel, "o executor não roda o plugin tirado do registro")
	})

	t.Run("trust com outro hash no meio não ganha o carimbo", func(t *testing.T) {
		binario := projetoClonado(t, "yby-plugin-manifesto.exe")
		require.NoError(t, TrustPlugin(binario))
		tirarCarimbo(t, "yby-plugin-manifesto.exe")
		noMeioDoHash(t, func() {
			registro, err := loadTrustRegistry()
			require.NoError(t, err)
			registro.Plugins["yby-plugin-manifesto.exe"] = TrustedPluginEntry{SHA256: "outro", TrustedAt: time.Now()}
			require.NoError(t, saveTrustRegistry(registro))
		})

		descobrir(t)
		entrada := entradaDoRegistro(t, "yby-plugin-manifesto.exe")
		assert.Equal(t, "outro", entrada.SHA256, "a entrada do outro processo fica como ele gravou")
		assert.False(t, entrada.temCarimbo(), "o carimbo do hash antigo não vai para a entrada nova")
	})

	t.Run("o carimbo muda só a entrada do plugin", func(t *testing.T) {
		binario := projetoClonado(t, "yby-plugin-manifesto.exe")
		require.NoError(t, TrustPlugin(binario))
		tirarCarimbo(t, "yby-plugin-manifesto.exe")
		noMeioDoHash(t, func() {
			registro, err := loadTrustRegistry()
			require.NoError(t, err)
			registro.Plugins["yby-plugin-outro.exe"] = TrustedPluginEntry{SHA256: "abc", TrustedAt: time.Now()}
			require.NoError(t, saveTrustRegistry(registro))
		})

		descobrir(t)
		assert.True(t, entradaDoRegistro(t, "yby-plugin-manifesto.exe").temCarimbo(), "o hash bateu: a entrada ganha o carimbo")
		assert.Equal(t, "abc", entradaDoRegistro(t, "yby-plugin-outro.exe").SHA256, "o trust do outro processo não se perde")
	})
}

func TestSaveTrustRegistry_GravacaoInterrompidaNaoTruncaORegistro(t *testing.T) {
	binario := projetoClonado(t, "yby-plugin-manifesto.exe")
	require.NoError(t, TrustPlugin(binario))
	caminho, err := trustRegistryPath()
	require.NoError(t, err)
	antes, err := os.ReadFile(caminho)
	require.NoError(t, err)

	original := escreverRegistro
	escreverRegistro = func(f *os.File, data []byte) error {
		// o processo morre no meio: metade do JSON e nada mais
		if _, err := f.Write(data[:len(data)/2]); err != nil {
			return err
		}
		return errors.New("processo interrompido")
	}
	t.Cleanup(func() { escreverRegistro = original })

	assert.Error(t, UntrustPlugin("yby-plugin-manifesto.exe"))
	depois, err := os.ReadFile(caminho)
	require.NoError(t, err)
	assert.Equal(t, string(antes), string(depois), "o registro fica inteiro, como estava")
	confiavel, err := IsTrusted(binario)
	require.NoError(t, err)
	assert.True(t, confiavel, "o registro continua legível")

	sobras, err := filepath.Glob(filepath.Join(filepath.Dir(caminho), "*.tmp"))
	require.NoError(t, err)
	assert.Empty(t, sobras, "o temporário da gravação interrompida é apagado")

	// a gravação seguinte troca o registro inteiro
	escreverRegistro = original
	require.NoError(t, UntrustPlugin("yby-plugin-manifesto.exe"))
	registro, err := loadTrustRegistry()
	require.NoError(t, err)
	assert.Empty(t, registro.Plugins)
	info, err := os.Stat(caminho)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "o registro continua só do dono")
	}
}

func TestDescoberta_CaminhoDoCarimboSemDiferenciarMaiusculasNoWindows(t *testing.T) {
	// o carimbo com o caminho em outra caixa, como o Windows aceita
	carimboEmOutraCaixa := func(t *testing.T) string {
		binario := projetoClonado(t, "yby-plugin-manifesto.exe")
		require.NoError(t, TrustPlugin(binario))
		registro, err := loadTrustRegistry()
		require.NoError(t, err)
		entrada := registro.Plugins["yby-plugin-manifesto.exe"]
		entrada.Path = strings.ToUpper(entrada.Path)
		registro.Plugins["yby-plugin-manifesto.exe"] = entrada
		require.NoError(t, saveTrustRegistry(registro))
		return binario
	}

	t.Run("windows confere sem diferenciar maiúsculas", func(t *testing.T) {
		semCaixa(t, true)
		binario := carimboEmOutraCaixa(t)
		hashes := contarHashes(t)
		assert.False(t, descobrir(t).Manifest.NaoConfiavel)
		assert.Equal(t, int32(0), hashes.Load(), "o carimbo é deste caminho: não lê o binário")

		// mtime diferente: o carimbo é deste caminho, então é renovado
		info, err := os.Stat(binario)
		require.NoError(t, err)
		depois := info.ModTime().Add(time.Hour)
		require.NoError(t, os.Chtimes(binario, depois, depois))
		assert.False(t, descobrir(t).Manifest.NaoConfiavel)
		assert.Equal(t, int32(1), hashes.Load())
		assert.True(t, depois.Equal(entradaDoRegistro(t, "yby-plugin-manifesto.exe").ModTime), "o carimbo é renovado")
		descobrir(t)
		assert.Equal(t, int32(1), hashes.Load(), "com o carimbo renovado, não lê o binário")
	})

	t.Run("fora do windows a caixa diferencia", func(t *testing.T) {
		semCaixa(t, false)
		carimboEmOutraCaixa(t)
		hashes := contarHashes(t)
		assert.False(t, descobrir(t).Manifest.NaoConfiavel)
		assert.Equal(t, int32(1), hashes.Load(), "o carimbo é de outro caminho: calcula o hash")
		entrada := entradaDoRegistro(t, "yby-plugin-manifesto.exe")
		assert.Equal(t, strings.ToUpper(entrada.Path), entrada.Path, "o carimbo do outro caminho não é trocado")
	})
}

func TestUntrustPlugin_NoWindowsTiraTodasAsEntradasQueBatem(t *testing.T) {
	semCaixa(t, true)
	projetoClonado(t, "yby-plugin-x.exe")
	registro := &TrustRegistry{Plugins: map[string]TrustedPluginEntry{
		"yby-plugin-X.exe":     {SHA256: "a"},
		"yby-plugin-x.exe":     {SHA256: "b"},
		"yby-plugin-x":         {SHA256: "c"},
		"yby-plugin-outro.exe": {SHA256: "d"},
	}}
	require.NoError(t, saveTrustRegistry(registro))

	require.NoError(t, UntrustPlugin("yby-plugin-x"))
	registro, err := loadTrustRegistry()
	require.NoError(t, err)
	assert.Equal(t, []string{"yby-plugin-outro.exe"}, slices.Sorted(maps.Keys(registro.Plugins)), "saem todas as entradas de x, e só elas")
	assert.Error(t, UntrustPlugin("yby-plugin-X.EXE"), "já saíram todas")
}

func TestUntrustPlugin_NoWindowsSemDiferenciarMaiusculas(t *testing.T) {
	confiar := func(t *testing.T, arquivo string) {
		binario := projetoClonado(t, arquivo)
		require.NoError(t, TrustPlugin(binario))
	}

	t.Run("windows acha com outra caixa e sem o .exe", func(t *testing.T) {
		semCaixa(t, true)
		for _, pedido := range []string{"yby-plugin-sentinel.exe", "YBY-PLUGIN-SENTINEL.EXE", "yby-plugin-Sentinel", "yby-plugin-sentinel"} {
			confiar(t, "yby-plugin-Sentinel.EXE")
			require.NoError(t, UntrustPlugin(pedido), pedido)
			registro, err := loadTrustRegistry()
			require.NoError(t, err)
			assert.Empty(t, registro.Plugins, "%s tira a entrada yby-plugin-Sentinel.EXE", pedido)
		}
	})

	t.Run("fora do windows o nome é exato", func(t *testing.T) {
		semCaixa(t, false)
		confiar(t, "yby-plugin-sentinel")
		assert.Error(t, UntrustPlugin("yby-plugin-Sentinel"))
		assert.Error(t, UntrustPlugin("yby-plugin-sentinel.exe"))
		require.NoError(t, UntrustPlugin("yby-plugin-sentinel"))
	})
}
