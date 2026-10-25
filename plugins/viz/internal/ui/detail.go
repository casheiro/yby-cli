package ui

// ViewMode controla se a tela mostra lista ou detalhe
type ViewMode int

const (
	// ModeList é o modo padrão de listagem de recursos
	ModeList ViewMode = iota
	// ModeDetail exibe conteúdo detalhado (logs, YAML, eventos)
	ModeDetail
)
