//go:build k8s

package profiles

var registry = map[string]ComplianceProfile{}

func register(p ComplianceProfile) {
	registry[p.Name] = p
}

// GetProfile retorna um perfil pelo nome.
func GetProfile(name string) (ComplianceProfile, bool) {
	p, ok := registry[name]
	return p, ok
}

// ListProfiles retorna todos os perfis disponíveis.
func ListProfiles() []ComplianceProfile {
	result := make([]ComplianceProfile, 0, len(registry))
	for _, p := range registry {
		result = append(result, p)
	}
	return result
}
