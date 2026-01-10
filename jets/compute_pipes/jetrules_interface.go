package compute_pipes

import "github.com/jackc/pgx/v4/pgxpool"

// This file contains the definition of the interface for jetrules native and go versions integration.

type JetRulesFactory interface {
	// Create a JetRulesEngine instance
	NewJetRulesEngine(dbpool *pgxpool.Pool, processName string) (JetRulesEngine, error)
}

type JetRulesEngine interface {
	// Assert source period info (date, period, type) to rdf graph
	AssertSourcePeriodInfo(config *JetrulesSpec) error
	// Assert rule config to meta graph from the pipeline configuration
	AssertMetadataSource(config *JetrulesSpec, env map[string]any) error
	// Assert rule config to meta graph from the pipeline configuration
	AssertRuleConfiguration(config *JetrulesSpec) error
	GetMetaGraphTriples() []string
	NewWorker(config *JetrulesSpec, source *InputChannel,
		outputChannels []*JetrulesOutputChan,
		done chan struct{}, errCh chan error) JetRulesWorker
}

type JetRulesWorker interface {
	// Execute rules on the input data row
	DoWork(mgr *JrPoolManager, resultCh chan JetrulesWorkerResult)
}
