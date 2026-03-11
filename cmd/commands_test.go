package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- bootstrap.go ---

func TestBootstrapCmd_Structure(t *testing.T) {
	assert.Equal(t, "bootstrap", bootstrapCmd.Use)
	assert.NotEmpty(t, bootstrapCmd.Short)
	// Deve ter subcomandos vps e cluster
	subs := map[string]bool{}
	for _, c := range bootstrapCmd.Commands() {
		subs[c.Name()] = true
	}
	assert.True(t, subs["vps"], "bootstrap deve ter subcomando vps")
	assert.True(t, subs["cluster"], "bootstrap deve ter subcomando cluster")
}

// --- generate.go ---

func TestGenerateCmd_Structure(t *testing.T) {
	assert.Equal(t, "generate", generateCmd.Use)
	assert.Contains(t, generateCmd.Aliases, "gen")
	assert.NotEmpty(t, generateCmd.Short)
}

// --- chart.go ---

func TestChartCmd_Structure(t *testing.T) {
	assert.Equal(t, "chart", chartCmd.Use)
	assert.NotEmpty(t, chartCmd.Short)
}

// --- plugin.go ---

func TestPluginCmd_Structure(t *testing.T) {
	assert.Equal(t, "plugin", pluginCmd.Use)
	assert.NotEmpty(t, pluginCmd.Short)
	subs := map[string]bool{}
	for _, c := range pluginCmd.Commands() {
		subs[c.Name()] = true
	}
	assert.True(t, subs["list"], "plugin deve ter subcomando list")
	assert.True(t, subs["install"], "plugin deve ter subcomando install")
	assert.True(t, subs["remove"], "plugin deve ter subcomando remove")
	assert.True(t, subs["update"], "plugin deve ter subcomando update")
}

func TestPluginInstallCmd_Flags(t *testing.T) {
	f := pluginInstallCmd.Flags().Lookup("version")
	assert.NotNil(t, f, "flag --version deve estar registrada")
	f2 := pluginInstallCmd.Flags().Lookup("force")
	assert.NotNil(t, f2, "flag --force deve estar registrada")
}

func TestPluginRemoveCmd_Aliases(t *testing.T) {
	assert.Contains(t, pluginRemoveCmd.Aliases, "rm")
	assert.Contains(t, pluginRemoveCmd.Aliases, "uninstall")
	assert.Contains(t, pluginRemoveCmd.Aliases, "delete")
}

// --- completion.go ---

func TestCompletionCmd_Structure(t *testing.T) {
	assert.Equal(t, "completion [bash|zsh|fish|powershell]", completionCmd.Use)
	assert.Equal(t, []string{"bash", "zsh", "fish", "powershell"}, completionCmd.ValidArgs)
	assert.NotEmpty(t, completionCmd.Short)
}

// --- gen_docs.go ---

func TestGenDocsCmd_Structure(t *testing.T) {
	assert.Equal(t, "gen-docs [output-dir]", genDocsCmd.Use)
	assert.True(t, genDocsCmd.Hidden, "gen-docs deve ser um comando oculto")
	assert.NotEmpty(t, genDocsCmd.Short)
}

// --- style.go ---

func TestStyles_Defined(t *testing.T) {
	// Verifica que os styles foram inicializados (não são zero value)
	assert.NotEmpty(t, titleStyle.Render("test"), "titleStyle deve renderizar conteúdo")
	assert.NotEmpty(t, headerStyle.Render("test"), "headerStyle deve renderizar conteúdo")
	assert.NotEmpty(t, stepStyle.Render("test"), "stepStyle deve renderizar conteúdo")
	assert.NotEmpty(t, checkStyle.String(), "checkStyle deve ter conteúdo definido")
	assert.NotEmpty(t, crossStyle.String(), "crossStyle deve ter conteúdo definido")
	assert.NotEmpty(t, warningStyle.String(), "warningStyle deve ter conteúdo definido")
	assert.NotEmpty(t, grayStyle.Render("test"), "grayStyle deve renderizar conteúdo")
}

// --- env.go ---

func TestEnvCmd_Structure(t *testing.T) {
	assert.Equal(t, "env", envCmd.Use)
	assert.Contains(t, envCmd.Aliases, "context")
	assert.NotEmpty(t, envCmd.Short)
	subs := map[string]bool{}
	for _, c := range envCmd.Commands() {
		subs[c.Name()] = true
	}
	assert.True(t, subs["list"], "env deve ter subcomando list")
	assert.True(t, subs["use"], "env deve ter subcomando use")
	assert.True(t, subs["show"], "env deve ter subcomando show")
	assert.True(t, subs["create"], "env deve ter subcomando create")
}

func TestEnvCreateCmd_Flags(t *testing.T) {
	f := envCreateCmd.Flags().Lookup("type")
	assert.NotNil(t, f, "flag --type deve estar registrada")
	f2 := envCreateCmd.Flags().Lookup("description")
	assert.NotNil(t, f2, "flag --description deve estar registrada")
}

// --- uki.go ---

func TestUkiCmd_Structure(t *testing.T) {
	assert.Equal(t, "uki", ukiCmd.Use)
	assert.NotEmpty(t, ukiCmd.Short)
	subs := map[string]bool{}
	for _, c := range ukiCmd.Commands() {
		subs[c.Name()] = true
	}
	assert.True(t, subs["capture"], "uki deve ter subcomando capture")
}

func TestCaptureCmd_Flags(t *testing.T) {
	f := captureCmd.Flags().Lookup("ai-provider")
	assert.NotNil(t, f, "flag --ai-provider deve estar registrada")
}

// --- access.go ---

func TestAccessCmd_Structure(t *testing.T) {
	assert.Equal(t, "access", accessCmd.Use)
	assert.NotEmpty(t, accessCmd.Short)
	f := accessCmd.Flags().Lookup("context")
	assert.NotNil(t, f, "flag --context deve estar registrada")
}

// --- destroy.go ---

func TestDestroyCmd_Structure(t *testing.T) {
	assert.Equal(t, "destroy", destroyCmd.Use)
	assert.NotEmpty(t, destroyCmd.Short)
}

// --- version.go ---

func TestVersionCmd_Structure(t *testing.T) {
	assert.Equal(t, "version", versionCmd.Use)
	assert.NotEmpty(t, versionCmd.Short)
}

// --- secrets.go ---

func TestSecretsCmd_Structure(t *testing.T) {
	assert.Equal(t, "secret", secretsCmd.Use)
	assert.NotEmpty(t, secretsCmd.Short)
	subs := map[string]bool{}
	for _, c := range secretsCmd.Commands() {
		subs[c.Name()] = true
	}
	assert.True(t, subs["webhook"], "secret deve ter subcomando webhook")
	assert.True(t, subs["minio"], "secret deve ter subcomando minio")
	assert.True(t, subs["github-token"], "secret deve ter subcomando github-token")
	assert.True(t, subs["backup"], "secret deve ter subcomando backup")
	assert.True(t, subs["restore"], "secret deve ter subcomando restore")
	assert.True(t, subs["seal"], "secret deve ter subcomando seal")
}

// --- seal.go ---

func TestSealCmd_Structure(t *testing.T) {
	assert.Equal(t, "seal", sealCmd.Use)
	assert.NotEmpty(t, sealCmd.Short)
}

// --- root.go ---

func TestRootCmd_PersistentFlags(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("context")
	assert.NotNil(t, f, "flag persistente --context deve estar registrada")

	f2 := rootCmd.PersistentFlags().Lookup("log-level")
	assert.NotNil(t, f2, "flag persistente --log-level deve estar registrada")
	assert.Equal(t, "info", f2.DefValue, "valor padrão de --log-level deve ser info")

	f3 := rootCmd.PersistentFlags().Lookup("log-format")
	assert.NotNil(t, f3, "flag persistente --log-format deve estar registrada")
	assert.Equal(t, "text", f3.DefValue, "valor padrão de --log-format deve ser text")
}
