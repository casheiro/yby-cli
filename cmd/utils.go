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
	if node.Kind == yaml.DocumentNode {
		for _, c := range node.Content {
			updateFilePaths(c, prefix)
		}
		return
	}

	if node.Kind == yaml.MappingNode {
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
	} else if node.Kind == yaml.SequenceNode {
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
	// If targetDir is ".", mapping is 1:1 for everything relevant
	// If targetDir is "infra", we map specific folders to "infra/"

	mapping := make(map[string]string)

	// Core Config (Always Root)
	mapping[".yby/"] = ".yby/"

	// Infrastructure Components
	// "charts/" -> "targetDir/charts/"
	infraComponents := []string{"charts/", "config/", "manifests/", "local/", ".synapstor/", ".agent/", ".trae/"}

	destPrefix := ""
	if targetDir != "." && targetDir != "" {
		destPrefix = targetDir + "/"
	}

	for _, c := range infraComponents {
		// Map source "c" to "destPrefix + c"
		mapping[c] = destPrefix + c
	}

	return extractZip(data, mapping)
}
