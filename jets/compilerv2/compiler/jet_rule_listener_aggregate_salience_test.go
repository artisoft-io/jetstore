package compiler

import (
	"errors"
	"strings"
	"testing"
)

// The cases are the shapes found in production rules: a maximum over values a
// sibling rule records at the same salience (walrus_ws R96_DaysOfSupply11,
// ReversalClaim10), a count over links (ET_ValidateRecord01), and a config
// resource naming the aggregated properties (Adherence21).

const aggregateHeader = `
resource rdfType = "rdf:type";
resource ex:Item = "ex:Item";
resource ex:State = "ex:State";
resource ex:value = "ex:value";
resource ex:link = "ex:link";
resource ex:best = "ex:best";
resource ex:count = "ex:count";
resource ex:ready = "ex:ready";
resource ex:cfg = "ex:cfg";
resource jets:entity_property = "jets:entity_property";
resource jets:value_property = "jets:value_property";
resource jets:iState = "jets:iState";
resource jets:completed = "jets:completed";
`

// producer writes ex:value at the given salience header suffix.
func producerRule(props string) string {
	return "[Producer" + props + "]:\n  (?x rdfType ex:Item).\n  (?x ex:link ?y)\n->\n  (?x ex:value ?y).\n  (?x ex:ready true)\n;\n"
}

func aggregateRule(props string, extraAntecedent string) string {
	return "[Aggregate" + props + "]:\n  (?x rdfType ex:Item).\n" + extraAntecedent + "  (?x ex:ready true)\n->\n  (?x ex:best (?x max_of ex:value))\n;\n"
}

func compileAggregateCase(t *testing.T, rules string) (errs []string) {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "main.jr", aggregateHeader+rules)
	c := NewCompiler(dir, "main.jr", false, false, true)
	err := c.Compile()
	if err == nil {
		return nil
	}
	var cerr *CompilationError
	if !errors.As(err, &cerr) {
		t.Fatalf("expected a *CompilationError, got %T: %v", err, err)
	}
	for _, d := range cerr.Errors() {
		errs = append(errs, d.Message)
	}
	return errs
}

func TestAggregateSalience_EqualSalienceIsAnError(t *testing.T) {
	errs := compileAggregateCase(t, producerRule("")+aggregateRule("", ""))
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %d: %v", len(errs), errs)
	}
	for _, want := range []string{"rule Aggregate", "max_of ex:value", "salience 100", "Producer (s=100, writes ex:value)", "below 100"} {
		if !strings.Contains(errs[0], want) {
			t.Errorf("error does not mention %q: %s", want, errs[0])
		}
	}
	if strings.Contains(errs[0], "warning:") {
		t.Errorf("must be an error, not a warning: %s", errs[0])
	}
}

func TestAggregateSalience_ProducerBelowAggregateIsAnError(t *testing.T) {
	// The rule 96 shape: the values arrive at s=30, the aggregate sat at 100.
	errs := compileAggregateCase(t, producerRule(", s=30")+aggregateRule("", ""))
	if len(errs) != 1 || !strings.Contains(errs[0], "Producer (s=30, writes ex:value)") || !strings.Contains(errs[0], "below 30") {
		t.Fatalf("expected one error naming the s=30 producer, got %v", errs)
	}
}

func TestAggregateSalience_AggregateBelowProducerCompiles(t *testing.T) {
	if errs := compileAggregateCase(t, producerRule("")+aggregateRule(", s=10", "")); len(errs) != 0 {
		t.Fatalf("expected a clean compile, got %v", errs)
	}
}

func TestAggregateSalience_ConfigPropertiesAreChecked(t *testing.T) {
	// A config resource names the entity and value properties; a producer of
	// either one feeds the aggregate.
	rules := `
triple(ex:cfg, jets:entity_property, ex:link);
triple(ex:cfg, jets:value_property, ex:value);
[Linker]:
  (?x rdfType ex:Item).
  (?x ex:ready true)
->
  (?x ex:link ?x)
;
[Summer, s=20]:
  (?x rdfType ex:Item).
  (?x ex:ready true)
->
  (?x ex:count (?x sum_values ex:cfg))
;
[LateValue, s=20]:
  (?x rdfType ex:Item).
  (?x ex:ready true)
->
  (?x ex:value 1)
;
`
	errs := compileAggregateCase(t, rules)
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "LateValue (s=20, writes ex:value)") || strings.Contains(errs[0], "Linker") {
		t.Errorf("expected only the s=20 value producer, not the s=100 linker: %s", errs[0])
	}
}

func TestAggregateSalience_GatedOnCompletionIsExempt(t *testing.T) {
	gate := "  (jets:iState jets:completed ?done).\n"
	if errs := compileAggregateCase(t, producerRule("")+aggregateRule("", gate)); len(errs) != 0 {
		t.Fatalf("a rule gated on jets:completed must compile, got %v", errs)
	}
}

func TestAggregateSalience_OptOutIsExempt(t *testing.T) {
	if errs := compileAggregateCase(t, producerRule("")+aggregateRule(", aggregate_check=false", "")); len(errs) != 0 {
		t.Fatalf("aggregate_check=false must exempt the rule, got %v", errs)
	}
}

func TestAggregateSalience_InputDataAndSelfNeedNothing(t *testing.T) {
	// ex:value is written by no other rule: it is input data. A rule that both
	// writes and aggregates a property is not its own producer for this check.
	rules := `
[SelfFed]:
  (?x rdfType ex:Item).
  (?x ex:link ?y)
->
  (?x ex:link ?x).
  (?x ex:count (?x size_of ex:link))
;
` + aggregateRule("", "")
	if errs := compileAggregateCase(t, rules); len(errs) != 0 {
		t.Fatalf("expected a clean compile, got %v", errs)
	}
}
