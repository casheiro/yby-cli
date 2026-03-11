//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/suite"
)

var binaryPath string

// TestMain acts as the entry point for the E2E test suite.
// It compiles the yby CLI binary once before running the tests, ensuring fast, parallel execution.
func TestMain(m *testing.M) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "yby-e2e-build-*")
	if err != nil {
		fmt.Printf("Falha ao criar diretório temporário para build: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	binaryName := "yby"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath = filepath.Join(tmpDir, binaryName)

	fmt.Printf("🔨 Compilando binário E2E em: %s\n", binaryPath)
	cmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/yby")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Falha na compilação do yby para testes E2E: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup will happen via defer (os.RemoveAll)
	os.Exit(code)
}

// E2ESuite is the testify suite containing the End-to-End tests
type E2ESuite struct {
	suite.Suite
	workDir string
}

func (s *E2ESuite) SetupTest() {
	// Cria um diretório de trabalho focado por teste
	dir, err := os.MkdirTemp("", "yby-e2e-work-*")
	s.Require().NoError(err, "Falha ao criar workdir do teste")
	s.workDir = dir
}

func (s *E2ESuite) TearDownTest() {
	// Limpa o ambiente após o teste
	os.RemoveAll(s.workDir)
}

// RunYby executa o binário do yby com os argumentos e retorna stdout, stderr e error
func (s *E2ESuite) RunYby(args ...string) (string, string, error) {
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = s.workDir // Roda de dentro do diretório temporário do teste

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func (s *E2ESuite) TestInitHeadless() {
	s.T().Log("Executando yby init no modo headless (--non-interactive)")

	args := []string{
		"init",
		"--non-interactive",
		"--topology", "standard",
		"--workflow", "gitflow",
		"--project-name", "e2e-app",
		"--git-repo", "https://github.com/casheiro-org/e2e-app.git",
		"--target-dir", "infra",
	}

	stdout, stderr, err := s.RunYby(args...)
	s.Require().NoError(err, "O comando yby init falhou: %s\nStdout: %s", stderr, stdout)

	s.T().Log("Validando se a estrutura de pastas foi gerada")
	infraDir := filepath.Join(s.workDir, "infra")
	s.Require().DirExists(infraDir, "O diretório alvo 'infra' não foi criado")

	// Verify specific expected files
	expectedFiles := []string{
		"charts/cluster-config/Chart.yaml",
		"config/cluster-values.yaml",
	}

	for _, file := range expectedFiles {
		fp := filepath.Join(infraDir, file)
		s.Require().FileExists(fp, "O arquivo esperado '%s' não foi gerado", file)
	}

	s.T().Log("yby init headless gerou os artefatos com sucesso.")
}

func TestE2ESuite(t *testing.T) {
	suite.Run(t, new(E2ESuite))
}
