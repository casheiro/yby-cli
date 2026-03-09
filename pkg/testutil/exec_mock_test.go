package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockExecCommand(t *testing.T) {
	cmd := MockExecCommand("echo", "hello")
	assert.NotNil(t, cmd)
	assert.Contains(t, cmd.Args, "-test.run=TestHelperProcess")
}

func TestHelperProcessVerifier_SemEnv(t *testing.T) {
	// Sem GO_WANT_HELPER_PROCESS definido, deve retornar false
	t.Setenv("GO_WANT_HELPER_PROCESS", "")
	assert.False(t, HelperProcessVerifier())
}

func TestHelperProcessVerifier_ComEnv(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	assert.True(t, HelperProcessVerifier())
}
