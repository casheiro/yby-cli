package tools

import (
	"fmt"
	"regexp"
)

// guarda é uma operação destrutiva que a IA não pode montar numa ferramenta.
type guarda struct {
	Padrao *regexp.Regexp
	Motivo string
}

// guardas são as operações que mudam o cluster de um jeito difícil de desfazer. Uma
// palavra só conta como comando inteira, separada por espaço: o pod "drain-worker" não é
// um drain.
var guardas = []guarda{
	{
		Padrao: regexp.MustCompile(`(?i)(^|\s)delete\s+(namespaces?|ns)(\s|/|$)`),
		Motivo: "exclusão de namespace destrói todos os recursos contidos nele",
	},
	{
		Padrao: regexp.MustCompile(`(?i)(^|\s)delete\s+(clusterroles?|clusterrolebindings?)(\s|/|$)`),
		Motivo: "exclusão de ClusterRole ou ClusterRoleBinding pode comprometer o RBAC do cluster",
	},
	{
		Padrao: regexp.MustCompile(`(?i)(^|\s)drain(\s|$)`),
		Motivo: "drain de node despeja todos os pods dele",
	},
	{
		Padrao: regexp.MustCompile(`(?i)(^|\s)cordon(\s|$)`),
		Motivo: "cordon de node impede scheduling e pode causar degradação do cluster",
	},
	{
		Padrao: regexp.MustCompile(`(?i)(^|\s)apply\s.*--force(\s|=|$)`),
		Motivo: "apply com --force pode recriar recursos destrutivamente",
	},
	{
		Padrao: regexp.MustCompile(`(?i)(^|\s)scale\s.*--replicas[= ]0+(\s|$)`),
		Motivo: "escalar para 0 réplicas derruba o serviço",
	},
}

// conferirGuardas barra a ferramenta externa que, com os valores da IA, viraria uma
// operação destrutiva. linha é o comando como vai rodar (e os valores dos parâmetros,
// que chegam também pela env a um script); modelo é o comando do YAML, com os {{}}. O
// que o próprio YAML já faz (um "kubectl cordon {{no}}") foi escolha de quem o escreveu
// e confiou nele: só bloqueia o que aparece na linha e não no modelo.
func conferirGuardas(modelo, linha string) error {
	for _, g := range guardas {
		if g.Padrao.MatchString(linha) && !g.Padrao.MatchString(modelo) {
			return fmt.Errorf("operação bloqueada: %s; rode o comando você mesmo, se for o caso", g.Motivo)
		}
	}
	return nil
}
