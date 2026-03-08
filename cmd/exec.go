package cmd

import (
	"os/exec"
)

// execCommand is a variable to allow mocking in tests
var execCommand = exec.Command

// lookPath is a variable to allow mocking in tests
var lookPath = exec.LookPath
