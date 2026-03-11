package cmd

import (
	"fmt"
	"os"
	"testing"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// ========================================================
// initConfig — testes diretos (cobertura 66.7%)
// ========================================================

func TestInitConfig_ComContextFlag(t *testing.T) {
	// Salva o valor original
	originalContextFlag := contextFlag
	defer func() {
		contextFlag = originalContextFlag
		os.Unsetenv("YBY_ENV")
	}()

	// Simula a flag --context definida
	contextFlag = "staging"

	cmd := &cobra.Command{}
	initConfig(cmd, []string{})

	// Deve ter setado a variável de ambiente
	assert.Equal(t, "staging", os.Getenv("YBY_ENV"),
		"initConfig deveria setar YBY_ENV quando contextFlag está definido")
}

func TestInitConfig_SemContextFlag(t *testing.T) {
	// Salva o valor original
	originalContextFlag := contextFlag
	originalEnv := os.Getenv("YBY_ENV")
	defer func() {
		contextFlag = originalContextFlag
		if originalEnv != "" {
			os.Setenv("YBY_ENV", originalEnv)
		} else {
			os.Unsetenv("YBY_ENV")
		}
	}()

	// Limpa a variável de ambiente e a flag
	os.Unsetenv("YBY_ENV")
	contextFlag = ""

	cmd := &cobra.Command{}
	initConfig(cmd, []string{})

	// Não deve ter setado YBY_ENV
	assert.Empty(t, os.Getenv("YBY_ENV"),
		"initConfig não deveria setar YBY_ENV quando contextFlag está vazio")
}

func TestInitConfig_NiveisDeLog(t *testing.T) {
	// Salva os valores originais
	originalLogLevel := logLevelFlag
	originalLogFormat := logFormatFlag
	originalContextFlag := contextFlag
	defer func() {
		logLevelFlag = originalLogLevel
		logFormatFlag = originalLogFormat
		contextFlag = originalContextFlag
	}()

	tests := []struct {
		nome   string
		level  string
		format string
	}{
		{"debug/json", "debug", "json"},
		{"info/text", "info", "text"},
		{"warn/json", "warn", "json"},
		{"error/text", "error", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			logLevelFlag = tt.level
			logFormatFlag = tt.format
			contextFlag = ""

			// Não deve entrar em pânico com diferentes configurações de log
			assert.NotPanics(t, func() {
				cmd := &cobra.Command{}
				initConfig(cmd, []string{})
			})
		})
	}
}

// ========================================================
// discoverPlugins — testes da lógica de descoberta
// ========================================================

// createFakePlugin cria um script shell fake que responde ao hook "manifest"
// retornando JSON válido com os dados do plugin.
func createFakePlugin(t *testing.T, dir, name, description string, hooks []string) {
	t.Helper()
	pluginsDir := dir + "/.yby/plugins"
	err := os.MkdirAll(pluginsDir, 0755)
	assert.NoError(t, err)

	// Monta o array de hooks como JSON
	hooksJSON := "["
	for i, h := range hooks {
		if i > 0 {
			hooksJSON += ","
		}
		hooksJSON += fmt.Sprintf(`"%s"`, h)
	}
	hooksJSON += "]"

	descField := ""
	if description != "" {
		descField = fmt.Sprintf(`"description":"%s",`, description)
	}

	// Script simples usando printf para evitar problemas com heredoc aninhado
	jsonResp := fmt.Sprintf(`{"data":{"name":"%s","version":"0.1.0",%s"hooks":%s}}`, name, descField, hooksJSON)
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '%s'\n", jsonResp)

	binaryName := "yby-plugin-" + name
	err = os.WriteFile(pluginsDir+"/"+binaryName, []byte(script), 0755)
	assert.NoError(t, err)
}

func TestDiscoverPlugins_EmptyDir(t *testing.T) {
	// Usa diretório temporário vazio como HOME para que o Manager não encontre plugins
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cmd := &cobra.Command{Use: "test-root"}
	pm := newRootPluginManager()

	// Não deve entrar em pânico mesmo sem plugins
	assert.NotPanics(t, func() {
		discoverPlugins(cmd, pm)
	})
}

func TestDiscoverPlugins_WithPlugins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Cria plugin fake com hook "command"
	createFakePlugin(t, dir, "meu-plugin", "Plugin de teste", []string{"command"})

	cmd := &cobra.Command{Use: "test-root"}
	pm := newRootPluginManager()
	discoverPlugins(cmd, pm)

	// Verifica que o subcomando foi registrado
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "meu-plugin" {
			found = true
			assert.Equal(t, "Plugin de teste", sub.Short,
				"descrição do subcomando deveria ser a do manifesto")
			break
		}
	}
	assert.True(t, found, "subcomando 'meu-plugin' deveria ter sido registrado")
}

func TestDiscoverPlugins_PluginWithDescription(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	createFakePlugin(t, dir, "desc-plugin", "Descrição customizada do plugin", []string{"command"})

	cmd := &cobra.Command{Use: "test-root"}
	pm := newRootPluginManager()
	discoverPlugins(cmd, pm)

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "desc-plugin" {
			found = true
			assert.Equal(t, "Descrição customizada do plugin", sub.Short,
				"descrição deveria vir do manifesto")
			break
		}
	}
	assert.True(t, found, "subcomando 'desc-plugin' deveria ter sido registrado")
}

func TestDiscoverPlugins_PluginSemDescricao(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	createFakePlugin(t, dir, "nodesc-plugin", "", []string{"command"})

	cmd := &cobra.Command{Use: "test-root"}
	pm := newRootPluginManager()
	discoverPlugins(cmd, pm)

	for _, sub := range cmd.Commands() {
		if sub.Use == "nodesc-plugin" {
			assert.Equal(t, "Executa o plugin nodesc-plugin", sub.Short,
				"sem descrição no manifesto, deveria usar descrição padrão")
			return
		}
	}
	t.Fatal("subcomando 'nodesc-plugin' deveria ter sido registrado")
}

func TestDiscoverPlugins_PluginCollision(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Cria plugin com nome que colide com comando existente
	createFakePlugin(t, dir, "existente", "Plugin colidindo", []string{"command"})

	cmd := &cobra.Command{Use: "test-root"}
	// Adiciona comando pré-existente com mesmo nome
	cmd.AddCommand(&cobra.Command{Use: "existente", Short: "Comando original"})

	pm := newRootPluginManager()
	discoverPlugins(cmd, pm)

	// Deve ter somente 1 comando "existente" (o original)
	count := 0
	for _, sub := range cmd.Commands() {
		if sub.Use == "existente" {
			count++
			assert.Equal(t, "Comando original", sub.Short,
				"comando original não deveria ser substituído pelo plugin")
		}
	}
	assert.Equal(t, 1, count, "não deveria registrar plugin com nome duplicado")
}

func TestDiscoverPlugins_PluginNoCommandHook(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Plugin que só suporta hook "context", não "command"
	createFakePlugin(t, dir, "context-only", "Plugin sem command", []string{"context"})

	cmd := &cobra.Command{Use: "test-root"}
	pm := newRootPluginManager()
	discoverPlugins(cmd, pm)

	// Não deve registrar nenhum subcomando
	for _, sub := range cmd.Commands() {
		assert.NotEqual(t, "context-only", sub.Use,
			"plugin sem hook 'command' não deveria ser registrado como subcomando")
	}
}

func TestDiscoverPlugins_DiscoverError(t *testing.T) {
	// Quando Discover não encontra nada (diretório inexistente),
	// não deve registrar nenhum comando e não deve entrar em pânico
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// HOME/.yby/plugins não existe — Discover retorna nil mas ListPlugins fica vazia

	cmd := &cobra.Command{Use: "test-root"}
	pm := newRootPluginManager()

	initialCount := len(cmd.Commands())
	discoverPlugins(cmd, pm)

	assert.Equal(t, initialCount, len(cmd.Commands()),
		"nenhum comando deveria ser registrado quando Discover não encontra plugins")
}

// ========================================================
// handleExecutionError — testes do handler de erros
// ========================================================

func TestHandleExecutionError_YbyError(t *testing.T) {
	origLogLevel := logLevelFlag
	defer func() { logLevelFlag = origLogLevel }()

	logLevelFlag = "info"

	yerr := errors.New(errors.ErrCodeValidation, "campo obrigatório ausente")

	// Não deve entrar em pânico
	assert.NotPanics(t, func() {
		handleExecutionError(yerr)
	})
}

func TestHandleExecutionError_YbyError_Debug(t *testing.T) {
	origLogLevel := logLevelFlag
	defer func() { logLevelFlag = origLogLevel }()

	logLevelFlag = "debug"

	yerr := errors.New(errors.ErrCodeIO, "falha ao ler arquivo").
		WithContext("path", "/tmp/test.yaml")

	// Não deve entrar em pânico e deve usar formato verboso %+v
	assert.NotPanics(t, func() {
		handleExecutionError(yerr)
	})
}

func TestHandleExecutionError_RegularError(t *testing.T) {
	origLogLevel := logLevelFlag
	defer func() { logLevelFlag = origLogLevel }()

	logLevelFlag = "info"

	regularErr := fmt.Errorf("erro genérico inesperado")

	// Não deve entrar em pânico
	assert.NotPanics(t, func() {
		handleExecutionError(regularErr)
	})
}

func TestHandleExecutionError_WrappedYbyError(t *testing.T) {
	origLogLevel := logLevelFlag
	defer func() { logLevelFlag = origLogLevel }()

	logLevelFlag = "info"

	baseErr := fmt.Errorf("conexão recusada")
	yerr := errors.Wrap(baseErr, errors.ErrCodeNetworkTimeout, "timeout na conexão SSH")

	assert.NotPanics(t, func() {
		handleExecutionError(yerr)
	})
}
