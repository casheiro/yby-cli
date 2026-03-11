package testutil

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMockRunner_RunStdin_Custom verifica o branch com RunStdinFunc definida
func TestMockRunner_RunStdin_Custom(t *testing.T) {
	runner := &MockRunner{
		RunStdinFunc: func(ctx context.Context, stdin string, name string, args ...string) error {
			if stdin == "fail" {
				return errors.New("falha stdin")
			}
			return nil
		},
	}

	ctx := context.Background()
	assert.Error(t, runner.RunStdin(ctx, "fail", "cmd"))
	assert.NoError(t, runner.RunStdin(ctx, "ok", "cmd"))
}

// TestMockRunner_RunStdinOutput_Custom verifica o branch com RunStdinOutputFunc definida
func TestMockRunner_RunStdinOutput_Custom(t *testing.T) {
	runner := &MockRunner{
		RunStdinOutputFunc: func(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
			return []byte("saida-" + stdin), nil
		},
	}

	ctx := context.Background()
	out, err := runner.RunStdinOutput(ctx, "teste", "cmd")
	assert.NoError(t, err)
	assert.Equal(t, "saida-teste", string(out))
}

// TestMockFilesystem_WriteFile_Custom verifica o branch com WriteFileFunc definida
func TestMockFilesystem_WriteFile_Custom(t *testing.T) {
	var gravado []byte
	mfs := &MockFilesystem{
		WriteFileFunc: func(name string, data []byte, perm fs.FileMode) error {
			gravado = data
			return nil
		},
	}

	err := mfs.WriteFile("test.txt", []byte("conteudo"), 0644)
	assert.NoError(t, err)
	assert.Equal(t, "conteudo", string(gravado))
}

// TestMockFilesystem_MkdirAll_Custom verifica o branch com MkdirAllFunc definida
func TestMockFilesystem_MkdirAll_Custom(t *testing.T) {
	var caminho string
	mfs := &MockFilesystem{
		MkdirAllFunc: func(path string, perm fs.FileMode) error {
			caminho = path
			return nil
		},
	}

	err := mfs.MkdirAll("/a/b/c", 0755)
	assert.NoError(t, err)
	assert.Equal(t, "/a/b/c", caminho)
}

// TestMockFilesystem_UserHomeDir_Custom verifica o branch com UserHomeDirFunc definida
func TestMockFilesystem_UserHomeDir_Custom(t *testing.T) {
	mfs := &MockFilesystem{
		UserHomeDirFunc: func() (string, error) {
			return "/custom/home", nil
		},
	}

	home, err := mfs.UserHomeDir()
	assert.NoError(t, err)
	assert.Equal(t, "/custom/home", home)
}

// TestMockFilesystem_WalkDir_Custom verifica o branch com WalkDirFunc definida
func TestMockFilesystem_WalkDir_Custom(t *testing.T) {
	chamado := false
	mfs := &MockFilesystem{
		WalkDirFunc: func(root string, fn fs.WalkDirFunc) error {
			chamado = true
			return nil
		},
	}

	err := mfs.WalkDir("/root", nil)
	assert.NoError(t, err)
	assert.True(t, chamado)
}

// TestMockExecutor_Close_Custom verifica o branch com CloseFunc definida
func TestMockExecutor_Close_Custom(t *testing.T) {
	m := &MockExecutor{
		CloseFunc: func() error {
			return errors.New("falha ao fechar")
		},
	}

	err := m.Close()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao fechar")
}
