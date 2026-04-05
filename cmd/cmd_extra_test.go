package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/setup"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/stretchr/testify/assert"
)

// ---- ConfigureDirenv — via serviço de setup ----

func TestConfigureDirenv_CreatesEnvrc(t *testing.T) {
	tmpDir := t.TempDir()
	runner := &shared.RealRunner{}
	fsys := &shared.RealFilesystem{}
	checker := &setup.SystemToolChecker{Runner: runner}
	pkg := &setup.SystemPackageManager{Runner: runner}
	svc := setup.NewService(checker, pkg, runner, fsys)

	// Ignora erro de direnv allow (pode não estar instalado no CI)
	_ = svc.ConfigureDirenv(tmpDir)

	_, err := os.Stat(filepath.Join(tmpDir, ".envrc"))
	assert.NoError(t, err, ".envrc deveria ter sido criado")
}

func TestConfigureDirenv_DoesNotOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	envrcPath := filepath.Join(tmpDir, ".envrc")
	os.WriteFile(envrcPath, []byte("# existing"), 0644)

	runner := &shared.RealRunner{}
	fsys := &shared.RealFilesystem{}
	checker := &setup.SystemToolChecker{Runner: runner}
	pkg := &setup.SystemPackageManager{Runner: runner}
	svc := setup.NewService(checker, pkg, runner, fsys)

	// Ignora erro de direnv allow
	_ = svc.ConfigureDirenv(tmpDir)

	data, _ := os.ReadFile(envrcPath)
	assert.Equal(t, "# existing", string(data))
}

// ---- Validações de estrutura cobra sem executar comandos ----

func TestSetupCmd_FlagRegistered(t *testing.T) {
	flag := setupCmd.Flags().Lookup("profile")
	assert.NotNil(t, flag, "profile flag should be registered")
	assert.Equal(t, "dev", flag.DefValue)
}

func TestSealCmd_IsSubOfSecrets(t *testing.T) {
	found := false
	for _, sub := range secretsCmd.Commands() {
		if sub.Name() == "seal" {
			found = true
			break
		}
	}
	assert.True(t, found, "seal should be a subcommand of secrets")
}

func TestRootCmd_HasExpectedSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, sub := range rootCmd.Commands() {
		names[sub.Name()] = true
	}

	// Only check commands confirmed to be direct subcommands of rootCmd
	expected := []string{"version", "env", "doctor", "up", "setup", "access", "destroy", "status", "validate", "uninstall", "generate", "chart", "uki"}
	for _, name := range expected {
		assert.True(t, names[name], "expected subcommand %q to exist on rootCmd", name)
	}
}
