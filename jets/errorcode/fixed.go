package errorcode

// JetStore's fixed codes: the per-record messages JetStore itself writes to
// process_errors, one entry per message form. Each match prefix is the form's
// head as its writer formats it, after the operator's fixed prefix
// (StripPrefix), with Wildcard where the writer puts a value before the text
// that tells the forms apart.
//
// **Keep this list beside the writers.** A message whose head changes and whose
// entry does not reads uncatalogued from then on. fixed_test.go formats each
// writer's message the way the writer does, and fails when an entry no longer
// matches it.
//
// Not here, deliberately:
//   - a site operator's messages, which are the site's own text: they read
//     "<operator>: uncatalogued" unless the workspace catalogues them;
//   - the run manifest's error message, which is a channel's, read from
//     pipeline_execution_channel_details, and is never a process_errors row.

// Codes of the fixed entries, for a reader that wants to name one.
const (
	CodeExecuteRulesError   = "jetrules: ExecuteRules returned error"
	CodeMaxLoopReached      = "jetrules: max loop reached"
	CodeMultipleTextValues  = "jetrules: multiple text values kept as an array"
	CodeMultipleValuesNull  = "jetrules: multiple values set to null"
	CodeTextTestedForTruth  = "jetrules: a rule tested text for truth"
	CodeSubstringFrom       = "jetrules: substring_of invalid jets:from"
	CodeSubstringLength     = "jetrules: substring_of invalid jets:length"
	CodeReplaceCharsInvalid = "jetrules: replace_char_of invalid jets:replace_chars"
	CodeReplaceWithInvalid  = "jetrules: replace_char_of invalid jets:replace_with"
	CodeReplaceCharsUTF8    = "jetrules: replace_char_of jets:replace_chars not UTF-8"
	CodeMappingColumn       = "map_record: error mapping column"
	CodeCastToRdfType       = "map_record: cast to rdf type"
	CodeCaseExpr            = "map_record: case_expr"
	CodeLookupKey           = "map_record: lookup key"
	CodeLookupRow           = "map_record: lookup row"
	CodeRender              = "render: render error"
)

// Inference operator codes, by operator type: InferenceCode(op),
// ProvenanceFindingsCode(op) and ProvenanceUncheckedCode(op).
func InferenceCode(op string) string           { return op + ": inference error" }
func ProvenanceFindingsCode(op string) string  { return op + ": provenance findings" }
func ProvenanceUncheckedCode(op string) string { return op + ": provenance not checked" }

// fixedEntry is a fixed code's entry.
func fixedEntry(op, prefix, code, ref, desc string) Entry {
	return Entry{
		OperatorType: op,
		MatchPrefix:  prefix,
		Code:         code,
		Kind:         KindError,
		Source:       SourceJetStore,
		SourceRef:    ref,
		Description:  desc,
	}
}

// InferenceOperators are the operator types of the inference operators, whose
// row-level messages start "<type> operator (model <model>): ".
var InferenceOperators = []string{OperatorOllama, OperatorEmbed, OperatorVllm}

// Fixed returns JetStore's fixed codes, a new slice on every call.
func Fixed() []Entry {
	out := []Entry{
		// The jetrules pool worker's own messages (reportRuleSet, reportMultipleValues)
		fixedEntry(OperatorJetrules, "ExecuteRules returned error", CodeExecuteRulesError,
			"jets/compute_pipes jetrules_pool_worker.go reportRuleSet",
			"The rule engine returned an error while executing a rule set."),
		fixedEntry(OperatorJetrules, "MAX LOOP REACHED", CodeMaxLoopReached,
			"jets/compute_pipes jetrules_pool_worker.go reportRuleSet",
			"A rule set looped until its maximum loop count."),
		fixedEntry(OperatorJetrules, "property % is not multi-value but has % text values for subject ",
			CodeMultipleTextValues,
			"jets/compute_pipes jetrules_pool_worker.go reportMultipleValues",
			"A text property the model declares single-valued had several values; all were kept as an array."),
		fixedEntry(OperatorJetrules, "property % is not multi-value but has multiple values for subject ",
			CodeMultipleValuesNull,
			"jets/compute_pipes jetrules_pool_worker.go reportMultipleValues",
			"A property the model declares single-valued had several values; the column was set to null."),

		// The rule engine's exceptions, asserted as jets:exception and written
		// after RuleExceptionPrefix (jets/rete)
		fixedEntry(OperatorJetrules, "error: a rule tested text for truth", CodeTextTestedForTruth,
			"jets/rete rete_session.cc truth_of",
			"A rule used a text value as a condition and the text is not a boolean."),
		fixedEntry(OperatorJetrules, "error: invalid jets:from property for operator substring_of", CodeSubstringFrom,
			"jets/rete expr_op_strings.h substring_of",
			"The config resource of substring_of has no integer jets:from."),
		fixedEntry(OperatorJetrules, "error: invalid jets:length property for operator substring_of", CodeSubstringLength,
			"jets/rete expr_op_strings.h substring_of",
			"The config resource of substring_of has no integer jets:length."),
		fixedEntry(OperatorJetrules, "error: invalid jets:replace_chars property for operator replace_char_of", CodeReplaceCharsInvalid,
			"jets/rete expr_op_strings.h replace_char_of",
			"The config resource of replace_char_of has no text jets:replace_chars."),
		fixedEntry(OperatorJetrules, "error: invalid jets:replace_with property for operator replace_char_of", CodeReplaceWithInvalid,
			"jets/rete expr_op_strings.h replace_char_of",
			"The config resource of replace_char_of has no text jets:replace_with."),
		fixedEntry(OperatorJetrules, "error: jets:replace_chars is not valid UTF-8 for operator replace_char_of", CodeReplaceCharsUTF8,
			"jets/rete expr_op_strings.h replace_char_of",
			"The jets:replace_chars of replace_char_of is not valid UTF-8."),

		// map_record's per-record forms, read after MapRecordPrefix, which
		// matches the rows written before the prefix and after it alike
		fixedEntry(OperatorMapRecord, mappingColumnHead, CodeMappingColumn,
			"jets/compute_pipes column_evaluators_mapping.go mapColumnEval.Update",
			"A mapped column had no usable value and the mapping names an error message, or its cleansing function rejected the value."),
		fixedEntry(OperatorMapRecord, "while casting value to rdf type", CodeCastToRdfType,
			"jets/compute_pipes column_evaluators_mapping.go mapColumnEval.Update",
			"A mapped value could not be cast to the column's rdf type."),
		fixedEntry(OperatorMapRecord, "while evaluating case_expr", CodeCaseExpr,
			"jets/compute_pipes column_evaluators_case_expr.go caseExprEvaluator.Update",
			"A case_expr column failed to evaluate."),
		fixedEntry(OperatorMapRecord, "while making the lookup key", CodeLookupKey,
			"jets/compute_pipes column_evaluators_lookup.go lookupColumnTransformationEval.Update",
			"A lookup column's key could not be made from the record."),
		fixedEntry(OperatorMapRecord, "while fetching the lookup row", CodeLookupRow,
			"jets/compute_pipes column_evaluators_lookup.go lookupColumnTransformationEval.Update",
			"A lookup column's row could not be fetched."),

		// render (RenderTransformationPipe.reportError)
		fixedEntry(OperatorRender, "render operator (template '", CodeRender,
			"jets/compute_pipes pipe_transformation_render.go reportError",
			"The render operator could not render a record."),
	}
	// The inference operators: a failed call (inferWorker's error report), and
	// the provenance check of an answer (reportProvenance)
	for _, op := range InferenceOperators {
		head := op + " operator (model %): "
		out = append(out,
			fixedEntry(op, head+"provenance schema '", ProvenanceFindingsCode(op),
				"jets/compute_pipes infer_provenance.go reportProvenance",
				"A briefing was answered and is not grounded in its provenance schema."),
			fixedEntry(op, head+"the briefing could not be checked against provenance schema '", ProvenanceUncheckedCode(op),
				"jets/compute_pipes infer_provenance.go reportProvenance",
				"A briefing could not be checked against its provenance schema."),
			fixedEntry(op, op+" operator (model ", InferenceCode(op),
				"jets/compute_pipes pipe_transformation_infer.go",
				"An inference call failed for a record."),
		)
	}
	return out
}
