package rete

import (
	"fmt"

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
	DataPropertyMap map[string]*DataPropertyNode
	DomainTableMap  map[string]*TableNode
}

func NewReteMetaStore(rm *rdf.ResourceManager, mg *rdf.RdfGraph, ltm *LookupTableManager,
	an []*AlphaNode, nv []*NodeVertex, config *map[string]string,
	dataPropertyMap map[string]*DataPropertyNode,
	domainTableMap map[string]*TableNode) (*ReteMetaStore, error) {
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
