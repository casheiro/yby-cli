package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/services/validate"
	"github.com/stretchr/testify/assert"
)

// mockValidateHelmRunner implementa validate.HelmRunner para testes no cmd.
type mockValidateHelmRunner struct {
	dependencyBuildErr error
	lintErr            error
	templateErr        error
}

func (m *mockValidateHelmRunner) DependencyBuild(_ context.Context, _ string) error {
	return m.dependencyBuildErr
}

func (m *mockValidateHelmRunner) Lint(_ context.Context, _ string) error {
	return m.lintErr
}

func (m *mockValidateHelmRunner) Template(_ context.Context, _, _, _ string) ([]byte, error) {
	if m.templateErr != nil {
		return []byte("Error: template failed"), m.templateErr
	}
	return []byte("---\napiVersion: v1"), nil
}

func TestValidateCmd_Sucesso(t *testing.T) {
	original := newValidateService
	defer func() { newValidateService = original }()

	newValidateService = func(_ shared.Runner) validate.Service {
		return validate.NewService(&mockValidateHelmRunner{})
	}

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".yby"), 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "system"), 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "bootstrap"), 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "cluster-config"), 0755)
	os.MkdirAll(filepath.Join(dir, "config"), 0755)
	os.WriteFile(filepath.Join(dir, "config", "cluster-values.yaml"), []byte("{}"), 0644)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	err := validateCmd.RunE(validateCmd, []string{})
	assert.NoError(t, err)
}

func TestValidateCmd_FalhaLint(t *testing.T) {
	original := newValidateService
	defer func() { newValidateService = original }()

	newValidateService = func(_ shared.Runner) validate.Service {
		return validate.NewService(&mockValidateHelmRunner{
			lintErr: fmt.Errorf("lint falhou"),
		})
	}

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".yby"), 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "system"), 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "bootstrap"), 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "cluster-config"), 0755)
	os.MkdirAll(filepath.Join(dir, "config"), 0755)
	os.WriteFile(filepath.Join(dir, "config", "cluster-values.yaml"), []byte("{}"), 0644)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	err := validateCmd.RunE(validateCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lint")
}

func TestValidateCmd_FalhaTemplate(t *testing.T) {
	original := newValidateService
	defer func() { newValidateService = original }()

	newValidateService = func(_ shared.Runner) validate.Service {
		return validate.NewService(&mockValidateHelmRunner{
			templateErr: fmt.Errorf("template falhou"),
		})
	}

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".yby"), 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "system"), 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "bootstrap"), 0755)
	os.MkdirAll(filepath.Join(dir, "charts", "cluster-config"), 0755)
	os.MkdirAll(filepath.Join(dir, "config"), 0755)
	os.WriteFile(filepath.Join(dir, "config", "cluster-values.yaml"), []byte("{}"), 0644)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	err := validateCmd.RunE(validateCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template")
}
