package errors

// defaultHints mapeia códigos de erro a dicas acionáveis em PT-BR.
var defaultHints = map[string]string{
	ErrCodeIO:             "Verifique as permissões do arquivo/diretório e se o caminho existe.",
	ErrCodeCmdNotFound:    "O comando necessário não foi encontrado. Verifique se está instalado e no PATH.",
	ErrCodeExec:           "A execução do comando falhou. Rode com --log-level=debug para detalhes.",
	ErrCodeNetworkTimeout: "Tempo de conexão esgotado. Verifique sua conexão de rede e tente novamente.",
	ErrCodeUnreachable:    "O serviço está inacessível. Verifique se está em execução e acessível na rede.",
	ErrCodePortForward:    "Falha no port-forward. Verifique se o cluster está ativo e o serviço existe.",
	ErrCodeClusterOffline: "O cluster não está respondendo. Verifique se está em execução com 'kubectl cluster-info'.",
	ErrCodeManifest:       "O manifesto Kubernetes é inválido. Verifique a sintaxe YAML e os campos obrigatórios.",
	ErrCodeHelm:           "O comando Helm falhou. Verifique se o Helm está instalado e o chart existe.",
	ErrCodeValidation:     "Dados de entrada inválidos. Verifique os parâmetros informados.",
	ErrCodeConfig:         "Configuração inválida. Verifique o arquivo ~/.yby/config.yaml.",
	ErrCodePlugin:         "Erro no plugin. Rode com --log-level=debug para mais detalhes.",
	ErrCodePluginRPC:      "Falha na comunicação com o plugin. Verifique se o binário está funcional.",
	ErrCodePluginNotFound: "Plugin não encontrado. Verifique se está instalado em ~/.yby/plugins/.",
	ErrCodeScaffold:       "Falha na geração de scaffold. Verifique os templates e parâmetros.",
	ErrCodeTokenLimit:     "O texto excede o limite de tokens do modelo. Reduza o conteúdo ou use um modelo com contexto maior.",
}

// GenericHint é a dica padrão para erros sem hint específico.
const GenericHint = "Rode com --log-level=debug para mais detalhes."

// GetDefaultHint retorna a dica padrão para um código de erro, ou vazio se não houver.
func GetDefaultHint(code string) string {
	if hint, ok := defaultHints[code]; ok {
		return hint
	}
	return ""
}
