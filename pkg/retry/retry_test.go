package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDoSuccessOnFirstTry(t *testing.T) {
	opts := Options{
		InitialInterval: 1 * time.Millisecond,
		MaxElapsedTime:  50 * time.Millisecond,
	}

	attempts := 0
	err := Do(context.Background(), opts, func() error {
		attempts++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestDoSuccessAfterRetries(t *testing.T) {
	opts := Options{
		InitialInterval: 5 * time.Millisecond,
		MaxElapsedTime:  200 * time.Millisecond,
	}

	attempts := 0
	err := Do(context.Background(), opts, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestDoTimeout(t *testing.T) {
	opts := Options{
		InitialInterval: 5 * time.Millisecond,
		MaxElapsedTime:  50 * time.Millisecond,
	}

	attempts := 0
	err := Do(context.Background(), opts, func() error {
		attempts++
		return errors.New("persistent error")
	})

	assert.Error(t, err)
	assert.Greater(t, attempts, 1)
}
