package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// arvoreDeAjuda monta uma árvore pequena com a ajuda da carta: raiz, um grupo que só
// mostra a ajuda (como o generate) e um subcomando com argumentos e flags.
func arvoreDeAjuda() (raiz, grupo, sub *cobra.Command) {
	raiz = &cobra.Command{Use: "yby", Short: "Assistente"}
	raiz.PersistentFlags().String("log-level", "info", "Nível de log")
	grupo = &cobra.Command{Use: "generate", Aliases: []string{"gen"}, Short: "Gera recursos", Run: func(c *cobra.Command, _ []string) { _ = c.Help() }}
	sub = &cobra.Command{
		Use:     "keda [nome]",
		Short:   "Gera um ScaledObject",
		Example: "  yby generate keda api\n  # Cria o ScaledObject",
		Run:     func(*cobra.Command, []string) {},
	}
	sub.Flags().StringP("namespace", "n", "", "Namespace do recurso")
	sub.Flags().Int("replicas", 2, "Réplicas mínimas")
	sub.Flags().Bool("dry-run", false, "Só mostra")
	sub.Flags().Duration("janela", 30*time.Second, "Janela")
	sub.Flags().String("oculta", "", "não aparece")
	_ = sub.Flags().MarkHidden("oculta")
	grupo.AddCommand(sub)
	raiz.AddCommand(grupo)
	configurarAjuda(raiz)
	return raiz, grupo, sub
}

func TestAjudaDe_Subcomando(t *testing.T) {
	raiz, _, _ := arvoreDeAjuda()
	buf := new(bytes.Buffer)
	raiz.SetOut(buf)
	raiz.SetArgs([]string{"generate", "keda", "--help"})
	assert.NoError(t, raiz.Execute())

	saida := buf.String()
	for _, esperado := range []string{
		"YBY  GENERATE KEDA", "yby generate keda [nome] [flags]",
		"-n, --namespace texto", "--replicas n", "Réplicas mínimas (padrão 2)",
		"--janela duração", "(padrão 30s)", "-h, --help", "mostra esta ajuda",
		"FLAGS GLOBAIS", "--log-level texto", "(padrão info)",
		"yby generate keda api", "# Cria o ScaledObject",
	} {
		assert.Contains(t, saida, esperado)
	}
	assert.NotContains(t, saida, "oculta")
	assert.NotContains(t, saida, "Só mostra (padrão")
	for _, ingles := range []string{"Usage:", "Flags:", "help for", "Examples:", "default"} {
		assert.NotContains(t, saida, ingles)
	}
}

func TestAjudaDe_GrupoSemArgumentos(t *testing.T) {
	_, grupo, _ := arvoreDeAjuda()
	a := ajudaDe(grupo)
	assert.Equal(t, []string{"yby generate <comando>"}, a.Uso, "grupo que só mostra a ajuda não tem linha própria de uso")
	assert.Equal(t, []string{"gen"}, a.Apelidos)
	assert.Equal(t, "yby generate <comando> --help mostra os detalhes de cada um", a.Rodape)
	assert.Len(t, a.Comandos, 1)
	assert.Equal(t, "keda", a.Comandos[0].Nome)
}

func TestAjudaDe_Raiz(t *testing.T) {
	raiz, _, _ := arvoreDeAjuda()
	raiz.InitDefaultHelpCmd()
	a := ajudaDe(raiz)
	assert.Equal(t, []string{"yby <comando>"}, a.Uso)
	var help string
	for _, c := range a.Comandos {
		if c.Nome == "help" {
			help = c.Descricao
		}
	}
	assert.Equal(t, "Mostra a ajuda de um comando", help)

	for _, c := range raiz.Commands() {
		if c.Name() == "help" {
			ah := ajudaDe(c)
			assert.Equal(t, "Mostra a ajuda de qualquer comando: yby help <comando>", ah.Descricao)
			assert.True(t, strings.HasPrefix(ah.Uso[0], "yby help [comando]"))
		}
	}
}

func TestAjuda_RootReal(t *testing.T) {
	usarHomeTemporaria(t)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	t.Cleanup(func() { rootCmd.SetOut(nil) })
	rootCmd.SetArgs([]string{"env", "--help"})
	assert.NoError(t, rootCmd.Execute())

	saida := buf.String()
	assert.Contains(t, saida, "YBY  ENV")
	assert.Contains(t, saida, "yby env <comando>")
	assert.Contains(t, saida, "também yby context")
	assert.NotContains(t, saida, "Available Commands")
}
