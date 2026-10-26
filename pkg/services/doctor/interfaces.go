package doctor

import "context"

// CheckResult representa o resultado de uma verificação individual.
type CheckResult struct {
	Name     string
	Status   bool
	Message  string
	Opcional bool // a falha não compromete o ambiente (ex.: ferramenta de uma estratégia de secrets)
}

// DoctorReport agrupa os resultados de todas as verificações.
type DoctorReport struct {
	System  []CheckResult
	Tools   []CheckResult
	Cluster []CheckResult
	CRDs    []CheckResult
	Cloud   []CheckResult
}

// Service define o contrato do serviço de diagnóstico.
type Service interface {
	Run(ctx context.Context) *DoctorReport
}
