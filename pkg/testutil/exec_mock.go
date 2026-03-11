package testutil

import (
	"os"
	"os/exec"
	"testing"
)

// MockExecCommand prepares a command that will call sending the test binary to run TestHelperProcess.
// It sets environment variables that TestHelperProcess will use to determine what to output/exit.
func MockExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// HelperProcessVerifier should be called inside TestHelperProcess in your test files.
// It checks if the process is being run as a helper and returns true if so.
// If it returns true, the caller should handle the logic for the mocked command.
// If it returns false, the caller (TestHelperProcess) should do nothing (it might be the main test run).
func HelperProcessVerifier() bool {
	return os.Getenv("GO_WANT_HELPER_PROCESS") == "1"
}

// StandardHelperProcess is a convenience function that prints standard output and exits with 0 or 1
// based on enviroment variables or arguments. You can implement custom logic in your specific test files instead.
func StandardHelperProcess(t *testing.T) {
	if !HelperProcessVerifier() {
		return
	}
	// args[0] is the test binary name
	// args[1] is -test.run=...
	// args[2] is --
	// args[3] is the command name
	// args[4...] are the arguments

	// You can add logic here if needed, but usually tests will define their own TestHelperProcess
	// to match specific commands.
	os.Exit(0)
}
