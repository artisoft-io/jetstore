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
