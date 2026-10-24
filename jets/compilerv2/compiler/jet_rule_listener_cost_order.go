package compiler

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/optstats"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// This file contains the cost order: the antecedents of a rule ordered by the
// rows each is estimated to add, from statistics of the graph a workspace
// commits in OptimizerStatisticsFile (rules_benchmark Phase 9, BY.1). The file's
// format is version 1 of jetstore.optimizer_statistics.
//
// Without statistics nothing here runs, and a rule is ordered by evaluateScore
// as before. With them, a rule is cost-ordered only if its rule set is in the
// file, its o property is not false, and every positive antecedent has a
// constant predicate the file counts. Any other rule keeps the score order.

// OptimizerStatisticsFile is where a workspace keeps the statistics, relative
// to the workspace's base path.
const OptimizerStatisticsFile = "jet_rules/optimizer_statistics.json"

// RuleSetStatistics is what the cost order reads of one rule set's
// statistics: per-session values, each a sum in the file divided by the rule
// set's sessions, and false where the file has no entry. *optstats.RuleSet is
// the one the compiler reads.
type RuleSetStatistics interface {
	// Has reports whether predicate p has statistics.
	Has(p string) bool
	// N is p's triples per session.
	N(p string) (float64, bool)
	// DS is p's distinct subjects per session, floored at 1.
	DS(p string) (float64, bool)
	// DO is p's distinct objects per session, floored at 1.
	DO(p string) (float64, bool)
	// SubjectCount is the triples (s, p, *) per session for a subject key s.
	SubjectCount(p, s string) (float64, bool)
	// ObjectCount is the triples (*, p, o) per session for an object key o.
	ObjectCount(p, o string) (float64, bool)
}

// costOrderReport counts, for one rule set, what the cost order did. It is
// written to the parse log only when a statistics file is present.
type costOrderReport struct {
	costOrdered int // rules ordered by the estimate
	changed     int // of those, rules whose order differs from the score order
	varPred     int // fell back: a positive antecedent's predicate is a variable
	missingPred int // fell back: a positive antecedent's predicate is not in the file
	optOff      int // rules with o=false
	trace       []string
}

// termKey renders a rule constant as the statistics file keys it: its form in
// the engine's graph (BX.1 rule 5), by optstats.TermKey, given the resource's
// Value, which is what the engine names it by, never its Id. The engine makes a
// node from the row the compiler writes in workspace.db's resources table
// (ReteMetaStoreFactory::read_resources_cb). A volatile resource's Value has
// the _0: prefix when declared and not when inlined; TermKey adds it only when
// it is absent, so both give the engine's name. Any kind TermKey does not count
// gives false.
func termKey(r *rete.ResourceNode) (string, bool) {
	if r == nil {
		return "", false
	}
	return optstats.TermKey(r.Type, r.Value)
}

// loadOptimizerStatistics reads the workspace's statistics file, when it has
// one, for the rule set being compiled. A file that is not version 1 of the
// format, or is malformed, refuses the compile with an error naming it. Unless
// SetOptimizerStatistics was called first.
func (c *Compiler) loadOptimizerStatistics() error {
	l := c.listener
	if os.Getenv("JETS_COST_ORDER_TRACE") != "" {
		l.costTrace = true
	}
	if l.statisticsFile != "" {
		return nil
	}
	path, err := utils.ConfineFilePath(l.basePath, OptimizerStatisticsFile)
	if err != nil {
		return fmt.Errorf("while resolving the optimizer statistics file: %w", err)
	}
	f, err := optstats.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("while reading the optimizer statistics: %w", err)
	}
	if rs := f.ForRuleSet(l.mainRuleFileName); rs != nil {
		c.SetOptimizerStatistics(OptimizerStatisticsFile, rs)
	} else {
		c.SetOptimizerStatistics(OptimizerStatisticsFile, nil)
	}
	return nil
}

// SetOptimizerStatistics makes the compile order rules by the cost estimate,
// in place of the file Compile would read. file names the statistics file in the report; st is the rule set's
// statistics, nil when the file does not have the rule set, which compiles
// with the score order and says so in the report.
func (c *Compiler) SetOptimizerStatistics(file string, st RuleSetStatistics) {
	c.listener.statisticsFile = file
	c.listener.statistics = st
}

// SetCostOrderTrace lists, in the parse log, each cost-ordered rule's
// antecedents in their score order and in their cost order. It changes nothing
// in the compiled model.
func (c *Compiler) SetCostOrderTrace(on bool) {
	c.listener.costTrace = on
}

// costEstimator estimates, for one rule, the rows a positive antecedent adds
// per row of the antecedents placed before it (BX.1 rule 8).
type costEstimator struct {
	s     *JetRuleListener
	st    RuleSetStatistics
	preds map[*rete.RuleTerm]predicateStats
}

type predicateStats struct {
	key       string
	N, DS, DO float64
}

// newCostEstimator returns nil when the rule is not cost-ordered, and counts
// the reason in the report.
func (s *JetRuleListener) newCostEstimator(antecedents []*rete.RuleTerm) *costEstimator {
	e := &costEstimator{s: s, st: s.statistics, preds: make(map[*rete.RuleTerm]predicateStats)}
	for _, a := range antecedents {
		if a.IsNot {
			continue
		}
		p := s.Resource(a.PredicateKey)
		if p == nil || p.Type == "var" {
			s.costReport.varPred++
			return nil
		}
		key, ok := termKey(p)
		if !ok || !e.st.Has(key) {
			s.costReport.missingPred++
			return nil
		}
		ps := predicateStats{key: key}
		ps.N, _ = e.st.N(key)
		ps.DS, _ = e.st.DS(key)
		ps.DO, _ = e.st.DO(key)
		e.preds[a] = ps
	}
	return e
}

// rows is the estimate for a positive antecedent, given the variables bound by
// the antecedents placed before it. Bound means a constant or a bound variable.
func (e *costEstimator) rows(a *rete.RuleTerm, bound map[string]bool) float64 {
	ps := e.preds[a]
	sub := e.s.Resource(a.SubjectKey)
	obj := e.s.Resource(a.ObjectKey)
	subConst, objConst := sub.Type != "var", obj.Type != "var"
	subBound := subConst || bound[sub.Id]
	objBound := objConst || bound[obj.Id]
	switch {
	case subBound && objBound:
		if ps.N == 0 {
			return 0
		}
		return min(1, ps.N/(ps.DS*ps.DO))
	case subBound:
		if subConst {
			if key, ok := termKey(sub); ok {
				if c, ok := e.st.SubjectCount(ps.key, key); ok {
					return c
				}
			}
		}
		return ps.N / ps.DS
	case objBound:
		if objConst {
			if key, ok := termKey(obj); ok {
				if c, ok := e.st.ObjectCount(ps.key, key); ok {
					return c
				}
			}
		}
		return ps.N / ps.DO
	}
	return ps.N
}

// rowsHeld is the sum, over an order's positive antecedents, of the rows
// estimated after each: the proxy for the beta rows the rule's chain holds.
func (e *costEstimator) rowsHeld(order []*rete.RuleTerm) float64 {
	rows, total := 1.0, 0.0
	bound := make(map[string]bool)
	for _, a := range order {
		if a.IsNot {
			continue
		}
		rows *= e.rows(a, bound)
		total += rows
		e.s.collectVars(bound, a)
	}
	return total
}

// orderByCost is the greedy cost order: at each step a negation whose
// variables are all bound is placed at once, the first of them as written;
// otherwise the positive antecedent with the smallest estimate goes next, the
// first written among equals. Filters are re-placed afterwards, as for the
// score order.
func (s *JetRuleListener) orderByCost(antecedents []*rete.RuleTerm, e *costEstimator) []*rete.RuleTerm {
	left := slices.Clone(antecedents)
	ordered := make([]*rete.RuleTerm, 0, len(left))
	bound := make(map[string]bool)
	for len(left) > 0 {
		pick := -1
		for i, a := range left {
			if a.IsNot && s.allVarsBinded(bound, a) {
				pick = i
				break
			}
		}
		if pick < 0 {
			best := math.Inf(1)
			for i, a := range left {
				if a.IsNot {
					continue
				}
				if r := e.rows(a, bound); pick < 0 || r < best {
					pick, best = i, r
				}
			}
		}
		if pick < 0 {
			// Only negated terms are left, each reading a variable no term binds.
			// The rule is valid, so this does not happen; keep the authored order
			s.LogError("Internal error: no antecedent can be placed during rule optimization")
			pick = 0
		}
		a := left[pick]
		ordered = append(ordered, a)
		if !a.IsNot {
			s.collectVars(bound, a)
		}
		left = slices.Delete(left, pick, pick+1)
	}
	return ordered
}

// orderLabel writes an order as the antecedents' authored positions, T for a
// positive antecedent and N for a negation, as rules_benchmark's feasibility
// study printed them: positions only, never a value.
func orderLabel(order []*rete.RuleTerm, authored map[*rete.RuleTerm]int) string {
	parts := make([]string, len(order))
	for i, a := range order {
		k := "T"
		if a.IsNot {
			k = "N"
		}
		parts[i] = fmt.Sprintf("%s%d", k, authored[a])
	}
	return strings.Join(parts, " ")
}

// validateOrderedRule checks a reordered rule as validation checks the
// authored one: every variable of a negated antecedent, and of a filter, is
// bound by a positive antecedent placed before it (a filter's own antecedent
// included). Validation runs before the optimizer, so without this an order
// that placed a negation too early compiled with a clean exit (I-232).
func (s *JetRuleListener) validateOrderedRule(rule *rete.JetruleNode) bool {
	isValid := true
	bound := make(map[string]bool)
	for i, a := range rule.Antecedents {
		if a.IsNot {
			vars := make(map[string]bool)
			s.collectVars(vars, a)
			for _, v := range slices.Sorted(maps.Keys(vars)) {
				if !bound[v] {
					s.logErrorf("** error: rule %s (%s): after ordering, negated antecedent %d reads variable %s before an antecedent binds it\n",
						rule.Name, rule.SourceFileName, i, s.varLabel(rule, v))
					isValid = false
				}
			}
		} else {
			s.collectVars(bound, a)
		}
		if a.Filter != nil {
			exprVarSet := make(map[int]bool)
			s.collectVarResourcesFromExpr(a.Filter, exprVarSet)
			for _, k := range slices.Sorted(maps.Keys(exprVarSet)) {
				if r := s.Resource(k); r != nil && !bound[r.Id] {
					s.logErrorf("** error: rule %s (%s): after ordering, the filter of antecedent %d reads variable %s before an antecedent binds it\n",
						rule.Name, rule.SourceFileName, i, r.Value)
					isValid = false
				}
			}
		}
	}
	return isValid
}

// varLabel names a variable of the rule by its authored name.
func (s *JetRuleListener) varLabel(rule *rete.JetruleNode, id string) string {
	for _, a := range rule.Antecedents {
		for _, k := range []int{a.SubjectKey, a.PredicateKey, a.ObjectKey} {
			if r := s.Resource(k); r != nil && r.Type == "var" && r.Id == id {
				return r.Value
			}
		}
	}
	return id
}

// writeCostOrderReport writes the rule set's counts to the parse log, when a
// statistics file is present.
func (s *JetRuleListener) writeCostOrderReport() {
	if s.statisticsFile == "" {
		return
	}
	if s.statistics == nil {
		fmt.Fprintf(s.parseLog, "Cost order: rule set %s is not in %s; rules ordered by score\n",
			s.mainRuleFileName, s.statisticsFile)
		return
	}
	r := &s.costReport
	fmt.Fprintf(s.parseLog,
		"Cost order: rule set %s, statistics %s: %d rules cost-ordered (%d differ from the score order); "+
			"fell back to the score order: %d with a variable predicate, %d with a predicate not in the file; %d with o=false\n",
		s.mainRuleFileName, s.statisticsFile, r.costOrdered, r.changed, r.varPred, r.missingPred, r.optOff)
	for _, line := range r.trace {
		s.parseLog.WriteString(line)
		s.parseLog.WriteString("\n")
	}
}
