package executor

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
)

func TestNewSSHExecutor_SemSSHAgent(t *testing.T) {
	// Salva e remove SSH_AUTH_SOCK para forçar falha na conexão com o SSH Agent
	original := os.Getenv("SSH_AUTH_SOCK")
	t.Setenv("SSH_AUTH_SOCK", "/tmp/nonexistent-socket-for-test")
	defer func() {
		if original != "" {
			os.Setenv("SSH_AUTH_SOCK", original)
		}
	}()

	_, err := NewSSHExecutor("root", "127.0.0.1", "22")
	assert.Error(t, err, "deve falhar quando SSH_AUTH_SOCK aponta para socket inexistente")
	assert.Contains(t, err.Error(), "SSH Agent", "mensagem de erro deve mencionar SSH Agent")
}

func TestNewSSHExecutor_SocketVazio(t *testing.T) {
	// SSH_AUTH_SOCK vazio deve falhar
	t.Setenv("SSH_AUTH_SOCK", "")

	_, err := NewSSHExecutor("root", "127.0.0.1", "22")
	assert.Error(t, err, "deve falhar quando SSH_AUTH_SOCK está vazio")
}

func TestSSHExecutor_ImplementaInterface(t *testing.T) {
	// Verifica que SSHExecutor implementa a interface Executor em tempo de compilação
	var _ Executor = (*SSHExecutor)(nil)
}

// mockSSHClient simula um cliente SSH para testes
type mockSSHClient struct {
	newSessionFunc func() (*ssh.Session, error)
	closeFunc      func() error
}

func (m *mockSSHClient) NewSession() (*ssh.Session, error) {
	if m.newSessionFunc != nil {
		return m.newSessionFunc()
	}
	return nil, fmt.Errorf("mock: NewSession não configurado")
}

func (m *mockSSHClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestSSHExecutor_Close_Success(t *testing.T) {
	closed := false
	mock := &mockSSHClient{
		closeFunc: func() error {
			closed = true
			return nil
		},
	}
	exec := &SSHExecutor{client: mock}
	err := exec.Close()
	assert.NoError(t, err)
	assert.True(t, closed, "Close deve chamar client.Close()")
}

func TestSSHExecutor_Close_Error(t *testing.T) {
	mock := &mockSSHClient{
		closeFunc: func() error {
			return fmt.Errorf("erro ao fechar conexão")
		},
	}
	exec := &SSHExecutor{client: mock}
	err := exec.Close()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "erro ao fechar conexão")
}

func TestSSHExecutor_Run_SessionError(t *testing.T) {
	mock := &mockSSHClient{
		newSessionFunc: func() (*ssh.Session, error) {
			return nil, fmt.Errorf("falha ao criar sessão SSH")
		},
	}
	exec := &SSHExecutor{client: mock}
	err := exec.Run("teste", "echo hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao criar sessão SSH")
}

func TestSSHExecutor_FetchFile_SessionError(t *testing.T) {
	mock := &mockSSHClient{
		newSessionFunc: func() (*ssh.Session, error) {
			return nil, fmt.Errorf("falha ao criar sessão SSH")
		},
	}
	exec := &SSHExecutor{client: mock}
	_, err := exec.FetchFile("/etc/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao criar sessão SSH")
}
