/*
Copyright © 2025 Yby Team

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/spf13/cobra"
)

// surveyAsk é uma variável para permitir mocking nos testes
var surveyAsk = survey.Ask

var sealStrategy string

// sealCmd represents the seal command
var sealCmd = &cobra.Command{
	Use:   "seal",
	Short: "Cria e encripta um Kubernetes Secret (SealedSecret ou SOPS)",
	Long: `Helper interativo para criar secrets encriptados.
Coleta nome, namespace e dados, gera um Secret Kubernetes e encripta
usando a estratégia configurada (sealed-secrets ou sops).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Yby Secret Seal - Criador de Segredos")
		fmt.Println("---------------------------------------")

		// Verificar kubectl sempre
		if _, err := lookPath("kubectl"); err != nil {
			return errors.New(errors.ErrCodeCmdNotFound, "'kubectl' não encontrado")
		}

		// Verificar dependência da estratégia
		if sealStrategy == "sops" {
			if _, err := lookPath("sops"); err != nil {
				return errors.New(errors.ErrCodeCmdNotFound, "'sops' não encontrado. Instale em: https://github.com/getsops/sops")
			}
		} else {
			if _, err := lookPath("kubeseal"); err != nil {
				return errors.New(errors.ErrCodeCmdNotFound, "'kubeseal' não encontrado")
			}
		}

		answers := struct {
			Name      string
			Namespace string
			Key       string
			Value     string
		}{}

		qs := []*survey.Question{
			{
				Name: "Name",
				Prompt: &survey.Input{
					Message: "Nome do Secret:",
				},
				Validate: survey.Required,
			},
			{
				Name: "Namespace",
				Prompt: &survey.Input{
					Message: "Namespace:",
					Default: "default",
				},
				Validate: survey.Required,
			},
			{
				Name: "Key",
				Prompt: &survey.Input{
					Message: "Chave do Dado (ex: password):",
				},
				Validate: validateSecretKey,
			},
			{
				Name: "Value",
				Prompt: &survey.Password{
					Message: "Valor do Dado:",
				},
				Validate: survey.Required,
			},
		}

		err := surveyAsk(qs, &answers)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao coletar dados do secret")
		}

		// 1. Gerar Secret (Dry Run)
		fmt.Println("Gerando Secret...")
		kubectlCmd := execCommand("kubectl", "create", "secret", "generic", answers.Name,
			"--namespace", answers.Namespace,
			fmt.Sprintf("--from-literal=%s=%s", answers.Key, answers.Value),
			"--dry-run=client", "-o", "yaml")

		var secretYaml bytes.Buffer
		kubectlCmd.Stdout = &secretYaml
		if err := kubectlCmd.Run(); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao gerar secret")
		}

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}

		if sealStrategy == "sops" {
			// 2a. Encriptar com SOPS + age
			fmt.Println("Encriptando com SOPS...")
			filename := fmt.Sprintf("sops-secret-%s.yaml", answers.Name)
			targetDir := JoinInfra(root, "charts/cluster-config/templates/secrets")

			pathPrompt := &survey.Input{
				Message: "Onde salvar o arquivo?",
				Default: filepath.Join(targetDir, filename),
			}
			var finalPath string
			_ = askOne(pathPrompt, &finalPath)

			runner := &shared.RealRunner{}
			fsys := &shared.RealFilesystem{}
			svc := newSecretsService(runner, fsys)

			if err := svc.EncryptWithSOPS(cmd.Context(), "", secretYaml.Bytes(), finalPath); err != nil {
				return errors.Wrap(err, errors.ErrCodeExec, "falha ao encriptar secret com SOPS")
			}

			fmt.Printf("\nSecret SOPS salvo em: %s\n", finalPath)
			fmt.Println("Proximo passo: Commit e Push para o GitOps aplicar.")
			fmt.Println("Para decriptar no cluster: sops --decrypt " + finalPath + " | kubectl apply -f -")
		} else {
			// 2b. Selar com Kubeseal (comportamento original)
			fmt.Println("Selando com Kubeseal...")
			kubesealCmd := execCommand("kubeseal", "--format", "yaml")
			kubesealCmd.Stdin = &secretYaml

			var sealedYaml bytes.Buffer
			kubesealCmd.Stdout = &sealedYaml

			if err := kubesealCmd.Run(); err != nil {
				return errors.Wrap(err, errors.ErrCodeExec, "falha ao selar secret")
			}

			filename := fmt.Sprintf("sealed-secret-%s.yaml", answers.Name)
			targetDir := JoinInfra(root, "charts/cluster-config/templates/events")

			pathPrompt := &survey.Input{
				Message: "Onde salvar o arquivo?",
				Default: filepath.Join(targetDir, filename),
			}
			var finalPath string
			_ = askOne(pathPrompt, &finalPath)

			_ = os.MkdirAll(filepath.Dir(finalPath), 0755)

			if err := os.WriteFile(finalPath, sealedYaml.Bytes(), 0600); err != nil {
				return errors.Wrap(err, errors.ErrCodeIO, "falha ao salvar sealed secret")
			}

			fmt.Printf("\nSealedSecret salvo em: %s\n", finalPath)
			fmt.Println("Proximo passo: Commit e Push para o GitOps aplicar.")
		}

		return nil
	},
}

func init() {
	secretsCmd.AddCommand(sealCmd)
	sealCmd.Flags().StringVar(&sealStrategy, "strategy", "sealed-secrets", "Estrategia de encriptacao: sealed-secrets, sops")
}

func validateSecretKey(val interface{}) error {
	s, ok := val.(string)
	if !ok || s == "" {
		return fmt.Errorf("chave é obrigatória")
	}
	if strings.Contains(s, "=") {
		return fmt.Errorf("a chave não pode conter '='")
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_') {
			return fmt.Errorf("caractere inválido: %c. Use apenas [a-zA-Z0-9.-_]", r)
		}
	}
	return nil
}
