package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFlush_SemEventos cobre o branch de events vazio no Flush (linha 51).
func TestFlush_SemEventos(t *testing.T) {
	mu.Lock()
	events = nil
	mu.Unlock()

	// Flush com lista vazia não deve entrar em pânico
	assert.NotPanics(t, func() {
		Flush()
	})
}
