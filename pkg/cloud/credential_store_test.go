package cloud

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestKeychainStore_SaveLoadDelete(t *testing.T) {
	// Usa mock backend do go-keyring para evitar dependência de keychain real
	keyring.MockInit()

	ks := &KeychainStore{ServiceName: "yby-cli-test"}

	// Save
	if err := ks.Save("test-key", "test-value"); err != nil {
		t.Fatalf("Save() erro inesperado: %v", err)
	}

	// Load
	val, err := ks.Load("test-key")
	if err != nil {
		t.Fatalf("Load() erro inesperado: %v", err)
	}
	if val != "test-value" {
		t.Errorf("Load() = %q, esperado %q", val, "test-value")
	}

	// Delete
	if err := ks.Delete("test-key"); err != nil {
		t.Fatalf("Delete() erro inesperado: %v", err)
	}

	// Load após delete
	_, err = ks.Load("test-key")
	if err == nil {
		t.Error("Load() após Delete() deveria retornar erro")
	}
}

func TestKeychainStore_LoadNotFound(t *testing.T) {
	keyring.MockInit()

	ks := &KeychainStore{ServiceName: "yby-cli-test"}
	_, err := ks.Load("chave-inexistente")
	if err == nil {
		t.Error("Load() de chave inexistente deveria retornar erro")
	}
}

func newTestEncryptedStore(t *testing.T, passphrase string) *EncryptedFileStore {
	t.Helper()
	dir := t.TempDir()
	return &EncryptedFileStore{
		FilePath: filepath.Join(dir, "credentials.enc"),
		PassphraseProvider: func() (string, error) {
			return passphrase, nil
		},
	}
}

func TestEncryptedFileStore_SaveLoadDelete(t *testing.T) {
	store := newTestEncryptedStore(t, "minha-senha-segura")

	// Save
	if err := store.Save("aws-token", "abc123"); err != nil {
		t.Fatalf("Save() erro: %v", err)
	}

	// Save segunda credencial
	if err := store.Save("gcp-token", "xyz789"); err != nil {
		t.Fatalf("Save() segundo erro: %v", err)
	}

	// Load primeira
	val, err := store.Load("aws-token")
	if err != nil {
		t.Fatalf("Load() erro: %v", err)
	}
	if val != "abc123" {
		t.Errorf("Load() = %q, esperado %q", val, "abc123")
	}

	// Load segunda
	val, err = store.Load("gcp-token")
	if err != nil {
		t.Fatalf("Load() erro: %v", err)
	}
	if val != "xyz789" {
		t.Errorf("Load() = %q, esperado %q", val, "xyz789")
	}

	// Delete primeira
	if err := store.Delete("aws-token"); err != nil {
		t.Fatalf("Delete() erro: %v", err)
	}

	// Load após delete
	_, err = store.Load("aws-token")
	if err == nil {
		t.Error("Load() após Delete() deveria retornar erro")
	}

	// Segunda ainda existe
	val, err = store.Load("gcp-token")
	if err != nil {
		t.Fatalf("Load() da segunda credencial após delete da primeira erro: %v", err)
	}
	if val != "xyz789" {
		t.Errorf("Load() = %q, esperado %q", val, "xyz789")
	}
}

func TestEncryptedFileStore_WrongPassphrase(t *testing.T) {
	store := newTestEncryptedStore(t, "senha-correta")

	if err := store.Save("segredo", "valor"); err != nil {
		t.Fatalf("Save() erro: %v", err)
	}

	// Trocar passphrase
	store.PassphraseProvider = func() (string, error) {
		return "senha-errada", nil
	}

	_, err := store.Load("segredo")
	if err == nil {
		t.Error("Load() com passphrase errada deveria retornar erro")
	}
}

func TestEncryptedFileStore_CorruptedFile(t *testing.T) {
	store := newTestEncryptedStore(t, "senha")

	if err := store.Save("chave", "valor"); err != nil {
		t.Fatalf("Save() erro: %v", err)
	}

	// Corromper arquivo
	if err := os.WriteFile(store.FilePath, []byte("dados-corrompidos"), 0600); err != nil {
		t.Fatalf("erro ao corromper arquivo: %v", err)
	}

	_, err := store.Load("chave")
	if err == nil {
		t.Error("Load() de arquivo corrompido deveria retornar erro")
	}
}

func TestEncryptedFileStore_Permissions(t *testing.T) {
	store := newTestEncryptedStore(t, "senha-perm")

	if err := store.Save("chave", "valor"); err != nil {
		t.Fatalf("Save() erro: %v", err)
	}

	info, err := os.Stat(store.FilePath)
	if err != nil {
		t.Fatalf("Stat() erro: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("permissão do arquivo = %o, esperado 0600", perm)
	}
}

func TestEncryptedFileStore_FileNotFound(t *testing.T) {
	store := newTestEncryptedStore(t, "senha")

	_, err := store.Load("chave")
	if err == nil {
		t.Error("Load() sem arquivo deveria retornar erro")
	}
}

func TestEncryptedFileStore_EmptyAfterDeleteAll(t *testing.T) {
	store := newTestEncryptedStore(t, "senha")

	if err := store.Save("a", "1"); err != nil {
		t.Fatalf("Save() erro: %v", err)
	}
	if err := store.Delete("a"); err != nil {
		t.Fatalf("Delete() erro: %v", err)
	}

	// Arquivo ainda existe mas sem credenciais
	_, err := store.Load("a")
	if err == nil {
		t.Error("Load() de chave deletada deveria retornar erro")
	}
}

func TestEncryptedFileStore_EnvPassphrase(t *testing.T) {
	dir := t.TempDir()
	store := &EncryptedFileStore{
		FilePath: filepath.Join(dir, "credentials.enc"),
		// Sem PassphraseProvider — deve usar env var
	}

	t.Setenv("YBY_CREDENTIAL_PASSPHRASE", "env-senha")

	if err := store.Save("env-key", "env-value"); err != nil {
		t.Fatalf("Save() com env passphrase erro: %v", err)
	}

	val, err := store.Load("env-key")
	if err != nil {
		t.Fatalf("Load() com env passphrase erro: %v", err)
	}
	if val != "env-value" {
		t.Errorf("Load() = %q, esperado %q", val, "env-value")
	}
}

func TestEncryptedFileStore_NoPassphrase(t *testing.T) {
	dir := t.TempDir()
	store := &EncryptedFileStore{
		FilePath: filepath.Join(dir, "credentials.enc"),
	}

	t.Setenv("YBY_CREDENTIAL_PASSPHRASE", "")

	err := store.Save("chave", "valor")
	if err == nil {
		t.Error("Save() sem passphrase deveria retornar erro")
	}
}

func TestNewCredentialStore_Fallback(t *testing.T) {
	// Em ambiente de teste sem keychain real, deve retornar EncryptedFileStore
	store := NewCredentialStore()
	if store == nil {
		t.Fatal("NewCredentialStore() retornou nil")
	}

	// Verificar que retornou alguma implementação válida
	switch store.(type) {
	case *KeychainStore:
		// OK — keychain disponível
	case *EncryptedFileStore:
		// OK — fallback para arquivo encriptado
	default:
		t.Errorf("NewCredentialStore() retornou tipo inesperado: %T", store)
	}
}
