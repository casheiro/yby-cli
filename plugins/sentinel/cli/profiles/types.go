//go:build k8s

// Package profiles define perfis de compliance para o Sentinel.
package profiles

// ComplianceProfile define um perfil de compliance com checks associados.
type ComplianceProfile struct {
	Name        string
	Description string
	CheckIDs    []string
}
