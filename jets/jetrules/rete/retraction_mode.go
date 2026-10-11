package rete

// The two LEGACY retraction defects, as a mode fixed when the rule network is built.
//
// Both engines keep two retraction defects by Michel's decision of 2026-10-09, because
// usi_ws's loop rules and CM04 op01 terminate only because of them (README.md, entry
// of 2026-10-08, revised 2026-10-09):
//
//  1. a consequent that reads the graph -- exist, exist_not, size_of, the aggregates,
//     lookup -- is recomputed on retraction, so once its input has moved the triple
//     retracted is not the triple inferred;
//  2. a withdrawn row never fires again: after its consequents are retracted it is
//     left in its relation as kProcessed, so an equal row arriving later is taken for
//     one already inferred.
//
// RetractionMode turns either one off. It is README option (b), built for the rules
// unit test harness only: the harness sets it from a test definition and passes it to
// NewReteMetaStoreFactory. No pipeline can reach it -- there is no JetrulesSpec field
// and no rule_config key for it -- so production runs the zero value, which is legacy
// for both. The fixed behaviours are the ones 1d993844 built and c2de6953 narrowed.
//
// The mode is read in two places, both decided by the meta store rather than per
// session: AddConsequentTerm, when the rules load, chooses which vertexes record their
// inferred triples (defect 1); and ReteSession.ComputeConsequentTriples, when a row is
// retracted, chooses whether to erase it (defect 2). The C++ engine has the same
// struct, jets::rete::RetractionMode in jets/rete/rete_meta_store.h.
type RetractionMode struct {
	// ReplayGraphReadingConsequents turns defect 1 off: a vertex whose consequent reads
	// the graph records what its rows inferred and retraction replays the record, as a
	// vertex whose consequent mints a resource always does.
	ReplayGraphReadingConsequents bool
	// EraseWithdrawnRows turns defect 2 off: a row is erased from its relation once its
	// consequents are retracted, so an equal row arriving later fires.
	EraseWithdrawnRows bool
}

// LegacyRetraction is the mode every pipeline runs: both defects kept. It is the zero
// value, so a meta store built without a mode is legacy.
var LegacyRetraction = RetractionMode{}

// IsLegacy is true when both defects are kept.
func (m RetractionMode) IsLegacy() bool {
	return !m.ReplayGraphReadingConsequents && !m.EraseWithdrawnRows
}

// Label names the mode as a result records it: "legacy" with both defects kept,
// "fixed" with both turned off, and "fixed-1" or "fixed-2" with only that defect
// turned off.
func (m RetractionMode) Label() string {
	switch {
	case m.ReplayGraphReadingConsequents && m.EraseWithdrawnRows:
		return "fixed"
	case m.ReplayGraphReadingConsequents:
		return "fixed-1"
	case m.EraseWithdrawnRows:
		return "fixed-2"
	default:
		return "legacy"
	}
}

// AllRetractionModes lists the four modes, legacy first, for tests that run a case in
// each of them.
func AllRetractionModes() []RetractionMode {
	return []RetractionMode{
		{},
		{ReplayGraphReadingConsequents: true},
		{EraseWithdrawnRows: true},
		{ReplayGraphReadingConsequents: true, EraseWithdrawnRows: true},
	}
}

// FactoryOption configures NewReteMetaStoreFactory.
type FactoryOption func(*ReteMetaStoreFactory)

// WithRetractionMode builds every meta store of the factory in mode. Without it the
// factory is legacy for both defects.
func WithRetractionMode(mode RetractionMode) FactoryOption {
	return func(f *ReteMetaStoreFactory) {
		f.RetractionMode = mode
	}
}
