package retry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	assert.NotNil(t, opts)
	assert.Equal(t, 1*time.Second, opts.InitialInterval)
	assert.Equal(t, 15*time.Second, opts.MaxInterval)
	assert.Equal(t, 2*time.Minute, opts.MaxElapsedTime)
	assert.Equal(t, 0.5, opts.RandomizationFactor)
	assert.Equal(t, 1.5, opts.Multiplier)
}

func TestDoWithDefault_Sucesso(t *testing.T) {
	count := 0
	err := DoWithDefault(context.Background(), func() error {
		count++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestDoWithDefault_RetentativaComSucesso(t *testing.T) {
	count := 0
	err := DoWithDefault(context.Background(), func() error {
		count++
		if count < 2 {
			return fmt.Errorf("falha temporária")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, 2)
}

func TestDoWithDefault_ContextoCancelado(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancela imediatamente

	count := 0
	err := DoWithDefault(ctx, func() error {
		count++
		return fmt.Errorf("erro persistente")
	})
	// Com contexto cancelado, deve parar de tentar rapidamente
	assert.Error(t, err)
}

func TestDo_ComOpcoesCustomizadas(t *testing.T) {
	opts := Options{
		InitialInterval:     1 * time.Millisecond,
		MaxInterval:         5 * time.Millisecond,
		MaxElapsedTime:      50 * time.Millisecond,
		RandomizationFactor: 0,
		Multiplier:          1.0,
	}

	count := 0
	err := Do(context.Background(), opts, func() error {
		count++
		if count < 3 {
			return fmt.Errorf("falha")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestDo_SemMultiplicador(t *testing.T) {
	opts := Options{
		InitialInterval:     1 * time.Millisecond,
		MaxInterval:         1 * time.Millisecond,
		MaxElapsedTime:      100 * time.Millisecond,
		RandomizationFactor: 0,
		Multiplier:          1.0,
	}

	count := 0
	err := Do(context.Background(), opts, func() error {
		count++
		return fmt.Errorf("sempre falha")
	})
	assert.Error(t, err)
	assert.Greater(t, count, 1) // Deve ter tentado mais de uma vez
}
