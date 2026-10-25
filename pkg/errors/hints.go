package errors

// defaultHints mapeia códigos de erro a dicas acionáveis em PT-BR, no tom da carta do
// terminal: no imperativo, em minúscula e sem ponto final (saem na linha "faça").
var defaultHints = map[string]string{
	ErrCodeIO:                 "confira o caminho e as permissões do arquivo ou da pasta",
	ErrCodeCmdNotFound:        "instale o comando e confira se ele está no PATH",
	ErrCodeExec:               "rode de novo com --log-level=debug para ver a saída do comando",
	ErrCodeNetworkTimeout:     "confira a conexão de rede e tente de novo",
	ErrCodeUnreachable:        "confira se o serviço está no ar e acessível pela rede",
	ErrCodePortForward:        "confira se o cluster está no ar e se o serviço existe",
	ErrCodeClusterOffline:     "confira se o cluster está no ar com kubectl cluster-info",
	ErrCodeManifest:           "confira a sintaxe YAML e os campos obrigatórios do manifesto",
	ErrCodeHelm:               "confira se o helm está instalado e se o chart existe",
	ErrCodeValidation:         "confira os parâmetros informados",
	ErrCodeConfig:             "confira o arquivo ~/.yby/config.yaml",
	ErrCodePlugin:             "rode de novo com --log-level=debug para ver o que o plugin respondeu",
	ErrCodePluginRPC:          "confira se o binário do plugin roda sozinho",
	ErrCodePluginNotFound:     "rode yby plugin list para ver os plugins de ~/.yby/plugins",
	ErrCodeScaffold:           "confira os templates e os parâmetros do projeto",
	ErrCodeTokenLimit:         "reduza o texto ou use um modelo com contexto maior",
	ErrCodeCloudTokenExpired:  "rode yby cloud refresh para renovar as credenciais",
	ErrCodeCloudCLIMissing:    "instale o CLI do provider (aws, az ou gcloud) e confira o PATH",
	ErrCodeCloudModelDisabled: "confira as cotas e as permissões do modelo na conta cloud",
}

// GenericHint é a dica padrão para erros sem hint específico.
const GenericHint = "rode de novo com --log-level=debug para ver os detalhes"

// GetDefaultHint retorna a dica padrão para um código de erro, ou vazio se não houver.
func GetDefaultHint(code string) string {
	if hint, ok := defaultHints[code]; ok {
		return hint
	}
	return ""
}
