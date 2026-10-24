/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"log/slog"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
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
		w := cmd.OutOrStdout()
		tituloComando(w, "secret", "rotate")

		// as linhas antes e depois da rotação ficam nas mesmas colunas
		var politica []linhaCheck
		if rotateSchedule != "" {
			politica = append(politica, linhaCheck{ui.CheckNeutro, "política", rotateSchedule, "agende por CronJob no cluster ou pipeline de CI"})
		}
		resultado := []linhaCheck{
			{ui.CheckOk, "chave", "rotacionada", "novos SealedSecrets usam a chave nova"},
			{ui.CheckNeutro, "existentes", "valem", "o controller guarda as chaves antigas para decriptar"},
		}
		col := colunasDe(append(politica, resultado...)...)

		if rotateSchedule != "" {
			slog.Info("política de rotação documentada", "schedule", rotateSchedule)
			checksCom(w, col, politica...)
		}

		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSecretsService(runner, fsys)

		linhaEmAndamento(w, "removendo a chave ativa e reiniciando o controller")
		if err := svc.RotateKeys(cmd.Context()); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha na rotação de chaves Sealed Secrets").
				WithHint(hintSealedSecrets)
		}

		checksCom(w, col, resultado...)
		return nil
	},
}

func init() {
	secretsCmd.AddCommand(rotateKeysCmd)
	rotateKeysCmd.Flags().StringVar(&rotateSchedule, "schedule", "", "Expressão cron para documentar política de rotação (ex: \"0 0 1 */3 *\")")
}
