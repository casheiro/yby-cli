package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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

		// Create Scaffold Context
		// We reuse BlueprintContext logic but focused on this chart
		// We need ProjectName, Domain, GithubOrg to fill defaults.
		// How do we get them? We can look at existing project context or use flags.
		// For simplicity, we assume we are in a Yby project.

		// TODO: Load context from .yby/blueprint.yaml or init vars?
		// For now, simple placeholders.
		ctx := &scaffold.BlueprintContext{
			ProjectName: name,
			Domain:      "yby.local", // Placeholder
			GithubOrg:   "org",       // Placeholder
		}

		// Use internal template "charts/app-template"
		// We can't use scaffold.Apply easily because it applies explicit logic.
		// We'll write a simple "CopyTemplate" helper using templates.Assets?
		// Or assume scaffold package has "CopyDir" from Embed?

		// Implementation: Walk embedded "charts/app-template" and copy/render to "./charts/<name>"

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
