/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"os"
	"text/template"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// Options for KEDA generation
var kedaOpts struct {
	Name        string
	Deployment  string
	Namespace   string
	Schedule    string
	EndSchedule string
	Replicas    string
	Timezone    string
}

// Template for ScaledObject

// Re-thinking template to match docs/GUIA-KEDA.md exactly
const kedaCronTemplate = `apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  scaleTargetRef:
    name: {{ .Deployment }}
  minReplicaCount: 0
  maxReplicaCount: {{ .Replicas }}
  triggers:
  - type: cron
    metadata:
      timezone: {{ .Timezone }}
      start: {{ .Schedule }}
      end: {{ .EndSchedule }}
      desiredReplicas: "0"
`

var kedaCmd = &cobra.Command{
	Use:   "keda",
	Short: "Gerar um KEDA ScaledObject (Cron)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Modo headless: se deployment está preenchido por flag, aplicar defaults e pular prompts
		if kedaOpts.Deployment != "" {
			if kedaOpts.Name == "" {
				kedaOpts.Name = "scale-to-zero"
			}
			if kedaOpts.Namespace == "" {
				kedaOpts.Namespace = "default"
			}
		}

		// Coletar informações faltantes via prompts
		if kedaOpts.Name == "" {
			val, err := prompter.Input("Nome do ScaledObject:", "scale-to-zero")
			if err != nil {
				return errors.Wrap(err, errors.ErrCodeExec, "falha ao coletar dados KEDA")
			}
			kedaOpts.Name = val
		}
		if kedaOpts.Namespace == "" {
			val, err := prompter.Input("Namespace da aplicação:", "apps")
			if err != nil {
				return errors.Wrap(err, errors.ErrCodeExec, "falha ao coletar dados KEDA")
			}
			kedaOpts.Namespace = val
		}
		if kedaOpts.Deployment == "" {
			val, err := prompter.Input("Nome do Deployment alvo:", "")
			if err != nil {
				return errors.Wrap(err, errors.ErrCodeExec, "falha ao coletar dados KEDA")
			}
			if val == "" {
				return errors.New(errors.ErrCodeValidation, "nome do deployment é obrigatório")
			}
			kedaOpts.Deployment = val
		}

		// Defaults for schedule if not provided (Flags have defaults, but check logic)
		// ... actually flags handle defaults well.

		// Data for template
		data := struct {
			Name        string
			Namespace   string
			Deployment  string
			Replicas    string
			Timezone    string
			Schedule    string
			EndSchedule string
		}{
			Name:        kedaOpts.Name,
			Namespace:   kedaOpts.Namespace,
			Deployment:  kedaOpts.Deployment,
			Replicas:    kedaOpts.Replicas,
			Timezone:    kedaOpts.Timezone,
			Schedule:    kedaOpts.Schedule,
			EndSchedule: kedaOpts.EndSchedule,
		}

		t := template.Must(template.New("keda").Parse(kedaCronTemplate))
		if err := t.Execute(os.Stdout, data); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao renderizar template KEDA")
		}
		return nil
	},
}

func init() {
	generateCmd.AddCommand(kedaCmd)

	kedaCmd.Flags().StringVar(&kedaOpts.Name, "name", "", "Nome do ScaledObject")
	kedaCmd.Flags().StringVar(&kedaOpts.Deployment, "deployment", "", "Nome do Deployment")
	kedaCmd.Flags().StringVar(&kedaOpts.Namespace, "namespace", "", "Namespace")
	kedaCmd.Flags().StringVar(&kedaOpts.Schedule, "schedule", "0 20 * * *", "Cron de início (desligar)")
	kedaCmd.Flags().StringVar(&kedaOpts.Replicas, "replicas", "1", "Máximo de réplicas ao ligar")
	kedaCmd.Flags().StringVar(&kedaOpts.Timezone, "timezone", "America/Sao_Paulo", "Timezone")
	kedaCmd.Flags().StringVar(&kedaOpts.EndSchedule, "end-schedule", "0 8 * * *", "Cron schedule para desligar o scaler (padrão: 08:00 diário)")
}
