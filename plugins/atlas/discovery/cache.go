package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

// DefaultManifestPath é o caminho padrão do manifesto de cache.
const DefaultManifestPath = ".yby/atlas-manifest.json"

// FileEntry representa uma entrada de arquivo no manifesto de cache.
type FileEntry struct {
	SHA256    string    `json:"sha256"`
	ScannedAt time.Time `json:"scanned_at"`
}

// ScanManifest representa o manifesto de cache do scanner.
type ScanManifest struct {
	Files map[string]FileEntry `json:"files"`
}

// NewScanManifest cria um manifesto vazio.
func NewScanManifest() *ScanManifest {
	return &ScanManifest{
		Files: make(map[string]FileEntry),
	}
}

// LoadManifest carrega o manifesto de cache a partir de um arquivo JSON.
// Retorna um manifesto vazio se o arquivo não existir.
func LoadManifest(path string) (*ScanManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewScanManifest(), nil
		}
		return nil, err
	}

	var manifest ScanManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	if manifest.Files == nil {
		manifest.Files = make(map[string]FileEntry)
	}

	return &manifest, nil
}

// SaveManifest salva o manifesto de cache em um arquivo JSON.
func SaveManifest(path string, manifest *ScanManifest) error {
	// Garantir que o diretório pai existe
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// HashFile calcula o SHA-256 de um arquivo.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// NeedsRescan verifica se um arquivo precisa ser reescaneado comparando o hash atual
// com o hash armazenado no manifesto.
func (m *ScanManifest) NeedsRescan(path string) bool {
	entry, ok := m.Files[path]
	if !ok {
		return true
	}

	currentHash, err := HashFile(path)
	if err != nil {
		return true
	}

	return currentHash != entry.SHA256
}

// Update atualiza a entrada de um arquivo no manifesto com o hash atual.
func (m *ScanManifest) Update(path string) error {
	hash, err := HashFile(path)
	if err != nil {
		return err
	}

	m.Files[path] = FileEntry{
		SHA256:    hash,
		ScannedAt: time.Now(),
	}

	return nil
}

// RemoveStale remove entradas do manifesto cujos arquivos não existem mais.
func (m *ScanManifest) RemoveStale() {
	for path := range m.Files {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			delete(m.Files, path)
		}
	}
}
