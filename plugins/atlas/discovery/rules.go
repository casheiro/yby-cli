package discovery

// DefaultRules defines the standard matching rules.
var DefaultRules = []Rule{
	{MatchFile: "go.mod", Type: "app"}, // Or lib, ambiguous
	{MatchFile: "package.json", Type: "app"},
	{MatchFile: "Dockerfile", Type: "infra"},
	{MatchFile: "Taskfile.yml", Type: "config"},
	{MatchFile: "Makefile", Type: "config"},
}

// Match checks a filename against rules and returns the type if matched.
func Match(filename string) string {
	for _, rule := range DefaultRules {
		if filename == rule.MatchFile {
			return rule.Type
		}
	}
	return ""
}
