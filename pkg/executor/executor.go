package executor

import "os/exec"

// CommandExecutor is an interface for executing system commands.
// This allows for easy mocking in tests.
type CommandExecutor interface {
	// LookPath searches for an executable in PATH
	LookPath(file string) (string, error)

	// Command creates a new command to execute
	Command(name string, arg ...string) Command
}

// Command represents a command to be executed
type Command interface {
	Run() error
	Output() ([]byte, error)
	CombinedOutput() ([]byte, error)
}

// RealCommandExecutor implements CommandExecutor using real system calls
type RealCommandExecutor struct{}

func (r *RealCommandExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (r *RealCommandExecutor) Command(name string, arg ...string) Command {
	return &realCommand{cmd: exec.Command(name, arg...)}
}

// realCommand wraps exec.Cmd to implement the Command interface
type realCommand struct {
	cmd *exec.Cmd
}

func (r *realCommand) Run() error {
	return r.cmd.Run()
}

func (r *realCommand) Output() ([]byte, error) {
	return r.cmd.Output()
}

func (r *realCommand) CombinedOutput() ([]byte, error) {
	return r.cmd.CombinedOutput()
}
