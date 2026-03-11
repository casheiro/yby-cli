package executor

import "errors"

// MockCommandExecutor is a mock implementation for testing
type MockCommandExecutor struct {
	LookPathFunc func(file string) (string, error)
	CommandFunc  func(name string, arg ...string) Command
}

func (m *MockCommandExecutor) LookPath(file string) (string, error) {
	if m.LookPathFunc != nil {
		return m.LookPathFunc(file)
	}
	return "", errors.New("not found")
}

func (m *MockCommandExecutor) Command(name string, arg ...string) Command {
	if m.CommandFunc != nil {
		return m.CommandFunc(name, arg...)
	}
	return &mockCommand{}
}

// mockCommand is a mock implementation of Command
type mockCommand struct {
	RunFunc            func() error
	OutputFunc         func() ([]byte, error)
	CombinedOutputFunc func() ([]byte, error)
}

func (m *mockCommand) Run() error {
	if m.RunFunc != nil {
		return m.RunFunc()
	}
	return nil
}

func (m *mockCommand) Output() ([]byte, error) {
	if m.OutputFunc != nil {
		return m.OutputFunc()
	}
	return []byte{}, nil
}

func (m *mockCommand) CombinedOutput() ([]byte, error) {
	if m.CombinedOutputFunc != nil {
		return m.CombinedOutputFunc()
	}
	return []byte{}, nil
}
