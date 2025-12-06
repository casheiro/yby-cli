/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// Options for KEDA generation
var kedaOpts struct {
	Name       string
	Deployment string
	Namespace  string
	Schedule   string
	Replicas   string
	Timezone   string
}

// Template for ScaledObject
const kedaTemplate = `apiVersion: keda.sh/v1alpha1
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
      # Cron format: Minute Hour Day Month DayOfWeek
      # Logic: DesiredReplicas 0 during the defined window (start -> end).
      # Note: This simple template assumes a fixed window for simplicity or custom cron.
      # Ideally user inputs start/end, but we are using raw cron for flexibility in this MVP.
      # Wait, standard cron scaler uses start/end. Let's stick to the docs example structure for scale-to-zero.
      start: {{ .Schedule }}
      end: "0 8 * * *" # Default wake up, or we might need another prompt.
      desiredReplicas: "0"
`

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
	Run: func(cmd *cobra.Command, args []string) {
		// Collect missing info via prompts
		qs := []*survey.Question{}

		if kedaOpts.Name == "" {
			qs = append(qs, &survey.Question{
				Name: "name",
				Prompt: &survey.Input{
					Message: "Nome do ScaledObject:",
					Default: "scale-to-zero",
				},
			})
		}
		if kedaOpts.Namespace == "" {
			qs = append(qs, &survey.Question{
				Name: "namespace",
				Prompt: &survey.Input{
					Message: "Namespace da aplicação:",
					Default: "apps",
				},
			})
		}
		if kedaOpts.Deployment == "" {
			qs = append(qs, &survey.Question{
				Name: "deployment",
				Prompt: &survey.Input{
					Message: "Nome do Deployment alvo:",
				},
				Validate: survey.Required,
			})
		}

		// Prompt answers
		answers := struct {
			Name       string
			Namespace  string
			Deployment string
		}{}

		// Pre-fill with flags
		answers.Name = kedaOpts.Name
		answers.Namespace = kedaOpts.Namespace
		answers.Deployment = kedaOpts.Deployment

		if len(qs) > 0 {
			err := survey.Ask(qs, &answers)
			if err != nil {
				fmt.Println(err)
				return
			}
		}

		// Apply back to opts for generation
		kedaOpts.Name = answers.Name
		kedaOpts.Namespace = answers.Namespace
		kedaOpts.Deployment = answers.Deployment

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
			EndSchedule: "0 8 * * *", // Hardcoded end for now based on simplicity request, or could prompt.
		}

		t := template.Must(template.New("keda").Parse(kedaCronTemplate))
		err := t.Execute(os.Stdout, data)
		if err != nil {
			panic(err)
		}
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
}
