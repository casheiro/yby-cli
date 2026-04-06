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
	"sync"
	"time"
)

// TrustedPluginEntry representa uma entrada de plugin confiável no registro.
type TrustedPluginEntry struct {
	SHA256    string    `json:"sha256"`
	TrustedAt time.Time `json:"trusted_at"`
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

// saveTrustRegistry salva o registro de plugins confiáveis no disco.
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

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("falha ao salvar registro de confiança: %w", err)
	}

	return nil
}

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

// TrustPlugin registra um plugin como confiável, calculando seu SHA256.
func TrustPlugin(binaryPath string) error {
	trustMu.Lock()
	defer trustMu.Unlock()

	hash, err := computeFileSHA256(binaryPath)
	if err != nil {
		return err
	}

	registry, err := loadTrustRegistry()
	if err != nil {
		return err
	}

	name := filepath.Base(binaryPath)
	registry.Plugins[name] = TrustedPluginEntry{
		SHA256:    hash,
		TrustedAt: time.Now().UTC(),
	}

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
	currentHash, err := computeFileSHA256(binaryPath)
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

// UntrustPlugin remove um plugin do registro de confiança.
func UntrustPlugin(name string) error {
	trustMu.Lock()
	defer trustMu.Unlock()

	registry, err := loadTrustRegistry()
	if err != nil {
		return err
	}

	if _, exists := registry.Plugins[name]; !exists {
		return fmt.Errorf("plugin '%s' não está no registro de confiança", name)
	}

	delete(registry.Plugins, name)
	slog.Info("Plugin removido do registro de confiança", "nome", name)
	return saveTrustRegistry(registry)
}
