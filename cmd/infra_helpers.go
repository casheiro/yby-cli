package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindInfraRoot searches for the .yby directory upwards
func FindInfraRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(wd, ".yby")); err == nil {
			return wd, nil
		}

		parent := filepath.Dir(wd)
		if parent == wd {
			return "", fmt.Errorf("infra root not found")
		}
		wd = parent
	}
}

// JoinInfra joins paths preserving root context
func JoinInfra(root, path string) string {
	return filepath.Join(root, path)
}
