package rete

import (
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// RaiseException records a rule-time error in the session as
// (jets:iState jets:exception "msg"), the way the C++ engine's operators do
// (substring_of, jets/rete/expr_op_strings.h). The rule set carries on; the
// pipeline collects the exception after the rule set ends and reports it under
// its on_error policy (JrPoolWorker.executeRules).
func (rs *ReteSession) RaiseException(msg string) {
	rmgr := rs.RdfSession.ResourceMgr
	jr := rmgr.JetsResources
	if _, err := rs.RdfSession.Insert(jr.Jets__istate, jr.Jets__exception, rmgr.NewTextLiteral(msg)); err != nil {
		// Nowhere left to report it but the log.
		log.Printf("error: while raising jets:exception %q: %v", msg, err)
	}
}

// truthOf is v's truth value when a rule tests it, and whether it has one. Text
// outside the accepted bool spellings has none: the session gets a
// jets:exception naming the value and the caller treats the result as null, so
// a filter does not pass (I-45). Empty text is null, and false without an
// exception.
func (rs *ReteSession) truthOf(v *rdf.Node) (bool, bool) {
	b, err := v.Truth()
	if err != nil {
		rs.RaiseException(fmt.Sprintf("error: a rule tested text for truth: %v", err))
		return false, false
	}
	return b, true
}
