package executor

// Executor defines the interface for running commands and fetching files
// on a target system (either local or remote).
type Executor interface {
	// Run executes a script or command on the target.
	// name is for logging purposes.
	Run(name, script string) error

	// FetchFile reads a file from the target system.
	FetchFile(path string) ([]byte, error)

	// Close cleans up any resources (e.g. SSH connection).
	Close() error

	// StreamOutput allows capturing stdout/stderr (optional for now, but good for future)
	// For MVP Run() just streams to os.Stdout/Stderr as implemented in current bootstrap_vps
}
