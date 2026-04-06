package errors

import (
	"fmt"
)

// YbyError represents a structured, trackable error within the Yby CLI.
// It wraps standard Go errors while providing domain-specific context.
type YbyError struct {
	Code    string
	Message string
	Cause   error                  // Underlying error (for %w unwrapping)
	Context map[string]interface{} // Diagnostic data
	Hint    string                 // Sugestão de correção para o usuário
}

// Error implements the standard error interface.
// Format: "[CODE] Message: Cause" or "[CODE] Message"
func (e *YbyError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap allows errors.Is and errors.As to work with the underlying cause.
func (e *YbyError) Unwrap() error {
	return e.Cause
}

// New creates a new YbyError without an underlying cause.
func New(code, message string) *YbyError {
	return &YbyError{
		Code:    code,
		Message: message,
	}
}

// Wrap creates a new YbyError wrapping an existing error.
func Wrap(cause error, code, message string) *YbyError {
	return &YbyError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// WithContext returns a new YbyError (or modifies the existing one if preferred)
// appending contextual key-value pairs.
// For immutability in chaining, we return a mutated pointer (it modifies in-place).
func (e *YbyError) WithContext(key string, value interface{}) *YbyError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithHint adiciona uma sugestão de correção ao erro.
func (e *YbyError) WithHint(hint string) *YbyError {
	e.Hint = hint
	return e
}

// GetHint retorna o hint do erro, ou busca no registry de hints padrão pelo código.
func (e *YbyError) GetHint() string {
	if e.Hint != "" {
		return e.Hint
	}
	return GetDefaultHint(e.Code)
}

// --- Standardized Error Codes ---

const (
	// System / OS Level
	ErrCodeIO          = "ERR_IO"
	ErrCodeCmdNotFound = "ERR_CMD_NOT_FOUND"
	ErrCodeExec        = "ERR_EXEC_FAILED"

	// Network & Connectivity
	ErrCodeNetworkTimeout = "ERR_NETWORK_TIMEOUT"
	ErrCodeUnreachable    = "ERR_UNREACHABLE"
	ErrCodePortForward    = "ERR_PORT_FORWARD"

	// K8s & Cluster
	ErrCodeClusterOffline = "ERR_CLUSTER_OFFLINE"
	ErrCodeManifest       = "ERR_MANIFEST_INVALID"
	ErrCodeHelm           = "ERR_HELM_FAILED"

	// Validations & Configurations
	ErrCodeValidation = "ERR_VALIDATION"
	ErrCodeConfig     = "ERR_CONFIG_INVALID"

	// Plugins
	ErrCodePlugin         = "ERR_PLUGIN"
	ErrCodePluginRPC      = "ERR_PLUGIN_RPC"
	ErrCodePluginNotFound = "ERR_PLUGIN_NOT_FOUND"

	// Scaffold & Generation
	ErrCodeScaffold = "ERR_SCAFFOLD_FAILED"

	// AI / Token Limits
	ErrCodeTokenLimit = "ERR_TOKEN_LIMIT"
)
