/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Blueprint structures
type Blueprint struct {
	Prompts []Prompt `yaml:"prompts"`
}

type Prompt struct {
	ID       string   `yaml:"id"`
	Type     string   `yaml:"type"` // input, select, multiselect, list
	Label    string   `yaml:"label"`
	Default  any      `yaml:"default"`
	Options  []string `yaml:"options"`
	Required bool     `yaml:"required"`
	Target   Target   `yaml:"target"`
	Actions  []Action `yaml:"actions"`
	When     When     `yaml:"when"`
}

type When struct {
	PromptID string `yaml:"promptId"`
	Value    string `yaml:"value"`
}

type Target struct {
	File  string `yaml:"file"`
	Path  string `yaml:"path"`
	Value any    `yaml:"value"`
}

type Action struct {
	Condition string `yaml:"condition"`
	Target    Target `yaml:"target"`
}

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inicializa o projeto seguindo o Blueprint do template",
	Long: `Lê o arquivo .yby/blueprint.yaml e guia o usuário na configuração.
Edita o arquivo config/cluster-values.yaml existente preservando comentários.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(headerStyle.Render("🌱 Yby Smart Init (Blueprint Engine)"))

		blueprintPath := ".yby/blueprint.yaml"
		var blueprint Blueprint

		// 1. Check Environment State
		if _, err := os.Stat(blueprintPath); err == nil {
			// Scenario A: Blueprint exists - Load and Validate
			data, err := os.ReadFile(blueprintPath)
			if err != nil {
				fmt.Printf(crossStyle.Render("❌ Erro ao ler Blueprint: %v\n"), err)
				return
			}
			if err := yaml.Unmarshal(data, &blueprint); err != nil {
				fmt.Printf(crossStyle.Render("❌ Blueprint inválido: %v\n"), err)
				return
			}

			// Validate Targets
			if err := validateBlueprintTargets(blueprint); err != nil {
				fmt.Println(warningStyle.Render(fmt.Sprintf("⚠️  Blueprint encontrado, mas arquivos de configuração estão faltando:\n%v", err)))

				repair := false
				prompt := &survey.Confirm{
					Message: "Deseja reparar o projeto baixando os arquivos faltantes do template?",
					Default: true,
				}
				_ = survey.AskOne(prompt, &repair)

				if repair {
					// Repair Logic: Ask for target directory
					targetDir := "."
					prompt := &survey.Input{
						Message: "Para reparar, informe o diretório onde a infraestrutura deve estar (ex: infra ou .):",
						Default: "infra",
					}
					_ = survey.AskOne(prompt, &targetDir)
					if err := scaffoldFromZip(targetDir); err != nil {
						fmt.Printf(crossStyle.Render("❌ Erro ao reparar template: %v\n"), err)
						return
					}
				} else {
					fmt.Println(warningStyle.Render("Continuando com inicialização parcial. Isso pode causar erros..."))
				}
			} else {
				fmt.Println(checkStyle.Render("ℹ️  Projeto existente validado. Configuração íntegra."))
			}

		} else {
			// Scenario B: Blueprint missing
			shouldClone := false

			if isEmptyDir(".") {
				// Scenario B1: Empty Directory
				prompt := &survey.Confirm{
					Message: "Diretório vazio. Deseja inicializar um novo projeto a partir do template?",
					Default: true,
				}
				_ = survey.AskOne(prompt, &shouldClone)
			} else {
				// Scenario B2: Dirty Directory (Integration Mode)
				fmt.Println(stepStyle.Render("🌱 Projeto existente detectado."))
				prompt := &survey.Confirm{
					Message: "Deseja integrar a infraestrutura do Yby neste projeto?",
					Default: true,
				}
				_ = survey.AskOne(prompt, &shouldClone)
			}

			if shouldClone {
				// Initialize Safe Scaffold
				targetDir := "."

				// If strictly in integration mode (dirty dir), ask for target, defaulting to 'infra'
				if !isEmptyDir(".") {
					prompt := &survey.Input{
						Message: "Diretório de destino para a infraestrutura Yby:",
						Default: "infra",
						Help:    "Os arquivos de infraestrutura (charts, config, manifests) serão instalados aqui para não poluir a raiz.",
					}
					_ = survey.AskOne(prompt, &targetDir)
				}

				if err := scaffoldFromZip(targetDir); err != nil {
					fmt.Printf(crossStyle.Render("❌ Erro ao inicializar scaffold: %v\n"), err)
					return
				}

				// Patch blueprint only if we moved things to a subfolder
				if targetDir != "." && targetDir != "" {
					fmt.Println(stepStyle.Render("🔧 Ajustando caminhos do Blueprint..."))
					if err := patchBlueprint(blueprintPath, targetDir); err != nil {
						fmt.Printf(warningStyle.Render("⚠️ Falha ao ajustar blueprint: %v\n"), err)
					}
				}

				// Refresh blueprint
				data, err := os.ReadFile(blueprintPath)
				if err != nil {
					fmt.Printf(crossStyle.Render("❌ Erro ao ler Blueprint após download: %v\n"), err)
					return
				}
				if err := yaml.Unmarshal(data, &blueprint); err != nil {
					fmt.Printf(crossStyle.Render("❌ Blueprint baixado inválido: %v\n"), err)
					return
				}
			} else {
				fmt.Println(crossStyle.Render("❌ Blueprint obrigatório para a inicialização. Abortando."))
			}
		}

		fmt.Println("------------------------------------")
		// 3. Process Prompts
		values := make(map[string]interface{})

		// Map for env file generation (simple key-value store of answers)
		envMap := make(map[string]string)

		for _, p := range blueprint.Prompts {
			var answer interface{}

			// Build Survey Question
			var q survey.Prompt
			switch p.Type {
			case "input":
				input := &survey.Input{Message: p.Label}
				if def, ok := p.Default.(string); ok {
					input.Default = def
				}
				q = input
			case "select":
				sel := &survey.Select{Message: p.Label, Options: p.Options}
				if def, ok := p.Default.(string); ok {
					sel.Default = def
				}
				q = sel
			case "multiselect":
				ms := &survey.MultiSelect{Message: p.Label, Options: p.Options}
				// Default handling for multiselect is tricky with interface{}, blindly assuming []interface{} or []string
				// Simplify for now
				q = ms
			case "list":
				input := &survey.Input{Message: p.Label + " (separado por vírgula)"}
				// Default handling for list is intentionally skipped for now unless we implement proper casting
				q = input
			}

			// Ask
			if err := survey.AskOne(q, &answer, survey.WithValidator(func(ans interface{}) error {
				if p.Required {
					return survey.Required(ans)
				}
				return nil
			})); err != nil {
				fmt.Printf("Operação cancelada (Erro: %v).\n", err)
				return
			}

			// Post-process answer for 'list' type
			if p.Type == "list" {
				if strAns, ok := answer.(string); ok {
					if strings.TrimSpace(strAns) == "" {
						values[p.ID] = []string{}
						answer = []string{}
					} else {
						parts := strings.Split(strAns, ",")
						var finalParts []string
						for _, part := range parts {
							trimmed := strings.TrimSpace(part)
							if trimmed != "" {
								finalParts = append(finalParts, trimmed)
							}
						}
						values[p.ID] = finalParts
						answer = finalParts
					}
				}
			} else {
				values[p.ID] = answer
			}

			// Store in env map for later (string representation)
			envMap[p.ID] = fmt.Sprintf("%v", answer)

			// 3. Apply to YAML immediately (or simulate transaction)
			if p.Target.Path != "" {
				applyPatch(p.Target.File, p.Target.Path, answer)
			}
		}
	},
}

// applyPatch reads a YAML file, navigates to path, updates value, and saves.
func applyPatch(file, path string, value interface{}) {
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Printf("⚠️ Arquivo %s não encontrado para patch.\n", file)
		return
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		fmt.Printf("⚠️ Erro parsing YAML %s: %v\n", file, err)
		return
	}

	// Simple dot notation parser: .global.domainBase -> ["global", "domainBase"]
	keys := strings.Split(strings.TrimPrefix(path, "."), ".")

	if updateNode(&node, keys, value) {
		// Save back
		// Use 2 spaces indent
		var out strings.Builder
		enc := yaml.NewEncoder(&out)
		enc.SetIndent(2)
		_ = enc.Encode(&node)
		_ = os.WriteFile(file, []byte(out.String()), 0644)
		fmt.Printf("   ✏️  Atualizado %s: %s = %v\n", file, path, value)
	} else {
		fmt.Printf("   ⚠️ Falha ao encontrar path %s em %s\n", path, file)
	}
}

// updateNode recurses to find the key and update it
func updateNode(node *yaml.Node, keys []string, value interface{}) bool {
	if node.Kind == yaml.DocumentNode {
		return updateNode(node.Content[0], keys, value)
	}

	if len(keys) == 0 {
		return false // Should not happen if path is valid
	}

	currentKey := keys[0]

	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]

			if keyNode.Value == currentKey {
				if len(keys) == 1 {
					// Found target! Update valNode
					// We need to set valNode's value/tag/kind based on Go interface value
					setNodeValue(valNode, value)
					return true
				} else {
					// Recurse
					return updateNode(valNode, keys[1:], value)
				}
			}
		}
	}
	return false
}

func setNodeValue(node *yaml.Node, val interface{}) {
	switch v := val.(type) {
	case string:
		node.Tag = "!!str"
		node.Value = v
	case bool:
		node.Tag = "!!bool"
		if v {
			node.Value = "true"
		} else {
			node.Value = "false"
		}
	case int:
		node.Tag = "!!int"
		node.Value = fmt.Sprintf("%d", v)
	case []string:
		node.Kind = yaml.SequenceNode
		node.Tag = "!!seq"
		node.Content = []*yaml.Node{}
		for _, item := range v {
			node.Content = append(node.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: item,
			})
		}
	}
}

func isEmptyDir(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	return err != nil
}

func validateBlueprintTargets(bp Blueprint) error {
	var missingFiles []string

	for _, p := range bp.Prompts {
		if p.Target.File != "" {
			if _, err := os.Stat(p.Target.File); os.IsNotExist(err) {
				// Avoid duplicates
				found := false
				for _, f := range missingFiles {
					if f == p.Target.File {
						found = true
						break
					}
				}
				if !found {
					missingFiles = append(missingFiles, p.Target.File)
				}
			}
		}
	}

	if len(missingFiles) > 0 {
		return fmt.Errorf("arquivos não encontrados: %s", strings.Join(missingFiles, ", "))
	}
	return nil
}
