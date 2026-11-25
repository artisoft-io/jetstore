package workspace

// The workspace's error codes, as the compile sees them: every rule consequent
// that raises a jets:exception, read from the compiled model, and the compile
// log's comparison of those raises with jetsapi.error_code_catalogue. The
// catalogue's rows are the workspace's, in its base init script; the check
// says what is missing or stale and never writes the table nor fails a compile.
// See jets/errorcode for how a process_errors message gets its code.

import (
	"context"
	"fmt"
	"sort"

	"github.com/artisoft-io/jetstore/jets/errorcode"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/jackc/pgx/v5/pgxpool"
)

// exceptionPredicate is the reserved resource a rule asserts to raise an
// exception.
const exceptionPredicate = "jets:exception"

// CollectExceptionRaises makes CompileWorkspace append to dst every rule
// consequent that raises a jets:exception, once per rule file, rule and message
// head, so the caller can check them against the catalogue once the base init
// that follows the compile has loaded it (CheckErrorCodeCatalogue).
func CollectExceptionRaises(dst *[]errorcode.Raise) CompileOption {
	return func(o *compileOptions) {
		o.raises = dst
	}
}

// ExceptionRaises lists the consequents of model's rules whose predicate is
// jets:exception, with the message's literal head. The head is the literal
// object, a named text constant's value, or the literals that start a '+'
// chain up to its first value. A rule that is not valid is not in the rete
// network and raises nothing, so it is left out.
func ExceptionRaises(model *rete.JetruleModel) []errorcode.Raise {
	if model == nil {
		return nil
	}
	resources := make(map[int]*rete.ResourceNode, len(model.Resources))
	exceptionKey := -1
	for _, r := range model.Resources {
		resources[r.Key] = r
		if r.Type == "resource" && r.Value == exceptionPredicate {
			exceptionKey = r.Key
		}
	}
	if exceptionKey < 0 {
		return nil
	}
	var out []errorcode.Raise
	for _, rule := range model.Jetrules {
		if rule == nil || !rule.IsValid {
			continue
		}
		for _, c := range rule.Consequents {
			if c == nil || c.PredicateKey != exceptionKey {
				continue
			}
			head, hasValue := messageHead(c, resources)
			out = append(out, errorcode.Raise{
				File:     rule.SourceFileName,
				Rule:     rule.Name,
				Head:     head,
				HasValue: hasValue,
			})
		}
	}
	return out
}

// messageHead is the literal head of a consequent's object, and whether a value
// follows it.
func messageHead(c *rete.RuleTerm, resources map[int]*rete.ResourceNode) (string, bool) {
	if c.ObjectExpr == nil {
		if r := resources[c.ObjectKey]; r != nil && r.Type == "text" {
			return r.Value, false
		}
		return "", true
	}
	var leaves []*rete.ExpressionNode
	if !concatLeaves(c.ObjectExpr, &leaves) {
		// Not a '+' chain: a function of values, with no literal head
		return "", true
	}
	var head string
	for i, leaf := range leaves {
		r := resources[leaf.Value]
		if r == nil || r.Type != "text" {
			return head, true
		}
		head += r.Value
		if i == len(leaves)-1 {
			return head, false
		}
	}
	return head, false
}

// concatLeaves appends the leaves of a '+' chain to leaves, in order, and says
// whether nd is one: identifiers joined by binary '+' only.
func concatLeaves(nd *rete.ExpressionNode, leaves *[]*rete.ExpressionNode) bool {
	switch {
	case nd == nil:
		return false
	case nd.Type == "identifier":
		*leaves = append(*leaves, nd)
		return true
	case nd.Type == "binary" && nd.Op == "+":
		return concatLeaves(nd.Lhs, leaves) && concatLeaves(nd.Rhs, leaves)
	}
	return false
}

// addRaises appends raises to dst, leaving out those already there: a rule
// file imported by several main rule files is in each of their models.
func addRaises(dst *[]errorcode.Raise, raises []errorcode.Raise) {
	seen := make(map[errorcode.Raise]bool, len(*dst))
	for _, r := range *dst {
		seen[r] = true
	}
	for _, r := range raises {
		if !seen[r] {
			seen[r] = true
			*dst = append(*dst, r)
		}
	}
}

// SortRaises orders raises by file, rule and head.
func SortRaises(raises []errorcode.Raise) {
	sort.SliceStable(raises, func(i, j int) bool {
		a, b := raises[i], raises[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Head < b.Head
	})
}

// CheckErrorCodeCatalogue compares raises with the rows of
// jetsapi.error_code_catalogue and returns what the compile log should say.
// Run it after the base init that follows a successful compile, so the table
// holds the rows the workspace's base script has just loaded.
//
// It never fails: a database not yet migrated to the table, or any other read
// error, is one line of the log, and the compile stands.
func CheckErrorCodeCatalogue(ctx context.Context, dbpool *pgxpool.Pool, raises []errorcode.Raise) string {
	if dbpool == nil {
		return fmt.Sprintf("Error code catalogue: not checked, no database (%d jets:exception raises)\n", len(raises))
	}
	rows, err := dbpool.Query(ctx, errorcode.SelectCatalogueSQL)
	if err != nil {
		return catalogueReadFailure(err, len(raises))
	}
	entries, err := errorcode.ReadCatalogue(rows)
	rows.Close()
	if err != nil {
		return catalogueReadFailure(err, len(raises))
	}
	return errorcode.Check(raises, entries, errorcode.DefaultMaxErrorCount).String()
}

func catalogueReadFailure(err error, nraises int) string {
	if errorcode.IsMissingTable(err) {
		return fmt.Sprintf("Error code catalogue: not checked, the database has no table %s yet "+
			"(it is created by the release's migration, update_db -migrateDb); %d jets:exception raises\n",
			errorcode.TableName, nraises)
	}
	return fmt.Sprintf("Error code catalogue: not checked, reading %s: %v\n", errorcode.TableName, err)
}
