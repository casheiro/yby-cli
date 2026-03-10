package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/doctor"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/stretchr/testify/assert"
)

// capturarStdout executa fn e retorna o que foi impresso em os.Stdout.
func capturarStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("falha ao criar pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// --- Testes de printResult ---

func TestPrintResult_Success(t *testing.T) {
	res := doctor.CheckResult{
		Name:    "kubectl",
		Status:  true,
		Message: "/usr/bin/kubectl",
	}

	saida := capturarStdout(t, func() {
		printResult(res)
	})

	assert.Contains(t, saida, "kubectl")
	assert.Contains(t, saida, "/usr/bin/kubectl")
}

func TestPrintResult_Failure(t *testing.T) {
	res := doctor.CheckResult{
		Name:    "helm",
		Status:  false,
		Message: "versão incompatível",
	}

	saida := capturarStdout(t, func() {
		printResult(res)
	})

	assert.Contains(t, saida, "helm")
	assert.Contains(t, saida, "versão incompatível")
}

func TestPrintResult_Memory_Success(t *testing.T) {
	res := doctor.CheckResult{
		Name:    "Memória",
		Status:  true,
		Message: "16000000 kB",
	}

	saida := capturarStdout(t, func() {
		printResult(res)
	})

	assert.Contains(t, saida, "Memória")
	assert.Contains(t, saida, "16000000 kB")
}

func TestPrintResult_Memory_Failure(t *testing.T) {
	res := doctor.CheckResult{
		Name:    "Memória",
		Status:  false,
		Message: "Verificação detalhada ignorada (OS não Linux)",
	}

	saida := capturarStdout(t, func() {
		printResult(res)
	})

	assert.Contains(t, saida, "Verificação detalhada ignorada")
}

func TestPrintResult_Docker_Missing(t *testing.T) {
	res := doctor.CheckResult{
		Name:    "docker",
		Status:  false,
		Message: "Erro de permissão ou não rodando",
	}

	saida := capturarStdout(t, func() {
		printResult(res)
	})

	assert.Contains(t, saida, "docker")
	assert.Contains(t, saida, "Erro de permissão")
}

func TestPrintResult_Ausente(t *testing.T) {
	res := doctor.CheckResult{
		Name:    "kubeseal",
		Status:  false,
		Message: "Ausente (CRD não instalado)",
	}

	saida := capturarStdout(t, func() {
		printResult(res)
	})

	assert.Contains(t, saida, "kubeseal")
	assert.Contains(t, saida, "Ausente")
}

func TestPrintResult_NaoEncontrado(t *testing.T) {
	res := doctor.CheckResult{
		Name:    "argocd",
		Status:  false,
		Message: "Não encontrado",
	}

	saida := capturarStdout(t, func() {
		printResult(res)
	})

	assert.Contains(t, saida, "argocd")
	assert.Contains(t, saida, "Não encontrado")
}

// --- mockDoctorService implementa doctor.Service ---

type mockDoctorService struct {
	report *doctor.DoctorReport
}

func (m *mockDoctorService) Run(_ context.Context) *doctor.DoctorReport {
	return m.report
}

// --- Testes do comando doctor ---

func TestDoctorCmd_ComMock_TodosSaudaveis(t *testing.T) {
	original := newDoctorService
	defer func() { newDoctorService = original }()

	relatorio := &doctor.DoctorReport{
		System: []doctor.CheckResult{
			{Name: "Memória", Status: true, Message: "16000000 kB"},
		},
		Tools: []doctor.CheckResult{
			{Name: "kubectl", Status: true, Message: "/usr/bin/kubectl"},
			{Name: "helm", Status: true, Message: "/usr/bin/helm"},
			{Name: "docker", Status: true, Message: "Daemon acessível"},
		},
		Cluster: []doctor.CheckResult{
			{Name: "Conexão", Status: true, Message: "OK"},
		},
		CRDs: []doctor.CheckResult{
			{Name: "Prometheus Operator", Status: true, Message: "Instalado"},
			{Name: "Cert-Manager", Status: true, Message: "Instalado"},
		},
	}

	newDoctorService = func(r shared.Runner) doctor.Service {
		return &mockDoctorService{report: relatorio}
	}

	saida := capturarStdout(t, func() {
		_ = doctorCmd.RunE(doctorCmd, []string{})
	})

	assert.Contains(t, saida, "Memória")
	assert.Contains(t, saida, "16000000 kB")
	assert.Contains(t, saida, "kubectl")
	assert.Contains(t, saida, "helm")
	assert.Contains(t, saida, "Prometheus Operator")
	assert.Contains(t, saida, "Instalado")
}

func TestDoctorCmd_ComMock_ComFalhas(t *testing.T) {
	original := newDoctorService
	defer func() { newDoctorService = original }()

	relatorio := &doctor.DoctorReport{
		System: []doctor.CheckResult{
			{Name: "Memória", Status: false, Message: "Verificação detalhada ignorada (OS não Linux)"},
		},
		Tools: []doctor.CheckResult{
			{Name: "kubectl", Status: false, Message: "Não encontrado"},
			{Name: "helm", Status: true, Message: "/usr/bin/helm"},
			{Name: "docker", Status: false, Message: "Erro de permissão ou não rodando"},
		},
		Cluster: []doctor.CheckResult{
			{Name: "Conexão", Status: false, Message: "Falha ao conectar. Dica: Verifique seu KUBECONFIG."},
		},
		CRDs: []doctor.CheckResult{
			{Name: "Prometheus Operator", Status: false, Message: "Ausente (CRD não instalado)"},
		},
	}

	newDoctorService = func(r shared.Runner) doctor.Service {
		return &mockDoctorService{report: relatorio}
	}

	saida := capturarStdout(t, func() {
		_ = doctorCmd.RunE(doctorCmd, []string{})
	})

	assert.Contains(t, saida, "Não encontrado")
	assert.Contains(t, saida, "Falha ao conectar")
	assert.Contains(t, saida, "Ausente")
	assert.Contains(t, saida, "Verificação detalhada ignorada")
}

func TestDoctorCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "doctor", doctorCmd.Use)
	assert.NotEmpty(t, doctorCmd.Short)
	assert.NotEmpty(t, doctorCmd.Long)
	assert.NotEmpty(t, doctorCmd.Example)
}

func TestDoctorCmd_ConectividadeSucesso(t *testing.T) {
	original := newDoctorService
	defer func() { newDoctorService = original }()

	relatorio := &doctor.DoctorReport{
		System:  []doctor.CheckResult{},
		Tools:   []doctor.CheckResult{},
		Cluster: []doctor.CheckResult{{Name: "Conexão", Status: true, Message: "OK"}},
		CRDs:    []doctor.CheckResult{},
	}

	newDoctorService = func(r shared.Runner) doctor.Service {
		return &mockDoctorService{report: relatorio}
	}

	saida := capturarStdout(t, func() {
		_ = doctorCmd.RunE(doctorCmd, []string{})
	})

	// Quando conectividade OK, não deve mostrar "Falha ao conectar"
	assert.False(t, strings.Contains(saida, "Falha ao conectar"),
		"não deve mostrar mensagem de falha quando conectividade está OK")
}

func TestDoctorCmd_ConectividadeFalha(t *testing.T) {
	original := newDoctorService
	defer func() { newDoctorService = original }()

	relatorio := &doctor.DoctorReport{
		System:  []doctor.CheckResult{},
		Tools:   []doctor.CheckResult{},
		Cluster: []doctor.CheckResult{{Name: "Conexão", Status: false, Message: "Cluster indisponível"}},
		CRDs:    []doctor.CheckResult{},
	}

	newDoctorService = func(r shared.Runner) doctor.Service {
		return &mockDoctorService{report: relatorio}
	}

	saida := capturarStdout(t, func() {
		_ = doctorCmd.RunE(doctorCmd, []string{})
	})

	assert.Contains(t, saida, "Falha ao conectar")
	assert.Contains(t, saida, "Cluster indisponível")
}
