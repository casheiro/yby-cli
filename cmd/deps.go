package cmd

import (
	"os/exec"
)

// lookPath é uma variável para permitir mocking em testes
var lookPath = exec.LookPath
