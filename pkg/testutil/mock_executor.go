package testutil

// MockExecutor implementa executor.Executor para testes.
type MockExecutor struct {
	RunFunc       func(name, script string) error
	FetchFileFunc func(path string) ([]byte, error)
	CloseFunc     func() error
}

func (m *MockExecutor) Run(name, script string) error {
	if m.RunFunc != nil {
		return m.RunFunc(name, script)
	}
	return nil
}

func (m *MockExecutor) FetchFile(path string) ([]byte, error) {
	if m.FetchFileFunc != nil {
		return m.FetchFileFunc(path)
	}
	return []byte{}, nil
}

func (m *MockExecutor) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
