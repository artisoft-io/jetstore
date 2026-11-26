package errorcode

import (
	"fmt"
	"testing"
)

// TestFixedCodesMatchTheWriters formats each message the way its writer does
// and checks it gets its fixed code, from the operator type the writer records
// and from a null one. The format strings are copied from the writers, named in
// each case; when a writer's message changes, change its case here and the
// entry in fixed.go together.
func TestFixedCodesMatchTheWriters(t *testing.T) {
	c := NewCatalogue(nil)
	cases := []struct {
		writer, op, msg, code string
	}{
		// jets/compute_pipes/jetrules_pool_worker.go, reportRuleSet
		{"reportRuleSet ExecuteRules", OperatorJetrules,
			fmt.Sprintf("ExecuteRules returned error: %v", fmt.Errorf("rete error")), CodeExecuteRulesError},
		{"reportRuleSet max loop", OperatorJetrules,
			fmt.Sprintf("MAX LOOP REACHED, maxLooping is %d", 50), CodeMaxLoopReached},
		// jets/compute_pipes/jetrules_pool_worker.go, extractSessionData via reportMultipleValues
		{"multiple text values", OperatorJetrules,
			fmt.Sprintf("property %s is not multi-value but has %d text values for subject %s, keeping all of them as an array",
				"ex:p", 2, "_:b12"), CodeMultipleTextValues},
		{"multiple values to null", OperatorJetrules,
			fmt.Sprintf("property %s is not multi-value but has multiple values for subject %s, setting value to null",
				"ex:p", "ex:s1"), CodeMultipleValuesNull},
		// jets/rete/rete_session.cc truth_of and expr_op_logics.h, after the worker's prefix
		{"truth_of", OperatorJetrules,
			fmt.Sprintf("jets:exception caught: %s", "error: a rule tested text for truth: "+"'x' is not a boolean"),
			CodeTextTestedForTruth},
		// jets/rete/expr_op_strings.h
		{"substring_of from", OperatorJetrules,
			"jets:exception caught: error: invalid jets:from property for operator substring_of " + "abc", CodeSubstringFrom},
		{"substring_of length", OperatorJetrules,
			"jets:exception caught: error: invalid jets:length property for operator substring_of " + "abc", CodeSubstringLength},
		{"replace_char_of chars", OperatorJetrules,
			"jets:exception caught: error: invalid jets:replace_chars property for operator replace_char_of " + "abc",
			CodeReplaceCharsInvalid},
		{"replace_char_of with", OperatorJetrules,
			"jets:exception caught: error: invalid jets:replace_with property for operator replace_char_of " + "abc",
			CodeReplaceWithInvalid},
		{"replace_char_of utf8", OperatorJetrules,
			"jets:exception caught: error: jets:replace_chars is not valid UTF-8 for operator replace_char_of " + "abc",
			CodeReplaceCharsUTF8},
		// jets/compute_pipes/column_evaluators_mapping.go, mappingColumnError, as
		// map_record stores it (mapRecordErrorMessage) and as it stored it before
		{"mapping column", OperatorMapRecord,
			"map_record: " + fmt.Sprintf("error mapping column %s at output position %d: %s", "dob", 4, "bad date"),
			CodeMappingColumn},
		{"mapping column, before the prefix", OperatorMapRecord,
			fmt.Sprintf("error mapping column at output position %d: %s", 4, "bad date"), CodeMappingColumn},
		{"cast", OperatorMapRecord,
			"map_record: " + fmt.Sprintf("while casting value to rdf type '%s': %v", "date", "bad"), CodeCastToRdfType},
		// jets/compute_pipes/column_evaluators_case_expr.go
		{"case_expr when", OperatorMapRecord,
			"map_record: " + fmt.Sprintf("while evaluating case_expr when clause #%d: %v", 1, "x"), CodeCaseExpr},
		{"case_expr else", OperatorMapRecord,
			"map_record: " + fmt.Sprintf("while evaluating case_expr else clause: %v",
				"error mapping column at output position 0: x"), CodeCaseExpr},
		// jets/compute_pipes/column_evaluators_lookup.go
		{"lookup key", OperatorMapRecord, "map_record: " + fmt.Sprintf("while making the lookup key: %v", "x"), CodeLookupKey},
		{"lookup row", OperatorMapRecord, "map_record: " + fmt.Sprintf("while fetching the lookup row: %v", "x"), CodeLookupRow},
		// jets/compute_pipes/pipe_transformation_render.go, reportError
		{"render", OperatorRender, fmt.Sprintf("render operator (template '%s'): %v", "t1", "missing"), CodeRender},
		// jets/compute_pipes/pipe_transformation_infer.go and infer_provenance.go,
		// with ErrPrefix as pipe_transformation_ollama.go builds it
		{"inference", OperatorOllama,
			fmt.Sprintf("%s: %v", fmt.Sprintf("ollama operator (model %s)", "llama3:8b"),
				"error mapping column at output position 2: x"), InferenceCode(OperatorOllama)},
		{"provenance findings", OperatorOllama,
			fmt.Sprintf("%s: %s", "ollama operator (model m)", fmt.Sprintf("provenance schema '%s': %s", "brief", "2 findings")),
			ProvenanceFindingsCode(OperatorOllama)},
		{"provenance unchecked", OperatorEmbed,
			fmt.Sprintf("%s: %s", "embed operator (model m)",
				fmt.Sprintf("the briefing could not be checked against provenance schema '%s': %v", "brief", "bad")),
			ProvenanceUncheckedCode(OperatorEmbed)},
		{"vllm", OperatorVllm, "vllm operator (model a/b:c): 500", InferenceCode(OperatorVllm)},
	}
	for _, tc := range cases {
		for _, op := range []string{tc.op, ""} {
			m := c.Match(op, tc.msg)
			if m.Code() != tc.code || m.Uncatalogued || m.Threshold != 1 {
				t.Errorf("%s (operator %q): %q read %q at %d, want %q at 1", tc.writer, op, tc.msg, m.Code(), m.Threshold, tc.code)
			}
		}
	}
}
