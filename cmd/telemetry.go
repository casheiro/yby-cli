package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/telemetry"
	"github.com/spf13/cobra"
)

var telemetryCmd = &cobra.Command{
	Use:   "telemetry",
	Short: "Gerencia dados de telemetria do Yby CLI",
}

var telemetryExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Exporta dados de telemetria coletados (formato JSONL)",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := telemetry.TelemetryFilePath()
		if err != nil {
			return fmt.Errorf("erro ao resolver caminho de telemetria: %w", err)
		}

		data, err := telemetry.ExportEvents(path)
		if err != nil {
			return fmt.Errorf("erro ao exportar telemetria: %w", err)
		}

		if len(data) == 0 {
			fmt.Println("Nenhum dado de telemetria encontrado.")
			return nil
		}

		_, err = os.Stdout.Write(data)
		return err
	},
}

func init() {
	telemetryCmd.AddCommand(telemetryExportCmd)
	rootCmd.AddCommand(telemetryCmd)
}
