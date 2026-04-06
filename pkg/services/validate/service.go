package validate

import (
	"context"
	"fmt"

	"github.com/casheiro/yby-cli/pkg/errors"
)

type validateService struct {
	helm HelmRunner
}

// NewService cria uma nova instancia do servico de validacao.
func NewService(helm HelmRunner) Service {
	return &validateService{helm: helm}
}

// Run executa a validacao completa dos charts: dependencias, lint e template.
// Retorna um relatorio detalhado e erro caso alguma etapa falhe.
func (s *validateService) Run(ctx context.Context, charts []string, valuesFile string) (*ValidateReport, error) {
	report := &ValidateReport{
		Charts:  make([]ChartResult, len(charts)),
		Success: true,
	}

	for i, chart := range charts {
		report.Charts[i].Chart = chart
	}

	// Etapa 0: resolver dependencias
	for i, chart := range charts {
		if err := s.helm.DependencyBuild(ctx, chart); err != nil {
			report.Charts[i].Error = fmt.Sprintf("falha ao resolver dependencias: %v", err)
			report.Success = false
			return report, errors.Wrap(err, errors.ErrCodeExec,
				fmt.Sprintf("Erro ao atualizar subcharts de %s", chart))
		}
		report.Charts[i].DepsOK = true
	}

	// Etapa 1: lint
	for i, chart := range charts {
		if err := s.helm.Lint(ctx, chart); err != nil {
			report.Charts[i].Error = fmt.Sprintf("falha no lint: %v", err)
			report.Success = false
			return report, errors.Wrap(err, errors.ErrCodeExec,
				fmt.Sprintf("Erro no lint do chart %s", chart))
		}
		report.Charts[i].LintOK = true
	}

	// Etapa 2: template (dry-run)
	for i, chart := range charts {
		out, err := s.helm.Template(ctx, "release-name", chart, valuesFile)
		if err != nil {
			report.Charts[i].Error = fmt.Sprintf("falha no template: %s", string(out))
			report.Success = false
			return report, errors.Wrap(err, errors.ErrCodeManifest,
				fmt.Sprintf("Erro na renderizacao do template %s", chart))
		}
		report.Charts[i].TemplateOK = true
	}

	return report, nil
}
