package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUninstallCmd_Structure(t *testing.T) {
	assert.Equal(t, "uninstall", uninstallCmd.Use)
	assert.NotEmpty(t, uninstallCmd.Short)
}
