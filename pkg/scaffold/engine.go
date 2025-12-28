package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/casheiro/yby-cli/pkg/templates"
)

// Apply executes the scaffold process based on the provided context.
func Apply(targetDir string, ctx *BlueprintContext) error {
	// 1. Walk through embedded assets
	err := fs.WalkDir(templates.Assets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root "assets" directory itself
		if path == "assets" {
			return nil
		}

		// 2. Filter Logic
		if shouldSkip(path, ctx) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// 3. Resolve Target Path
		relPath, err := filepath.Rel("assets", path)
		if err != nil {
			return err
		}

		// Adjust for Workflow Patterns: Flatten the structure
		// assets/.github/workflows/gitflow/foo.yaml -> .github/workflows/foo.yaml
		if strings.Contains(relPath, ".github/workflows/") {
			parts := strings.Split(relPath, string(filepath.Separator))
			// expected: [.github, workflows, gitflow, foo.yaml]
			// we want: [.github, workflows, foo.yaml]
			if len(parts) >= 4 {
				// Remove the pattern directory (index 2)
				newParts := append(parts[:2], parts[3:]...)
				relPath = filepath.Join(newParts...)
			}
		}

		finalPath := filepath.Join(targetDir, relPath)

		// 4. Handle Directory Creation
		if d.IsDir() {
			return os.MkdirAll(finalPath, 0755)
		}

		// 5. Render or Copy File
		return processFile(path, finalPath, ctx)
	})

	if err != nil {
		return fmt.Errorf("scaffold failed: %w", err)
	}

	return nil
}

func processFile(srcPath, destPath string, ctx *BlueprintContext) error {
	// Ensure parent dir exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Read source content from embed.FS
	content, err := fs.ReadFile(templates.Assets, srcPath)
	if err != nil {
		return err
	}

	// Check if it's a template
	if strings.HasSuffix(srcPath, ".tmpl") {
		// Render Template
		destPath = strings.TrimSuffix(destPath, ".tmpl") // Remove .tmpl extension from target

		tmpl, err := template.New(filepath.Base(srcPath)).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", srcPath, err)
		}

		f, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := tmpl.Execute(f, ctx); err != nil {
			return fmt.Errorf("failed to execute template %s: %w", srcPath, err)
		}

		fmt.Printf("   📄 Rendered: %s\n", destPath)
	} else {
		// Regular Copy
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return err
		}
		fmt.Printf("   📄 Copied: %s\n", destPath)
	}

	return nil
}
