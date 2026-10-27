package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// TrustedPluginEntry representa uma entrada de plugin confiável no registro.
type TrustedPluginEntry struct {
	SHA256    string    `json:"sha256"`
	TrustedAt time.Time `json:"trusted_at"`
	// Path, Size e ModTime são o carimbo do binário quando o hash foi conferido: com o
	// mesmo caminho, tamanho e mtime, a descoberta o dá por confiável sem ler o arquivo
	// inteiro. Registros antigos não têm o carimbo.
	Path    string    `json:"path,omitempty"`
	Size    int64     `json:"size,omitempty"`
	ModTime time.Time `json:"mtime,omitzero"`
}

// temCarimbo diz se a entrada guarda o carimbo (caminho, tamanho e mtime) do binário.
func (e TrustedPluginEntry) temCarimbo() bool {
	return e.Path != "" && !e.ModTime.IsZero()
}

// bateCarimbo diz se o binário em caminho, com info, é o mesmo do carimbo da entrada.
func (e TrustedPluginEntry) bateCarimbo(caminho string, info os.FileInfo) bool {
	return e.temCarimbo() && mesmoCaminho(e.Path, caminho) && e.Size == info.Size() && e.ModTime.Equal(info.ModTime())
}

// mesmoCaminho compara dois caminhos de binário: no Windows, sem diferenciar maiúsculas,
// como o sistema de arquivos.
func mesmoCaminho(a, b string) bool {
	if registroSemCaixa {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// carimbar grava na entrada o carimbo do binário em caminho, com info.
func (e *TrustedPluginEntry) carimbar(caminho string, info os.FileInfo) {
	e.Path, e.Size, e.ModTime = caminho, info.Size(), info.ModTime().UTC()
}

// caminhoAbsoluto devolve o caminho absoluto do binário (ou o próprio, se não der).
func caminhoAbsoluto(binaryPath string) string {
	if abs, err := filepath.Abs(binaryPath); err == nil {
		return abs
	}
	return binaryPath
}

// TrustRegistry representa o arquivo de registro de plugins confiáveis.
type TrustRegistry struct {
	Plugins map[string]TrustedPluginEntry `json:"plugins"`
}

// trustRegistryPath retorna o caminho do arquivo trusted.json.
// Variável de pacote para permitir override em testes.
var trustRegistryPath = defaultTrustRegistryPath

func defaultTrustRegistryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("falha ao obter diretório home: %w", err)
	}
	return filepath.Join(home, ".yby", "plugins", "trusted.json"), nil
}

var trustMu sync.Mutex

// loadTrustRegistry carrega o registro de plugins confiáveis do disco.
func loadTrustRegistry() (*TrustRegistry, error) {
	path, err := trustRegistryPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &TrustRegistry{Plugins: make(map[string]TrustedPluginEntry)}, nil
		}
		return nil, fmt.Errorf("falha ao ler registro de confiança: %w", err)
	}

	var registry TrustRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("falha ao analisar registro de confiança: %w", err)
	}

	if registry.Plugins == nil {
		registry.Plugins = make(map[string]TrustedPluginEntry)
	}

	return &registry, nil
}

// saveTrustRegistry salva o registro de plugins confiáveis no disco. Grava num arquivo
// temporário da mesma pasta e o troca pelo registro com os.Rename (que no Windows substitui
// o destino): quem lê o registro, inclusive outro processo do yby, vê o arquivo antigo ou o
// novo inteiro, nunca um JSON pela metade, e um processo morto no meio da gravação deixa o
// registro como estava.
func saveTrustRegistry(registry *TrustRegistry) error {
	path, err := trustRegistryPath()
	if err != nil {
		return err
	}

	// Garantir que o diretório existe
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("falha ao criar diretório de plugins: %w", err)
	}

	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("falha ao serializar registro de confiança: %w", err)
	}

	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("falha ao salvar registro de confiança: %w", err)
	}
	trocado := false
	defer func() {
		if !trocado {
			_ = temp.Close()
			_ = os.Remove(temp.Name())
		}
	}()
	// o CreateTemp já cria com 0600; o Chmod garante mesmo com outro umask
	if err := temp.Chmod(0600); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("falha ao salvar registro de confiança: %w", err)
	}
	if err := escreverRegistro(temp, data); err != nil {
		return fmt.Errorf("falha ao salvar registro de confiança: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("falha ao salvar registro de confiança: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("falha ao salvar registro de confiança: %w", err)
	}
	if err := trocarArquivo(temp.Name(), path); err != nil {
		return fmt.Errorf("falha ao salvar registro de confiança: %w", err)
	}
	trocado = true
	return nil
}

// escreverRegistro escreve o registro serializado no arquivo temporário. Variável para os
// testes simularem uma gravação interrompida.
var escreverRegistro = func(f *os.File, data []byte) error {
	_, err := f.Write(data)
	return err
}

// trocarArquivo põe temp no lugar de destino. No Windows a troca falha enquanto outro
// processo está com o destino aberto (outro yby lendo o registro, por exemplo): tenta de
// novo algumas vezes antes de desistir.
func trocarArquivo(temp, destino string) error {
	var err error
	for tentativa := 0; tentativa < 10; tentativa++ {
		if err = os.Rename(temp, destino); err == nil {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return err
}

// calcularSHA256 lê o arquivo inteiro e calcula o SHA256. Variável para os testes contarem
// as leituras.
var calcularSHA256 = computeFileSHA256

// computeFileSHA256 calcula o hash SHA256 de um arquivo.
func computeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("falha ao abrir arquivo para checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("falha ao calcular checksum: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// nomeDoArquivo devolve o nome do plugin tirado do arquivo do binário: sem o prefixo
// yby-plugin- e sem a extensão .exe (yby-plugin-sentinel.exe vira sentinel). É o nome do
// plugin não confiável na descoberta, e o que o yby plugin trust aceita para ele.
func nomeDoArquivo(binaryPath string) string {
	nome := filepath.Base(binaryPath)
	if ext := filepath.Ext(nome); strings.EqualFold(ext, ".exe") {
		nome = strings.TrimSuffix(nome, ext)
	}
	return strings.TrimPrefix(nome, "yby-plugin-")
}

// TrustPlugin registra um plugin como confiável, calculando seu SHA256.
func TrustPlugin(binaryPath string) error {
	trustMu.Lock()
	defer trustMu.Unlock()

	// o carimbo sai antes do hash: se o arquivo mudar durante a leitura, o mtime do
	// carimbo fica para trás e a descoberta recalcula o hash
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("falha ao ler o binário do plugin: %w", err)
	}
	hash, err := calcularSHA256(binaryPath)
	if err != nil {
		return err
	}

	registry, err := loadTrustRegistry()
	if err != nil {
		return err
	}

	name := filepath.Base(binaryPath)
	entrada := TrustedPluginEntry{
		SHA256:    hash,
		TrustedAt: time.Now().UTC(),
	}
	entrada.carimbar(caminhoAbsoluto(binaryPath), info)
	registry.Plugins[name] = entrada

	slog.Info("Plugin registrado como confiável", "nome", name, "sha256", hash)
	return saveTrustRegistry(registry)
}

// IsTrusted verifica se um plugin está na whitelist e se o checksum bate.
func IsTrusted(binaryPath string) (bool, error) {
	trustMu.Lock()
	defer trustMu.Unlock()

	registry, err := loadTrustRegistry()
	if err != nil {
		return false, err
	}

	name := filepath.Base(binaryPath)
	entry, exists := registry.Plugins[name]
	if !exists {
		return false, nil
	}

	// Verificar checksum atual contra o registrado
	currentHash, err := calcularSHA256(binaryPath)
	if err != nil {
		return false, err
	}

	if currentHash != entry.SHA256 {
		slog.Warn("Checksum do plugin não confere com o registrado",
			"nome", name,
			"esperado", entry.SHA256,
			"atual", currentHash,
		)
		return false, nil
	}

	return true, nil
}

// confiavelNaDescoberta é o IsTrusted da descoberta, que roda em todo comando do yby (e no
// tab): com o caminho, o tamanho e o mtime do carimbo, dá o binário por confiável sem ler
// o arquivo inteiro; sem carimbo ou com ele diferente, calcula o hash como o IsTrusted. A
// verificação do executor, antes de rodar o command, o context e o assets, usa o
// IsTrusted, que sempre calcula o hash.
//
// Com o hash conferido, a entrada sem carimbo (registro antigo) ou com o carimbo deste
// mesmo caminho ganha o carimbo novo. O de outro caminho com o mesmo nome de arquivo (o
// mesmo plugin em ~/.yby/plugins e em .yby/plugins) fica como está.
func confiavelNaDescoberta(binaryPath string) (bool, error) {
	trustMu.Lock()
	defer trustMu.Unlock()

	registry, err := loadTrustRegistry()
	if err != nil {
		return false, err
	}

	name := filepath.Base(binaryPath)
	entry, exists := registry.Plugins[name]
	if !exists {
		return false, nil
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		return false, fmt.Errorf("falha ao ler o binário do plugin: %w", err)
	}
	caminho := caminhoAbsoluto(binaryPath)
	if entry.bateCarimbo(caminho, info) {
		return true, nil
	}

	currentHash, err := calcularSHA256(binaryPath)
	if err != nil {
		return false, err
	}
	if currentHash != entry.SHA256 {
		slog.Warn("Checksum do plugin não confere com o registrado",
			"nome", name,
			"esperado", entry.SHA256,
			"atual", currentHash,
		)
		return false, nil
	}

	if err := gravarCarimbo(name, currentHash, caminho, info); err != nil {
		// sem o carimbo, a próxima descoberta só calcula o hash de novo
		slog.Debug("Falha ao gravar o carimbo do plugin confiável", "nome", name, "error", err)
	}
	return true, nil
}

// gravarCarimbo grava o carimbo do binário em caminho na entrada name do registro, depois
// de o hash dele bater com hash. O trustMu só protege dentro do processo, e o hash demora:
// o registro é lido de novo do disco e só a entrada name muda, e só se ela ainda existir
// com o mesmo hash (um yby plugin untrust ou trust de outro processo no meio não é
// desfeito) e sem o carimbo de outro caminho.
func gravarCarimbo(name, hash, caminho string, info os.FileInfo) error {
	registry, err := loadTrustRegistry()
	if err != nil {
		return err
	}
	entry, exists := registry.Plugins[name]
	if !exists || entry.SHA256 != hash {
		return nil
	}
	if entry.temCarimbo() && !mesmoCaminho(entry.Path, caminho) {
		return nil
	}
	if entry.bateCarimbo(caminho, info) {
		return nil
	}
	entry.carimbar(caminho, info)
	registry.Plugins[name] = entry
	return saveTrustRegistry(registry)
}

// registroSemCaixa diz se o nome do arquivo no registro e o caminho do carimbo valem sem
// diferenciar maiúsculas (e o nome, com o .exe implícito), como no sistema de arquivos do
// Windows. Variável para os testes.
var registroSemCaixa = runtime.GOOS == "windows"

// chavesDoRegistro acha as entradas de name no registro: o nome exato e, no Windows, todas
// as que batem sem diferenciar maiúsculas, com ou sem o .exe (yby-plugin-Sentinel acha
// yby-plugin-sentinel.EXE e yby-plugin-SENTINEL.exe).
func chavesDoRegistro(registry *TrustRegistry, name string) []string {
	if !registroSemCaixa {
		if _, ok := registry.Plugins[name]; ok {
			return []string{name}
		}
		return nil
	}
	var chaves []string
	for chave := range registry.Plugins {
		if strings.EqualFold(chave, name) || strings.EqualFold(chave, name+".exe") {
			chaves = append(chaves, chave)
		}
	}
	sort.Strings(chaves)
	return chaves
}

// UntrustPlugin remove um plugin do registro de confiança. name é o nome do arquivo do
// binário; no Windows, sem diferenciar maiúsculas e com o .exe opcional, e sai toda entrada
// que bate.
func UntrustPlugin(name string) error {
	trustMu.Lock()
	defer trustMu.Unlock()

	registry, err := loadTrustRegistry()
	if err != nil {
		return err
	}

	chaves := chavesDoRegistro(registry, name)
	if len(chaves) == 0 {
		return fmt.Errorf("plugin '%s' não está no registro de confiança", name)
	}

	for _, chave := range chaves {
		delete(registry.Plugins, chave)
		slog.Info("Plugin removido do registro de confiança", "nome", chave)
	}
	return saveTrustRegistry(registry)
}
