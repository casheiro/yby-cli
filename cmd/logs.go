/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/logs"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/spf13/cobra"
)

// newLogsService permite substituição em testes.
var newLogsService = func(r shared.Runner) logs.Service {
	return logs.NewService(r)
}

// isTTY verifica se stdout é um terminal interativo.
var isTTY = func() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// surveySelect abstrai a seleção interativa para testes.
var surveySelect = func(message string, options []string) (string, error) {
	return prompter.Select(message, options, "")
}

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs [pod]",
	Short: "Exibe logs de um pod do cluster",
	Long: `Exibe ou acompanha logs de pods Kubernetes.

Sem argumentos, apresenta uma seleção interativa de pods.
Com argumento, detecta automaticamente o namespace do pod.

Exemplos:
  yby logs                          # Seleção interativa
  yby logs nginx-abc123             # Auto-detect namespace
  yby logs nginx-abc123 -f          # Acompanhar logs (follow)
  yby logs nginx-abc123 -t 100      # Últimas 100 linhas
  yby logs nginx-abc123 --container sidecar  # Container específico`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := lookPath("kubectl"); err != nil {
			return errors.New(errors.ErrCodeCmdNotFound, "kubectl não encontrado").
				WithHint(hintKubectl)
		}

		runner := &shared.RealRunner{}
		svc := newLogsService(runner)
		ctx := cmd.Context()

		follow, _ := cmd.Flags().GetBool("follow")
		container, _ := cmd.Flags().GetString("container")
		tail, _ := cmd.Flags().GetInt("tail")

		var podName, namespace string

		if len(args) == 0 {
			// Modo interativo: seleção de pod
			if !isTTY() {
				return errors.New(errors.ErrCodeValidation, "nome do pod é obrigatório em modo não-interativo").
					WithHint("informe o pod: yby logs <nome-do-pod>")
			}

			// Usar namespace padrão para listar pods
			ns := "default"
			pods, err := svc.ListPods(ctx, ns)
			if err != nil {
				return err
			}
			if len(pods) == 0 {
				w := cmd.OutOrStdout()
				tituloComando(w, "logs", ns)
				ui.Check(w, ui.CheckNeutro, "nenhum pod no namespace "+ns, "", "")
				return nil
			}

			selected, err := surveySelect("Selecione um pod:", pods)
			if err != nil {
				return errors.Wrap(err, errors.ErrCodeExec, "seleção de pod cancelada").
					WithHint("informe o pod: yby logs <pod>")
			}

			podName = selected
			namespace = ns
		} else {
			podName = args[0]

			// Auto-detect namespace
			ns, err := svc.DetectNamespace(ctx, podName)
			if err != nil {
				return err
			}
			namespace = ns

			// Atualizar podName se foi match por prefixo
			pods, _ := svc.ListPods(ctx, namespace)
			for _, p := range pods {
				if p == podName {
					break
				}
				if len(p) > len(podName) && p[:len(podName)] == podName {
					podName = p
					break
				}
			}
		}

		// Multi-container: prompt interativo se TTY
		if container == "" {
			containers, err := svc.ListContainers(ctx, namespace, podName)
			if err == nil && len(containers) > 1 {
				if isTTY() {
					selected, err := surveySelect("Pod com múltiplos containers. Selecione:", containers)
					if err != nil {
						return errors.Wrap(err, errors.ErrCodeExec, "seleção de container cancelada").
							WithHint("informe o container com -c <container>")
					}
					container = selected
				} else {
					// Em pipes/CI, usar o primeiro container
					container = containers[0]
				}
			}
		}

		opts := logs.LogOptions{
			Namespace: namespace,
			Pod:       podName,
			Container: container,
			Follow:    follow,
			Tail:      tail,
		}

		w := cmd.OutOrStdout()
		alvo := namespace + "/" + podName
		if container != "" {
			alvo += " · " + container
		}

		if follow {
			tituloComando(w, "logs", alvo)
			return svc.StreamLogs(ctx, opts)
		}

		output, err := svc.GetLogs(ctx, opts)
		if err != nil {
			return err
		}

		tituloComando(w, "logs", alvo)
		// as linhas do kubectl logs saem como vieram
		fmt.Fprint(w, output)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().BoolP("follow", "f", false, "Acompanhar logs em tempo real")
	logsCmd.Flags().String("container", "", "Nome do container (para pods multi-container)")
	logsCmd.Flags().IntP("tail", "t", 0, "Número de linhas do final (0 = todas)")
}
