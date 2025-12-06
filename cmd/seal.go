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
	"os/exec"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// sealCmd represents the seal command
var sealCmd = &cobra.Command{
	Use:   "seal",
	Short: "Cria e sela um Kubernetes Secret (SealedSecret)",
	Long: `Helper interativo para criar SealedSecrets.
Coleta nome, namespace e dados, gera um Secret Kubernetes,
sela usando 'kubeseal' e salva o arquivo YAML no local apropriado.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔒 Yby Secret Seal - Criador de Segredos")
		fmt.Println("---------------------------------------")

		// Verificar dependências
		if _, err := exec.LookPath("kubectl"); err != nil {
			fmt.Println("❌ Erro: 'kubectl' não encontrado.")
			return
		}
		if _, err := exec.LookPath("kubeseal"); err != nil {
			fmt.Println("❌ Erro: 'kubeseal' não encontrado.")
			return
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
				Validate: survey.Required,
			},
			{
				Name: "Value",
				Prompt: &survey.Password{
					Message: "Valor do Dado:",
				},
				Validate: survey.Required,
			},
		}

		err := survey.Ask(qs, &answers)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		// 1. Gerar Secret (Dry Run)
		fmt.Println("⚙️  Gerando Secret...")
		kubectlCmd := exec.Command("kubectl", "create", "secret", "generic", answers.Name,
			"--namespace", answers.Namespace,
			fmt.Sprintf("--from-literal=%s=%s", answers.Key, answers.Value),
			"--dry-run=client", "-o", "yaml")

		var secretYaml bytes.Buffer
		kubectlCmd.Stdout = &secretYaml
		if err := kubectlCmd.Run(); err != nil {
			fmt.Printf("❌ Erro ao gerar secret: %v\n", err)
			return
		}

		// 2. Selar com Kubeseal
		fmt.Println("🔒 Selando com Kubeseal...")
		kubesealCmd := exec.Command("kubeseal", "--format", "yaml")
		kubesealCmd.Stdin = &secretYaml

		var sealedYaml bytes.Buffer
		kubesealCmd.Stdout = &sealedYaml

		if err := kubesealCmd.Run(); err != nil {
			fmt.Printf("❌ Erro ao selar secret: %v\n", err)
			fmt.Println("Dica: Verifique se você tem conexão com o cluster ou o certificado público.")
			return
		}

		// 3. Salvar Arquivo
		filename := fmt.Sprintf("sealed-secret-%s.yaml", answers.Name)
		targetDir := "charts/cluster-config/templates/events" // Default location suggestion

		// Perguntar onde salvar
		pathPrompt := &survey.Input{
			Message: "Onde salvar o arquivo?",
			Default: filepath.Join(targetDir, filename),
		}
		var finalPath string
		_ = survey.AskOne(pathPrompt, &finalPath)

		// Garantir diretório
		_ = os.MkdirAll(filepath.Dir(finalPath), 0755)

		if err := os.WriteFile(finalPath, sealedYaml.Bytes(), 0644); err != nil {
			fmt.Printf("❌ Erro ao salvar arquivo: %v\n", err)
			return
		}

		fmt.Printf("\n✅ SealedSecret salvo em: %s\n", finalPath)
		fmt.Println("👉 Próximo passo: Commit e Push para o GitOps aplicar.")
	},
}

func init() {
	secretsCmd.AddCommand(sealCmd)
}
