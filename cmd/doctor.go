/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/pkg/services/doctor"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/spf13/cobra"
)

// newDoctorService permite override em testes para injetar mocks
var newDoctorService = func(r shared.Runner) doctor.Service {
	return doctor.NewService(r)
}

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verifica dependências e saúde do ambiente",
	Long: `Verifica se as ferramentas necessárias (kubectl, helm, kubeseal) estão instaladas
e se há conexão com o cluster Kubernetes configurado.`,
	Example: `  yby doctor`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(titleStyle.Render("🩺  Yby Doctor - Verificação de Saúde"))
		fmt.Println("----------------------------------------")

		// 1. Setup DI
		runner := &shared.RealRunner{}
		docSvc := newDoctorService(runner)

		// 2. Run All Checks
		report := docSvc.Run(cmd.Context())

		// 3. Render Output
		fmt.Println(headerStyle.Render("💻 Recursos do Sistema (Local)"))
		for _, res := range report.System {
			printResult(res)
		}

		fmt.Println(headerStyle.Render("🛠️  Ferramentas Essenciais"))
		for _, tool := range report.Tools {
			printResult(tool)
		}

		fmt.Println(headerStyle.Render("🌐 Conectividade"))
		for _, conn := range report.Cluster {
			if conn.Status {
				fmt.Printf("%s\n", checkStyle.String())
			} else {
				fmt.Printf("\n%s Falha ao conectar\n", crossStyle.String())
				fmt.Println(warningStyle.Render("   " + conn.Message))
			}
		}

		fmt.Println(headerStyle.Render("🏥 Integridade da Plataforma (CRDs)"))
		for _, crd := range report.CRDs {
			printResult(crd)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func printResult(res doctor.CheckResult) {
	if res.Status {
		if res.Name == "Memória" {
			// Explicit fallback style from original code if needed, but the original did:
			// %s Memória (Linux): 1600...
			fmt.Printf("%s %s: %s\n", checkStyle.String(), res.Name, res.Message)
		} else {
			fmt.Printf("%s %-25s: %s\n", checkStyle.String(), res.Name, grayStyle.Render(res.Message))
		}
	} else {
		if res.Name == "Memória" {
			fmt.Printf("%s %s\n", stepStyle.String(), res.Message)
		} else {
			msg := res.Message
			if res.Name == "docker" || strings.Contains(msg, "Ausente") || strings.Contains(msg, "Não encontrado") {
				msg = warningStyle.Render(msg)
			} else {
				msg = grayStyle.Render(msg)
			}
			fmt.Printf("%s %-25s: %s\n", crossStyle.String(), res.Name, msg)
		}
	}
}
