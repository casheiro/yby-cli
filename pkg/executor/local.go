package executor

import (
	"fmt"
	"os"
	"os/exec"
)

type LocalExecutor struct{}

func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{}
}

func (e *LocalExecutor) Run(name, script string) error {
	fmt.Printf("%s %s... ", stepStyle.Render("⚙️"), name)

	// Execute via bash -c to support script strings
	cmd := exec.Command("bash", "-c", script)

	// We could pipe stdout/stderr to os.Stdout/Stderr directly if we want verbose
	// For consistency with SSHExecutor (which captures buffers), we'll capture output but allow full interactivity?
	// SSHExecutor currently buffers. Let's buffer here too for consistency of "Fail message".

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("\n%s Falha!\n%s\n", crossStyle.String(), string(output))
		return err
	}

	fmt.Printf("%s\n", checkStyle.String())
	return nil
}

func (e *LocalExecutor) FetchFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (e *LocalExecutor) Close() error {
	return nil
}
