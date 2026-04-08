package analysis

import (
	"testing"

	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

func TestDiffBlueprints_Identicos(t *testing.T) {
	bp := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a", Type: "app"},
		},
		Relations: []discovery.Relation{
			{From: "a", To: "b", Type: "imports"},
		},
	}
	diff := DiffBlueprints(bp, bp)
	if len(diff.Added) != 0 {
		t.Errorf("esperado 0 adicionados, obtido %d", len(diff.Added))
	}
	if len(diff.Removed) != 0 {
		t.Errorf("esperado 0 removidos, obtido %d", len(diff.Removed))
	}
	if len(diff.ChangedRelations) != 0 {
		t.Errorf("esperado 0 relações alteradas, obtido %d", len(diff.ChangedRelations))
	}
}

func TestDiffBlueprints_ComponenteAdicionado(t *testing.T) {
	old := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a", Type: "app"},
		},
	}
	new := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a", Type: "app"},
			{Name: "b", Path: "b", Type: "lib"},
		},
	}
	diff := DiffBlueprints(old, new)
	if len(diff.Added) != 1 {
		t.Fatalf("esperado 1 adicionado, obtido %d", len(diff.Added))
	}
	if diff.Added[0].Path != "b" {
		t.Errorf("esperado path 'b', obtido %q", diff.Added[0].Path)
	}
	if len(diff.Removed) != 0 {
		t.Errorf("esperado 0 removidos, obtido %d", len(diff.Removed))
	}
}

func TestDiffBlueprints_ComponenteRemovido(t *testing.T) {
	old := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a", Type: "app"},
			{Name: "b", Path: "b", Type: "lib"},
		},
	}
	new := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a", Type: "app"},
		},
	}
	diff := DiffBlueprints(old, new)
	if len(diff.Removed) != 1 {
		t.Fatalf("esperado 1 removido, obtido %d", len(diff.Removed))
	}
	if diff.Removed[0].Path != "b" {
		t.Errorf("esperado path 'b', obtido %q", diff.Removed[0].Path)
	}
}

func TestDiffBlueprints_RelacaoAdicionada(t *testing.T) {
	old := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a"},
			{Name: "b", Path: "b"},
		},
	}
	new := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a"},
			{Name: "b", Path: "b"},
		},
		Relations: []discovery.Relation{
			{From: "a", To: "b", Type: "imports"},
		},
	}
	diff := DiffBlueprints(old, new)
	if len(diff.ChangedRelations) != 1 {
		t.Fatalf("esperado 1 relação alterada, obtido %d", len(diff.ChangedRelations))
	}
	if diff.ChangedRelations[0].Action != "added" {
		t.Errorf("esperado action 'added', obtido %q", diff.ChangedRelations[0].Action)
	}
}

func TestDiffBlueprints_RelacaoRemovida(t *testing.T) {
	old := &discovery.Blueprint{
		Relations: []discovery.Relation{
			{From: "a", To: "b", Type: "imports"},
		},
	}
	new := &discovery.Blueprint{}
	diff := DiffBlueprints(old, new)
	if len(diff.ChangedRelations) != 1 {
		t.Fatalf("esperado 1 relação alterada, obtido %d", len(diff.ChangedRelations))
	}
	if diff.ChangedRelations[0].Action != "removed" {
		t.Errorf("esperado action 'removed', obtido %q", diff.ChangedRelations[0].Action)
	}
}

func TestDiffBlueprints_NilOld(t *testing.T) {
	new := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a"},
		},
	}
	diff := DiffBlueprints(nil, new)
	if len(diff.Added) != 1 {
		t.Errorf("esperado 1 adicionado com old=nil, obtido %d", len(diff.Added))
	}
}

func TestDiffBlueprints_NilNew(t *testing.T) {
	old := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a"},
		},
	}
	diff := DiffBlueprints(old, nil)
	if len(diff.Removed) != 1 {
		t.Errorf("esperado 1 removido com new=nil, obtido %d", len(diff.Removed))
	}
}

func TestDiffBlueprints_MultiplasAlteracoes(t *testing.T) {
	old := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a"},
			{Name: "b", Path: "b"},
		},
		Relations: []discovery.Relation{
			{From: "a", To: "b", Type: "imports"},
		},
	}
	new := &discovery.Blueprint{
		Components: []discovery.Component{
			{Name: "a", Path: "a"},
			{Name: "c", Path: "c"},
		},
		Relations: []discovery.Relation{
			{From: "a", To: "c", Type: "deploys"},
		},
	}
	diff := DiffBlueprints(old, new)

	if len(diff.Added) != 1 || diff.Added[0].Path != "c" {
		t.Errorf("esperado 1 adicionado (c), obtido %v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Path != "b" {
		t.Errorf("esperado 1 removido (b), obtido %v", diff.Removed)
	}
	if len(diff.ChangedRelations) != 2 {
		t.Errorf("esperado 2 relações alteradas, obtido %d", len(diff.ChangedRelations))
	}
}
