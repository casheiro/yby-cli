/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"os"
	"strings"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/spf13/cobra"
)

// genDocsCmd represents the gen-docs command
var genDocsCmd = &cobra.Command{
	Use:    "gen-docs [output-dir]",
	Short:  "Gera documentação Markdown para todos os comandos",
	Hidden: true, // Internal tool only
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir := "./docs/wiki"
		if len(args) > 0 {
			outputDir = args[0]
		}

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao criar diretório de saída").
				WithHint("confira o caminho e a permissão de escrita em " + outputDir)
		}

		outputFile := outputDir + "/CLI-Reference.md"

		f, err := os.Create(outputFile)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao criar arquivo de documentação").
				WithHint("confira o caminho e a permissão de escrita em " + outputDir)
		}
		defer f.Close()

		cabecalho := "# Referência da CLI\n\nTodos os comandos do yby, com a mesma ajuda de `yby <comando> --help`.\n\n"
		if _, err := f.WriteString(cabecalho); err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao gravar a documentação").
				WithHint("confira se a pasta de destino pode ser gravada")
		}

		if err := writeCommandDocs(f, rootCmd); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao gerar a documentação").
				WithHint("confira se a pasta de destino pode ser gravada")
		}

		w := cmd.OutOrStdout()
		tituloComando(w, "gen-docs", "referência da CLI")
		checksAlinhados(w, linhaCheck{ui.CheckOk, "CLI-Reference.md", "gerado", outputFile})
		return nil
	},
}

// writeCommandDocs grava a seção de cmd e, depois, a de cada subcomando visível. A seção
// sai da mesma ajudaDe do --help: descrição (com a nota de beta), uso, apelidos,
// exemplos, flags e subcomandos, em português.
func writeCommandDocs(f *os.File, cmd *cobra.Command) error {
	if _, err := f.WriteString(secaoDeReferencia(cmd)); err != nil {
		return err
	}
	for _, c := range subcomandosDocumentados(cmd) {
		if err := writeCommandDocs(f, c); err != nil {
			return err
		}
	}
	return nil
}

// subcomandosDocumentados são os subcomandos que ganham seção na referência. Plugin não
// confiável fica de fora: a descoberta não o executou e só se sabe o nome do arquivo.
func subcomandosDocumentados(cmd *cobra.Command) []*cobra.Command {
	var subs []*cobra.Command
	for _, c := range cmd.Commands() {
		if c.IsAvailableCommand() && !c.IsAdditionalHelpTopicCommand() && c.Annotations[anotacaoNaoConfiavel] == "" {
			subs = append(subs, c)
		}
	}
	return subs
}

// secaoDeReferencia monta a seção Markdown de um comando.
func secaoDeReferencia(cmd *cobra.Command) string {
	a := ajudaDe(cmd)
	var b strings.Builder
	b.WriteString("## " + a.Comando + "\n\n")
	if d := strings.TrimSpace(a.Descricao); d != "" {
		b.WriteString(d + "\n\n")
	}
	if len(a.Uso) > 0 {
		b.WriteString("**Uso**\n\n```\n" + strings.Join(a.Uso, "\n") + "\n```\n\n")
	}
	if len(a.Apelidos) > 0 {
		b.WriteString("**Apelidos:** `" + strings.Join(a.Apelidos, "`, `") + "`\n\n")
	}
	if ex := semRecuoComum(a.Exemplos); ex != "" {
		b.WriteString("**Exemplos**\n\n```bash\n" + ex + "\n```\n\n")
	}
	tabelaDeFlags(&b, "Flags", a.Flags)
	tabelaDeFlags(&b, "Flags globais", a.Globais)
	if subs := subcomandosDocumentados(cmd); len(subs) > 0 {
		b.WriteString("**Comandos**\n\n")
		for _, c := range subs {
			ancora := strings.ReplaceAll(c.CommandPath(), " ", "-")
			b.WriteString("* [" + c.CommandPath() + "](#" + ancora + ") — " + descricaoCurta(c) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("---\n\n")
	return b.String()
}

// tabelaDeFlags escreve as flags numa tabela; a barra vertical das descrições é escapada.
func tabelaDeFlags(b *strings.Builder, titulo string, flags []ui.Termo) {
	if len(flags) == 0 {
		return
	}
	b.WriteString("**" + titulo + "**\n\n| Flag | Descrição |\n| :--- | :--- |\n")
	for _, t := range flags {
		descricao := strings.ReplaceAll(t.Descricao, "|", `\|`)
		b.WriteString("| `" + strings.TrimSpace(t.Nome) + "` | " + descricao + " |\n")
	}
	b.WriteString("\n")
}

// semRecuoComum tira o recuo que todas as linhas do bloco têm em comum (os exemplos do
// cobra vêm recuados com dois espaços).
func semRecuoComum(bloco string) string {
	linhas := strings.Split(strings.Trim(strings.ReplaceAll(bloco, "\r", ""), "\n"), "\n")
	recuo := -1
	for _, l := range linhas {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if n := len(l) - len(strings.TrimLeft(l, " \t")); recuo < 0 || n < recuo {
			recuo = n
		}
	}
	if recuo < 0 {
		return ""
	}
	for i, l := range linhas {
		if len(l) >= recuo {
			linhas[i] = strings.TrimRight(l[recuo:], " \t")
		}
	}
	return strings.Join(linhas, "\n")
}

func init() {
	rootCmd.AddCommand(genDocsCmd)
}
