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
	Type     string   `yaml:"type"` // input, select, multiselect
	Label    string   `yaml:"label"`
	Default  any      `yaml:"default"`
	Options  []string `yaml:"options"`
	Required bool     `yaml:"required"`
	Target   Target   `yaml:"target"`
	Actions  []Action `yaml:"actions"`
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
			values[p.ID] = answer

			// Store in env map for later (string representation)
			envMap[p.ID] = fmt.Sprintf("%v", answer)

			// 3. Apply to YAML immediately (or simulate transaction)
			if p.Target.Path != "" {
				applyPatch(p.Target.File, p.Target.Path, answer)
			}

			// 4. Process Actions (Side effects mainly for MultiSelect)
			if p.Type == "multiselect" {
				// answer is []string (survey core type) -> but reflected as []interface{} sometimes?
				// Survey AskOne unmarshals into answer which we passed as interface{}.
				// Actually survey puts it into the pointer type.
				// Let's cast properly.

				// Assert specific types based on known survey returns
				var selected []string
				// Try assert
				if s, ok := answer.(survey.OptionAnswer); ok {
					selected = []string{s.Value}
				} else if s, ok := answer.([]string); ok { // MultiSelect returns []string
					selected = s
				} else if s, ok := answer.(string); ok { // Input/Select returns string
					selected = []string{s}
				}

				for _, act := range p.Actions {
					// Check if condition is in selected
					match := false
					for _, s := range selected {
						if s == act.Condition {
							match = true
							break
						}
					}
					if match {
						applyPatch(act.Target.File, act.Target.Path, act.Target.Value)
					}
				}
			}
		}

		// 5. Generate .env Context (Hardcoded logic for legacy context support, but fueled by blueprint answers)
		// We expect the blueprint to have asked for 'environment' and 'gitRepo' etc.
		// If explicit keys exist in envMap, use them.
		envName := "dev"
		if v, ok := envMap["environment"]; ok {
			envName = v
		}

		envFileName := fmt.Sprintf(".env.%s", envName)
		fmt.Printf("\n🔒 Gerando Contexto Local (%s)...\n", envFileName)

		// Helper to safely get map val
		getVal := func(k string) string {
			if v, ok := envMap[k]; ok {
				return v
			}
			return ""
		}

		// Only write logic if we have minimal data? Or just write what we have?
		// We need GITHUB_TOKEN prompt to be useful.
		// If Blueprint asked for 'GithubToken' (capitalized?), we'd find it if we mapped IDs well.
		// But in this proof of concept, the Blueprint prompts 'gitRepo', 'domain', etc.
		// We are missing a prompt for SECRET (token) in the blueprint?
		// User's blueprint example didn't have token. Let's assume user adds it or we add imperative logic for Secrets.

		// IMPERATIVE FALLBACK for Secrets (Token) since Blueprint might not manage secrets securely (YAML patch isn't for secrets)
		// But .env creation needs the token.
		var token string
		prompt := &survey.Password{Message: "GitHub Token (PAT) para CI/CD:"}
		survey.AskOne(prompt, &token)

		envContent := fmt.Sprintf("# Contexto: %s\nGITHUB_TOKEN=%s\nCLUSTER_DOMAIN=%s\nGIT_REPO=%s\n",
			envName, token, getVal("domain"), getVal("gitRepo"))

		os.WriteFile(envFileName, []byte(envContent), 0600)

		// Update .gitignore
		f, _ := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		defer f.Close()
		f.WriteString(fmt.Sprintf("\n%s\n", envFileName))

		fmt.Println("✅ Init concluído via Blueprint!")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
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
		// Add other types as needed
	}
}
