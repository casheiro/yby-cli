package cmd

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"gopkg.in/yaml.v3"
)

// downloadZip downloads a zip file from a URL and returns the data as a byte slice.
func downloadZip(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// extractZip extracts specific files from a zip archive based on the provided mapping.
// mapping key: path prefix in zip (e.g. "yby-template-main/infra/")
// mapping value: destination path (e.g. "infra/")
func extractZip(zipData []byte, mapping map[string]string) error {
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return err
	}

	// Determine the root folder inside the zip (e.g., "yby-template-main/")
	var rootPrefix string
	if len(zipReader.File) > 0 {
		parts := strings.Split(zipReader.File[0].Name, "/")
		if len(parts) > 0 {
			rootPrefix = parts[0] + "/"
		}
	}

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// Normalize path: strip the root prefix of the zip
		relPath := strings.TrimPrefix(f.Name, rootPrefix)

		var targetPath string
		var matched bool

		// Check against mapping
		for srcPrefix, dstPrefix := range mapping {
			if strings.HasPrefix(relPath, srcPrefix) {
				// Matched! Construct target path
				// Remove the srcPrefix from relPath and append remainder to dstPrefix
				targetPath = filepath.Join(dstPrefix, strings.TrimPrefix(relPath, srcPrefix))
				matched = true
				break
			}
		}

		if matched {
			// Extract
			if err := extractFile(f, targetPath); err != nil {
				return err
			}
			fmt.Printf("   📄 Extraído: %s\n", targetPath)
		}
	}

	return nil
}

func extractFile(f *zip.File, destPath string) error {
	// Check if file exists
	if _, err := os.Stat(destPath); err == nil {
		// File exists, ask for confirmation
		overwrite := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Arquivo %s já existe. Deseja sobrescrever?", destPath),
			Default: false,
		}
		if err := survey.AskOne(prompt, &overwrite); err != nil {
			return err
		}

		if !overwrite {
			fmt.Printf("   ⏩ Pulado: %s\n", destPath)
			return nil
		}
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, rc)
	return err
}

// patchBlueprint updates file paths in the blueprint to reflect the integration target directory
func patchBlueprint(blueprintFile string, targetDir string) error {
	if targetDir == "." || targetDir == "" {
		return nil
	}

	data, err := os.ReadFile(blueprintFile)
	if err != nil {
		return err
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return err
	}

	updateFilePaths(&node, targetDir)

	// Save back
	var out strings.Builder
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	_ = enc.Encode(&node)

	return os.WriteFile(blueprintFile, []byte(out.String()), 0644)
}

func updateFilePaths(node *yaml.Node, prefix string) {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, c := range node.Content {
			updateFilePaths(c, prefix)
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]

			if keyNode.Value == "target" && valNode.Kind == yaml.MappingNode {
				// Found a target object, look for "file" inside it
				for j := 0; j < len(valNode.Content); j += 2 {
					tKey := valNode.Content[j]
					tVal := valNode.Content[j+1]
					if tKey.Value == "file" {
						// Prepend prefix if not already absolute (simple heuristic)
						if !strings.HasPrefix(tVal.Value, "/") {
							tVal.Value = filepath.Join(prefix, tVal.Value)
						}
					}
				}
			} else {
				// Recurse
				updateFilePaths(valNode, prefix)
			}
		}
	case yaml.SequenceNode:
		for _, c := range node.Content {
			updateFilePaths(c, prefix)
		}
	}
}

func scaffoldFromZip(targetDir string) error {
	fmt.Println(stepStyle.Render("📥 Baixando template oficial (ZIP)..."))

	// TODO: Make this configurable or versioned in future
	zipURL := "https://github.com/casheiro/yby-template/archive/refs/heads/main.zip"

	data, err := downloadZip(zipURL)
	if err != nil {
		return fmt.Errorf("falha no download: %w", err)
	}

	fmt.Println(stepStyle.Render("📦 Extraindo arquivos..."))

	// Define mapping
	// mapping[source_prefix] = dest_prefix
	mapping := make(map[string]string)

	destPrefix := ""
	if targetDir != "." && targetDir != "" {
		destPrefix = targetDir + "/"
	}

	// 1. Core Config
	// Always mapped to destination (root or infra/)
	// If targetDir=infra, source ".yby/" -> "infra/.yby/"
	mapping[".yby/"] = destPrefix + ".yby/"

	// Workflows -> Always root (GitHub requirement)
	mapping[".github/workflows/"] = ".github/workflows/"

	// 2. Infrastructure Components
	// Maps to destPrefix
	infraComponents := []string{
		"charts/",
		"config/",
		"manifests/",
		"local/",
	}

	for _, c := range infraComponents {
		mapping[c] = destPrefix + c
	}

	// 3. Governance Files (Filtered)
	// Only download if explicitly required or if we decide to include minimal set.
	// User requested minimal, so we exclude .synapstor, .agent, .trae for now unless specifically asked.
	// For now, let's include ONLY essential governance if they are critical for 'yby doctor' or similar,
	// but user explicitly complained about them.
	// Resolution: Exclude them from default scaffold. Users can run 'yby governance init' later if needed (feature idea).

	// For now, simply DO NOT add them to the mapping.
	// The extractZip function ONLY extracts what matches the mapping keys.

	return extractZip(data, mapping)
}

// FindInfraRoot discovers the root directory containing the .yby configuration folder.
// It searches in the following order:
// 1. Current directory (checks for .yby/)
// 2. "infra" subdirectory (checks for infra/.yby/)
// 3. First-level subdirectories (checks for */.yby/)
func FindInfraRoot() (string, error) {
	// 1. Check Standard Infra (Priority over root to avoid stale .yby issues)
	if _, err := os.Stat("infra/.yby"); err == nil {
		return "infra", nil
	}

	// 2. Check Root
	if _, err := os.Stat(".yby"); err == nil {
		return ".", nil
	}

	// 3. Scan first-level subdirs
	entries, err := os.ReadDir(".")
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && e.Name() != ".git" && e.Name() != ".yby" && e.Name() != "infra" {
				candidate := filepath.Join(e.Name(), ".yby")
				if _, err := os.Stat(candidate); err == nil {
					return e.Name(), nil
				}
			}
		}
	}

	return "", fmt.Errorf("diretório de infraestrutura (.yby/) não encontrado")
}

// Helper to join paths with infra root
func JoinInfra(root, path string) string {
	if root == "." {
		return path
	}
	return filepath.Join(root, path)
}
