package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/lipgloss"
)

// Validade do token no painel do cloud. A barra mostra quanto falta sobre uma referência
// fixa de 1 h (a duração típica dos tokens de STS, Entra ID e gcloud), limitada a 0..1: um
// token de 12 h aparece cheio até faltar menos de 1 h. Abaixo de 5 min, ▲ atenção.
const (
	referenciaToken = time.Hour
	avisoToken      = 5 * time.Minute
)

// estadoCluster é o estado de uma linha do painel, do mais grave ao mais tranquilo.
type estadoCluster int

const (
	estadoSemResposta estadoCluster = iota // o provider não respondeu: a linha é da leitura anterior
	estadoSemLogin                         // o CLI do provider não tem credencial válida
	estadoExpirado                         // o token já venceu
	estadoExpiraLogo                       // faltam menos de 5 min
	estadoNaoAtivo                         // o cluster não está ACTIVE/RUNNING
	estadoPronto
)

// validadeToken é a validade do token num instante, pronta para desenhar.
type validadeToken struct {
	conhecida bool
	restante  time.Duration // negativo depois de vencer
	fracao    float64       // restante / 1 h, limitado a 0..1
	aviso     bool          // faltam menos de 5 min
	expirado  bool
}

// validadeEm calcula a validade de um token que vence em expira (zero: desconhecida).
func validadeEm(expira, agora time.Time) validadeToken {
	if expira.IsZero() {
		return validadeToken{}
	}
	r := expira.Sub(agora)
	v := validadeToken{conhecida: true, restante: r, fracao: float64(r) / float64(referenciaToken)}
	v.fracao = min(1, max(0, v.fracao))
	v.expirado = r <= 0
	v.aviso = !v.expirado && r < avisoToken
	return v
}

// texto é o que vai à direita da barra ("11 min") ou, vencido, "expirou há 12 min";
// sem informação, "—".
func (v validadeToken) texto() string {
	switch {
	case !v.conhecida:
		return ui.G().Vazio
	case v.expirado:
		return textoExpirou(-v.restante)
	}
	return textoRestante(v.restante)
}

// celula é a coluna TOKEN: a cota da validade, o vencimento ou "—". Com barra abaixo de
// 4 células (coluna estreita), só o tempo, que é o dado principal.
func (v validadeToken) celula(barra int) string {
	s := ui.S()
	t := v.texto()
	switch {
	case !v.conhecida || v.expirado:
		return s.TintaSuave.Render(t)
	case barra < larguraMinimaBarra:
		if v.aviso {
			return s.Aviso.Render(t)
		}
		return s.Tinta.Render(t)
	}
	return ui.CotaRestante("", v.fracao, t, v.aviso, barra+2+lipgloss.Width(t))
}

// larguraMinimaBarra é a menor barra que a ui.Cota desenha.
const larguraMinimaBarra = 4

// textoRestante formata o tempo que falta: "45 s", "11 min", "1 h", "2 h 05 min".
func textoRestante(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d/time.Minute))
	}
	h, m := int(d/time.Hour), int(d%time.Hour/time.Minute)
	if m == 0 {
		return fmt.Sprintf("%d h", h)
	}
	return fmt.Sprintf("%d h %02d min", h, m)
}

// textoExpirou formata há quanto tempo o token venceu.
func textoExpirou(d time.Duration) string {
	switch {
	case d < time.Second:
		return "expirou agora"
	case d < time.Minute:
		return fmt.Sprintf("expirou há %d s", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("expirou há %d min", int(d/time.Minute))
	case d < 48*time.Hour:
		return fmt.Sprintf("expirou há %d h", int(d/time.Hour))
	}
	return fmt.Sprintf("expirou há %d dias", int(d/(24*time.Hour)))
}

// statusDoProvider traduz os status que os providers devolvem (EKS, AKS, GKE).
var statusDoProvider = map[string]string{
	"CREATING":     "criando",
	"UPDATING":     "atualizando",
	"DELETING":     "excluindo",
	"FAILED":       "falhou",
	"STOPPED":      "parado",
	"STOPPING":     "parando",
	"STARTING":     "iniciando",
	"PENDING":      "pendente",
	"PROVISIONING": "provisionando",
	"RECONCILING":  "ajustando",
	"ERROR":        "erro",
	"DEGRADED":     "degradado",
}

// statusLegivel é o status do provider em português; um status desconhecido sai como
// veio, em minúsculas.
func statusLegivel(status string) string {
	if t, ok := statusDoProvider[strings.ToUpper(status)]; ok {
		return t
	}
	return strings.ToLower(status)
}

// clusterAtivo diz se o status do provider é de cluster no ar (AWS ACTIVE, AKS Running,
// GKE RUNNING). Status vazio não é problema: o provider não informou.
func clusterAtivo(status string) bool {
	switch strings.ToUpper(status) {
	case "", "ACTIVE", "RUNNING", "SUCCEEDED":
		return true
	}
	return false
}

// estadoEm é o estado da linha no instante agora.
func (c clusterPainel) estadoEm(agora time.Time) estadoCluster {
	if c.desatualizado {
		return estadoSemResposta
	}
	if !c.autenticado {
		return estadoSemLogin
	}
	v := validadeEm(c.expiraEm, agora)
	switch {
	case v.expirado:
		return estadoExpirado
	case v.aviso:
		return estadoExpiraLogo
	case !clusterAtivo(c.info.Status):
		return estadoNaoAtivo
	}
	return estadoPronto
}

// celulaStatus é a coluna STATUS: glifo, cor e palavra.
func (c clusterPainel) celulaStatus(agora time.Time) string {
	s, g := ui.S(), ui.G()
	switch c.estadoEm(agora) {
	case estadoSemResposta:
		return s.Erro.Render(g.Falha + " sem resposta")
	case estadoSemLogin:
		return s.Erro.Render(g.Falha + " sem login")
	case estadoExpirado:
		return s.Erro.Render(g.Falha + " token")
	case estadoExpiraLogo:
		return s.Aviso.Render(g.Alerta) + " atenção"
	case estadoNaoAtivo:
		return s.Aviso.Render(g.Alerta) + " " + statusLegivel(c.info.Status)
	}
	return s.Ok.Render(g.Feito) + " pronto"
}

// celulaToken é a coluna TOKEN; sem login, não há token para medir.
func (c clusterPainel) celulaToken(agora time.Time, barra int) string {
	if !c.autenticado {
		return ui.S().TintaSuave.Render(ui.G().Vazio)
	}
	return validadeEm(c.expiraEm, agora).celula(barra)
}

// tokenNoDetalhe é a linha "token" do detalhe: "✓ válido até 12:18 · faltam 11 min".
func (c clusterPainel) tokenNoDetalhe(agora time.Time) string {
	s, g := ui.S(), ui.G()
	if !c.autenticado {
		return s.Erro.Render(g.Falha) + " sem login no CLI do provider"
	}
	v := validadeEm(c.expiraEm, agora)
	hora := c.expiraEm.In(agora.Location()).Format("15:04")
	switch {
	case !v.conhecida:
		return s.TintaSuave.Render("desconhecido")
	case v.expirado:
		return s.Erro.Render(g.Falha) + " expirou às " + hora
	case v.aviso:
		return s.Aviso.Render(g.Alerta) + " expira às " + hora + " · faltam " + s.Aviso.Render(v.texto())
	}
	return s.Ok.Render(g.Feito) + " válido até " + hora + " · faltam " + v.texto()
}
