package compute_pipes

// Hooks for the external test package compute_pipes_test.
//
// jetrules_execute_rules_test.go needs a real rule engine, and the Go engine's
// adaptor (jetrules_go_adaptor) imports this package, so a test that uses both
// has to live in compute_pipes_test. These hooks are what it reaches the
// unexported worker through. They are compiled into this package's tests only.

// ExecuteRulesForTest calls executeRules, unchanged.
func (ctx *JrPoolWorker) ExecuteRulesForTest(inputRecords *[]any, resultCh chan JetrulesWorkerResult) error {
	return ctx.executeRules(inputRecords, resultCh)
}

// ErrorCountForTest is the worker's running count of failed rule executions,
// which max_error_count is compared against.
func (ctx *JrPoolWorker) ErrorCountForTest() int {
	return ctx.errorCount
}

// NewBuilderContextForTest is a BuilderContext carrying what NewProcessError and
// the jetrules worker read from it: the run's pipeline_execution_status key, its
// session id, and the step's configuration (nil for none).
func NewBuilderContextForTest(peKey int, sessionId string, cpConfig *ComputePipesConfig) *BuilderContext {
	return &BuilderContext{peKey: peKey, sessionId: sessionId, cpConfig: cpConfig}
}

// ExtractRowsBothWaysForTest builds the row of every subject of className in
// rdfSession twice: by columnReader's one pass over the subject's triples, and by
// one FindSP per column, which is how extraction read them before rules_benchmark
// G11. keyed reports that the pass was used, i.e. the engine's nodes have identity
// keys. column2RdfType is what a pipeline takes from the class's domain properties.
func ExtractRowsBothWaysForTest(rdfSession JetRdfSession, className string, columns []string,
	multiValue map[string]bool, column2RdfType map[string]string,
	encodings map[string]*JrSpecialColumnEncoding) (pass, each [][]any, keyed bool) {
	ctx := &JrPoolWorker{
		config:                   &JetrulesSpec{CurrentSourcePeriod: 1},
		multiValueDataProperties: multiValue,
		column2RdfType:           column2RdfType,
	}
	outChannel := &JetrulesOutputChan{ClassName: className, JrColumnEncodings: encodings}
	rm := rdfSession.GetResourceManager()
	var subjects []RdfNode
	itor := rdfSession.FindSPO(nil, rdfSession.JetResources().Rdf__type, rm.NewResource(className))
	for !itor.IsEnd() {
		subjects = append(subjects, itor.GetSubject())
		itor.Next()
	}
	itor.Release()
	cols := newColumnReader(columns, outChannel)
	cols.bind(rdfSession)
	keyed = cols.slotOf != nil
	for _, s := range subjects {
		pass = append(pass, ctx.entityRow(rdfSession, s, columns, cols, 1, outChannel))
	}
	cols.slotOf = nil
	for _, s := range subjects {
		each = append(each, ctx.entityRow(rdfSession, s, columns, cols, 1, outChannel))
	}
	return pass, each, keyed
}
