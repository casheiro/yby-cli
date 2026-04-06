package validate

import "context"

// HelmRunner abstrai a execucao de comandos Helm para validacao de charts.
type HelmRunner interface {
	// DependencyBuild resolve e baixa as dependencias de um chart.
	DependencyBuild(ctx context.Context, chartPath string) error

	// Lint executa a validacao estatica (lint) de um chart.
	Lint(ctx context.Context, chartPath string) error

	// Template renderiza o template do chart sem instalar, retornando a saida combinada.
	Template(ctx context.Context, releaseName, chartPath, valuesFile string) ([]byte, error)
}

// ChartResult representa o resultado da validacao de um chart individual.
type ChartResult struct {
	Chart      string
	DepsOK     bool
	LintOK     bool
	TemplateOK bool
	Error      string
}

// ValidateReport agrupa os resultados de validacao de todos os charts.
type ValidateReport struct {
	Charts  []ChartResult
	Success bool
}

// Service define o contrato do servico de validacao de charts Helm.
type Service interface {
	// Run executa a validacao completa (dependencias, lint e template) para os charts informados.
	Run(ctx context.Context, charts []string, valuesFile string) (*ValidateReport, error)
}
