package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err := New(ErrCodeValidation, "invalid manifest path")

	assert.Equal(t, ErrCodeValidation, err.Code)
	assert.Equal(t, "invalid manifest path", err.Message)
	assert.Nil(t, err.Cause)
	assert.Equal(t, "[ERR_VALIDATION] invalid manifest path", err.Error())
}

func TestWrap(t *testing.T) {
	baseErr := fmt.Errorf("file not found")
	err := Wrap(baseErr, ErrCodeIO, "failed to read values.yaml")

	assert.Equal(t, ErrCodeIO, err.Code)
	assert.Equal(t, "failed to read values.yaml", err.Message)
	assert.Equal(t, baseErr, err.Cause)
	assert.Equal(t, "[ERR_IO] failed to read values.yaml: file not found", err.Error())
}

func TestWithContext(t *testing.T) {
	err := DefaultError().WithContext("path", "/tmp/config").WithContext("retries", 3)

	assert.NotNil(t, err.Context)
	assert.Equal(t, "/tmp/config", err.Context["path"])
	assert.Equal(t, 3, err.Context["retries"])
}

func TestUnwrap(t *testing.T) {
	baseErr := fmt.Errorf("underlying issue")
	err := Wrap(baseErr, ErrCodeExec, "command failed")

	assert.True(t, errors.Is(err, baseErr))

	extracted := errors.Unwrap(err)
	assert.Equal(t, baseErr, extracted)
}

// Helper for test
func DefaultError() *YbyError {
	return New("TEST_CODE", "test error")
}
