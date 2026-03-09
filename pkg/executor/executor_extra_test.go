package executor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- Testes para MockCommandExecutor ---

func TestMockCommandExecutor_Output(t *testing.T) {
	mock := &MockCommandExecutor{
		CommandFunc: func(name string, arg ...string) Command {
			return &mockCommand{
				OutputFunc: func() ([]byte, error) {
					return []byte("saída teste"), nil
				},
			}
		},
	}
	cmd := mock.Command("echo", "hello")
	out, err := cmd.Output()
	assert.NoError(t, err)
	assert.Equal(t, "saída teste", string(out))
}

func TestMockCommandExecutor_CombinedOutput(t *testing.T) {
	mock := &MockCommandExecutor{
		CommandFunc: func(name string, arg ...string) Command {
			return &mockCommand{
				CombinedOutputFunc: func() ([]byte, error) {
					return []byte("saída combinada"), nil
				},
			}
		},
	}
	cmd := mock.Command("echo", "hello")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Equal(t, "saída combinada", string(out))
}

func TestMockCommandExecutor_DefaultCommand(t *testing.T) {
	mock := &MockCommandExecutor{}
	cmd := mock.Command("test")
	// Run padrão não deve retornar erro
	assert.NoError(t, cmd.Run())
	// Output padrão deve retornar vazio
	out, err := cmd.Output()
	assert.NoError(t, err)
	assert.Empty(t, out)
	// CombinedOutput padrão deve retornar vazio
	combined, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Empty(t, combined)
}

func TestMockCommandExecutor_LookPath_Default(t *testing.T) {
	mock := &MockCommandExecutor{}
	_, err := mock.LookPath("qualquer-binario")
	assert.Error(t, err, "LookPath padrão deve retornar erro 'not found'")
}

func TestMockCommandExecutor_Output_ComErro(t *testing.T) {
	expectedErr := errors.New("falha na execução")
	mock := &MockCommandExecutor{
		CommandFunc: func(name string, arg ...string) Command {
			return &mockCommand{
				OutputFunc: func() ([]byte, error) {
					return nil, expectedErr
				},
			}
		},
	}
	cmd := mock.Command("falhar")
	out, err := cmd.Output()
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, out)
}

func TestMockCommandExecutor_CombinedOutput_ComErro(t *testing.T) {
	expectedErr := errors.New("falha combinada")
	mock := &MockCommandExecutor{
		CommandFunc: func(name string, arg ...string) Command {
			return &mockCommand{
				CombinedOutputFunc: func() ([]byte, error) {
					return nil, expectedErr
				},
			}
		},
	}
	cmd := mock.Command("falhar")
	out, err := cmd.CombinedOutput()
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, out)
}

func TestMockCommandExecutor_Run_ComErro(t *testing.T) {
	expectedErr := errors.New("falha no run")
	mock := &MockCommandExecutor{
		CommandFunc: func(name string, arg ...string) Command {
			return &mockCommand{
				RunFunc: func() error {
					return expectedErr
				},
			}
		},
	}
	cmd := mock.Command("falhar")
	err := cmd.Run()
	assert.ErrorIs(t, err, expectedErr)
}

// --- Testes para RealCommandExecutor ---

func TestRealCommandExecutor_LookPath_Encontrado(t *testing.T) {
	exec := &RealCommandExecutor{}
	// "go" deve ser encontrado pois estamos executando testes Go
	path, err := exec.LookPath("go")
	assert.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestRealCommandExecutor_LookPath_NaoEncontrado(t *testing.T) {
	exec := &RealCommandExecutor{}
	_, err := exec.LookPath("binario-inexistente-xyz-123")
	assert.Error(t, err)
}

func TestRealCommandExecutor_Command_Echo(t *testing.T) {
	exec := &RealCommandExecutor{}
	cmd := exec.Command("echo", "hello")
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(out), "hello")
}

func TestRealCommandExecutor_Command_Output(t *testing.T) {
	exec := &RealCommandExecutor{}
	cmd := exec.Command("echo", "saída")
	out, err := cmd.Output()
	assert.NoError(t, err)
	assert.Contains(t, string(out), "saída")
}

func TestRealCommandExecutor_Command_Run(t *testing.T) {
	exec := &RealCommandExecutor{}
	cmd := exec.Command("true")
	err := cmd.Run()
	assert.NoError(t, err)
}

func TestRealCommandExecutor_Command_RunFalha(t *testing.T) {
	exec := &RealCommandExecutor{}
	cmd := exec.Command("false")
	err := cmd.Run()
	assert.Error(t, err)
}

func TestRealCommandExecutor_Command_ComandoInexistente(t *testing.T) {
	exec := &RealCommandExecutor{}
	cmd := exec.Command("comando-que-nao-existe-xyz")
	err := cmd.Run()
	assert.Error(t, err, "deve falhar ao executar comando inexistente")
}
