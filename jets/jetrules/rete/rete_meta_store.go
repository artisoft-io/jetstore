package rete

import (
	"fmt"
	"sync"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// ReteMetaStore -- metadata store for a rete network named by it's ruleset uri
// The ReteMetaStore correspond to a complete rule set organized as a rete network.
type ReteMetaStore struct {
	ResourceMgr     *rdf.ResourceManager
	MetaGraph       *rdf.RdfGraph
	LookupTables    *LookupTableManager
	AlphaNodes      []*AlphaNode
	NodeVertices    []*NodeVertex
	JetStoreConfig  *map[string]string
	DataPropertyMap map[string]*PropertyNode
	DomainTableMap  map[string]*TableNode
	// RetractionMode is the mode the network was built in (retraction_mode.go). It must
	// be the mode AddConsequentTerm was given for every vertex: the factory sets both.
	// The zero value is legacy, which is what every pipeline runs.
	RetractionMode RetractionMode

	// exprOnce initialises the operators of the network's expressions once, for every
	// session of the meta store (initializeExpressions); exprErr is what a filter's
	// initialisation returned.
	exprOnce sync.Once
	exprErr  error
}

func NewReteMetaStore(rm *rdf.ResourceManager, mg *rdf.RdfGraph, ltm *LookupTableManager,
	an []*AlphaNode, nv []*NodeVertex, config *map[string]string,
	dataPropertyMap map[string]*PropertyNode,
	domainTableMap map[string]*TableNode) (*ReteMetaStore, error) {
	assignAntecedentQueryKeys(nv)
	return &ReteMetaStore{
		ResourceMgr:     rm,
		MetaGraph:       mg,
		LookupTables:    ltm,
		AlphaNodes:      an,
		NodeVertices:    nv,
		JetStoreConfig:  config,
		DataPropertyMap: dataPropertyMap,
		DomainTableMap:  domainTableMap,
	}, nil
}

// The meta store is shared by every session of a pool, and sessions only read it.
// Two things a session needs from it used to be written by every session, without a
// lock, and are now computed once (rules_benchmark I-5 and I-24, Phase 2 AH.1, AH.3).

// assignAntecedentQueryKeys gives each antecedent term the key of the query its parent
// beta relation holds for it. NewBetaRelation adds one query per child alpha node, in
// ChildAlphaNodes order, through AlphaNode.InitializeIndexes, so the keys are a
// property of the network and are the same in every session. Every session's
// InitializeIndexes used to write them on the shared NodeVertex.
func assignAntecedentQueryKeys(nodeVertices []*NodeVertex) {
	for _, parent := range nodeVertices {
		if parent == nil {
			continue
		}
		var next [4]int // by query arity, 1 to 3
		for _, an := range parent.ChildAlphaNodes {
			if !an.IsAntecedent {
				continue
			}
			arity := an.queryArity()
			if arity == 0 {
				continue
			}
			an.NdVertex.AntecedentQueryKey = next[arity]
			an.NdVertex.queryKeyAssigned = true
			next[arity]++
		}
	}
}

// initializeExpressions calls InitializeExpression on the filter and consequent
// expressions of the network, once. An operator's InitializeOperator reads its
// configuration from the meta graph (jets:entity_property, jets:operator, ...) into
// its own fields, and every session's Initialize used to call it on the shared
// operators: SortedHeadOp published a MinMaxOp with nil fields that a concurrent
// session evaluated, and answered wrongly (rules_benchmark I-5, F84).
//
// It runs at the first session's Initialize rather than when the meta store is built,
// because the engine adds meta triples (JetRuleEngine.Insert) after the rules load and
// until the first rdf session locks the meta graph. By the first rete session the meta
// graph is locked, so what the operators read is final. Only a filter's error is
// kept, as Initialize only ever reported a filter's. A panic inside it is kept as the
// error too: sync.Once counts a panicking function as done, so without the recover
// every later session would run on half-initialised operators with no error, where
// every session used to panic.
func (ms *ReteMetaStore) initializeExpressions() error {
	ms.exprOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				ms.exprErr = fmt.Errorf("panic while initializing the rete network's expressions: %v", r)
			}
		}()
		rs := &ReteSession{ms: ms}
		for _, alphaNode := range ms.AlphaNodes {
			if alphaNode.IsAntecedent {
				nodeVertex := alphaNode.NdVertex
				if nodeVertex.HasExpression() {
					if err := nodeVertex.FilterExpr.InitializeExpression(rs); err != nil && ms.exprErr == nil {
						ms.exprErr = err
					}
				}
			} else {
				alphaNode.Fw.InitializeExpression(rs)
			}
		}
	})
	return ms.exprErr
}

func (ms *ReteMetaStore) NbrVertices() int {
	if ms == nil {
		return 0
	}
	return len(ms.NodeVertices)
}

func (ms *ReteMetaStore) GetRangeDataType(dataProperty string) (string, bool, error) {
	if ms == nil {
		return "", false, fmt.Errorf("error: GetRangeDataType called with nil ReteMetaStore for data property: %s", dataProperty)
	}
	p := ms.DataPropertyMap[dataProperty]
	if p == nil {
		return "", false, fmt.Errorf("GetRangeDataType: unknown data property: %s", dataProperty)
	}
	return p.Type, p.AsArray, nil
}
