package testutil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockExecutor_Run_Default(t *testing.T) {
	m := &MockExecutor{}
	err := m.Run("test", "echo hello")
	assert.NoError(t, err)
}

func TestMockExecutor_Run_Custom(t *testing.T) {
	m := &MockExecutor{
		RunFunc: func(name, script string) error {
			return errors.New("falha simulada")
		},
	}
	err := m.Run("test", "echo hello")
	assert.Error(t, err)
}

func TestMockExecutor_FetchFile_Default(t *testing.T) {
	m := &MockExecutor{}
	data, err := m.FetchFile("/test")
	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestMockExecutor_FetchFile_Custom(t *testing.T) {
	m := &MockExecutor{
		FetchFileFunc: func(path string) ([]byte, error) {
			return []byte("conteúdo"), nil
		},
	}
	data, err := m.FetchFile("/test")
	require.NoError(t, err)
	assert.Equal(t, "conteúdo", string(data))
}

func TestMockExecutor_Close(t *testing.T) {
	m := &MockExecutor{}
	assert.NoError(t, m.Close())
}
