package cmd

import (
	"os"
	"os/exec"
)

// execCommand is a variable to allow mocking in tests
var execCommand = exec.Command

// osExit is a variable to allow mocking in tests
var osExit = func(code int) {
	os.Exit(code)
}
