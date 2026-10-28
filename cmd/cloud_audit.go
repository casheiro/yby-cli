/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"io"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/cloud"
	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/spf13/cobra"
)

var cloudAuditCmd = &cobra.Command{
	Use:     "audit",
	Short:   "Visualiza log de auditoria de operações cloud",
	Example: `  yby cloud audit --since 2026-04-01 --provider aws`,
	RunE:    runCloudAudit,
}

func init() {
	cloudCmd.AddCommand(cloudAuditCmd)
	cloudAuditCmd.Flags().String("since", "", "Filtrar eventos desde data (formato: 2006-01-02)")
	cloudAuditCmd.Flags().String("export", "", "Formato de exportação (json, csv)")
	cloudAuditCmd.Flags().String("provider", "", "Filtrar por provider (aws, azure, gcp)")
}

func runCloudAudit(cmd *cobra.Command, args []string) error {
	sinceStr, _ := cmd.Flags().GetString("since")
	exportFmt, _ := cmd.Flags().GetString("export")
	provider, _ := cmd.Flags().GetString("provider")

	if exportFmt != "" && exportFmt != "json" && exportFmt != "csv" {
		return errors.New(errors.ErrCodeValidation, "formato de exportação não suportado: "+exportFmt).
			WithHint("use --export json ou --export csv")
	}

	var sinceTime time.Time
	if sinceStr != "" {
		parsed, err := time.Parse("2006-01-02", sinceStr)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeValidation, "formato de data inválido em --since").
				WithHint("use o formato AAAA-MM-DD, ex.: --since 2026-04-01")
		}
		sinceTime = parsed
	}

	logger := auditoriaCloud()
	events, err := logger.ReadEvents(sinceTime)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeIO, "falha ao ler o log de auditoria").
			WithHint("confira se ~/.yby/audit.log pode ser lido")
	}

	if provider != "" {
		var filtered []cloud.CloudAuditEvent
		for _, e := range events {
			if e.Provider == provider {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	if exportFmt != "" {
		if len(events) == 0 {
			// o aviso vai para stderr: a saída do export fica só com os dados
			ui.Check(cmd.ErrOrStderr(), ui.CheckNeutro, "nenhum evento de auditoria encontrado", "", "")
			return nil
		}
		return logger.Export(exportFmt, sinceTime, cmd.OutOrStdout())
	}

	renderCloudAudit(cmd.OutOrStdout(), events)
	return nil
}

// renderCloudAudit escreve os eventos de auditoria em colunas, com a contagem no título.
func renderCloudAudit(w io.Writer, events []cloud.CloudAuditEvent) {
	tituloComando(w, "cloud", "audit · "+contagem(len(events), "evento", "eventos"))
	if len(events) == 0 {
		ui.Check(w, ui.CheckNeutro, "nenhum evento de auditoria encontrado", "", "")
		return
	}

	s, g := ui.S(), ui.G()
	linhas := make([]ui.Linha, len(events))
	for i, e := range events {
		// a célula inteira num estilo só, como ui.Status: a Listagem não reaplica a tinta
		status := s.Ok.Render(g.Feito + " ok")
		if !e.Success {
			status = s.Erro.Render(g.Falha + " " + e.Error)
		}
		linhas[i] = ui.Linha{Celulas: []string{e.Timestamp.Format("2006-01-02 15:04:05"), e.Provider, e.Action, e.Identity, status}}
	}
	ui.Listagem(w, []ui.Coluna{
		{Titulo: "data e hora", Prioridade: 1},
		{Titulo: "provider", Prioridade: 2},
		{Titulo: "ação"},
		{Titulo: "identidade", Max: 30, Prioridade: 3},
		{Titulo: "status", Max: 30, Prioridade: 1},
	}, linhas)
}
