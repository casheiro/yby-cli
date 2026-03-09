package executor

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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
