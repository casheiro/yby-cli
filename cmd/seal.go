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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/spf13/cobra"
)

// sealPrompt abstrai a coleta de dados do seal para testes.
// Retorna (name, namespace, key, value, error).
var sealPrompt = func() (string, string, string, string, error) {
	name, err := prompter.Input("Nome do Secret:", "")
	if err != nil {
		return "", "", "", "", err
	}
	if name == "" {
		return "", "", "", "", fmt.Errorf("nome é obrigatório")
	}
	ns, err := prompter.Input("Namespace:", "default")
	if err != nil {
		return "", "", "", "", err
	}
	if ns == "" {
		return "", "", "", "", fmt.Errorf("namespace é obrigatório")
	}
	key, err := prompter.Input("Chave do Dado (ex: password):", "")
	if err != nil {
		return "", "", "", "", err
	}
	if err := validateSecretKey(key); err != nil {
		return "", "", "", "", err
	}
	value, err := prompter.Password("Valor do Dado:")
	if err != nil {
		return "", "", "", "", err
	}
	if value == "" {
		return "", "", "", "", fmt.Errorf("valor é obrigatório")
	}
	return name, ns, key, value, nil
}

// sealPathPrompt abstrai a coleta do caminho de salvamento para testes.
var sealPathPrompt = func(defaultPath string) (string, error) {
	return prompter.Input("Onde salvar o arquivo?", defaultPath)
}

var sealStrategy string

// sealCmd represents the seal command
var sealCmd = &cobra.Command{
	Use:   "seal",
	Short: "Cria e encripta um Kubernetes Secret (SealedSecret ou SOPS)",
	Long: `Helper interativo para criar secrets encriptados.
Coleta nome, namespace e dados, gera um Secret Kubernetes e encripta
usando a estratégia configurada (sealed-secrets ou sops).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Verificar kubectl sempre
		if _, err := lookPath("kubectl"); err != nil {
			return errors.New(errors.ErrCodeCmdNotFound, "kubectl não encontrado").
				WithHint(hintKubectl)
		}

		// Verificar dependência da estratégia
		if sealStrategy == "sops" {
			if _, err := lookPath("sops"); err != nil {
				return errors.New(errors.ErrCodeCmdNotFound, "sops não encontrado").
					WithHint("instale o sops: https://github.com/getsops/sops")
			}
		} else {
			if _, err := lookPath("kubeseal"); err != nil {
				return errors.New(errors.ErrCodeCmdNotFound, "kubeseal não encontrado").
					WithHint(hintKubectl)
			}
		}

		w := cmd.OutOrStdout()
		tituloComando(w, "secret", "seal · "+sealStrategy)

		name, namespace, key, value, err := sealPrompt()
		if err != nil {
			return falhaComHint(err, errors.ErrCodeExec, "falha ao coletar dados do secret",
				"rode yby secret seal num terminal interativo e preencha todos os campos")
		}

		// 1. Criar serviço e gerar Secret (Dry Run)
		runner := &shared.RealRunner{}
		fsys := &shared.RealFilesystem{}
		svc := newSecretsService(runner, fsys)

		linhaEmAndamento(w, "gerando o Secret "+namespace+"/"+name)
		secretYaml, err := svc.GenerateSecretYAML(cmd.Context(), name, namespace, key, value)
		if err != nil {
			return falhaComHint(err, errors.ErrCodeExec, "falha ao gerar secret", "confira o kubectl e o acesso ao cluster com yby doctor")
		}

		root, errRoot := FindInfraRoot()
		if errRoot != nil {
			root = "."
		}

		if sealStrategy == "sops" {
			// 2a. Encriptar com SOPS + age
			filename := fmt.Sprintf("sops-secret-%s.yaml", name)
			targetDir := JoinInfra(root, "charts/cluster-config/templates/secrets")

			finalPath, _ := sealPathPrompt(filepath.Join(targetDir, filename))

			linhaEmAndamento(w, "encriptando com o SOPS")
			if err := svc.EncryptWithSOPS(cmd.Context(), "", secretYaml, finalPath); err != nil {
				return falhaComHint(err, errors.ErrCodeExec, "falha ao encriptar secret com SOPS",
					"confira o .sops.yaml do projeto e a chave age; para criar a chave rode yby secret init-sops")
			}

			checksAlinhados(w, linhaCheck{ui.CheckOk, "Secret SOPS", "salvo", caminhoExibido(finalPath)})
			ui.Section(w, "próximos passos")
			ui.Passos(w,
				"faça commit e push para o GitOps aplicar",
				"para aplicar à mão: sops --decrypt "+caminhoExibido(finalPath)+" | kubectl apply -f -",
			)
		} else {
			// 2b. Selar com Kubeseal
			filename := fmt.Sprintf("sealed-secret-%s.yaml", name)
			targetDir := JoinInfra(root, "charts/cluster-config/templates/events")

			finalPath, _ := sealPathPrompt(filepath.Join(targetDir, filename))

			linhaEmAndamento(w, "selando com o kubeseal")
			if err := svc.SealWithKubeseal(cmd.Context(), secretYaml, finalPath); err != nil {
				return falhaComHint(err, errors.ErrCodeExec, "falha ao selar secret", hintSealedSecrets)
			}

			checksAlinhados(w, linhaCheck{ui.CheckOk, "SealedSecret", "salvo", caminhoExibido(finalPath)})
			ui.Section(w, "próximo passo")
			ui.Passos(w, "faça commit e push para o GitOps aplicar")
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
