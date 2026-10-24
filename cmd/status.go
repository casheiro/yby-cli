/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/services/status"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/spf13/cobra"
)

// newStatusService permite substituição em testes.
var newStatusService = func(r shared.Runner) status.Service {
	inspector := &status.KubectlInspector{Runner: r}
	return status.NewService(inspector)
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Verifica status do cluster e apps",
	Long: `Exibe informações sobre os nós do cluster, pods do Argo CD e Ingresses configurados.
Equivalente ao antigo 'make status'.`,
	Example: `  yby status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := lookPath("kubectl"); err != nil {
			return errors.New(errors.ErrCodeCmdNotFound, "kubectl não encontrado")
		}

		runner := &shared.RealRunner{}
		svc := newStatusService(runner)
		report := svc.Check(cmd.Context())

		renderStatusReport(cmd.OutOrStdout(), report, agora())
		return nil
	},
}

// renderStatusReport escreve o relatório de status: uma seção por componente, com a
// saída do kubectl recuada ou uma linha de verificação quando não há o que listar. Sem
// acesso ao cluster, os componentes aparecem neutros com "sem cluster".
func renderStatusReport(w io.Writer, report *status.StatusReport, quando time.Time) {
	ui.Title(w, "status", quando.Format("02/01 15:04"))

	ui.Section(w, "nodes")
	if !report.Nodes.Available {
		ui.Check(w, ui.CheckErro, report.Nodes.Message, "", "")
		saidaRecuada(w, report.Nodes.Output, true)
	} else {
		saidaRecuada(w, report.Nodes.Output, false)
	}

	if report.SemCluster {
		for _, secao := range []string{"argo cd", "ingresses", "autoscaling · keda", "eficiência energética · kepler"} {
			ui.Section(w, secao)
			ui.Check(w, ui.CheckNeutro, "sem cluster", "", "")
		}
		return
	}

	componente := func(secao string, c status.ComponentStatus) {
		ui.Section(w, secao)
		switch {
		case !c.Available:
			ui.Check(w, ui.CheckAviso, c.Message, "", "")
		case c.Output == "":
			ui.Check(w, ui.CheckNeutro, c.Message, "", "")
		default:
			saidaRecuada(w, c.Output, false)
		}
	}
	componente("argo cd", report.ArgoCD)
	componente("ingresses", report.Ingress)
	componente("autoscaling · keda", report.KEDA)

	ui.Section(w, "eficiência energética · kepler")
	switch {
	case !report.Kepler.Available:
		ui.Check(w, ui.CheckNeutro, report.Kepler.Message, "", "")
	case report.Kepler.Output != "" && report.Kepler.Message == status.MensagemKeplerAtivo:
		ui.Check(w, ui.CheckOk, report.Kepler.Message, "", "")
	default:
		ui.Check(w, ui.CheckAviso, report.Kepler.Message, "", "")
	}
}

// saidaRecuada escreve a saída de um comando externo sem a quebra de linha final:
// recuada em 2 células e em Tinta quando é o conteúdo da seção; com detalhe, é o
// complemento da linha de verificação acima, recuado em 4 células, em TintaSuave e com
// as linhas longas quebradas na largura do terminal.
func saidaRecuada(w io.Writer, saida string, detalhe bool) {
	saida = strings.TrimRight(saida, "\n")
	if saida == "" {
		return
	}
	recuo, estilo := "  ", ui.S().Tinta
	if detalhe {
		recuo, estilo = "    ", ui.S().TintaSuave
	}
	for _, l := range strings.Split(saida, "\n") {
		partes := []string{strings.TrimRight(l, "\r")}
		if detalhe {
			partes = ui.Quebrar(partes[0], ui.Largura(w)-len(recuo))
		}
		for _, p := range partes {
			fmt.Fprintln(w, recuo+estilo.Render(p))
		}
	}
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
