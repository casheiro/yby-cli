package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/scaffold"
	"github.com/casheiro/yby-cli/pkg/templates"
	"github.com/spf13/cobra"
)

// chartCreateCmd represents the create command
var chartCreateCmd = &cobra.Command{
	Use:   "create [NAME]",
	Short: "Cria um novo Helm Chart otimizado para a stack Yby",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		targetDir := "."
		if len(args) > 1 {
			targetDir = args[1] // generic support
		}

		fmt.Printf("📦 Criando chart '%s'...\n", name)

		// Tenta carregar contexto do ProjectManifest; usa defaults se não encontrar
		domain := "yby.local"
		githubOrg := "org"
		if manifest, err := scaffold.LoadProjectManifest("."); err == nil {
			if manifest.Spec.Domain != "" {
				domain = manifest.Spec.Domain
			}
			if manifest.Spec.Git.Repo != "" {
				// Extrai organização do URL do repositório (ex: "https://github.com/org/repo" → "org")
				parts := strings.Split(strings.TrimSuffix(manifest.Spec.Git.Repo, ".git"), "/")
				if len(parts) >= 2 {
					githubOrg = parts[len(parts)-2]
				}
			}
		}

		ctx := &scaffold.BlueprintContext{
			ProjectName: name,
			Domain:      domain,
			GithubOrg:   githubOrg,
		}

		dest := filepath.Join(targetDir, "charts", name)
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			return errors.New(errors.ErrCodeValidation, fmt.Sprintf("❌ Diretório %s já existe", dest))
		}

		// Ensure charts dir
		_ = os.MkdirAll(filepath.Join(targetDir, "charts"), 0755)

		// Manual Walk and Render
		err := scaffold.RenderEmbedDir(templates.Assets, "assets/charts/app-template", dest, ctx)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeScaffold, "Erro ao criar chart")
		}

		fmt.Printf("✅ Chart '%s' criado em ./charts/%s\n", name, name)
		return nil
	},
}

func init() {
	chartCmd.AddCommand(chartCreateCmd)
}
