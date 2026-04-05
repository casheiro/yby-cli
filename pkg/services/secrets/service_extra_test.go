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

// TestEncryptWithSOPS_Success cobre o fluxo feliz de EncryptWithSOPS
func TestEncryptWithSOPS_Success(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)

	runner.On("RunStdinOutput", ctx, "secret-yaml", "sops",
		[]string{"--encrypt", "--input-type", "yaml", "--output-type", "yaml"}).
		Return([]byte("encrypted"), nil)
	fsys.On("MkdirAll", "/tmp", mock.Anything).Return(nil)
	fsys.On("WriteFile", "/tmp/out.yaml", []byte("encrypted"), mock.Anything).Return(nil)

	err := svc.EncryptWithSOPS(ctx, "", []byte("secret-yaml"), "/tmp/out.yaml")
	assert.NoError(t, err)
	runner.AssertExpectations(t)
}

// TestEncryptWithSOPS_ComRecipient verifica que --age é passado quando ageRecipient é informado
func TestEncryptWithSOPS_ComRecipient(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)

	runner.On("RunStdinOutput", ctx, "secret-yaml", "sops",
		[]string{"--encrypt", "--input-type", "yaml", "--output-type", "yaml", "--age", "age1xxx"}).
		Return([]byte("encrypted"), nil)
	fsys.On("MkdirAll", "/tmp", mock.Anything).Return(nil)
	fsys.On("WriteFile", "/tmp/out.yaml", []byte("encrypted"), mock.Anything).Return(nil)

	err := svc.EncryptWithSOPS(ctx, "age1xxx", []byte("secret-yaml"), "/tmp/out.yaml")
	assert.NoError(t, err)
	runner.AssertExpectations(t)
}

// TestEncryptWithSOPS_SopsError cobre erro do sops
func TestEncryptWithSOPS_SopsError(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	svc := NewService(runner, new(MockFS))

	runner.On("RunStdinOutput", ctx, "data", "sops", mock.Anything).
		Return(nil, errors.New("sops falhou"))

	err := svc.EncryptWithSOPS(ctx, "", []byte("data"), "/tmp/out.yaml")
	assert.ErrorContains(t, err, "sops")
}

// TestGenerateAgeKey_Success cobre geração de chave age
func TestGenerateAgeKey_Success(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)

	fsys.On("MkdirAll", "/home/user/.sops", mock.Anything).Return(nil)
	runner.On("RunCombinedOutput", ctx, "age-keygen",
		[]string{"-o", "/home/user/.sops/age-key.txt"}).
		Return([]byte("Public key: age1abc123\n"), nil)

	pubKey, err := svc.GenerateAgeKey(ctx, "/home/user/.sops/age-key.txt")
	assert.NoError(t, err)
	assert.Equal(t, "age1abc123", pubKey)
	runner.AssertExpectations(t)
}

// TestGenerateAgeKey_SemChavePublica cobre falha ao encontrar chave pública na saída
func TestGenerateAgeKey_SemChavePublica(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)

	fsys.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
	runner.On("RunCombinedOutput", ctx, "age-keygen", mock.Anything).
		Return([]byte("saida sem chave publica"), nil)

	_, err := svc.GenerateAgeKey(ctx, "/tmp/key.txt")
	assert.ErrorContains(t, err, "chave pública não encontrada")
}

// TestGenerateSecretYAML_Success cobre o fluxo feliz de GenerateSecretYAML
func TestGenerateSecretYAML_Success(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	svc := NewService(runner, new(MockFS))

	runner.On("RunCombinedOutput", ctx, "kubectl", []string{
		"create", "secret", "generic", "my-secret",
		"--namespace", "default",
		"--from-literal=password=s3cret",
		"--dry-run=client", "-o", "yaml",
	}).Return([]byte("apiVersion: v1\nkind: Secret\n"), nil)

	out, err := svc.GenerateSecretYAML(ctx, "my-secret", "default", "password", "s3cret")
	assert.NoError(t, err)
	assert.Contains(t, string(out), "Secret")
	runner.AssertExpectations(t)
}

// TestGenerateSecretYAML_KubectlError cobre falha do kubectl
func TestGenerateSecretYAML_KubectlError(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	svc := NewService(runner, new(MockFS))

	runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).
		Return(nil, errors.New("kubectl error"))

	_, err := svc.GenerateSecretYAML(ctx, "my-secret", "default", "key", "val")
	assert.ErrorContains(t, err, "falha ao gerar secret YAML")
}

// TestSealWithKubeseal_Success cobre o fluxo feliz de SealWithKubeseal
func TestSealWithKubeseal_Success(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)

	runner.On("RunStdinOutput", ctx, "secret-yaml", "kubeseal",
		[]string{"--format", "yaml"}).
		Return([]byte("sealed-content"), nil)
	fsys.On("MkdirAll", "/tmp", mock.Anything).Return(nil)
	fsys.On("WriteFile", "/tmp/sealed.yaml", []byte("sealed-content"), mock.Anything).Return(nil)

	err := svc.SealWithKubeseal(ctx, []byte("secret-yaml"), "/tmp/sealed.yaml")
	assert.NoError(t, err)
	runner.AssertExpectations(t)
}

// TestSealWithKubeseal_KubesealError cobre falha do kubeseal
func TestSealWithKubeseal_KubesealError(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	svc := NewService(runner, new(MockFS))

	runner.On("RunStdinOutput", ctx, "data", "kubeseal", mock.Anything).
		Return(nil, errors.New("kubeseal error"))

	err := svc.SealWithKubeseal(ctx, []byte("data"), "/tmp/out.yaml")
	assert.ErrorContains(t, err, "erro ao executar kubeseal")
}

// TestSealWithKubeseal_MkdirError cobre falha ao criar diretório
func TestSealWithKubeseal_MkdirError(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)

	runner.On("RunStdinOutput", ctx, "data", "kubeseal", mock.Anything).
		Return([]byte("sealed"), nil)
	fsys.On("MkdirAll", mock.Anything, mock.Anything).Return(errors.New("mkdir fail"))

	err := svc.SealWithKubeseal(ctx, []byte("data"), "/tmp/dir/out.yaml")
	assert.ErrorContains(t, err, "erro ao criar diretório")
}

// TestSealWithKubeseal_WriteError cobre falha ao salvar arquivo
func TestSealWithKubeseal_WriteError(t *testing.T) {
	ctx := context.Background()
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)

	runner.On("RunStdinOutput", ctx, "data", "kubeseal", mock.Anything).
		Return([]byte("sealed"), nil)
	fsys.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
	fsys.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("write fail"))

	err := svc.SealWithKubeseal(ctx, []byte("data"), "/tmp/out.yaml")
	assert.ErrorContains(t, err, "erro ao salvar sealed secret")
}
