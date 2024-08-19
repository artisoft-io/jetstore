package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// ReteMetaStore -- metadata store for a rete network named by it's ruleset uri
// The ReteMetaStore correspond to a complete rule set organized as a rete network.
type ReteMetaStore struct {
	ResourceMgr    *rdf.ResourceManager
	MetaGraph      *rdf.RdfGraph
	LookupTables   *LookupTableManager
	AlphaNodes     []*AlphaNode
	NodeVertices   []*NodeVertex
	JetStoreConfig *map[string]string
}

func NewReteMetaStore(rm *rdf.ResourceManager, mg *rdf.RdfGraph, ltm *LookupTableManager,
	an []*AlphaNode, nv []*NodeVertex, config *map[string]string) (*ReteMetaStore, error) {
	return &ReteMetaStore{
		ResourceMgr:    rm,
		MetaGraph:      mg,
		LookupTables:   ltm,
		AlphaNodes:     an,
		NodeVertices:   nv,
		JetStoreConfig: config,
	}, nil
}

func (ms *ReteMetaStore) NbrVertices() int {
	if ms == nil {
		return 0
	}
	return len(ms.NodeVertices)
}
