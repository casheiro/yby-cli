package cmd

import (
	"bytes"
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

func TestRenderValidateReport_Golden(t *testing.T) {
	casos := []struct {
		nome      string
		relatorio *validate.ValidateReport
	}{
		{"validate_ok", &validate.ValidateReport{Success: true, Charts: []validate.ChartResult{
			{Chart: "charts/system", DepsOK: true, LintOK: true, TemplateOK: true},
			{Chart: "charts/bootstrap", DepsOK: true, LintOK: true, TemplateOK: true},
		}}},
		{"validate_falha_template", &validate.ValidateReport{Charts: []validate.ChartResult{
			{Chart: "charts/system", DepsOK: true, LintOK: true, TemplateOK: true},
			{Chart: "charts/bootstrap", DepsOK: true, LintOK: true, Error: "falha no template: Error: template failed\nlinha 2\n"},
		}}},
		{"validate_falha_lint", &validate.ValidateReport{Charts: []validate.ChartResult{
			{Chart: "charts/system", DepsOK: true, Error: "falha no lint: exit status 1"},
			{Chart: "charts/bootstrap", DepsOK: true},
		}}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarSaidaTexto(t, false)
			var b bytes.Buffer
			renderValidateReport(&b, c.relatorio)
			conferirGolden(t, c.nome, b.String())
			conferirSemRestos(t, b.String())
		})
	}
}

func TestValidateCmd_Sucesso_Resumo(t *testing.T) {
	usarSaidaTexto(t, false)
	original := newValidateService
	defer func() { newValidateService = original }()
	newValidateService = func(_ shared.Runner) validate.Service {
		return validate.NewService(&mockValidateHelmRunner{})
	}

	b := saidaDoComando(t, validateCmd)
	assert.NoError(t, validateCmd.RunE(validateCmd, []string{}))
	saida := b.String()

	assert.Contains(t, saida, "◆ YBY  VALIDATE")
	assert.Contains(t, saida, "RESUMO  3 ok · 0 avisos · 0 erros  · charts válidos")
	conferirSemRestos(t, saida)
}
