package analysis

import (
	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

// RelationChange representa uma relação adicionada ou removida.
type RelationChange struct {
	Relation discovery.Relation `json:"relation"`
	Action   string             `json:"action"` // "added" ou "removed"
}

// BlueprintDiff contém as diferenças entre dois blueprints.
type BlueprintDiff struct {
	Added            []discovery.Component `json:"added,omitempty"`
	Removed          []discovery.Component `json:"removed,omitempty"`
	ChangedRelations []RelationChange      `json:"changed_relations,omitempty"`
}

// DiffBlueprints compara dois blueprints e retorna as diferenças.
// Componentes são comparados por Path; relações por From+To+Type.
func DiffBlueprints(old, new *discovery.Blueprint) *BlueprintDiff {
	diff := &BlueprintDiff{}

	oldComps := make(map[string]discovery.Component)
	newComps := make(map[string]discovery.Component)

	if old != nil {
		for _, c := range old.Components {
			oldComps[c.Path] = c
		}
	}
	if new != nil {
		for _, c := range new.Components {
			newComps[c.Path] = c
		}
	}

	// Componentes removidos (em old, não em new)
	for path, comp := range oldComps {
		if _, ok := newComps[path]; !ok {
			diff.Removed = append(diff.Removed, comp)
		}
	}

	// Componentes adicionados (em new, não em old)
	for path, comp := range newComps {
		if _, ok := oldComps[path]; !ok {
			diff.Added = append(diff.Added, comp)
		}
	}

	// Relações
	type relKey struct{ from, to, typ string }
	oldRels := make(map[relKey]discovery.Relation)
	newRels := make(map[relKey]discovery.Relation)

	if old != nil {
		for _, r := range old.Relations {
			oldRels[relKey{r.From, r.To, r.Type}] = r
		}
	}
	if new != nil {
		for _, r := range new.Relations {
			newRels[relKey{r.From, r.To, r.Type}] = r
		}
	}

	// Relações removidas
	for key, rel := range oldRels {
		if _, ok := newRels[key]; !ok {
			diff.ChangedRelations = append(diff.ChangedRelations, RelationChange{
				Relation: rel,
				Action:   "removed",
			})
		}
	}

	// Relações adicionadas
	for key, rel := range newRels {
		if _, ok := oldRels[key]; !ok {
			diff.ChangedRelations = append(diff.ChangedRelations, RelationChange{
				Relation: rel,
				Action:   "added",
			})
		}
	}

	return diff
}
