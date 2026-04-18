package cloud

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/pbkdf2"
)

// CredentialStore define interface para persistência segura de credenciais.
type CredentialStore interface {
	// Save armazena um valor associado à chave informada.
	Save(key, value string) error
	// Load recupera o valor associado à chave. Retorna erro se não encontrado.
	Load(key string) (string, error)
	// Delete remove a credencial associada à chave.
	Delete(key string) error
}

// --- KeychainStore ---

// KeychainStore persiste credenciais no keychain do sistema operacional via go-keyring.
type KeychainStore struct {
	ServiceName string // ex: "yby-cli"
}

// Save armazena a credencial no keychain do SO.
func (k *KeychainStore) Save(key, value string) error {
	return keyring.Set(k.ServiceName, key, value)
}

// Load recupera a credencial do keychain do SO.
func (k *KeychainStore) Load(key string) (string, error) {
	return keyring.Get(k.ServiceName, key)
}

// Delete remove a credencial do keychain do SO.
func (k *KeychainStore) Delete(key string) error {
	return keyring.Delete(k.ServiceName, key)
}

// --- EncryptedFileStore ---

const (
	// saltSize é o tamanho do salt para derivação de chave PBKDF2.
	saltSize = 32
	// nonceSize é o tamanho do nonce para AES-256-GCM.
	nonceSize = 12
	// pbkdf2Iterations é o número de iterações para derivação de chave.
	pbkdf2Iterations = 100000
	// aesKeySize é o tamanho da chave AES-256 em bytes.
	aesKeySize = 32
)

// PassphraseProvider é uma função que obtém a passphrase para encriptação.
// Permite injeção de dependência para testes e uso interativo.
type PassphraseProvider func() (string, error)

// EncryptedFileStore persiste credenciais em arquivo encriptado com AES-256-GCM.
// Usado como fallback quando o keychain do SO não está disponível.
type EncryptedFileStore struct {
	FilePath           string
	PassphraseProvider PassphraseProvider
	mu                 sync.Mutex
}

// Save armazena a credencial no arquivo encriptado.
func (e *EncryptedFileStore) Save(key, value string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	data, err := e.loadRawData()
	if err != nil {
		data = make(map[string]string)
	}

	data[key] = value
	return e.saveRawData(data)
}

// Load recupera a credencial do arquivo encriptado.
func (e *EncryptedFileStore) Load(key string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	data, err := e.loadRawData()
	if err != nil {
		return "", err
	}

	value, ok := data[key]
	if !ok {
		return "", fmt.Errorf("credencial não encontrada: %s", key)
	}
	return value, nil
}

// Delete remove a credencial do arquivo encriptado.
func (e *EncryptedFileStore) Delete(key string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	data, err := e.loadRawData()
	if err != nil {
		return err
	}

	delete(data, key)
	return e.saveRawData(data)
}

// loadRawData lê e decripta o arquivo, retornando o mapa de credenciais.
func (e *EncryptedFileStore) loadRawData() (map[string]string, error) {
	raw, err := os.ReadFile(e.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("arquivo de credenciais não encontrado: %s", e.FilePath)
		}
		return nil, fmt.Errorf("erro ao ler arquivo de credenciais: %w", err)
	}

	// Formato: salt (32 bytes) + nonce (12 bytes) + ciphertext
	minSize := saltSize + nonceSize + 1
	if len(raw) < minSize {
		return nil, fmt.Errorf("arquivo de credenciais corrompido: tamanho insuficiente")
	}

	salt := raw[:saltSize]
	nonce := raw[saltSize : saltSize+nonceSize]
	ciphertext := raw[saltSize+nonceSize:]

	passphrase, err := e.getPassphrase()
	if err != nil {
		return nil, err
	}

	key := deriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cipher AES: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("falha ao decriptar credenciais (passphrase incorreta ou arquivo corrompido): %w", err)
	}

	var data map[string]string
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return nil, fmt.Errorf("erro ao decodificar credenciais: %w", err)
	}

	return data, nil
}

// saveRawData encripta e salva o mapa de credenciais no arquivo.
func (e *EncryptedFileStore) saveRawData(data map[string]string) error {
	plaintext, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("erro ao codificar credenciais: %w", err)
	}

	passphrase, err := e.getPassphrase()
	if err != nil {
		return err
	}

	// Gerar salt aleatório
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("erro ao gerar salt: %w", err)
	}

	key := deriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("erro ao criar cipher AES: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("erro ao criar GCM: %w", err)
	}

	// Gerar nonce aleatório
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("erro ao gerar nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Formato: salt + nonce + ciphertext
	output := make([]byte, 0, saltSize+nonceSize+len(ciphertext))
	output = append(output, salt...)
	output = append(output, nonce...)
	output = append(output, ciphertext...)

	// Garantir que o diretório existe
	if err := os.MkdirAll(filepath.Dir(e.FilePath), 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório: %w", err)
	}

	// Escrever com permissões restritas (0600)
	if err := os.WriteFile(e.FilePath, output, 0600); err != nil {
		return fmt.Errorf("erro ao salvar arquivo de credenciais: %w", err)
	}

	return nil
}

// getPassphrase obtém a passphrase via provider ou variável de ambiente.
func (e *EncryptedFileStore) getPassphrase() (string, error) {
	// Prioridade: env var > provider
	if p := os.Getenv("YBY_CREDENTIAL_PASSPHRASE"); p != "" {
		return p, nil
	}

	if e.PassphraseProvider != nil {
		return e.PassphraseProvider()
	}

	return "", fmt.Errorf("passphrase não configurada: defina YBY_CREDENTIAL_PASSPHRASE ou forneça um PassphraseProvider")
}

// deriveKey deriva uma chave AES-256 a partir de passphrase e salt via PBKDF2.
func deriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, aesKeySize, sha256.New)
}

// --- Factory ---

// NewCredentialStore cria o CredentialStore mais seguro disponível.
// Tenta usar o keychain do SO primeiro; se indisponível, retorna EncryptedFileStore.
func NewCredentialStore() CredentialStore {
	ks := &KeychainStore{ServiceName: "yby-cli"}
	if err := ks.Save("__yby_test__", "test"); err == nil {
		_ = ks.Delete("__yby_test__")
		slog.Debug("credential_store.backend", "tipo", "keychain")
		return ks
	}

	home, _ := os.UserHomeDir()
	slog.Debug("credential_store.backend", "tipo", "encrypted_file")
	return &EncryptedFileStore{
		FilePath: filepath.Join(home, ".yby", "credentials.enc"),
	}
}
