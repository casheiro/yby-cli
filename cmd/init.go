/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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

					fmt.Println(stepStyle.Render("🔧 Ajustando caminhos dos Workflows..."))
					if err := patchWorkflows(targetDir); err != nil {
						fmt.Printf(warningStyle.Render("⚠️ Falha ao ajustar workflows: %v\n"), err)
					}

					fmt.Println(stepStyle.Render("🔧 Ajustando caminhos do Sensor (Argo Events)..."))
					if err := patchSensor(targetDir); err != nil {
						fmt.Printf(warningStyle.Render("⚠️ Falha ao ajustar sensor: %v\n"), err)
					}

					fmt.Println(stepStyle.Render("🔧 Ajustando Root App path..."))
					if err := patchRootApp(targetDir, ""); err != nil {
						fmt.Printf(warningStyle.Render("⚠️ Falha ao ajustar root-app: %v\n"), err)
					}

					fmt.Println(stepStyle.Render("🔧 Ajustando Root App path..."))
					if err := patchRootApp(targetDir, ""); err != nil {
						fmt.Printf(warningStyle.Render("⚠️ Falha ao ajustar root-app: %v\n"), err)
					}

					// We don't have repoURL yet (it's asked later in prompts).
					// But we should patch the PATH now. RepoURL will be handled by blueprint actions if defined,
					// or we need to defer this patch until after prompts?
					// Wait, scaffold happens BEFORE prompts.
					// We can patch path now. RepoURL is tricky if we don't know it.
					// BUT, usually we want this mostly for the PATH fix in Integration Mode (infra/).
					// Let's patch path now. RepoURL is handled by Blueprint Actions normally, or we rely on 'yby dev' self-repair.
					fmt.Println(stepStyle.Render("🔧 Ajustando Root App path..."))
					if err := patchRootApp(targetDir, ""); err != nil {
						fmt.Printf(warningStyle.Render("⚠️ Falha ao ajustar root-app: %v\n"), err)
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

			// Check 'When' condition
			if p.When.PromptID != "" {
				// We rely on string comparison from envMap
				storedVal, exists := envMap[p.When.PromptID]
				if !exists {
					// Dependency not found (yet?), skip or strictly fail?
					// Skipping is safer for flexible blueprints
					continue
				}
				if storedVal != p.When.Value {
					continue
				}
			}

			// 2.a Dynamic Default Logic (Repo Name)
			if p.ID == "git.repoName" {
				if repoUrl, ok := envMap["git.repoURL"]; ok {
					// repoUrl format: https://github.com/org/repo or git@github.com:org/repo.git
					parts := strings.Split(repoUrl, "/")
					if len(parts) > 0 {
						last := parts[len(parts)-1]
						last = strings.TrimSuffix(last, ".git")
						if last != "" {
							p.Default = last
						}
					}
				}
			}

			// Build Survey Question
			var q survey.Prompt
			switch p.Type {
			case "input":
				input := &survey.Input{Message: p.Label}
				if def, ok := p.Default.(string); ok {
					input.Default = def
				}
				q = input
			case "confirm":
				confirm := &survey.Confirm{Message: p.Label}
				if def, ok := p.Default.(bool); ok {
					confirm.Default = def
				}
				q = confirm
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

			// Prepare concrete types for survey result
			var strResult string
			var boolResult bool
			var sliceResult []string

			// Ask
			var err error
			if p.Type == "multiselect" {
				err = survey.AskOne(q, &sliceResult, survey.WithValidator(func(ans interface{}) error {
					if p.Required {
						return survey.Required(ans)
					}
					return nil
				}))
				answer = sliceResult
			} else if p.Type == "confirm" {
				err = survey.AskOne(q, &boolResult)
				answer = boolResult
			} else {
				// input, select, list -> all return string initially
				err = survey.AskOne(q, &strResult, survey.WithValidator(func(ans interface{}) error {
					if p.Required {
						return survey.Required(ans)
					}
					return nil
				}))
				answer = strResult
			}

			if err != nil {
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

			// 4. Process Actions (Conditional Side Effects)
			// 4. Process Actions (Conditional Side Effects)
			for _, action := range p.Actions {
				match := false

				switch v := answer.(type) {
				case []string:
					// Check if slice contains condition
					for _, item := range v {
						if item == action.Condition {
							match = true
							break
						}
					}
				default:
					// Fallback to string comparison for scalars
					answerStr := fmt.Sprintf("%v", answer)
					match = (answerStr == action.Condition)
				}

				if match {
					fmt.Printf("   ⚡ Ação disparada (Condição: %s)\n", action.Condition)
					if action.Target.Path != "" {
						// Apply the value defined in the action target
						applyPatch(action.Target.File, action.Target.Path, action.Target.Value)
					}
				}
			}
		}

		// 5. Finalize: Ensure RootApp has the correct RepoURL if collected
		// Note: The key must match the prompt ID in blueprint.yaml (gitRepo)
		if repoURL, ok := envMap["gitRepo"]; ok {
			effectiveTargetDir := "."
			if !isEmptyDir(".") {
				if _, err := os.Stat("infra/manifests/argocd/root-app.yaml"); err == nil {
					effectiveTargetDir = "infra"
				}
			}
			if err := patchRootApp(effectiveTargetDir, repoURL); err != nil {
				fmt.Printf(warningStyle.Render("⚠️ Falha ao ajustar root-app (finalização): %v\n"), err)
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
		if err := enc.Encode(&node); err != nil {
			fmt.Printf("⚠️ Erro ao codificar YAML %s: %v\n", file, err)
			return
		}
		if err := os.WriteFile(file, []byte(out.String()), 0644); err != nil {
			fmt.Printf("⚠️ Erro ao salvar arquivo %s: %v\n", file, err)
			return
		}
		fmt.Printf("   ✏️  Atualizado %s: %s = %v\n", file, path, value)
	} else {
		fmt.Printf("   ⚠️ Falha ao encontrar path %s em %s\n", path, file)
	}
}

// updateNode recurses to find the key and update it
// updateNode recurses to find the key and update it, or create if missing (Upsert)
func updateNode(node *yaml.Node, keys []string, value interface{}) bool {
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			// Initialize root if empty
			node.Content = append(node.Content, &yaml.Node{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
			})
		}
		return updateNode(node.Content[0], keys, value)
	}

	if len(keys) == 0 {
		return false
	}

	currentKey := keys[0]

	if node.Kind == yaml.MappingNode {
		// 1. Try to find existing key
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]

			if keyNode.Value == currentKey {
				if len(keys) == 1 {
					// Found target key! Update its value
					setNodeValue(valNode, value)
					return true
				} else {
					// Check if valNode is map (it should be if we are recursing)
					// If it's null (e.g. empty key), initialize it as map
					if valNode.Kind == yaml.ScalarNode && valNode.Tag == "!!null" {
						valNode.Kind = yaml.MappingNode
						valNode.Tag = "!!map"
						valNode.Value = ""
					}
					// Recurse
					return updateNode(valNode, keys[1:], value)
				}
			}
		}

		// 2. Not found - Create it!
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: currentKey,
		}

		var valNode *yaml.Node

		if len(keys) == 1 {
			// Reached target leaf
			valNode = &yaml.Node{}
			setNodeValue(valNode, value)
		} else {
			// Intermediate node - create new Map
			valNode = &yaml.Node{
				Kind:    yaml.MappingNode,
				Tag:     "!!map",
				Content: []*yaml.Node{}, // Explicit init
			}
			// Recurse to populate the child
			updateNode(valNode, keys[1:], value)
		}

		node.Content = append(node.Content, keyNode, valNode)
		return true
	}
	return false
}

func setNodeValue(node *yaml.Node, val interface{}) {
	switch v := val.(type) {
	case string:
		node.Kind = yaml.ScalarNode
		node.Tag = "!!str"
		node.Value = v
	case bool:
		node.Kind = yaml.ScalarNode
		node.Tag = "!!bool"
		if v {
			node.Value = "true"
		} else {
			node.Value = "false"
		}
	case int:
		node.Kind = yaml.ScalarNode
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

// patchWorkflows iterates over .github/workflows and replaces specific paths with targetDir/path
func patchWorkflows(targetDir string) error {
	workflowsDir := ".github/workflows"
	files, err := os.ReadDir(workflowsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Paths to look for and replace
	// We want to be careful not to match substrings incorrectly, but standard yby paths are quite specific in usage.
	// Common refs: "config/cluster-values.yaml", "manifests/", "charts/"
	replacements := map[string]string{
		"config/cluster-values.yaml": filepath.Join(targetDir, "config/cluster-values.yaml"),
		"charts/":                    filepath.Join(targetDir, "charts") + "/", // ensure trailing slash if used as dir
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".yaml") && !strings.HasSuffix(file.Name(), ".yml") {
			continue
		}

		path := filepath.Join(workflowsDir, file.Name())
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		content := string(contentBytes)
		changed := false
		for old, newRef := range replacements {
			if strings.Contains(content, old) {
				content = strings.ReplaceAll(content, old, newRef)
				changed = true
			}
		}

		if changed {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return err
			}
			fmt.Printf("   ✏️  Workflow %s atualizado.\n", file.Name())
		}
	}
	return nil
}

// patchSensor updates the Argo Events Sensor to look for files in the correct subdirectory
func patchSensor(targetDir string) error {
	// Path to sensor.yaml in the extracted structure
	// targetDir/charts/bootstrap/templates/events/sensor.yaml
	sensorPath := filepath.Join(targetDir, "charts", "bootstrap", "templates", "events", "sensor.yaml")

	if _, err := os.Stat(sensorPath); os.IsNotExist(err) {
		// Might not exist if chart structure changed, warn but don't fail hard
		return fmt.Errorf("arquivo sensor.yaml não encontrado em %s", sensorPath)
	}

	contentBytes, err := os.ReadFile(sensorPath)
	if err != nil {
		return err
	}

	content := string(contentBytes)

	// We need to replace "cluster-config/" with "targetDir/charts/cluster-config/"
	// Note: yby-template has "charts/cluster-config" which maps to "targetDir/charts/cluster-config"
	// But the sensor script hardcodes "grep 'cluster-config/'".
	// If we change it to "grep 'infra/charts/cluster-config/'", it relies on the internal structure of the clone.
	// The clone in the sensor script is: git clone ... /tmp/repo.
	// So the files are at /tmp/repo/infra/charts/cluster-config/...

	// Construct the new path relative to repo root
	// If targetDir is "infra", new path is "infra/charts/cluster-config/"
	newPath := filepath.Join(targetDir, "charts", "cluster-config") + "/"

	replacements := map[string]string{
		"cluster-config/": newPath,
	}

	changed := false
	for old, newRef := range replacements {
		if strings.Contains(content, old) {
			content = strings.ReplaceAll(content, old, newRef)
			changed = true
		}
	}

	if changed {
		if err := os.WriteFile(sensorPath, []byte(content), 0644); err != nil {
			return err
		}
		fmt.Printf("   ✏️  Sensor atualizado: cluster-config/ -> %s\n", newPath)
	}
	return nil
}

// patchRootApp updates the Root App manifest with the correct repo path in Integration Mode
func patchRootApp(targetDir string, repoURL string) error {
	rootAppPath := filepath.Join(targetDir, "manifests", "argocd", "root-app.yaml")

	if _, err := os.Stat(rootAppPath); os.IsNotExist(err) {
		return fmt.Errorf("arquivo root-app.yaml não encontrado em %s", rootAppPath)
	}

	contentBytes, err := os.ReadFile(rootAppPath)
	if err != nil {
		return err
	}

	content := string(contentBytes)
	changed := false

	// Path Prefix Patching
	// Usually points to 'charts/bootstrap'
	// In Integration Mode, we might need 'infra/charts/bootstrap' or similar,
	// but strictly 'targetDir' is relative to where we run yby.
	// If User runs from Root -> targetDir="infra" -> path should be "infra/charts/bootstrap"
	// The repo path in ArgoCD must be relative to Git Root.
	// Assuming yby init is run from Git Root:
	newPath := filepath.Join(targetDir, "charts", "bootstrap")
	// If targetDir is "." then newPath is "charts/bootstrap" (unchanged).

	// The default template has "path: charts/bootstrap"
	if targetDir != "." && targetDir != "" {
		if strings.Contains(content, "path: charts/bootstrap") {
			content = strings.ReplaceAll(content, "path: charts/bootstrap", "path: "+newPath)
			changed = true
		}
	}

	// RepoURL Patching
	// Default template has "repoURL: https://github.com/my-user/yby-template"
	// We want to replace it with the provided repoURL
	if repoURL != "" {
		// Try to match the exact placeholder if possible, or naive replace if we trust the context
		placeholder := "https://github.com/my-user/yby-template"
		if strings.Contains(content, placeholder) {
			content = strings.ReplaceAll(content, placeholder, repoURL)
			changed = true
		}
	}

	if changed {
		if err := os.WriteFile(rootAppPath, []byte(content), 0644); err != nil {
			return err
		}
		fmt.Printf("   ✏️  Root App atualizado: path -> %s, repo -> %s\n", newPath, repoURL)
	}
	return nil
}
