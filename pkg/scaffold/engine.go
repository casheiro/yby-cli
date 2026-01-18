package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// Apply executes the scaffold process based on the provided context and source filesystem.
func Apply(targetDir string, ctx *BlueprintContext, sourceFS fs.FS) error {
	// 1. Walk through assets in the provided filesystem
	// Note: We assume the sourceFS root IS the assets root or contains "assets" folder?
	// The CompositeFS will contain layers.
	// We need to decide if we walk "." or "assets".
	// Engine usually expected "assets" prefix in embed.FS.
	// Let's assume sourceFS contains "assets" directory at root if it replaces templates.Assets.
	err := fs.WalkDir(sourceFS, "assets", func(path string, d fs.DirEntry, err error) error {
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

		// 4. Handle Root-Level Assets (.github, .devcontainer, .synapstor)
		// These should always go to the Project Root (Git Root or Parent), not the targetDir (if it is a subdir)
		finalPath := filepath.Join(targetDir, relPath)
		isRootAsset := strings.HasPrefix(relPath, ".github") ||
			strings.HasPrefix(relPath, ".devcontainer")

		if isRootAsset {
			// Try to find Git Root
			gitRoot, err := GetGitRoot()

			// Logic: Use GitRoot if valid.
			// If GetGitRoot fails (git not found, or not a repo), we fallback.
			if err == nil && gitRoot != "" {
				// Repoint to Git Root
				finalPath = filepath.Join(gitRoot, relPath)
			} else {
				// Fallback:
				// If error is "git binary not found", we assume CWD is the root we want.
				// If targetDir is explicitly "infra" or similar, we might still want CWD as root for .github
				// logic: if targetDir != "." and targetDir != "", try to use CWD
				if targetDir != "." && targetDir != "" {
					wd, _ := os.Getwd()
					finalPath = filepath.Join(wd, relPath)
					// Log warning only once? Or just be silent in non-verbose?
					// fmt.Printf("⚠️  Git root not found (using CWD): %s\n", relPath)
				}
			}
		}

		// 4.5 Skip creation of flattened source directories
		// If we are flattening patterns (e.g. .github/workflows/gitflow -> .github/workflows),
		// we should NOT create the "gitflow" directory itself in the destination.
		// The file copy prevents "gitflow" from being part of the path, but this Dir check
		// prevents the empty folder from being created.
		if d.IsDir() {
			// If this directory name matches the active Workflow Pattern, skip it
			// because checks in step 3 hide it from the file paths
			if ctx.WorkflowPattern != "" && d.Name() == ctx.WorkflowPattern {
				// Double check parent
				if strings.Contains(filepath.ToSlash(path), ".github/workflows/"+ctx.WorkflowPattern) {
					return nil // Skip creating the empty pattern directory
				}
			}
		}

		// 5. Handle Directory Creation
		if d.IsDir() {
			return os.MkdirAll(finalPath, 0755)
		}

		// 6. Render or Copy File
		return processFile(sourceFS, path, finalPath, ctx)
	})

	if err != nil {
		return fmt.Errorf("scaffold falhou: %w", err)
	}

	return nil
}

func GetGitRoot() (string, error) {
	// Check if git is installed
	_, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("binário git não encontrado")
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func processFile(fsys fs.FS, srcPath, destPath string, ctx *BlueprintContext) error {
	// Ensure parent dir exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Read source content from provided FS
	content, err := fs.ReadFile(fsys, srcPath)
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
