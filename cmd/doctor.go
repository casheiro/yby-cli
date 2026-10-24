/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/doctor"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/spf13/cobra"
)

// newDoctorService permite override em testes para injetar mocks
var newDoctorService = func(r shared.Runner) doctor.Service {
	return doctor.NewService(r)
}

// agora permite fixar o relógio em testes (data e hora no título das saídas).
var agora = time.Now

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verifica dependências e saúde do ambiente",
	Long: `Verifica se as ferramentas necessárias (kubectl, helm, kubeseal) estão instaladas
e se há conexão com o cluster Kubernetes configurado.`,
	Example: `  yby doctor`,
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &shared.RealRunner{}
		docSvc := newDoctorService(runner)
		report := docSvc.Run(cmd.Context())

		renderDoctorReport(cmd.OutOrStdout(), report, agora())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

// linhaCheck é uma linha de verificação já classificada, pronta para ui.CheckCom.
type linhaCheck struct {
	tipo               ui.TipoCheck
	nome, estado, nota string
}

// secaoCheck agrupa as linhas de uma seção do relatório.
type secaoCheck struct {
	nome   string
	linhas []linhaCheck
}

// renderChecks escreve as seções com colunas alinhadas pelo nome mais largo e o resumo
// com a contagem no fim. frase vale para quando há erro; sem erro nem aviso, o resumo
// diz que o ambiente está pronto.
func renderChecks(w io.Writer, secoes []secaoCheck, fraseErro string) {
	var nomes []string
	for _, s := range secoes {
		for _, l := range s.linhas {
			// linha só com nome não tem coluna a alinhar
			if l.estado != "" || l.nota != "" {
				nomes = append(nomes, l.nome)
			}
		}
	}
	col := ui.ColunasPara(nomes...)
	col.Versao = 12

	var ok, aviso, erro int
	for _, s := range secoes {
		if len(s.linhas) == 0 {
			continue
		}
		ui.Section(w, s.nome)
		for _, l := range s.linhas {
			switch l.tipo {
			case ui.CheckOk:
				ok++
			case ui.CheckAviso:
				aviso++
			case ui.CheckErro:
				erro++
			}
			ui.CheckCom(w, col, l.tipo, l.nome, l.estado, l.nota)
		}
	}

	frase := ""
	switch {
	case erro > 0:
		frase = fraseErro
	case aviso == 0:
		frase = "ambiente pronto"
	}
	ui.Summary(w, ok, aviso, erro, frase)
}

// renderDoctorReport escreve o relatório do doctor: uma seção por grupo de verificações
// e o resumo com a contagem no fim.
func renderDoctorReport(w io.Writer, report *doctor.DoctorReport, quando time.Time) {
	ui.Title(w, "doctor", quando.Format("02/01 15:04"))

	sistema := secaoCheck{nome: "sistema"}
	for _, res := range report.System {
		if res.Status {
			sistema.linhas = append(sistema.linhas, linhaCheck{ui.CheckOk, "memória", formatarMemoria(res.Message), ""})
		} else {
			sistema.linhas = append(sistema.linhas, linhaCheck{ui.CheckNeutro, "memória", "ignorada", "verificação só no Linux"})
		}
	}

	ferramentas := secaoCheck{nome: "ferramentas"}
	for _, res := range report.Tools {
		l := linhaCheck{nome: res.Name}
		switch {
		case res.Name == "docker" && res.Status:
			l.tipo, l.estado = ui.CheckOk, "ativo"
		case res.Name == "docker":
			l.tipo, l.estado, l.nota = ui.CheckErro, "parado", res.Message
		case res.Status:
			l.tipo, l.estado, l.nota = ui.CheckOk, "instalado", res.Message
		case res.Opcional:
			l.tipo, l.estado, l.nota = ui.CheckNeutro, "ausente", "opcional"
		case res.Message == "Não encontrado":
			l.tipo, l.estado = ui.CheckErro, "ausente"
		default:
			l.tipo, l.estado, l.nota = ui.CheckErro, "falhou", res.Message
		}
		ferramentas.linhas = append(ferramentas.linhas, l)
	}

	cluster := secaoCheck{nome: "cluster"}
	clusterOk := true
	for _, res := range report.Cluster {
		if res.Status {
			cluster.linhas = append(cluster.linhas, linhaCheck{ui.CheckOk, "conexão", "ok", ""})
		} else {
			clusterOk = false
			cluster.linhas = append(cluster.linhas, linhaCheck{ui.CheckAviso, "conexão", "sem acesso", res.Message})
		}
	}

	plataforma := secaoCheck{nome: "plataforma"}
	for _, res := range report.CRDs {
		l := linhaCheck{nome: res.Name}
		switch {
		case !clusterOk:
			l.tipo, l.estado = ui.CheckNeutro, "sem cluster"
		case res.Status:
			l.tipo, l.estado = ui.CheckOk, "instalado"
		default:
			l.tipo, l.estado, l.nota = ui.CheckAviso, "ausente", "CRD não instalado"
		}
		plataforma.linhas = append(plataforma.linhas, l)
	}

	cloud := secaoCheck{nome: "cloud"}
	for _, res := range report.Cloud {
		l := linhaCheck{nome: res.Name, estado: res.Message}
		switch {
		case res.Name == "Cloud Providers":
			l = linhaCheck{tipo: ui.CheckNeutro, nome: "nenhum provider detectado"}
		case res.Status:
			l.tipo = ui.CheckOk
		default:
			l.tipo = ui.CheckAviso
		}
		cloud.linhas = append(cloud.linhas, l)
	}

	renderChecks(w, []secaoCheck{sistema, ferramentas, cluster, plataforma, cloud}, "corrija os erros antes do yby up")
}

// formatarMemoria converte "16000000 kB" (MemTotal do /proc/meminfo) em "15,3 GB".
// Valores em outro formato voltam como vieram.
func formatarMemoria(s string) string {
	campos := strings.Fields(s)
	if len(campos) != 2 || campos[1] != "kB" {
		return s
	}
	kb, err := strconv.ParseFloat(campos[0], 64)
	if err != nil {
		return s
	}
	gb := fmt.Sprintf("%.1f GB", kb/1024/1024)
	return strings.Replace(gb, ".", ",", 1)
}
