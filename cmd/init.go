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
		fmt.Println("🌱 Yby Smart Init (Blueprint Engine)")
		fmt.Println("------------------------------------")

		// 1. Load Blueprint
		blueprintPath := ".yby/blueprint.yaml"
		if _, err := os.Stat(blueprintPath); os.IsNotExist(err) {
			fmt.Printf("❌ Blueprint não encontrado em %s\n", blueprintPath)
			fmt.Println("   Certifique-se de estar na raiz do repo yby-template.")
			return
		}

		data, err := os.ReadFile(blueprintPath)
		if err != nil {
			panic(err)
		}

		var blueprint Blueprint
		if err := yaml.Unmarshal(data, &blueprint); err != nil {
			fmt.Printf("❌ Erro ao ler Blueprint: %v\n", err)
			return
		}

		// 2. Process Prompts
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
				fmt.Println("Operação cancelada.")
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

			// 4. Process Actions (Side effects mainly for MultiSelect)
			// 4. Process Actions (Side effects mainly for MultiSelect)
			if p.Type == "multiselect" {
				// TODO: Implement specific logic for multiselect side-effects if needed in the future
			}
		}

		// ... (rest of function) ...
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
		enc.Encode(&node)
		os.WriteFile(file, []byte(out.String()), 0644)
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
