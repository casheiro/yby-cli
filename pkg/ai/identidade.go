package ai

import (
	"context"
	"errors"
	"fmt"
)

// IdentidadeDoIndice diz com que embeddings os vetores de um índice foram gerados.
// Vetores de providers, modelos ou dimensões diferentes não se comparam: a busca com
// outros embeddings falha ou devolve qualquer coisa.
type IdentidadeDoIndice struct {
	// Provider é um nome estável do provider (ollama, local, openai…), sem endereço.
	Provider string `json:"provider"`
	// Modelo é o modelo efetivo dos embeddings; vazio quando o provider não diz.
	Modelo string `json:"modelo,omitempty"`
	// Dimensao é o tamanho dos vetores; 0 quando ainda não se sabe (antes de embutir).
	Dimensao int `json:"dimensao,omitempty"`
}

// String escreve a identidade como "provider/modelo (N dimensões)".
func (i IdentidadeDoIndice) String() string {
	s := i.Provider
	if i.Modelo != "" {
		s += "/" + i.Modelo
	}
	if i.Dimensao > 0 {
		s += fmt.Sprintf(" (%d dimensões)", i.Dimensao)
	}
	return s
}

// combina diz se os vetores das duas identidades se comparam. Dimensão 0 é a de quem
// ainda não embutiu nada e combina com qualquer uma.
func (i IdentidadeDoIndice) combina(outra IdentidadeDoIndice) bool {
	if i.Provider != outra.Provider || i.Modelo != outra.Modelo {
		return false
	}
	return i.Dimensao == 0 || outra.Dimensao == 0 || i.Dimensao == outra.Dimensao
}

// IdentificaEmbeddings é o provider que diz quem gera os seus embeddings: o nome estável
// do provider e o modelo efetivo (no Ollama sem config, o primeiro baixado).
type IdentificaEmbeddings interface {
	IdentidadeDosEmbeddings(ctx context.Context) (IdentidadeDoIndice, error)
}

// decorador é o provider que embrulha outro (cache, retry, rate limit…).
type decorador interface {
	Unwrap() Provider
}

// IdentidadeDosEmbeddings diz quem gera os embeddings de p, atravessando os decorators.
// O provider que não se identifica fica pelo Name(), sem modelo.
func IdentidadeDosEmbeddings(ctx context.Context, p Provider) (IdentidadeDoIndice, error) {
	for p != nil {
		if id, ok := p.(IdentificaEmbeddings); ok {
			return id.IdentidadeDosEmbeddings(ctx)
		}
		d, ok := p.(decorador)
		if !ok {
			return IdentidadeDoIndice{Provider: p.Name()}, nil
		}
		p = d.Unwrap()
	}
	return IdentidadeDoIndice{}, errors.New("nenhum provider de embeddings")
}

// ErrIndiceDeOutroModelo é o erro de um índice feito com outros embeddings que não os de
// agora; o detalhe vem em *IndiceDeOutroModeloError.
var ErrIndiceDeOutroModelo = errors.New("o índice foi feito com outros embeddings")

// IndiceDeOutroModeloError diz com que embeddings o índice foi feito e de onde vêm os de
// agora. Indice vazio é o de um índice antigo, de antes de o yby registrar o modelo.
type IndiceDeOutroModeloError struct {
	Indice IdentidadeDoIndice
	Atual  IdentidadeDoIndice
}

// FeitoCom diz com que embeddings o índice foi feito, mesmo sem identidade registrada.
func (e *IndiceDeOutroModeloError) FeitoCom() string {
	if e.Indice.Provider == "" {
		return "um modelo de embeddings que não ficou registrado"
	}
	return e.Indice.String()
}

// Diferenca descreve a troca, sem dizer o que fazer.
func (e *IndiceDeOutroModeloError) Diferenca() string {
	return fmt.Sprintf("o índice foi feito com %s, e os embeddings agora vêm de %s", e.FeitoCom(), e.Atual)
}

func (e *IndiceDeOutroModeloError) Error() string {
	return e.Diferenca() + "; rode yby synapstor index --full"
}

// Is faz o errors.Is(err, ErrIndiceDeOutroModelo) reconhecer o erro.
func (e *IndiceDeOutroModeloError) Is(alvo error) bool {
	return alvo == ErrIndiceDeOutroModelo
}
