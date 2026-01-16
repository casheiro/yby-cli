package executor

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/lipgloss"
)

var (
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // Orange
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
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}

	// If Permission Denied, try sudo
	if os.IsPermission(err) {
		fmt.Println(warningStyle.Render("⚠️  Permissão negada. Tentando leitura com sudo..."))
		cmd := exec.Command("sudo", "cat", path)
		return cmd.CombinedOutput()
	}

	return nil, err
}

func (e *LocalExecutor) Close() error {
	return nil
}
