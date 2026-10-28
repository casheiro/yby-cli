package cmd

import (
	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/telemetry"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
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
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao localizar o arquivo de telemetria").
				WithHint("confira se a pasta ~/.yby existe e pode ser lida")
		}

		data, err := telemetry.ExportEvents(path)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao exportar a telemetria").
				WithHint("confira se ~/.yby/telemetry.jsonl pode ser lido")
		}

		if len(data) == 0 {
			// o aviso vai para stderr: a saída do export fica só com os dados
			ui.Check(cmd.ErrOrStderr(), ui.CheckNeutro, "nenhum dado de telemetria encontrado", "", "")
			return nil
		}

		_, err = cmd.OutOrStdout().Write(data)
		return err
	},
}

func init() {
	telemetryCmd.AddCommand(telemetryExportCmd)
	rootCmd.AddCommand(telemetryCmd)
}
