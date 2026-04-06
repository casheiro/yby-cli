package ui

import "strings"

// filterByName filtra itens por substring case-insensitive no nome
func filterByName[T any](items []T, query string, getName func(T) string) []T {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)
	var result []T
	for _, item := range items {
		if strings.Contains(strings.ToLower(getName(item)), q) {
			result = append(result, item)
		}
	}
	return result
}
