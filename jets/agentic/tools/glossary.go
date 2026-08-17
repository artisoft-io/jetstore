package tools

import (
	"fmt"
	"sort"
	"strings"
)

// Glossary renders one short paragraph per requested entity — a function,
// not a file, because sections 5.1 and 5.6 both narrow per agent and a
// static document cannot narrow (A3.1). Two description sources, one
// emitter: the sidecar for schema-first entities, the workspace triples for
// .jr-authored ones. An entity with neither is reported undocumented, not
// omitted — the gap must be visible to whoever asks. A narrowed glossary
// inlines each entity's vocabularies in full: a paragraph on Incident
// without the eleven IncidentStatus values is incomplete, not narrow — the
// taxonomy is the domain knowledge, stated here for the prompt and again in
// the schema's enum for the decode (section 5.1's deliberate twice).
func (ws *Workspace) Glossary(entities []string) (string, error) {
	var b strings.Builder
	for _, name := range entities {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		if ws.Meta != nil {
			if ent, ok := ws.Meta.Entities[name]; ok {
				ws.sidecarParagraph(&b, name, &ent)
				continue
			}
		}
		cls, declared := ws.Classes[name]
		if !declared {
			fmt.Fprintf(&b, "%s: not declared in this workspace.\n", name)
			continue
		}
		triples, err := ws.TriplesAbout(classSubjects(name, cls))
		if err != nil {
			return "", err
		}
		if len(triples) == 0 {
			fmt.Fprintf(&b, "%s: declared, no metadata authored. Properties: %s.\n",
				name, strings.Join(classPropertyNames(cls), ", "))
			continue
		}
		fmt.Fprintf(&b, "%s: documented by workspace triples.\n", name)
		for _, t := range triples {
			fmt.Fprintf(&b, "  %s %s %q\n", t.Subject, t.Predicate, t.Object)
		}
	}
	return b.String(), nil
}

func (ws *Workspace) sidecarParagraph(b *strings.Builder, name string, ent *SidecarEntity) {
	fmt.Fprintf(b, "%s: %s\n", name, ent.Description)
	vocabularies := map[string]bool{}
	for _, pname := range sortedKeys(ent.Properties) {
		prop := ent.Properties[pname]
		var notes []string
		if prop.Required {
			notes = append(notes, "required")
		}
		if prop.Key {
			notes = append(notes, "key")
		}
		if prop.Vocabulary != "" {
			notes = append(notes, "one of "+prop.Vocabulary+" below")
			vocabularies[prop.Vocabulary] = true
		}
		if prop.Entity != "" {
			notes = append(notes, "references "+prop.Entity)
		}
		if prop.DataClassification != "" {
			notes = append(notes, prop.DataClassification)
		}
		suffix := ""
		if len(notes) > 0 {
			suffix = " (" + strings.Join(notes, "; ") + ")"
		}
		typ := prop.Type
		if prop.Array {
			typ = "array of " + typ
		}
		fmt.Fprintf(b, "  - %s [%s]%s: %s\n", pname, typ, suffix, prop.Description)
	}
	for _, vname := range sortedSet(vocabularies) {
		vocab := ws.Meta.Vocabularies[vname]
		fmt.Fprintf(b, "  %s (%s): %s\n", vname, vocab.Description,
			strings.Join(vocab.Values, ", "))
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedSet(m map[string]bool) []string {
	return sortedKeys(m)
}
