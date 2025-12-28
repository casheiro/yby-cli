package scaffold

import (
	"strings"
)

// shouldSkip determines if a file path should be ignored based on the context configuration.
func shouldSkip(path string, ctx *BlueprintContext) bool {
	// 1. CI/CD Filters
	if !ctx.EnableCI {
		if strings.Contains(path, ".github/workflows") {
			return true
		}
	} else {
		// If CI is enabled, filtering depends on the selected Workflow Pattern.
		// Path comes in as "assets/.github/workflows/gitflow/foo.yaml"
		// We need to match the pattern directory.

		// Normalize checking:
		// If pattern is "gitflow", we should ONLY include "assets/.github/workflows/gitflow/*" case.
		// AND we should EXCLUDE other patterns like "assets/.github/workflows/essential/*"

		isWorkflowFile := strings.Contains(path, ".github/workflows")
		if isWorkflowFile {
			// Special case: Don't skip the root 'workflows' directory itself,
			// otherwise we never reach the subdirectories.
			if strings.HasSuffix(path, ".github/workflows") {
				return false
			}

			// Check which pattern folder is in the path
			if ctx.WorkflowPattern != "" {
				targetPatternDir := "/" + ctx.WorkflowPattern
				// We expect path to contain /gitflow or /gitflow/
				if !strings.Contains(path, targetPatternDir) {
					return true // Skip if it doesn't match the selected pattern
				}
			} else {
				// Default behavior if no pattern selected? maybe skip all or default to gitflow?
				// Allowing "gitflow" as default if empty is risky, better strict.
				// However, if we have common workflows at root of workflows/, we keep them.
				// But we moved everything to subfolders.
				return true
			}
		}
	}

	// 2. DevContainer Filter
	if !ctx.EnableDevContainer {
		if strings.Contains(path, ".devcontainer") {
			return true
		}
	}

	// 3. Module Filters (Kepler, MinIO, KEDA)
	// Assuming these are organized in charts/ or manifests/
	// Example: assets/charts/kepler
	if !ctx.EnableKepler {
		if strings.Contains(path, "charts/kepler") {
			return true
		}
	}
	if !ctx.EnableMinio {
		if strings.Contains(path, "charts/minio") || strings.Contains(path, "config/minio") {
			return true
		}
	}
	if !ctx.EnableKEDA {
		if strings.Contains(path, "charts/keda") {
			return true
		}
	}

	// 4. Discovery Filter
	if !ctx.EnableDiscovery {
		if strings.Contains(path, "discovery") || strings.Contains(path, "crossplane") {
			return true
		}
	}

	return false
}
