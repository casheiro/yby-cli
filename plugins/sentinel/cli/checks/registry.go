//go:build k8s

package checks

import "sync"

var (
	mu       sync.RWMutex
	registry []SecurityCheck
)

// Register adiciona um check ao registro global.
func Register(check SecurityCheck) {
	mu.Lock()
	defer mu.Unlock()
	registry = append(registry, check)
}

// GetAll retorna todos os checks registrados.
func GetAll() []SecurityCheck {
	mu.RLock()
	defer mu.RUnlock()
	result := make([]SecurityCheck, len(registry))
	copy(result, registry)
	return result
}

// GetByCategory retorna checks filtrados por categoria.
func GetByCategory(cat Category) []SecurityCheck {
	mu.RLock()
	defer mu.RUnlock()
	var result []SecurityCheck
	for _, c := range registry {
		if c.Category() == cat {
			result = append(result, c)
		}
	}
	return result
}

// GetByIDs retorna checks filtrados por IDs específicos.
func GetByIDs(ids []string) []SecurityCheck {
	mu.RLock()
	defer mu.RUnlock()
	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	var result []SecurityCheck
	for _, c := range registry {
		if _, ok := idSet[c.ID()]; ok {
			result = append(result, c)
		}
	}
	return result
}
