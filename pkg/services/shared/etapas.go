package shared

import (
	"sync"
	"time"
)

// Etapas recebe o andamento de um processo longo (yby up, bootstrap) para mostrar às
// pessoas. *ui.Progresso implementa esta interface; os serviços não escrevem na tela.
type Etapas interface {
	// Iniciar começa uma etapa.
	Iniciar(rotulo string)
	// Avancar informa a fração (0 a 1) já feita da etapa atual.
	Avancar(fracao float64)
	// Concluir encerra a etapa atual com sucesso; nota é um complemento curto.
	Concluir(nota string)
	// Falhar encerra a etapa atual com erro.
	Falhar(nota string)
	// Detalhe acrescenta um texto abaixo das etapas (ex.: a saída de um script que falhou).
	Detalhe(texto string)
	// Avisar acrescenta um alerta (▲) abaixo das etapas, para o que não interrompe o
	// processo mas pede atenção; chame depois de Concluir a etapa a que ele se refere.
	Avisar(texto string)
}

// EtapasMudas descarta o andamento; é o padrão dos serviços sem Etapas configuradas.
type EtapasMudas struct{}

// Iniciar não faz nada.
func (EtapasMudas) Iniciar(string) {}

// Avancar não faz nada.
func (EtapasMudas) Avancar(float64) {}

// Concluir não faz nada.
func (EtapasMudas) Concluir(string) {}

// Falhar não faz nada.
func (EtapasMudas) Falhar(string) {}

// Detalhe não faz nada.
func (EtapasMudas) Detalhe(string) {}

// Avisar não faz nada.
func (EtapasMudas) Avisar(string) {}

// OuMudas devolve e, ou EtapasMudas quando e é nil.
func OuMudas(e Etapas) Etapas {
	if e == nil {
		return EtapasMudas{}
	}
	return e
}

// AcompanharLimite informa em e, a cada intervalo, a fração de limite já gasta numa
// espera longa (helm --wait, convergência do Argo CD), que vira a cota da etapa atual.
// Devolve a função que para o acompanhamento; chame-a quando a espera terminar.
func AcompanharLimite(e Etapas, limite, intervalo time.Duration) (parar func()) {
	if e == nil || limite <= 0 || intervalo <= 0 {
		return func() {}
	}
	inicio := time.Now()
	fim := make(chan struct{})
	pronto := make(chan struct{})
	go func() {
		defer close(pronto)
		t := time.NewTicker(intervalo)
		defer t.Stop()
		for {
			select {
			case <-fim:
				return
			case agora := <-t.C:
				e.Avancar(float64(agora.Sub(inicio)) / float64(limite))
			}
		}
	}()
	var uma sync.Once
	return func() {
		uma.Do(func() {
			close(fim)
			<-pronto
		})
	}
}
