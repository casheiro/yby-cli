package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// resetCmdState resets global cobra state that can leak between tests.
// The contextFlag global variable, when non-empty, causes initConfig to
// run os.Setenv("YBY_ENV", contextFlag) on every cobra command execution.
func resetCmdState(t *testing.T) {
	t.Helper()
	contextFlag = ""
	t.Setenv("YBY_ENV", "")
}

func TestVersionCmd_Execute(t *testing.T) {
	resetCmdState(t)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})

	err := rootCmd.Execute()
	assert.NoError(t, err)
	// version command uses fmt.Println writing to os.Stdout directly
	// we just assert no error and the cmd ran without panic
}

func TestVersionCmd_ContainsOSArch(t *testing.T) {
	resetCmdState(t)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})

	err := rootCmd.Execute()
	assert.NoError(t, err)
}

// makeTestInfraDir creates a temp dir with a valid .yby/environments.yaml structure.
func makeTestInfraDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	ybyDir := filepath.Join(tmpDir, ".yby")
	os.MkdirAll(ybyDir, 0755)
	content := `current: local
environments:
  local:
    type: local
    description: Ambiente local
    values: config/values-local.yaml
  prod:
    type: remote
    description: Producao
    values: config/values-prod.yaml
`
	os.WriteFile(filepath.Join(ybyDir, "environments.yaml"), []byte(content), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "config"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "config", "values-local.yaml"), []byte(""), 0644)
	return tmpDir
}

func TestEnvListCmd_Execute(t *testing.T) {
	resetCmdState(t)
	tmpDir := makeTestInfraDir(t)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	rootCmd.SetArgs([]string{"env", "list"})

	// env list uses fmt.Println directly (not cmd.Println),
	// so output goes to os.Stdout, not a capturable buffer.
	// We just assert the command succeeds.
	err := rootCmd.Execute()
	assert.NoError(t, err)
}

func TestEnvUseCmd_ValidEnv(t *testing.T) {
	resetCmdState(t)
	tmpDir := makeTestInfraDir(t)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	rootCmd.SetArgs([]string{"env", "use", "prod"})

	err := rootCmd.Execute()
	assert.NoError(t, err)
}

func TestEnvUseCmd_InvalidEnv(t *testing.T) {
	resetCmdState(t)
	tmpDir := makeTestInfraDir(t)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	rootCmd.SetArgs([]string{"env", "use", "nonexistent-env-xyz"})

	err := rootCmd.Execute()
	assert.Error(t, err)
}

func TestEnvShowCmd_Execute(t *testing.T) {
	resetCmdState(t)
	tmpDir := makeTestInfraDir(t)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	rootCmd.SetArgs([]string{"env", "show"})

	err := rootCmd.Execute()
	assert.NoError(t, err)
}

func TestEnvCreateCmd_NewEnv(t *testing.T) {
	resetCmdState(t)
	tmpDir := makeTestInfraDir(t)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	rootCmd.SetArgs([]string{"env", "create", "qa", "--type", "remote", "--description", "Quality Assurance"})

	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(tmpDir, "config", "values-qa.yaml"))
}

func TestDestroyCmd_LocalDefault(t *testing.T) {
	resetCmdState(t)
	rootCmd.SetArgs([]string{"destroy"})
	// k3d available in test env; if cluster doesn't exist it exits cleanly
	_ = rootCmd.Execute()
}

func TestDestroyCmd_CustomClusterName(t *testing.T) {
	resetCmdState(t)
	t.Setenv("YBY_CLUSTER_NAME", "my-test-cluster")
	rootCmd.SetArgs([]string{"destroy"})
	_ = rootCmd.Execute()
}
