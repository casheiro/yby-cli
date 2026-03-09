package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGenerateMinIO_SealAndSaveFalha cobre a linha 90-92 do service.go
func TestGenerateMinIO_SealAndSaveFalha(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)

	opts := Options{OutputPath: "/tmp/minio.yaml"}

	runner.On("RunCombinedOutput", ctx, "openssl", mock.Anything).Return([]byte("pass123\n"), nil)
	runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return([]byte("secret"), nil)
	runner.On("RunStdinOutput", ctx, "secret", "kubeseal", mock.Anything).Return(nil, errors.New("kubeseal falhou"))

	_, err := svc.GenerateMinIO(ctx, opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubeseal")
}

// TestSealAndSave_MkdirAllFalha cobre a linha 169-171 do service.go
func TestSealAndSave_MkdirAllFalha(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)

	runner.On("RunStdinOutput", ctx, "input", "kubeseal", mock.Anything).Return([]byte("sealed"), nil)
	fsys.On("MkdirAll", mock.Anything, mock.Anything).Return(errors.New("mkdir falhou"))

	err := svc.(*secretsService).sealAndSave(ctx, []byte("input"), "/tmp/dir/output.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "erro ao criar diretório")
}

// TestSealAndSave_WriteFileFalha cobre a linha 173-175 do service.go
func TestSealAndSave_WriteFileFalha(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)

	runner.On("RunStdinOutput", ctx, "input", "kubeseal", mock.Anything).Return([]byte("sealed"), nil)
	fsys.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
	fsys.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("write falhou"))

	err := svc.(*secretsService).sealAndSave(ctx, []byte("input"), "/tmp/output.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "erro ao salvar arquivo")
}
