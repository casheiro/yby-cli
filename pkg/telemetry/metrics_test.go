package telemetry

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRecordAndFlush(t *testing.T) {
	// Reset global state for test
	mu.Lock()
	events = nil
	mu.Unlock()

	// Test success record
	Record("test-success", 100*time.Millisecond, nil)

	// Test failure record
	errFail := errors.New("test failure")
	Record("test-fail", 200*time.Millisecond, errFail)

	mu.Lock()
	assert.Len(t, events, 2)
	assert.Equal(t, "test-success", events[0].Name)
	assert.True(t, events[0].Success)
	assert.Equal(t, "test-fail", events[1].Name)
	assert.False(t, events[1].Success)
	assert.Equal(t, errFail, events[1].Error)
	mu.Unlock()

	// Flush should not panic (output goes to slog)
	assert.NotPanics(t, func() {
		Flush()
	})
}

func TestTrack(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	err := func() (err error) {
		start := time.Now()
		defer Track("tracked-op", start, &err)
		time.Sleep(10 * time.Millisecond)
		return errors.New("tracked error")
	}()

	assert.Error(t, err)

	mu.Lock()
	assert.Len(t, events, 1)
	assert.Equal(t, "tracked-op", events[0].Name)
	assert.False(t, events[0].Success)
	assert.GreaterOrEqual(t, events[0].Duration, 10*time.Millisecond)
	mu.Unlock()
}
