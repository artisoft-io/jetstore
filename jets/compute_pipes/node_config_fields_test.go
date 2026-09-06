package compute_pipes

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"testing"
)

// The two literals that decide what a node sees, and the fields each deliberately
// leaves out.
//
// **This test exists because the literals omit by default.** `cpReducingConfig`
// and `cpShardingConfig` are built field by field, so a field of
// `ComputePipesConfig` that nobody lists simply does not reach the node — silently,
// and only for the operators that read it. `PromptTemplates` was missing from both
// from the day they were written: `resolveInferTemplate` looks a
// `prompt_template_name` up in `cpConfig.PromptTemplates` at build time *on the
// node*, so every named template resolved to nothing, while an inline
// `prompt_template` worked. Nothing failed until a live run used the named form.
//
// **So the guard is on the class rather than on that field.** Adding a field to
// `ComputePipesConfig` now fails here until someone says, in the map below,
// whether a node needs it. That converts a silent omission into a decision.
var nodeConfigOmissions = map[string]map[string]string{
	"actions_start_reducing_cp.go": {
		"Comment":                "free text for the reader; nothing reads it at run time",
		"ReducingPipesConfig":    "the step's pipes are resolved into PipesConfig before this literal is built",
		"ConditionalPipesConfig": "same: the node is given its one step, not the list to choose from",
	},
	"actions_start_sharding_cp.go": {
		"Comment":                "free text for the reader; nothing reads it at run time",
		"ReducingPipesConfig":    "the step's pipes are resolved into PipesConfig before this literal is built",
		"ConditionalPipesConfig": "same: the node is given its one step, not the list to choose from",
	},
}

// literalFields returns the field names assigned in the first `ComputePipesConfig`
// composite literal in the file.
func literalFields(t *testing.T, file string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}
	found := map[string]bool{}
	ast.Inspect(parsed, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		ident, ok := lit.Type.(*ast.Ident)
		if !ok || ident.Name != "ComputePipesConfig" {
			return true
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok {
				found[key.Name] = true
			}
		}
		return false
	})
	if len(found) == 0 {
		t.Fatalf("%s: found no ComputePipesConfig literal — has the node config moved?", file)
	}
	return found
}

func TestNodeConfigCarriesEveryFieldOrSaysWhyNot(t *testing.T) {
	all := reflect.TypeOf(ComputePipesConfig{})
	for file, omitted := range nodeConfigOmissions {
		t.Run(file, func(t *testing.T) {
			assigned := literalFields(t, file)
			var missing []string
			for i := 0; i < all.NumField(); i++ {
				name := all.Field(i).Name
				if assigned[name] {
					if reason, ok := omitted[name]; ok {
						t.Errorf("%s is both assigned and listed as omitted (%q) — drop one", name, reason)
					}
					continue
				}
				if _, ok := omitted[name]; !ok {
					missing = append(missing, name)
				}
			}
			sort.Strings(missing)
			if len(missing) > 0 {
				t.Errorf(
					"%s does not give the node %v, and does not say why not.\n"+
						"A node sees this literal and nothing else, so an unlisted field reaches no\n"+
						"operator. Either assign it or add it to nodeConfigOmissions with a reason.",
					file, missing)
			}
			for name := range omitted {
				if _, ok := all.FieldByName(name); !ok {
					t.Errorf("nodeConfigOmissions names %s, which ComputePipesConfig no longer has", name)
				}
			}
		})
	}
}
