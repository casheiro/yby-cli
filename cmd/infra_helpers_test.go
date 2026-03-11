package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- FindInfraRoot Tests ----

func TestFindInfraRoot_InCurrentDir(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".yby"), 0755)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	root, err := FindInfraRoot()
	assert.NoError(t, err)
	assert.Equal(t, tmpDir, root)
}

func TestFindInfraRoot_InInfraSubdir(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "infra", ".yby"), 0755)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	root, err := FindInfraRoot()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "infra"), root)
}

func TestFindInfraRoot_NotFound(t *testing.T) {
	// Navigate to /tmp which should not have .yby
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_, err := FindInfraRoot()
	assert.Error(t, err)
}

func TestFindInfraRoot_TraversesUpward(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".yby"), 0755)
	subDir := filepath.Join(tmpDir, "apps", "backend")
	os.MkdirAll(subDir, 0755)

	origDir, _ := os.Getwd()
	os.Chdir(subDir)
	defer os.Chdir(origDir)

	root, err := FindInfraRoot()
	assert.NoError(t, err)
	assert.Equal(t, tmpDir, root)
}

// ---- JoinInfra Tests ----

func TestJoinInfra_Basic(t *testing.T) {
	result := JoinInfra("/infra", "charts/system")
	expected := filepath.Join("/infra", "charts/system")
	assert.Equal(t, expected, result)
}

func TestJoinInfra_WithRelativePath(t *testing.T) {
	result := JoinInfra(".", "config/values.yaml")
	assert.Contains(t, result, "config")
	assert.Contains(t, result, "values.yaml")
}

func TestJoinInfra_Empty(t *testing.T) {
	result := JoinInfra("", "file.txt")
	assert.Equal(t, "file.txt", result)
}

// ---- cmd/status - smoke test ----

func TestStatusCmd_Execute(t *testing.T) {
	resetCmdState(t)
	rootCmd.SetArgs([]string{"status"})
	// kubectl is in the test environment; command should run without panic
	_ = rootCmd.Execute()
}

// ---- cmd/doctor - smoke test ----

func TestDoctorCmd_Execute(t *testing.T) {
	resetCmdState(t)
	rootCmd.SetArgs([]string{"doctor"})
	err := rootCmd.Execute()
	// doctor should succeed (it's a diagnostic that never returns an error)
	assert.NoError(t, err)
}

// ---- cmd/validate - smoke test ----

func TestValidateCmd_Execute_NoCharts(t *testing.T) {
	resetCmdState(t)
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	rootCmd.SetArgs([]string{"validate"})
	// Will fail because there are no helm charts, but should not panic
	_ = rootCmd.Execute()
}

// ---- cmd/context_dump - smoke test ----

func TestContextDumpCmd_Execute(t *testing.T) {
	resetCmdState(t)
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	rootCmd.SetArgs([]string{"context-dump"})
	// Will print context info; should not panic
	_ = rootCmd.Execute()
}

// ---- cmd/uninstall - smoke test ----

func TestUninstallCmd_Execute(t *testing.T) {
	resetCmdState(t)
	rootCmd.SetArgs([]string{"uninstall", "--help"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
}

// ---- cmd/generate - smoke test ----

func TestGenerateCmd_Help(t *testing.T) {
	resetCmdState(t)
	rootCmd.SetArgs([]string{"generate", "--help"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
}

// ---- cmd/chart - smoke test ----

func TestChartCmd_Help(t *testing.T) {
	resetCmdState(t)
	rootCmd.SetArgs([]string{"chart", "--help"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
}

// ---- cmd/uki - smoke test ----

func TestUkiCmd_Help(t *testing.T) {
	resetCmdState(t)
	rootCmd.SetArgs([]string{"uki", "--help"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
}
