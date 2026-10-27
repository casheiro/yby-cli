//go:build k8s

package remediation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// correcao é uma correção segura de um contêiner: só endurece o securityContext, sem
// mudar o usuário do processo, o sistema de arquivos nem os recursos dele.
type correcao struct {
	// descricao diz o que a correção faz, para a linha do patch.
	descricao string
	// falta devolve a linha do diff quando o contêiner ainda precisa da correção, ou o
	// motivo quando ele não a aceita; os dois vazios querem dizer que ele já a tem.
	falta func(c corev1.Container) (mudanca, recusa string)
	// aplicar escreve a correção no securityContext do patch do contêiner.
	aplicar func(sc map[string]any)
}

var semEscalacao = correcao{
	descricao: "bloqueia a escalação de privilégios",
	falta: func(c corev1.Container) (string, string) {
		sc := c.SecurityContext
		if sc != nil && sc.Privileged != nil && *sc.Privileged {
			// O servidor recusa allowPrivilegeEscalation false num contêiner privilegiado.
			return "", "o contêiner " + c.Name + " é privilegiado: tire o privileged no manifesto antes"
		}
		atual := "(vazio)"
		if sc != nil && sc.AllowPrivilegeEscalation != nil {
			if !*sc.AllowPrivilegeEscalation {
				return "", ""
			}
			atual = "true"
		}
		return "allowPrivilegeEscalation " + atual + " → false", ""
	},
	aplicar: func(sc map[string]any) { sc["allowPrivilegeEscalation"] = false },
}

var semCapabilities = correcao{
	descricao: "tira as capabilities",
	falta: func(c corev1.Container) (string, string) {
		var drop []string
		if c.SecurityContext != nil && c.SecurityContext.Capabilities != nil {
			for _, capab := range c.SecurityContext.Capabilities.Drop {
				if strings.EqualFold(string(capab), "ALL") {
					return "", ""
				}
				drop = append(drop, string(capab))
			}
		}
		atual := "(vazio)"
		if len(drop) > 0 {
			atual = "[" + strings.Join(drop, " ") + "]"
		}
		return "capabilities.drop " + atual + " → [ALL]", ""
	},
	// Só o drop muda: as capabilities que o contêiner adiciona continuam.
	aplicar: func(sc map[string]any) { sc["capabilities"] = map[string]any{"drop": []string{"ALL"}} },
}

// correcoes liga os checks às correções seguras. Os outros (runAsNonRoot,
// readOnlyRootFilesystem, privileged, limites, automount do token) mudam o comportamento
// do workload: a imagem que roda como root não sobe com runAsNonRoot, o processo que
// escreve no disco cai com o sistema de arquivos só de leitura, limites inventados matam
// o processo e o operador que fala com a API perde o token. Ficam sem correção
// automática, com a recomendação do achado.
var correcoes = map[string]correcao{
	"POD_PRIVILEGE_ESCALATION":           semEscalacao,
	"polaris/privilegeEscalationAllowed": semEscalacao,
	"POD_CAPABILITIES":                   semCapabilities,
	"polaris/insecureCapabilities":       semCapabilities,
}

// correcaoPara devolve a correção do achado. O POD_CAPABILITIES crítico é o SYS_ADMIN
// adicionado, que pede análise manual.
func correcaoPara(f checks.SecurityFinding) (correcao, bool) {
	if f.CheckID == "POD_CAPABILITIES" && f.Severity == checks.SeverityCritical {
		return correcao{}, false
	}
	c, ok := correcoes[f.CheckID]
	return c, ok
}

// origem é de onde o achado vem: o pod e o contêiner (checks internos) ou o controlador,
// sem contêiner (Polaris, que confere todos os do template).
type origem struct{ kind, nome, conteiner string }

func origensDe(f checks.SecurityFinding) []origem {
	if f.Pod != "" {
		return []origem{{kind: "Pod", nome: f.Pod, conteiner: f.Container}}
	}
	recursos := f.Resources
	if len(recursos) == 0 {
		recursos = []string{f.Resource}
	}
	var origens []origem
	for _, r := range recursos {
		if kind, nome, ok := strings.Cut(r, "/"); ok && nome != "" {
			origens = append(origens, origem{kind: kind, nome: nome})
		}
	}
	return origens
}

// montagem junta as correções de um recurso: o patch sai um só, com todos os contêineres.
type montagem struct {
	alvo alvo
	// conteineres guarda o securityContext do patch de cada contêiner, por lista
	// ("containers" ou "initContainers") e nome.
	conteineres map[string]map[string]map[string]any
	feitas      map[string]bool // contêiner|correção já aplicada
	descricoes  []string
	mudancas    []string
}

// Planejar monta as correções dos achados: sobe do pod ao dono (ou parte do controlador
// do achado do Polaris), deixa de fora o recurso gerido pelo Argo CD e junta num patch só
// as correções seguras de cada recurso. Nada vai ao servidor além das leituras.
func Planejar(ctx context.Context, client kubernetes.Interface, findings []checks.SecurityFinding) Plano {
	var plano Plano
	porAlvo := map[string]*montagem{}
	recusado := map[string]bool{}
	recusar := func(recurso, motivo string) {
		if !recusado[recurso+"|"+motivo] {
			recusado[recurso+"|"+motivo] = true
			plano.Recusas = append(plano.Recusas, Recusa{Recurso: recurso, Motivo: motivo})
		}
	}

	for _, f := range findings {
		corr, ok := correcaoPara(f)
		if !ok {
			plano.SemCorrecao++
			continue
		}
		for _, o := range origensDe(f) {
			a, err := donoDe(ctx, client, f.Namespace, o.kind, o.nome)
			if err != nil {
				var nc naoCorrigivel
				if errors.As(err, &nc) {
					recusar(o.kind+"/"+o.nome, nc.motivo)
				} else {
					recusar(o.kind+"/"+o.nome, motivoDaLeitura(err))
				}
				continue
			}
			if app, gerido := geridoPeloArgo(a.meta); gerido {
				recusar(a.recurso(), fmt.Sprintf("gerido pelo Argo CD (Application %s), que desfaria a mudança na próxima sincronização: corrija o manifesto no Git", app))
				continue
			}
			m := porAlvo[a.recurso()]
			if m == nil {
				m = &montagem{alvo: a, conteineres: map[string]map[string]map[string]any{}, feitas: map[string]bool{}}
				porAlvo[a.recurso()] = m
			}
			corrigirConteineres(m, corr, o.conteiner, recusar)
		}
	}

	for _, m := range porAlvo {
		if len(m.mudancas) > 0 {
			plano.Patches = append(plano.Patches, m.patch())
		}
	}
	sort.Slice(plano.Patches, func(i, j int) bool { return plano.Patches[i].Recurso() < plano.Patches[j].Recurso() })
	sort.SliceStable(plano.Recusas, func(i, j int) bool { return plano.Recusas[i].Recurso < plano.Recusas[j].Recurso })
	return plano
}

// corrigirConteineres aplica a correção ao contêiner nome do template ou, com nome vazio,
// a todos os que ainda não a têm.
func corrigirConteineres(m *montagem, corr correcao, nome string, recusar func(recurso, motivo string)) {
	achou := false
	for _, lista := range []struct {
		chave string
		cs    []corev1.Container
	}{{"containers", m.alvo.spec.Containers}, {"initContainers", m.alvo.spec.InitContainers}} {
		for _, c := range lista.cs {
			if nome != "" && c.Name != nome {
				continue
			}
			achou = true
			chave := lista.chave + "|" + c.Name + "|" + corr.descricao
			if m.feitas[chave] {
				continue
			}
			m.feitas[chave] = true
			mudanca, recusa := corr.falta(c)
			if recusa != "" {
				recusar(m.alvo.recurso(), recusa)
				continue
			}
			if mudanca == "" {
				continue
			}
			porNome := m.conteineres[lista.chave]
			if porNome == nil {
				porNome = map[string]map[string]any{}
				m.conteineres[lista.chave] = porNome
			}
			if porNome[c.Name] == nil {
				porNome[c.Name] = map[string]any{}
			}
			corr.aplicar(porNome[c.Name])
			rotulo := "contêiner " + c.Name
			if lista.chave == "initContainers" {
				rotulo = "contêiner de init " + c.Name
			}
			m.mudancas = append(m.mudancas, rotulo+": "+mudanca)
			if !slices.Contains(m.descricoes, corr.descricao) {
				m.descricoes = append(m.descricoes, corr.descricao)
			}
		}
	}
	if nome != "" && !achou {
		recusar(m.alvo.recurso(), "o contêiner "+nome+" não está no template: o recurso mudou depois do pod")
	}
}

// patch monta o strategic merge patch no caminho do template dos pods do recurso.
func (m *montagem) patch() RemediationPatch {
	podSpec := map[string]any{}
	for _, chave := range []string{"containers", "initContainers"} {
		porNome := m.conteineres[chave]
		nomes := make([]string, 0, len(porNome))
		for n := range porNome {
			nomes = append(nomes, n)
		}
		sort.Strings(nomes)
		var itens []map[string]any
		for _, n := range nomes {
			itens = append(itens, map[string]any{"name": n, "securityContext": porNome[n]})
		}
		if len(itens) > 0 {
			podSpec[chave] = itens
		}
	}
	template := map[string]any{"template": map[string]any{"spec": podSpec}}
	spec := template
	if m.alvo.kind == "CronJob" {
		spec = map[string]any{"jobTemplate": map[string]any{"spec": template}}
	}
	dados, _ := json.Marshal(map[string]any{"spec": spec})
	mudancas := append([]string(nil), m.mudancas...)
	sort.Strings(mudancas)
	return RemediationPatch{
		ResourceKind: m.alvo.kind,
		ResourceName: m.alvo.nome,
		Namespace:    m.alvo.namespace,
		PatchType:    "strategic-merge",
		Patch:        string(dados),
		Description:  strings.Join(m.descricoes, " e "),
		Mudancas:     mudancas,
	}
}
