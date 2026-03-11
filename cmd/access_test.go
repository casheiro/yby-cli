package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessCmd_HasContextFlag(t *testing.T) {
	f := accessCmd.Flags().Lookup("context")
	assert.NotNil(t, f)
	assert.Equal(t, "", f.DefValue)
}

func TestAccessCmd_HasRunE(t *testing.T) {
	assert.NotNil(t, accessCmd.RunE)
}
