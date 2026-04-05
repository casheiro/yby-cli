package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateKedaCmd(t *testing.T) {
	// Backup original stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set flags purely
	kedaOpts.Name = "test-scaler"
	kedaOpts.Deployment = "test-deploy"
	kedaOpts.Namespace = "test-ns"
	kedaOpts.Schedule = "0 18 * * *"
	kedaOpts.EndSchedule = "0 8 * * *"
	kedaOpts.Replicas = "5"
	kedaOpts.Timezone = "UTC"

	// Run command directly (mocking cobra execution context is hard, just testing the logic block if possible or run Run())
	// Since Run uses survey which requires TTY, we might fail if we don't provide all flags.
	// We provided all flags so it should skip prompts.

	_ = kedaCmd.RunE(kedaCmd, []string{})

	// Restore
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	expected := `apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: test-scaler
  namespace: test-ns
spec:
  scaleTargetRef:
    name: test-deploy
  minReplicaCount: 0
  maxReplicaCount: 5
  triggers:
  - type: cron
    metadata:
      timezone: UTC
      start: 0 18 * * *
      end: 0 8 * * *
      desiredReplicas: "0"
`

	if output != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, output)
	}
}

func TestGenerateKedaCmd_ValoresDiferentes(t *testing.T) {
	// Capturar stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Configurar com valores diferentes
	kedaOpts.Name = "prod-scaler"
	kedaOpts.Deployment = "nginx"
	kedaOpts.Namespace = "production"
	kedaOpts.Schedule = "0 22 * * 1-5"
	kedaOpts.EndSchedule = "0 8 * * *"
	kedaOpts.Replicas = "10"
	kedaOpts.Timezone = "America/Sao_Paulo"

	_ = kedaCmd.RunE(kedaCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verificar que os valores foram aplicados corretamente
	assert.Contains(t, output, "name: prod-scaler", "Deveria conter o nome do ScaledObject")
	assert.Contains(t, output, "namespace: production", "Deveria conter o namespace")
	assert.Contains(t, output, "name: nginx", "Deveria conter o nome do deployment")
	assert.Contains(t, output, "maxReplicaCount: 10", "Deveria conter o máximo de réplicas")
	assert.Contains(t, output, "timezone: America/Sao_Paulo", "Deveria conter o timezone")
	assert.Contains(t, output, "start: 0 22 * * 1-5", "Deveria conter o schedule")
	assert.Contains(t, output, "end: 0 8 * * *", "Deveria conter o end schedule fixo")
}

func TestGenerateKedaCmd_SaidaEhYAMLValido(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	kedaOpts.Name = "yaml-test"
	kedaOpts.Deployment = "app"
	kedaOpts.Namespace = "default"
	kedaOpts.Schedule = "0 20 * * *"
	kedaOpts.EndSchedule = "0 8 * * *"
	kedaOpts.Replicas = "1"
	kedaOpts.Timezone = "UTC"

	_ = kedaCmd.RunE(kedaCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verificar estrutura YAML básica
	assert.True(t, strings.HasPrefix(output, "apiVersion:"), "Saída deveria começar com apiVersion")
	assert.Contains(t, output, "kind: ScaledObject", "Deveria conter kind: ScaledObject")
	assert.Contains(t, output, "metadata:", "Deveria conter metadata")
	assert.Contains(t, output, "spec:", "Deveria conter spec")
	assert.Contains(t, output, "triggers:", "Deveria conter triggers")
}

func TestKedaCmd_Estrutura(t *testing.T) {
	assert.Equal(t, "keda", kedaCmd.Use, "Use deveria ser 'keda'")
	assert.NotEmpty(t, kedaCmd.Short, "Short não deveria ser vazio")
	assert.NotNil(t, kedaCmd.RunE, "RunE não deveria ser nil")
}

func TestKedaCmd_FlagsRegistradas(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"name", ""},
		{"deployment", ""},
		{"namespace", ""},
		{"schedule", "0 20 * * *"},
		{"replicas", "1"},
		{"timezone", "America/Sao_Paulo"},
		{"end-schedule", "0 8 * * *"},
	}

	for _, f := range flags {
		t.Run("flag_"+f.name, func(t *testing.T) {
			flag := kedaCmd.Flags().Lookup(f.name)
			assert.NotNil(t, flag, "Flag %q deveria estar registrada", f.name)
			if flag != nil {
				assert.Equal(t, f.defValue, flag.DefValue, "Valor padrão da flag %q", f.name)
			}
		})
	}
}

func TestKedaCmd_EhSubcomandoDeGenerate(t *testing.T) {
	found := false
	for _, sub := range generateCmd.Commands() {
		if sub.Name() == "keda" {
			found = true
			break
		}
	}
	assert.True(t, found, "keda deveria ser subcomando de generate")
}

func TestKedaCronTemplate_NaoEstaVazio(t *testing.T) {
	assert.NotEmpty(t, kedaCronTemplate, "kedaCronTemplate não deveria estar vazio")
	assert.Contains(t, kedaCronTemplate, "keda.sh/v1alpha1", "Template deveria conter apiVersion do KEDA")
	assert.Contains(t, kedaCronTemplate, "ScaledObject", "Template deveria conter kind ScaledObject")
}

func TestKedaCmd_RunE_Exists(t *testing.T) {
	assert.NotNil(t, kedaCmd.RunE, "kedaCmd deve usar RunE (sem panic)")
	assert.Nil(t, kedaCmd.Run, "kedaCmd não deve usar Run")
}

func TestGenerateKedaCmd_Headless(t *testing.T) {
	// Capturar stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Modo headless: apenas --deployment preenchido
	kedaOpts.Name = ""
	kedaOpts.Deployment = "my-app"
	kedaOpts.Namespace = ""
	kedaOpts.Schedule = "0 20 * * *"
	kedaOpts.EndSchedule = "0 8 * * *"
	kedaOpts.Replicas = "1"
	kedaOpts.Timezone = "UTC"

	err := kedaCmd.RunE(kedaCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err, "Modo headless não deve retornar erro")
	assert.Contains(t, output, "name: scale-to-zero", "Default name deve ser 'scale-to-zero'")
	assert.Contains(t, output, "namespace: default", "Default namespace deve ser 'default'")
	assert.Contains(t, output, "name: my-app", "Deployment deve ser o passado por flag")
}
