/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"log/slog"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/spf13/cobra"
)

var rotateSchedule string

var rotateKeysCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotaciona a chave de encriptação do Sealed Secrets",
	Long: `Remove a chave ativa atual do Sealed Secrets e reinicia o controller,
forçando a geração de uma nova chave de encriptação.

IMPORTANTE: Faça backup da chave atual antes de rotacionar!
  yby secret backup

Após a rotação, todos os SealedSecrets existentes continuam funcionando
(o controller mantém chaves antigas para decriptação), mas novos secrets
serão selados com a nova chave.

A flag --schedule é apenas informativa para documentar a política de rotação
recomendada (ex: --schedule "0 0 1 */3 *" para rotação trimestral).
O agendamento real deve ser configurado via CronJob no cluster ou pipeline CI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(titleStyle.Render("🔐 Rotação de Chave - Sealed Secrets"))

		if rotateSchedule != "" {
			slog.Info("política de rotação documentada", "schedule", rotateSchedule)
			fmt.Printf("Política de rotação: %s\n", rotateSchedule)
			fmt.Println("Nota: o agendamento deve ser configurado via CronJob ou pipeline CI.")
		}

		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSecretsService(runner, fsys)

		fmt.Println("Removendo chave ativa e reiniciando controller...")
		if err := svc.RotateKeys(cmd.Context()); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha na rotação de chaves Sealed Secrets").
				WithHint("Verifique se o Sealed Secrets está instalado: kubectl get pods -n sealed-secrets")
		}

		fmt.Println(checkStyle.Render("✅ Chave rotacionada com sucesso."))
		fmt.Println("Novos SealedSecrets serão selados com a nova chave.")
		fmt.Println("Secrets existentes continuam funcionando (chaves antigas são mantidas para decriptação).")
		return nil
	},
}

func init() {
	secretsCmd.AddCommand(rotateKeysCmd)
	rotateKeysCmd.Flags().StringVar(&rotateSchedule, "schedule", "", "Expressão cron para documentar política de rotação (ex: \"0 0 1 */3 *\")")
}
