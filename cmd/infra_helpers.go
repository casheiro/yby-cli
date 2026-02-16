package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindInfraRoot searches for the .yby directory upwards or in infra/ subdir
func FindInfraRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	startWd := wd
	for {
		// 1. Check current dir for .yby
		if _, err := os.Stat(filepath.Join(wd, ".yby")); err == nil {
			return wd, nil
		}

		// 2. Check current dir for infra/.yby (Monorepo Root Case)
		// Only if we are traversing upwards? Or specifically if we are at root.
		// Let's check it at every step to be safe, or just initially?
		// Better: If we are at a potential root, look for infra/.yby
		if _, err := os.Stat(filepath.Join(wd, "infra", ".yby")); err == nil {
			return filepath.Join(wd, "infra"), nil
		}

		parent := filepath.Dir(wd)
		if parent == wd {
			break
		}
		wd = parent
	}

	return "", fmt.Errorf("raiz da infra não encontrada (procurado em .yby e infra/.yby a partir de %s)", startWd)
}

// JoinInfra joins paths preserving root context
func JoinInfra(root, path string) string {
	return filepath.Join(root, path)
}
