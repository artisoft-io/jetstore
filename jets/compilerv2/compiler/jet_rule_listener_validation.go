package compiler

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// This file contains the validation logic

// collectVarResourcesFromExpr recursively collects variable resource keys from an ExpressionNode.
func (s *JetRuleListener) collectVarResourcesFromExpr(expr *rete.ExpressionNode, varSet map[int]bool) {
	if expr == nil {
		return
	}
	// ExpressionNode can be of Type "binary", "unary", "identifier"
	// For "binary", recurse on Lhs and Rhs
	// For "unary", recurse on Arg
	// For "identifier", check if it's a variable and add to varSet
	if expr.Type == "identifier" {
		res := s.Resource(expr.Value)
		if res != nil && res.Type == "var" {
			varSet[expr.Value] = true
		}
	}
	if expr.Lhs != nil {
		s.collectVarResourcesFromExpr(expr.Lhs, varSet)
	}
	if expr.Rhs != nil {
		s.collectVarResourcesFromExpr(expr.Rhs, varSet)
	}
	if expr.Arg != nil {
		s.collectVarResourcesFromExpr(expr.Arg, varSet)
	}
}

// ValidateRuleTerm validates a RuleTerm:
// Must have at least one of Type "var" among subject, predicate, object
func (s *JetRuleListener) ValidateRuleTerm(term *rete.RuleTerm) {
	if term.Type == "antecedent" {
		// Must have at least one of Type "var" among subject, predicate, object
		if s.Resource(term.SubjectKey).Type != "var" &&
			s.Resource(term.PredicateKey).Type != "var" &&
			s.Resource(term.ObjectKey).Type != "var" {
			fmt.Fprintf(s.errorLog,
				"** error: antecedent must have at least one of subject, predicate, object as variable: (%s, %s, %s)\n",
				s.Resource(term.SubjectKey).SKey(),
				s.Resource(term.PredicateKey).SKey(),
				s.Resource(term.ObjectKey).SKey())
		}
	}
}

// ValidateJetruleNode validates a JetruleNode:
// - All RuleTerm in Antecedents must have at least one of subject, predicate, object as variable
// - All ResourceNode of Type "?var" in a filter expression must appear in the antecedent having
//   the filter or in previous antecedents.
// - All ResourceNode of Type "?var" in a negated antecedent must appear in the previous antecedents.
// - All ResourceNode of Type "?var" in the Consequents must appear in the Antecedents
//
// Returns true if the rule is valid, false otherwise
func (s *JetRuleListener) ValidateJetruleNode(rule *rete.JetruleNode) bool {
	isValid := true
	// Build a set of visited variables (aka binded variable)
	visitedVarSet := make(map[string]bool)
	for i := range rule.Antecedents {
		//** check for IsNot first before adding current var to varSet (rename to bindedVars)
		// - All RuleTerm in Antecedents must have at least one of subject, predicate, object as variable
		hasVar := false
		r := s.Resource(rule.Antecedents[i].SubjectKey)
		if r.Type == "var" {
			hasVar = true
			if rule.Antecedents[i].IsNot {
				if !visitedVarSet[r.Id] {
					fmt.Fprintf(s.errorLog,
						"** error: antecedent subject variable %s not found in previous antecedents for a negated term\n", r.SKey())
					isValid = false
				}
			}	else {
				visitedVarSet[r.Id] = true
			}
		}
		r = s.Resource(rule.Antecedents[i].PredicateKey)
		if r.Type == "var" {
			hasVar = true
			if rule.Antecedents[i].IsNot {
				if !visitedVarSet[r.Id] {
					fmt.Fprintf(s.errorLog,
						"** error: antecedent predicate variable %s not found in previous antecedents for a negated term\n", r.SKey())
					isValid = false
				}
			}	else {
				visitedVarSet[r.Id] = true
			}
		}
		r = s.Resource(rule.Antecedents[i].ObjectKey)
		if r.Type == "var" {
			hasVar = true
			if rule.Antecedents[i].IsNot {
				if !visitedVarSet[r.Id] {
					fmt.Fprintf(s.errorLog,
						"** error: antecedent object variable %s not found in previous antecedents for a negated term\n", r.SKey())
					isValid = false
				}
			}	else {
				visitedVarSet[r.Id] = true
			}
		}
		if !hasVar {
			fmt.Fprintf(s.errorLog,
				"** error: antecedent must have at least one of subject, predicate, object as variable: (%s, %s, %s)\n",
				s.Resource(rule.Antecedents[i].SubjectKey).SKey(),
				s.Resource(rule.Antecedents[i].PredicateKey).SKey(),
				s.Resource(rule.Antecedents[i].ObjectKey).SKey())
			isValid = false
		}
		// Check filter expression for variables
		expr := rule.Antecedents[i].Filter
		if expr != nil {
			// Build a set of variable resource keys from the expression
			exprVarSet := make(map[int]bool)
			s.collectVarResourcesFromExpr(expr, exprVarSet)
			// Check that all variables in the expression are in visitedVarSet
			for vKey := range exprVarSet {
				r := s.Resource(vKey)
				if !visitedVarSet[r.Id] {
					fmt.Fprintf(s.errorLog,
						"** error: antecedent filter expression variable %s not found in previous antecedents\n",	r.SKey())
					isValid = false
				}
			}
		}
	}
	// All ResourceNode of Type "?var" in the Consequents must appear in the visitedVarSet
	// Check subject, predicate, object and object expression
	for i := range rule.Consequents {
		r := s.Resource(rule.Consequents[i].SubjectKey)
		if r.Type == "var" {
			if !visitedVarSet[r.Id] {
				fmt.Fprintf(s.errorLog,
					"** error: consequent subject variable %s not found in antecedents\n", r.SKey())
				isValid = false
			}
		}
		r = s.Resource(rule.Consequents[i].PredicateKey)
		if r.Type == "var" {
			if !visitedVarSet[r.Id] {
				fmt.Fprintf(s.errorLog,
					"** error: consequent predicate variable %s not found in antecedents\n", r.SKey())
				isValid = false
			}
		}
		o := s.Resource(rule.Consequents[i].ObjectKey)
		if o != nil && o.Type == "var" {
			if !visitedVarSet[o.Id] {
				fmt.Fprintf(s.errorLog,
					"** error: consequent object variable %s not found in antecedents\n", o.SKey())
				isValid = false
			}
		}
		objExpr := rule.Consequents[i].ObjectExpr
		if objExpr != nil {
			// Build a set of variables resource keys from the expression
			exprVarSet := make(map[int]bool)
			s.collectVarResourcesFromExpr(objExpr, exprVarSet)
			// Check that all variables in the expression are in visitedVarSet
			for vKey := range exprVarSet {
				r := s.Resource(vKey)
				if !visitedVarSet[r.Id] {
					fmt.Fprintf(s.errorLog,
						"** error: consequent object expression variable %s not found in antecedents\n", r.SKey())
					isValid = false
				}
			}
		}
	}
	return isValid
}

// PostProcessJetruleNode performs post-processing on a JetruleNode:
//   - Add rule Label and NormalizedLabel
//
// see makeRuleLabel function
func (s *JetRuleListener) PostProcessJetruleNode(rule *rete.JetruleNode) {
	// Add rule Label and NormalizedLabel
	rule.Label = s.makeRuleLabel(rule, false)
	rule.NormalizedLabel = s.makeRuleLabel(rule, true)
}

// Perform post-processing on rule properties:
// if property "o" (optimization) is "false", "f", "0" (case insensitive) then set property "optimization" to "false" (otherwise it's true)
// if property "s" (salience) is an integer, set property "salience" to that integer, default salience is 100
func (s *JetRuleListener) PostProcessJetruleProperties(rule *rete.JetruleNode) {
	rule.Optimization = true // default optimization is true
	v := strings.ToUpper(rule.Properties["o"])
	if v == "FALSE" || v == "F" || v == "0" {
		rule.Optimization = false
	}
	rule.Salience = 100 // default salience
	v = rule.Properties["s"]
	if len(v) > 0 {
		salience, err := strconv.Atoi(v)
		if err != nil {
			fmt.Fprintf(s.errorLog,
				"** error: invalid salience value '%s' in rule %s, must be an integer\n",
				v, rule.Name)
		} else {
			rule.Salience = salience
		}
	}
}

// Make rule label string as follows:
// [ruleName, prop1=val1, prop2=val2]: (subj1 pred1 obj1).(subj2 pred2 obj2) -> (subj3 pred3 obj3).(subj4 pred4 obj4);
// If normalize is true, variable resources are represented by their Id instead of their Value
// Example where the Id ?x1 is used:
// [MyRule, o=true, s=10]: (?x1 rdf:type ex:Person).not(?x1 ex:hasAge ?age) -> (?x1 ex:isAdult true);
func (l *JetRuleListener) makeRuleLabel(rule *rete.JetruleNode, normalize bool) string {
	label := &strings.Builder{}
	label.WriteString(fmt.Sprintf("[%s", rule.Name))
	for k, v := range rule.Properties {
		fmt.Fprintf(label, ", %s=%s", k, v)
	}
	label.WriteString("]: ")
	// Antecedents
	for i := range rule.Antecedents {
		if i > 0 {
			label.WriteString(".")
		}
		// Use a separate string builder for the RuleTerm so we can assign the
		// normalized label to the RuleTerm's NormalizedLabel field
		ruleTermLabel := &strings.Builder{}
		a := rule.Antecedents[i]
		if a.IsNot {
			ruleTermLabel.WriteString("not")
		}
		fmt.Fprintf(ruleTermLabel, "(%s %s %s)",
			l.makeResourceLabel(l.Resource(a.SubjectKey), normalize),
			l.makeResourceLabel(l.Resource(a.PredicateKey), normalize),
			l.makeResourceLabel(l.Resource(a.ObjectKey), normalize))
		if a.Filter != nil {
			ruleTermLabel.WriteString(".[")
			l.makeExpressionLabel(a.Filter, ruleTermLabel, normalize)
			ruleTermLabel.WriteString("]")
		}
		label.WriteString(ruleTermLabel.String())
		if normalize {
			a.NormalizedLabel = ruleTermLabel.String()
		}
	}
	label.WriteString(" -> ")
	// Consequents
	for i := range rule.Consequents {
		if i > 0 {
			label.WriteString(".")
		}
		ruleTermLabel := &strings.Builder{}
		c := rule.Consequents[i]
		fmt.Fprintf(ruleTermLabel, "(%s %s ",
			l.makeResourceLabel(l.Resource(c.SubjectKey), normalize),
			l.makeResourceLabel(l.Resource(c.PredicateKey), normalize))
		if c.ObjectExpr != nil {
			l.makeExpressionLabel(c.ObjectExpr, ruleTermLabel, normalize)
		} else {
			ruleTermLabel.WriteString(l.makeResourceLabel(l.Resource(c.ObjectKey), normalize))
		}
		ruleTermLabel.WriteString(")")
		label.WriteString(ruleTermLabel.String())
		if normalize {
			c.NormalizedLabel = ruleTermLabel.String()
		}
	}
	label.WriteString(";")
	return label.String()
}

func (l *JetRuleListener) makeResourceLabel(rule *rete.ResourceNode, normalize bool) string {
	if rule == nil {
		return "nil"
	}
	txt := rule.Value
	switch rule.Type {
	case "var", "resource", "volatile_resource":
		if normalize {
			txt = rule.Id
		}
		return txt
	default:
		return fmt.Sprintf("%s(%s)", rule.Type, txt)
	}
}

func (l *JetRuleListener) makeExpressionLabel(expr *rete.ExpressionNode, buf *strings.Builder, normalize bool) {
	if expr == nil {
		return
	}
	// Recursively build the expression label
	switch expr.Type {
	case "binary":
		buf.WriteString("(")
		l.makeExpressionLabel(expr.Lhs, buf, normalize)
		fmt.Fprintf(buf, " %s ", expr.Op)
		l.makeExpressionLabel(expr.Rhs, buf, normalize)
		buf.WriteString(")")

	case "unary":
		fmt.Fprintf(buf, "%s(", expr.Op)
		l.makeExpressionLabel(expr.Arg, buf, normalize)
		buf.WriteString(")")

	case "identifier":
		res := l.Resource(expr.Value)
		if res != nil {
			buf.WriteString(l.makeResourceLabel(res, normalize))
		} else {
			buf.WriteString("null")
		}
	}
}

// PostProcessJetruleModel performs post-processing and validation on the Jetrule model
// - PostProcessClasses: process class inheritance and create rules for class inheritance
// - Create Table for classes having asTable = true
// - Generate the Rete network from the rules
func (l *JetRuleListener) PostProcessJetruleModel() {
	if l.trace {
		fmt.Fprint(l.parseLog, "** entering PostProcessJetruleModel\n")
	}
	// Perform post-processing and validation on the Jetrule model
	l.PostProcessClasses()

	// Create Table for classes having asTable = true
	for i := range l.jetRuleModel.Classes {
		class := l.jetRuleModel.Classes[i]
		if class.AsTable {
			l.MakeTableFromClass(class)
		}
	}
	// Generate the Rete network from the rules
	l.BuildReteNetwork()
}

// IsValidIdentifier checks if a string is a valid identifier
// A valid identifier starts with a letter, followed by letters, digits, semicolons, or underscores
func (l *JetRuleListener) IsValidIdentifier(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !unicode.IsLetter(r) {
				return false
			}
		} else {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != ':' {
				return false
			}
		}
	}
	return true
}

func (l *JetRuleListener) PostProcessClasses() {
	// Add base class owl:Thing
	l.classesByName["owl:Thing"] = &rete.ClassNode{
		Type:        "class",
		Name:        "owl:Thing",
		BaseClasses: []string{},
		SubClasses:  []string{},
	}
	// Build classesByName map
	for i := range l.jetRuleModel.Classes {
		class := l.jetRuleModel.Classes[i]
		l.classesByName[class.Name] = class
	}
	// Add subClasses to parent Classes
	for i := range l.jetRuleModel.Classes {
		class := l.jetRuleModel.Classes[i]
		for _, baseClassName := range class.BaseClasses {
			if baseClass, exists := l.classesByName[baseClassName]; exists {
				baseClass.SubClasses = append(baseClass.SubClasses, class.Name)
			} else {
				fmt.Fprintf(l.errorLog, "** error: base class %s not found for class %s\n", baseClassName, class.Name)
			}
		}
	}
	// visit classes and create rules for class inheritance axioms
	for className, class := range l.classesByName {
		// Create a single rule to infer all base classes of the class:
		// (?x1 rdf:type <class>) -> (?x1 rdf:type <baseClass1>).(?x1 rdf:type <baseClass2>)...;
		// Rule name: ClassInh_<class>_<baseClass>
		// Antecedent: ?x1 rdf:type <class>
		// Consequents: (?x1 rdf:type <baseClass1>).(?x1 rdf:type <baseClass2>)...
		// Properties: none
		// Label: [ClassInh_<class>_<baseClass>]: (?x1 rdf:type <class>) -> (?x1 rdf:type <baseClass1>)...;
		// NormalizedLabel: [ClassInh_<class>_<baseClass>]: (?x1 rdf:type <class>) -> (?x1 rdf:type <baseClass1>)...;
		// Note: ?x1 is the variable name and also it's normalized name
		// Note: All classes have at least one base class owl:Thing, except owl:Thing itself
		if className == "owl:Thing" {
			continue
		}
		if len(class.BaseClasses) == 0 {
			fmt.Fprintf(l.errorLog, "** error: class %s has no base classes, should at least have owl:Thing\n", className)
			continue
		}
		// Create a rule for the class inheritance
		l.currentRuleVarByValue = make(map[string]*rete.ResourceNode)
		name := fmt.Sprintf("ClassInh_%s", className)
		rule := &rete.JetruleNode{
			Name:       name,
			Properties: map[string]string{},
			Antecedents: []*rete.RuleTerm{{
				Type:         "antecedent",
				SubjectKey:   l.AddV("?x1").Key,
				PredicateKey: l.AddR("rdf:type").Key,
				ObjectKey:    l.AddR(className).Key,
			}},
		}
		for _, baseClassName := range class.BaseClasses {
			if _, exists := l.classesByName[baseClassName]; exists {
				rule.Consequents = append(rule.Consequents, &rete.RuleTerm{
					Type:         "consequent",
					SubjectKey:   l.AddV("?x1").Key,
					PredicateKey: l.AddR("rdf:type").Key,
					ObjectKey:    l.AddR(baseClassName).Key,
				})
			} else {
				fmt.Fprintf(l.errorLog, "** error: base class %s not found for class %s\n", baseClassName, className)
			}
		}
		l.ValidateJetruleNode(rule)
		l.PostProcessJetruleNode(rule)
		l.jetRuleModel.Jetrules = append(l.jetRuleModel.Jetrules, rule)
	}
}

// MakeTableFromClass creates a TableNode from a ClassNode
// It visits the ClassNode and its base classes to collect data properties
// as well as the properties of all its subclasses
func (l *JetRuleListener) MakeTableFromClass(cls *rete.ClassNode) {
	if cls == nil {
		return
	}
	// Create a new table for the class
	table := &rete.TableNode{
		TableName:      cls.Name,
		ClassName:      cls.Name,
		SourceFileName: cls.SourceFileName,
	}

	// The columns of the table include the data properties of:
	// - the class itself
	// - all its base classes, and the base classes of their base classes (recursively upwards)
	// - all its subclasses, and the subclasses of their subclasses (recursively downwards)
	// Use a map to avoid duplicates
	store := make(map[string]*rete.TableColumnNode)
	visitedClasses := make(map[string]bool)
	l.visitClass(true, store, visitedClasses, cls)
	l.visitClass(false, store, visitedClasses, cls)
	visitedClasses[cls.Name] = true

	// Convert the map to a slice of TableColumnNode and assign it to the table
	for _, col := range store {
		table.Columns = append(table.Columns, *col)
	}
	// Sort table columns by ColumnName
	sort.Slice(table.Columns, func(i, j int) bool {
		return table.Columns[i].ColumnName < table.Columns[j].ColumnName
	})

	// Add the table to the model
	l.jetRuleModel.Tables = append(l.jetRuleModel.Tables, table)
}

// Collect TableColumnNode from the properties of cls and its base classes and subclasses
func (l *JetRuleListener) visitClass(doUp bool, store map[string]*rete.TableColumnNode,
	visitedClasses map[string]bool, cls *rete.ClassNode) {
	if cls == nil || visitedClasses[cls.Name] {
		return
	}

	// Add properties of the class
	for i := range cls.DataProperties {
		if _, exists := store[cls.DataProperties[i].Name]; !exists {
			store[cls.DataProperties[i].Name] = &rete.TableColumnNode{
				Type:       cls.DataProperties[i].Type,
				ColumnName: cls.DataProperties[i].Name,
				AsArray:    cls.DataProperties[i].AsArray,
			}
		}
	}
	if doUp {
		// Visit base classes recursively
		for _, baseClass := range cls.BaseClasses {
			l.visitClass(doUp, store, visitedClasses, l.classesByName[baseClass])
			visitedClasses[baseClass] = true
		}
	} else {
		// Visit subclasses recursively
		for _, subClass := range cls.SubClasses {
			l.visitClass(doUp, store, visitedClasses, l.classesByName[subClass])
			visitedClasses[subClass] = true
		}
	}
}
