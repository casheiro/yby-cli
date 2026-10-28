package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/casheiro/yby-cli/v5/pkg/services/doctor"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
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

// --- mockDoctorService implementa doctor.Service ---

type mockDoctorService struct {
	report *doctor.DoctorReport
}

func (m *mockDoctorService) Run(_ context.Context) *doctor.DoctorReport {
	return m.report
}

// relatorioSaudavel tem todas as verificações passando.
func relatorioSaudavel() *doctor.DoctorReport {
	return &doctor.DoctorReport{
		System: []doctor.CheckResult{
			{Name: "Memória", Status: true, Message: "16000000 kB"},
		},
		Tools: []doctor.CheckResult{
			{Name: "kubectl", Status: true, Message: "/usr/bin/kubectl"},
			{Name: "helm", Status: true, Message: "/usr/bin/helm"},
			{Name: "docker", Status: true, Message: "Daemon acessível"},
			{Name: "SOPS", Status: true, Message: "/usr/bin/sops", Opcional: true},
		},
		Cluster: []doctor.CheckResult{
			{Name: "Conexão", Status: true, Message: "OK"},
		},
		CRDs: []doctor.CheckResult{
			{Name: "Prometheus Operator", Status: true, Message: "Instalado"},
			{Name: "Cert-Manager", Status: true, Message: "Instalado"},
		},
		Cloud: []doctor.CheckResult{
			{Name: "aws", Status: true, Message: "CLI 2.17.0 instalado"},
			{Name: "aws credenciais", Status: true, Message: "Autenticado como arn:aws:iam::123:user/ana"},
		},
	}
}

// relatorioComFalhas mistura erro, aviso e itens opcionais ausentes.
func relatorioComFalhas() *doctor.DoctorReport {
	return &doctor.DoctorReport{
		System: []doctor.CheckResult{
			{Name: "Memória", Status: false, Message: "Verificação detalhada ignorada (OS não Linux)"},
		},
		Tools: []doctor.CheckResult{
			{Name: "kubectl", Status: false, Message: "Não encontrado"},
			{Name: "helm", Status: true, Message: "/usr/bin/helm"},
			{Name: "docker", Status: false, Message: "Erro de permissão ou não rodando"},
			{Name: "Sealed Secrets", Status: false, Message: "Não encontrado (opcional)", Opcional: true},
		},
		Cluster: []doctor.CheckResult{
			{Name: "Conexão", Status: false, Message: "confira o KUBECONFIG ou se o cluster está no ar"},
		},
		CRDs: []doctor.CheckResult{
			{Name: "Prometheus Operator", Status: false, Message: "Ausente (CRD não instalado)"},
		},
		Cloud: []doctor.CheckResult{
			{Name: "Cloud Providers", Status: true, Message: "Nenhum provider cloud detectado"},
		},
	}
}

func TestRenderDoctorReport_Golden(t *testing.T) {
	casos := []struct {
		nome      string
		relatorio *doctor.DoctorReport
		ascii     bool
	}{
		{"doctor_saudavel", relatorioSaudavel(), false},
		{"doctor_falhas", relatorioComFalhas(), false},
		{"doctor_falhas_ascii", relatorioComFalhas(), true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarSaidaTexto(t, c.ascii)
			var b bytes.Buffer
			renderDoctorReport(&b, c.relatorio, quandoGolden)
			conferirGolden(t, c.nome, b.String())
			conferirSemRestos(t, b.String())
		})
	}
}

func TestRenderDoctorReport_CRDsSemCluster(t *testing.T) {
	usarSaidaTexto(t, false)
	relatorio := &doctor.DoctorReport{
		Cluster: []doctor.CheckResult{{Name: "Conexão", Status: false, Message: "Cluster indisponível"}},
		CRDs:    []doctor.CheckResult{{Name: "KEDA", Status: false, Message: "Ausente (CRD não instalado)"}},
	}

	var b bytes.Buffer
	renderDoctorReport(&b, relatorio, quandoGolden)

	assert.Contains(t, b.String(), "▲ conexão")
	assert.Contains(t, b.String(), "Cluster indisponível")
	// sem cluster, a ausência do CRD não conta como aviso
	assert.Contains(t, b.String(), "○ KEDA")
	assert.Contains(t, b.String(), "RESUMO  0 ok · 1 aviso · 0 erros\n")
}

func TestRenderDoctorReport_Resumo(t *testing.T) {
	usarSaidaTexto(t, false)

	var b bytes.Buffer
	renderDoctorReport(&b, relatorioSaudavel(), quandoGolden)
	assert.Contains(t, b.String(), "· ambiente pronto")

	b.Reset()
	renderDoctorReport(&b, relatorioComFalhas(), quandoGolden)
	assert.Contains(t, b.String(), "2 erros  · corrija os erros antes do yby up")
}

func TestFormatarMemoria(t *testing.T) {
	assert.Equal(t, "15,3 GB", formatarMemoria("16000000 kB"))
	assert.Equal(t, "512 MB", formatarMemoria("512 MB"))
	assert.Equal(t, "abc kB", formatarMemoria("abc kB"))
}

// --- Testes do comando doctor ---

func TestDoctorCmd_ComMock_EscreveRelatorio(t *testing.T) {
	usarSaidaTexto(t, false)
	original := newDoctorService
	defer func() { newDoctorService = original }()
	newDoctorService = func(r shared.Runner) doctor.Service {
		return &mockDoctorService{report: relatorioSaudavel()}
	}

	b := saidaDoComando(t, doctorCmd)
	assert.NoError(t, doctorCmd.RunE(doctorCmd, []string{}))
	saida := b.String()

	assert.Contains(t, saida, "◆ YBY  DOCTOR")
	assert.Contains(t, saida, "24/10 09:30")
	assert.Contains(t, saida, "✓ kubectl")
	assert.Contains(t, saida, "/usr/bin/kubectl")
}

func TestDoctorCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "doctor", doctorCmd.Use)
	assert.NotEmpty(t, doctorCmd.Short)
	assert.NotEmpty(t, doctorCmd.Long)
	assert.NotEmpty(t, doctorCmd.Example)
}
