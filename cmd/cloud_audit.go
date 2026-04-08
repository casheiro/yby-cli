/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/casheiro/yby-cli/pkg/cloud"
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

	var sinceTime time.Time
	if sinceStr != "" {
		parsed, err := time.Parse("2006-01-02", sinceStr)
		if err != nil {
			return fmt.Errorf("formato de data inválido (esperado: 2006-01-02): %w", err)
		}
		sinceTime = parsed
	}

	logger := cloud.NewAuditLogger()
	events, err := logger.ReadEvents(sinceTime)
	if err != nil {
		return fmt.Errorf("erro ao ler eventos de auditoria: %w", err)
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

	if len(events) == 0 {
		fmt.Println(grayStyle.Render("Nenhum evento de auditoria encontrado."))
		return nil
	}

	if exportFmt != "" {
		return logger.Export(exportFmt, sinceTime, os.Stdout)
	}

	// Renderizar tabela
	fmt.Println(titleStyle.Render("Auditoria Cloud"))
	fmt.Println()
	fmt.Printf("  %-20s  %-8s  %-16s  %-30s  %s\n",
		"Data/Hora", "Provider", "Ação", "Identidade", "Status")
	fmt.Printf("  %-20s  %-8s  %-16s  %-30s  %s\n",
		"────────────────────", "────────", "────────────────", "──────────────────────────────", "──────")

	for _, e := range events {
		ts := e.Timestamp.Format("2006-01-02 15:04:05")
		status := checkStyle.Render("")
		if !e.Success {
			errMsg := e.Error
			if len(errMsg) > 20 {
				errMsg = errMsg[:20] + "..."
			}
			status = crossStyle.Render("") + " " + errMsg
		}

		identity := e.Identity
		if len(identity) > 30 {
			identity = identity[:27] + "..."
		}

		fmt.Printf("  %-20s  %-8s  %-16s  %-30s  %s\n",
			ts, e.Provider, e.Action, identity, status)
	}

	return nil
}
