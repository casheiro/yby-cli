package cmd

import (
	"bytes"
	stdErr "errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/plugin"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pluginClonado põe em raiz/.yby/plugins um yby-plugin-<nome> (com .exe no Windows) que,
// se alguém o rodar, cria o arquivo devolvido em marcador (no Windows ele nem roda).
func pluginClonado(t *testing.T, raiz, nome string) (binario, marcador string) {
	t.Helper()
	pasta := filepath.Join(raiz, ".yby", "plugins")
	require.NoError(t, os.MkdirAll(pasta, 0o755))
	arquivo := "yby-plugin-" + nome
	if runtime.GOOS == "windows" {
		arquivo += ".exe"
	}
	binario = filepath.Join(pasta, arquivo)
	marcador = filepath.Join(raiz, "executou")
	require.NoError(t, os.WriteFile(binario, []byte("#!/bin/sh\ntouch '"+marcador+"'\n"), 0o755))
	return binario, marcador
}

func TestDiscoverPlugins_NaoConfiavelNaoRodaEDaADicaDoTrust(t *testing.T) {
	raiz := usarProjeto(t, "")
	_, marcador := pluginClonado(t, raiz, "intruso")
	anterior := linhaDoYby
	linhaDoYby = func() []string { return []string{"intruso"} }
	t.Cleanup(func() { linhaDoYby = anterior })

	raizCmd := &cobra.Command{Use: "test-root"}
	pm := plugin.NewManager()
	pm.EnableTrustCheck()
	discoverPlugins(raizCmd, pm)
	// no Windows o script nem roda, e o marcador não prova nada: quem prova que o binário
	// não rodou é o manifesto sintético
	carregado, achado := pm.GetPlugin("intruso")
	require.True(t, achado)
	assert.True(t, carregado.Manifest.NaoConfiavel, "o plugin entra com o manifesto sintético, sem rodar o hook manifest")

	var sub *cobra.Command
	for _, c := range raizCmd.Commands() {
		if c.Name() == "intruso" {
			sub = c
		}
	}
	require.NotNil(t, sub, "o plugin não confiável entra pelo nome do arquivo")
	assert.Equal(t, "não confiável · rode yby plugin trust intruso", sub.Short)
	assert.NotContains(t, subcomandosDocumentados(raizCmd), sub, "o gen-docs não documenta plugin não confiável")

	err := sub.RunE(sub, nil)
	require.Error(t, err)
	var yerr *errors.YbyError
	require.True(t, stdErr.As(err, &yerr))
	assert.Equal(t, "o plugin intruso falhou", yerr.Message)
	assert.Equal(t, "confira a origem do binário e rode yby plugin trust intruso", yerr.GetHint(),
		"a dica do executor chega ao usuário, com o nome que o trust aceita")

	assert.NoFileExists(t, marcador, "nem a descoberta nem o comando podem rodar o binário")
}

func TestPluginTrust_PluginNaoConfiavelPeloNomeDoArquivo(t *testing.T) {
	raiz := usarProjeto(t, "")
	binario, marcador := pluginClonado(t, raiz, "intruso")

	lista := executar(t, pluginListCmd, raiz)
	assert.Contains(t, lista, "intruso")
	assert.Contains(t, lista, "não confiável · rode yby plugin trust intruso")
	assert.NoFileExists(t, marcador, "o yby plugin list não roda binário não confiável")

	saida := executar(t, pluginTrustCmd, raiz, "intruso")
	assert.Contains(t, saida, "confiável")
	assert.NotContains(t, saida, "não encontrado")
	confiavel, err := plugin.IsTrusted(binario)
	require.NoError(t, err)
	assert.True(t, confiavel, "o trust acha o plugin pelo nome que a dica mostra")

	saida = executar(t, pluginUntrustCmd, raiz, "intruso")
	assert.Contains(t, saida, "removido")
	confiavel, err = plugin.IsTrusted(binario)
	require.NoError(t, err)
	assert.False(t, confiavel, "o untrust tira pelo mesmo nome, com o .exe no Windows")
	assert.NoFileExists(t, marcador)
}

func TestDiscoverPlugins_PluginNaoTomaComandoDoCore(t *testing.T) {
	raiz := usarProjeto(t, "")
	var marcadores []string
	for _, nome := range []string{"help", "completion", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd, "init", "intruso"} {
		_, marcador := pluginClonado(t, raiz, nome)
		marcadores = append(marcadores, marcador)
	}

	raizCmd := &cobra.Command{Use: "test-root"}
	raizCmd.AddCommand(&cobra.Command{Use: "init", Run: func(*cobra.Command, []string) {}})
	pm := plugin.NewManager()
	pm.EnableTrustCheck()
	discoverPlugins(raizCmd, pm)

	var nomes []string
	for _, c := range raizCmd.Commands() {
		nomes = append(nomes, c.Name())
	}
	assert.ElementsMatch(t, []string{"init", "intruso"}, nomes,
		"help, completion e __complete são do cobra, init é do core: plugin nenhum toma o lugar")

	// o help e o __complete que o cobra cria depois da descoberta são os dele
	raizCmd.InitDefaultHelpCmd()
	help, _, err := raizCmd.Find([]string{"help"})
	require.NoError(t, err)
	assert.Equal(t, "Help about any command", help.Short, "o yby help continua sendo o do cobra")
	var saida bytes.Buffer
	raizCmd.SetOut(&saida)
	raizCmd.SetErr(&bytes.Buffer{})
	raizCmd.SetArgs([]string{cobra.ShellCompRequestCmd, "intr"})
	require.NoError(t, raizCmd.Execute())
	assert.Contains(t, saida.String(), "intruso", "o __complete do cobra responde o tab")

	for _, marcador := range marcadores {
		assert.NoFileExists(t, marcador)
	}
}

func TestInit_PluginNaoConfiavelNaoRodaEOAvisoDizONome(t *testing.T) {
	real := novoGerenciadorPluginsInit
	usarSaidaTexto(t, true)
	isolarInit(t)
	usarScaffoldFalso(t)
	novoGerenciadorPluginsInit = real

	dir := t.TempDir()
	_, marcador := pluginClonado(t, dir, "intruso")
	o := opcoesInit()
	o.NonInteractive, o.TargetDir, o.ProjectName = true, "infra", "loja"
	o.Topology, o.Workflow = "standard", "gitflow"

	var saida bytes.Buffer
	noDiretorio(t, dir, func() { require.NoError(t, executarInit(&saida, o)) })
	assert.Regexp(t, `plugins +1 não confiável +intruso · não rodou; rode yby plugin trust intruso\n`, saida.String())
	assert.NotContains(t, saida.String(), "<nome>")
	assert.NoFileExists(t, marcador, "o init não roda nenhum hook do plugin não confiável")
}

func TestAvisoNaoConfiaveis_ConcordaComAQuantidade(t *testing.T) {
	assert.Equal(t, "intruso · não rodou; rode yby plugin trust intruso", avisoNaoConfiaveis([]string{"intruso"}))
	assert.Equal(t, "a, b · não rodaram; rode yby plugin trust <nome>", avisoNaoConfiaveis([]string{"a", "b"}))
}

func TestPluginUntrust_SemDiferenciarMaiusculasNoWindows(t *testing.T) {
	raiz := usarProjeto(t, "")
	binario, _ := pluginClonado(t, raiz, "intruso")
	require.NoError(t, plugin.TrustPlugin(binario))

	saida := executar(t, pluginUntrustCmd, raiz, "INTRUSO")
	confiavel, err := plugin.IsTrusted(binario)
	require.NoError(t, err)
	if runtime.GOOS == "windows" {
		assert.Contains(t, saida, "removido")
		assert.False(t, confiavel, "no Windows o nome do arquivo não diferencia maiúsculas")
	} else {
		assert.Contains(t, saida, "não está no registro de confiança")
		assert.True(t, confiavel, "fora do Windows yby-plugin-INTRUSO é outro arquivo")
	}

	saida = executar(t, pluginUntrustCmd, raiz, "YBY-PLUGIN-intruso")
	if runtime.GOOS == "windows" {
		assert.Contains(t, saida, "não está no registro de confiança", "já saiu")
		assert.NotContains(t, saida, "yby-plugin-YBY-PLUGIN", "o prefixo em maiúsculas não é repetido")
	}
}
