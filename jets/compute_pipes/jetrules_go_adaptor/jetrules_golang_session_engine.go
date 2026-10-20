package jetrules_go_adaptor

import "github.com/artisoft-io/jetstore/jets/compute_pipes"

// JetRuleEngine is the engine this session was opened on. jets/rulesbench/graphdump
// reads the engine's meta graph through it, to leave the meta graph out of a dump:
// the session's Find returns it with the session's own triples.
func (ses *JetRdfSessionGo) JetRuleEngine() compute_pipes.JetRuleEngine {
	if ses.re == nil {
		return nil
	}
	return ses.re
}
