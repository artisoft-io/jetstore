package symbolic

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// FromModel is the inverse direction: a compiled model's rules in symbolic
// form. Every key site — subject_key, predicate_key, object_key and an
// identifier's value inside an expression — takes the same substitution, the
// resource the key resolves to, written as its id where it has one and as a
// {type, value} literal where it does not.
//
// It is what lets the form be checked against the corpus rather than against
// its author's expectations: compile a workspace, take its rules to symbolic
// form, render them and compile them again, and the two must agree rule for
// rule (TestCorpusRoundTrip).
//
// A variable is written by its authored name where the model still carries it
// and by its normalised id (?x01) where the rete build has replaced it with a
// binding that carries only the id. Either is the same variable to the
// compiler, which numbers variables by first appearance.
//
// Only the rules whose source file is in sourceFiles are taken; an empty set
// takes them all. The rules the compiler generates for class inheritance
// (ci_<class>, property i=true; PostProcessClasses) are never taken: nobody
// wrote them, and the declarations they come from regenerate them. A rule's
// own outputs — labels, vertexes, validity — are not
// carried, because the compiler computes them.
func FromModel(m *rete.JetruleModel, sourceFiles map[string]bool) (*RuleFile, error) {
	byKey := make(map[int]*rete.ResourceNode, len(m.Resources))
	for _, r := range m.Resources {
		byKey[r.Key] = r
	}
	f := &RuleFile{}
	for _, jr := range m.Jetrules {
		if len(sourceFiles) > 0 && !sourceFiles[jr.SourceFileName] {
			continue
		}
		if IsGenerated(jr) {
			continue
		}
		rule, err := ruleFromModel(jr, byKey)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", jr.Name, err)
		}
		f.Rules = append(f.Rules, *rule)
	}
	return f, nil
}

// IsGenerated says whether the compiler wrote a rule rather than an author:
// the class-inheritance rules carry i=true (PostProcessClasses).
func IsGenerated(jr *rete.JetruleNode) bool {
	return jr.Properties["i"] == "true"
}

func ruleFromModel(jr *rete.JetruleNode, byKey map[int]*rete.ResourceNode) (*Rule, error) {
	rule := &Rule{Name: jr.Name}
	for k, raw := range jr.Properties {
		switch k {
		case "s":
			s, err := strconv.Atoi(raw)
			if err != nil {
				return nil, fmt.Errorf("salience %q: %w", raw, err)
			}
			rule.Salience = &s
		case "o":
			// The compiler's own reading (PostProcessJetruleProperties).
			o := true
			switch strings.ToUpper(raw) {
			case "FALSE", "F", "0":
				o = false
			}
			rule.Optimization = &o
		default:
			if rule.Properties == nil {
				rule.Properties = map[string]PropertyValue{}
			}
			rule.Properties[k] = propertyFromRaw(raw)
		}
	}
	for i, a := range jr.Antecedents {
		s, err := termFromKey(a.SubjectKey, byKey)
		if err != nil {
			return nil, fmt.Errorf("antecedent %d subject: %w", i, err)
		}
		p, err := termFromKey(a.PredicateKey, byKey)
		if err != nil {
			return nil, fmt.Errorf("antecedent %d predicate: %w", i, err)
		}
		o, err := termFromKey(a.ObjectKey, byKey)
		if err != nil {
			return nil, fmt.Errorf("antecedent %d object: %w", i, err)
		}
		ant := Antecedent{Not: a.IsNot, Subject: s, Predicate: p, Object: o}
		if a.Filter != nil {
			ant.Filter, err = exprFromModel(a.Filter, byKey)
			if err != nil {
				return nil, fmt.Errorf("antecedent %d filter: %w", i, err)
			}
		}
		rule.Antecedents = append(rule.Antecedents, ant)
	}
	for i, c := range jr.Consequents {
		s, err := termFromKey(c.SubjectKey, byKey)
		if err != nil {
			return nil, fmt.Errorf("consequent %d subject: %w", i, err)
		}
		p, err := termFromKey(c.PredicateKey, byKey)
		if err != nil {
			return nil, fmt.Errorf("consequent %d predicate: %w", i, err)
		}
		con := Consequent{Subject: s, Predicate: p}
		if c.ObjectExpr != nil {
			e, err := exprFromModel(c.ObjectExpr, byKey)
			if err != nil {
				return nil, fmt.Errorf("consequent %d object: %w", i, err)
			}
			con.Object = *e
		} else {
			o, err := termFromKey(c.ObjectKey, byKey)
			if err != nil {
				return nil, fmt.Errorf("consequent %d object: %w", i, err)
			}
			con.Object = Expr{Term: &o}
		}
		rule.Consequents = append(rule.Consequents, con)
	}
	return rule, nil
}

func termFromKey(key int, byKey map[int]*rete.ResourceNode) (Term, error) {
	r, ok := byKey[key]
	if !ok {
		return Term{}, fmt.Errorf("no resource has key %d", key)
	}
	switch r.Type {
	case "var":
		if r.Value != "" {
			return Term{ID: r.Value}, nil
		}
		return Term{ID: r.Id}, nil
	case "resource":
		if r.Id != "" {
			return Term{ID: r.Id}, nil
		}
		return Term{ID: r.Value}, nil
	case "volatile_resource":
		// A declared one keeps the _0: prefix in its value and is named by
		// its id. One the compiler created from an undeclared _0:name has the
		// bare name as both id and value (ParseObjectAtom), so the prefix is
		// what says it is volatile and has to be written back.
		if strings.HasPrefix(r.Value, "_0:") {
			return Term{ID: r.Id}, nil
		}
		return Term{ID: "_0:" + r.Id}, nil
	default:
		if r.Id != "" && !r.Inline {
			// A declared literal, referenced by the name it was declared with.
			return Term{ID: r.Id}, nil
		}
		return Term{Literal: &Literal{Type: r.Type, Value: r.Value}}, nil
	}
}

func exprFromModel(e *rete.ExpressionNode, byKey map[int]*rete.ResourceNode) (*Expr, error) {
	switch e.Type {
	case "identifier":
		t, err := termFromKey(e.Value, byKey)
		if err != nil {
			return nil, err
		}
		return &Expr{Term: &t}, nil
	case "unary":
		arg, err := exprFromModel(e.Arg, byKey)
		if err != nil {
			return nil, err
		}
		return &Expr{Op: e.Op, Arg: arg}, nil
	case "binary":
		lhs, err := exprFromModel(e.Lhs, byKey)
		if err != nil {
			return nil, err
		}
		rhs, err := exprFromModel(e.Rhs, byKey)
		if err != nil {
			return nil, err
		}
		return &Expr{Op: e.Op, Lhs: lhs, Rhs: rhs}, nil
	default:
		return nil, fmt.Errorf("expression type %q", e.Type)
	}
}

// propertyFromRaw reads a property value the way the grammar wrote it
// (propertyValue: a STRING, true, false or an integer), which the model
// carries as raw source text.
func propertyFromRaw(raw string) PropertyValue {
	switch {
	case len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"':
		return PropertyValue{V: raw[1 : len(raw)-1]}
	case raw == "true":
		return PropertyValue{V: true}
	case raw == "false":
		return PropertyValue{V: false}
	}
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return PropertyValue{V: i}
	}
	return PropertyValue{V: raw}
}
