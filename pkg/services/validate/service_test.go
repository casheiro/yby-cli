package validate

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockHelmRunner implementa HelmRunner para testes.
type mockHelmRunner struct {
	DependencyBuildFunc func(ctx context.Context, chartPath string) error
	LintFunc            func(ctx context.Context, chartPath string) error
	TemplateFunc        func(ctx context.Context, releaseName, chartPath, valuesFile string) ([]byte, error)
}

func (m *mockHelmRunner) DependencyBuild(ctx context.Context, chartPath string) error {
	if m.DependencyBuildFunc != nil {
		return m.DependencyBuildFunc(ctx, chartPath)
	}
	return nil
}

func (m *mockHelmRunner) Lint(ctx context.Context, chartPath string) error {
	if m.LintFunc != nil {
		return m.LintFunc(ctx, chartPath)
	}
	return nil
}

func (m *mockHelmRunner) Template(ctx context.Context, releaseName, chartPath, valuesFile string) ([]byte, error) {
	if m.TemplateFunc != nil {
		return m.TemplateFunc(ctx, releaseName, chartPath, valuesFile)
	}
	return []byte("---\napiVersion: v1"), nil
}

func TestValidateService_SucessoTotal(t *testing.T) {
	helm := &mockHelmRunner{}
	svc := NewService(helm)

	charts := []string{"charts/system", "charts/bootstrap", "charts/cluster-config"}
	report, err := svc.Run(context.Background(), charts, "config/values.yaml")

	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.True(t, report.Success)
	assert.Len(t, report.Charts, 3)

	for _, c := range report.Charts {
		assert.True(t, c.DepsOK, "DepsOK deveria ser true para %s", c.Chart)
		assert.True(t, c.LintOK, "LintOK deveria ser true para %s", c.Chart)
		assert.True(t, c.TemplateOK, "TemplateOK deveria ser true para %s", c.Chart)
		assert.Empty(t, c.Error, "Error deveria estar vazio para %s", c.Chart)
	}
}

func TestValidateService_FalhaDependencyBuild(t *testing.T) {
	helm := &mockHelmRunner{
		DependencyBuildFunc: func(_ context.Context, chartPath string) error {
			if chartPath == "charts/bootstrap" {
				return fmt.Errorf("dependencia nao encontrada")
			}
			return nil
		},
	}
	svc := NewService(helm)

	charts := []string{"charts/system", "charts/bootstrap", "charts/cluster-config"}
	report, err := svc.Run(context.Background(), charts, "config/values.yaml")

	assert.Error(t, err)
	assert.NotNil(t, report)
	assert.False(t, report.Success)
	assert.Contains(t, err.Error(), "subcharts")

	// O primeiro chart passou, o segundo falhou
	assert.True(t, report.Charts[0].DepsOK)
	assert.False(t, report.Charts[1].DepsOK)
	assert.Contains(t, report.Charts[1].Error, "dependencias")

	// O terceiro nao foi processado nesta etapa
	assert.False(t, report.Charts[2].DepsOK)
}

func TestValidateService_FalhaLint(t *testing.T) {
	helm := &mockHelmRunner{
		LintFunc: func(_ context.Context, chartPath string) error {
			if chartPath == "charts/system" {
				return fmt.Errorf("erro de lint: valores invalidos")
			}
			return nil
		},
	}
	svc := NewService(helm)

	charts := []string{"charts/system", "charts/bootstrap"}
	report, err := svc.Run(context.Background(), charts, "config/values.yaml")

	assert.Error(t, err)
	assert.NotNil(t, report)
	assert.False(t, report.Success)
	assert.Contains(t, err.Error(), "lint")

	// Deps passaram para todos
	assert.True(t, report.Charts[0].DepsOK)
	assert.True(t, report.Charts[1].DepsOK)

	// Lint falhou no primeiro
	assert.False(t, report.Charts[0].LintOK)
	assert.Contains(t, report.Charts[0].Error, "lint")
}

func TestValidateService_FalhaTemplate(t *testing.T) {
	helm := &mockHelmRunner{
		TemplateFunc: func(_ context.Context, _, chartPath, _ string) ([]byte, error) {
			if chartPath == "charts/cluster-config" {
				return []byte("Error: template rendering failed"), fmt.Errorf("exit status 1")
			}
			return []byte("---\napiVersion: v1"), nil
		},
	}
	svc := NewService(helm)

	charts := []string{"charts/system", "charts/bootstrap", "charts/cluster-config"}
	report, err := svc.Run(context.Background(), charts, "config/values.yaml")

	assert.Error(t, err)
	assert.NotNil(t, report)
	assert.False(t, report.Success)
	assert.Contains(t, err.Error(), "template")

	// Deps e lint passaram para todos
	for _, c := range report.Charts {
		assert.True(t, c.DepsOK)
		assert.True(t, c.LintOK)
	}

	// Template passou nos dois primeiros, falhou no terceiro
	assert.True(t, report.Charts[0].TemplateOK)
	assert.True(t, report.Charts[1].TemplateOK)
	assert.False(t, report.Charts[2].TemplateOK)
	assert.Contains(t, report.Charts[2].Error, "template")
}

func TestValidateService_ChartVazio(t *testing.T) {
	helm := &mockHelmRunner{}
	svc := NewService(helm)

	report, err := svc.Run(context.Background(), []string{}, "config/values.yaml")

	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.True(t, report.Success)
	assert.Empty(t, report.Charts)
}
