package jetrules_go_adaptor

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains the adaptor code to implement JetrulesInterface for the Go rule engine.

// function for compute_pipes.JetRulesFactory
// -------------------------------------------

func (factory *JetRulesFactoryGo) JetRulesName() string {
	return "JetRules Go Engine"
}

func (factory *JetRulesFactoryGo) ClearCache() bool {
	// No cache to clear for Go rules engine
	return true
}

func (factory *JetRulesFactoryGo) NewJetRuleEngine(_ *pgxpool.Pool, processName string, isDebug bool) (compute_pipes.JetRuleEngine, error) {
	f, err := rete.NewReteMetaStoreFactory(processName)
	if err != nil {
		return nil, err
	}
	ruleEngine := &JetRuleEngineGo{
		processName:          processName,
		factory:              factory,
		reteMetaStoreFactory: f,
		isDebug:              isDebug,
	}
	return ruleEngine, nil
}

// function for compute_pipes.JetRuleEngine
// -------------------------------------------

func (engine *JetRuleEngineGo) MainRuleFile() string {
	return engine.processName
}

func (engine *JetRuleEngineGo) JetResources() *compute_pipes.JetResources {
	if engine.jetResources == nil {
		engine.jetResources = compute_pipes.NewJetResources(engine.GetMetaResourceManager())
	}
	return engine.jetResources
}

func (engine *JetRuleEngineGo) GetMetaGraphTriples() []string {
	return engine.reteMetaStoreFactory.MetaGraph.ToTriples()
}

func (engine *JetRuleEngineGo) Insert(s, p, o compute_pipes.RdfNode) error {
	sGo, pGo, oGo, err := toSPO(s, p, o)
	if err != nil {
		return err
	}
	_, err = engine.reteMetaStoreFactory.MetaGraph.Insert(sGo, pGo, oGo)
	return err
}

func (engine *JetRuleEngineGo) GetMetaResourceManager() compute_pipes.JetResourceManager {
	return &JetResourceManagerGo{
		rm: engine.reteMetaStoreFactory.ResourceMgr,
	}
}

func (engine *JetRuleEngineGo) NewRdfSession() (compute_pipes.JetRdfSession, error) {
	rdfSession := rdf.NewRdfSession(engine.reteMetaStoreFactory.ResourceMgr, engine.reteMetaStoreFactory.MetaGraph)
	return &JetRdfSessionGo{
		re:         engine,
		rdfSession: rdfSession,
	}, nil
}

func (engine *JetRuleEngineGo) Release() error {
	engine.factory.ClearCache()
	return nil
}

// function for compute_pipes.JetResourceManager
// -------------------------------------------

func (rm *JetResourceManagerGo) RdfNull() compute_pipes.RdfNode {
	return &RdfNodeGo{
		node: rdf.Null(),
	}
}

func (rm *JetResourceManagerGo) CreateBNode(key int) compute_pipes.RdfNode {
	return &RdfNodeGo{
		node: rm.rm.CreateBNode(key),
	}
}

func (rm *JetResourceManagerGo) NewDateLiteral(date string) compute_pipes.RdfNode {
	dt, err := rdf.ParseDate(date)
	if err != nil {
		return &RdfNodeGo{
			node: rdf.Null(),
		}
	}
	return &RdfNodeGo{
		node: rm.rm.NewDateLiteral(rdf.LDate{Date: dt}),
	}
}

func (rm *JetResourceManagerGo) NewDateDetails(year, month, day int) compute_pipes.RdfNode {
	dt := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return &RdfNodeGo{
		node: rm.rm.NewDateLiteral(rdf.LDate{Date: &dt}),
	}
}

func (rm *JetResourceManagerGo) NewDatetimeLiteral(date string) compute_pipes.RdfNode {
	dt, err := rdf.ParseDatetime(date)
	if err != nil {
		return &RdfNodeGo{
			node: rdf.Null(),
		}
	}
	return &RdfNodeGo{
		node: rm.rm.NewDatetimeLiteral(rdf.LDatetime{Datetime: dt}),
	}
}

func (rm *JetResourceManagerGo) NewDatetimeDetails(year, month, day, hour, min, sec int) compute_pipes.RdfNode {
	dt := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
	return &RdfNodeGo{
		node: rm.rm.NewDatetimeLiteral(rdf.LDatetime{Datetime: &dt}),
	}
}

func (rm *JetResourceManagerGo) NewDoubleLiteral(x float64) compute_pipes.RdfNode {
	return &RdfNodeGo{
		node: rm.rm.NewDoubleLiteral(x),
	}
}

func (rm *JetResourceManagerGo) NewIntLiteral(data int) compute_pipes.RdfNode {
	return &RdfNodeGo{
		node: rm.rm.NewIntLiteral(data),
	}
}

func (rm *JetResourceManagerGo) NewUIntLiteral(data uint) compute_pipes.RdfNode {
	return &RdfNodeGo{
		node: rm.rm.NewUIntLiteral(data),
	}
}

func (rm *JetResourceManagerGo) NewResource(name string) compute_pipes.RdfNode {
	return &RdfNodeGo{
		node: rm.rm.NewResource(name),
	}
}

func (rm *JetResourceManagerGo) NewTextLiteral(data string) compute_pipes.RdfNode {
	return &RdfNodeGo{
		node: rm.rm.NewTextLiteral(data),
	}
}

// function for compute_pipes.JetRdfSession
// -----------------------------------------

func (ses *JetRdfSessionGo) GetResourceManager() compute_pipes.JetResourceManager {
	return &JetResourceManagerGo{
		rm: ses.rdfSession.ResourceMgr,
	}
}

func (ses *JetRdfSessionGo) JetResources() *compute_pipes.JetResources {
	return ses.re.JetResources()
}

func (ses *JetRdfSessionGo) NewReteSession(ruleset string) (compute_pipes.JetReteSession, error) {
	ms := ses.re.reteMetaStoreFactory.MetaStoreLookup[ruleset]
	if ms == nil {
		return nil, fmt.Errorf("error: metastore not found for %s", ruleset)
	}
	reteSession := rete.NewReteSession(ses.rdfSession)
	reteSession.Initialize(ms)
	return &JetReteSessionGo{
		rdfSession:  ses.rdfSession,
		reteSession: reteSession,
	}, nil
}

func toSPO(s, p, o compute_pipes.RdfNode) (*rdf.Node, *rdf.Node, *rdf.Node, error) {
	var u, v, w *rdf.Node
	if s != nil {
		u2, ok := s.Hdle().(*RdfNodeGo)
		if !ok {
			return nil, nil, nil, fmt.Errorf("toSPO: invalid subject node")
		}
		u = u2.node
	}
	if p != nil {
		v2, ok := p.Hdle().(*RdfNodeGo)
		if !ok {
			return nil, nil, nil, fmt.Errorf("toSPO: invalid predicate node")
		}
		v = v2.node
	}
	if o != nil {
		w2, ok := o.Hdle().(*RdfNodeGo)
		if !ok {
			return nil, nil, nil, fmt.Errorf("toSPO: invalid object node")
		}
		w = w2.node
	}
	return u, v, w, nil
}

func (ses *JetRdfSessionGo) Insert(s, p, o compute_pipes.RdfNode) error {
	sGo, pGo, oGo, err := toSPO(s, p, o)
	if err != nil {
		return err
	}
	_, err = ses.rdfSession.Insert(sGo, pGo, oGo)
	return err
}

func (ses *JetRdfSessionGo) Erase(s, p, o compute_pipes.RdfNode) (bool, error) {
	sGo, pGo, oGo, err := toSPO(s, p, o)
	if err != nil {
		return false, err
	}
	var b bool
	b, err = ses.rdfSession.Erase(sGo, pGo, oGo)
	return b, err
}

func (ses *JetRdfSessionGo) Retract(s, p, o compute_pipes.RdfNode) (bool, error) {
	sGo, pGo, oGo, err := toSPO(s, p, o)
	if err != nil {
		return false, err
	}
	b, err := ses.rdfSession.Retract(sGo, pGo, oGo)
	return b, err
}

func (ses *JetRdfSessionGo) Contains(s, p, o compute_pipes.RdfNode) bool {
	sGo, pGo, oGo, err := toSPO(s, p, o)
	if err != nil {
		return false
	}
	return ses.rdfSession.Contains(sGo, pGo, oGo)
}

func (ses *JetRdfSessionGo) ContainsSP(s, p compute_pipes.RdfNode) bool {
	sGo, pGo, _, err := toSPO(s, p, nil)
	if err != nil {
		return false
	}
	return ses.rdfSession.ContainsSP(sGo, pGo)
}

func (ses *JetRdfSessionGo) GetObject(s, p compute_pipes.RdfNode) compute_pipes.RdfNode {
	sGo, pGo, _, err := toSPO(s, p, nil)
	if err != nil {
		return nil
	}
	return &RdfNodeGo{
		node: ses.rdfSession.GetObject(sGo, pGo),
	}
}

func (ses *JetRdfSessionGo) FindSPO(s, p, o compute_pipes.RdfNode) compute_pipes.TripleIterator {
	sGo, pGo, oGo, err := toSPO(s, p, o)
	if err != nil {
		return nil
	}
	itor := ses.rdfSession.FindSPO(sGo, pGo, oGo)
	return NewTripleIteratorGo(nil, itor)
}

func (ses *JetRdfSessionGo) FindSP(s, p compute_pipes.RdfNode) compute_pipes.TripleIterator {
	sGo, pGo, _, err := toSPO(s, p, nil)
	if err != nil {
		return nil
	}
	itor := ses.rdfSession.FindSP(sGo, pGo)
	return NewTripleIteratorGo(nil, itor)
}

func (ses *JetRdfSessionGo) FindS(s compute_pipes.RdfNode) compute_pipes.TripleIterator {
	sGo, _, _, err := toSPO(s, nil, nil)
	if err != nil {
		return nil
	}
	itor := ses.rdfSession.FindS(sGo)
	return NewTripleIteratorGo(nil, itor)
}

func (ses *JetRdfSessionGo) Find() compute_pipes.TripleIterator {
	itor := ses.rdfSession.Find()
	return NewTripleIteratorGo(nil, itor)
}

func (ses *JetRdfSessionGo) EncodeRdfSession() string {
	if ses.rdfSession == nil {
		return ""
	}
	
	enc, err := ses.encodeSession()
	if err != nil {
		enc = map[string]any{"error": err.Error()}
	}
	r, _ := json.Marshal(enc)
	return string(r)
}

// Returns map[string]any which is
//    {
//      "rdf_types": string (json of [][]string),
// 	    "entity_key_by_type": map[string]string (json of [][]string),
//      "entity_details_by_key": map[string]string (json of [][]string),
//    }
// rdf_type: JetModel ([][]string): List of rdf:type, single column model
// entity_key_by_type: Map[rdf:type]JetModel: JetModel is list of jet:key, single column model
// entity_details_by_key: Map[jets:key]EncodedJetModel, 
// where EncodedJetModel is encoded json of JetModel ([][]string): List of ["property", "value", "value.type"] of obj w/ jets:key, 2 columns JetModel
// If there is an error, it returns a map with only one key "error" and the error message as value.
func (ses *JetRdfSessionGo) encodeSession() (map[string]any, error) {
	if ses.rdfSession == nil {
		return nil, fmt.Errorf("EncodeRdfSession (native): error rdfSession cannot be nil")
	}
	// Set of rdf:type
	rdfTypeSet := make(map[string]bool)

	// Set of entity
	entitySet := make(map[string]*rdf.Node)

	// Map of rdf:key by rdf:type: map[rdf:type][]rdf:key
	entityKeyByType := make(map[string]*[][]string)

	ri := ses.re.JetResources()
	// Create the rdf_type (rdfTypeSet) and entity_key_by_type (entityKeyByType) data structures
	ctor  := ses.rdfSession.FindSPO(nil, ri.Rdf__type.Hdle().(*rdf.Node), nil)
	for t3 := range ctor.Itor {
		entity := t3[0]
		entityName := entity.String()

		rdfType := t3[2].String()
		rdfTypeSet[rdfType] = true
		entitySet[entityName] = entity
		entities := entityKeyByType[rdfType]
		if entities == nil {
			entities = &[][]string{}
			entityKeyByType[rdfType] = entities
		}
		*entities = append(*entities, []string{entityName})
	}
	ctor.Done()

	// Now create the entity_details_by_key: Map[jets:key]*[][]string
	entityDetailsByKey := make(map[string]*[][]string)
	for entityKey, entity := range entitySet {
		ctor := ses.rdfSession.FindS(entity)
		for t3 := range ctor.Itor {
			propertyName := t3[1].String()
			value := t3[2]
			valueType := value.GetTypeName()
			model := entityDetailsByKey[entityKey]
			if model == nil {
				model = &[][]string{}
				entityDetailsByKey[entityKey] = model
			}
			*model = append(*model, []string{propertyName, value.String(), valueType})
		}
		ctor.Done()
	}

	// Put all the results in the output map
	results := make(map[string]interface{})

	// Package rdfTypeSet
	rdfTypesResult := make([][]string, 0)
	for rdfType := range rdfTypeSet {
		rdfTypesResult = append(rdfTypesResult, []string{rdfType})
	}
	sort.Slice(rdfTypesResult, func(i, j int) bool {
		return rdfTypesResult[i][0] < rdfTypesResult[j][0]
	})
	r, err := json.Marshal(rdfTypesResult)
	if err != nil {
		return nil, err
	}
	results["rdf_types"] = string(r)

	// Package entityKeyByType
	entityKeyByTypeResult := make(map[string]string)
	for rdfType, keys := range entityKeyByType {
		sort.Slice(*keys, func(i, j int) bool {
			return (*keys)[i][0] < (*keys)[j][0]
		})
		r, err := json.Marshal(*keys)
		if err != nil {
			return nil, err
		}
		entityKeyByTypeResult[rdfType] = string(r)
	}
	results["entity_key_by_type"] = entityKeyByTypeResult

	// Package entityDetailsByKey
	entityDetailsByKeyResult := make(map[string]string)
	for key, details := range entityDetailsByKey {
		sort.Slice(*details, func(i, j int) bool {
			if (*details)[i][0] == (*details)[j][0] {
				if (*details)[i][1] == (*details)[j][1] {
					return (*details)[i][2] < (*details)[j][2]
				}
				return (*details)[i][1] < (*details)[j][1]
			}
			return (*details)[i][0] < (*details)[j][0]
		})
		r, err := json.Marshal(*details)
		if err != nil {
			return nil, err
		}
		entityDetailsByKeyResult[key] = string(r)
	}
	results["entity_details_by_key"] = entityDetailsByKeyResult

	return results, nil
}

func (ses *JetRdfSessionGo) Release() error {
	if ses.rdfSession != nil {
		// ses.rdfSession.Done()
		ses.rdfSession = nil
	}
	return nil
}

// function for compute_pipes.JetReteSession
// -------------------------------------------

func (ses *JetReteSessionGo) ExecuteRules() error {
	return ses.reteSession.ExecuteRules()
}

func (ses *JetReteSessionGo) Release() error {
	if ses.reteSession != nil {
		ses.reteSession.Done()
		ses.reteSession = nil
	}
	return nil
}

// function for compute_pipes.TripleIterator
// -------------------------------------------

func (i *TripleIteratorGo) Next() bool {
	if i.isEnd {
		return false
	}
	// read next value, if one
	triple, ok := <-i.Itor
	if !ok {
		i.isEnd = true
		return false
	}
	i.value[0] = triple[0]
	i.value[1] = triple[1]
	i.value[2] = triple[2]
	return true
}

func (i *TripleIteratorGo) Value() [3]compute_pipes.RdfNode {
	if i.isEnd {
		return [3]compute_pipes.RdfNode{
			&RdfNodeGo{node: rdf.Null()},
			&RdfNodeGo{node: rdf.Null()},
			&RdfNodeGo{node: rdf.Null()},
		}
	}
	return [3]compute_pipes.RdfNode{
		&RdfNodeGo{node: i.value[0]},
		&RdfNodeGo{node: i.value[1]},
		&RdfNodeGo{node: i.value[2]},
	}
}

func (it *TripleIteratorGo) IsEnd() bool {
	return it.isEnd
}

func (it *TripleIteratorGo) GetSubject() compute_pipes.RdfNode {
	return &RdfNodeGo{
		node: it.value[0],
	}
}

func (it *TripleIteratorGo) GetPredicate() compute_pipes.RdfNode {
	return &RdfNodeGo{
		node: it.value[1],
	}
}

func (it *TripleIteratorGo) GetObject() compute_pipes.RdfNode {
	return &RdfNodeGo{
		node: it.value[2],
	}
}

func (it *TripleIteratorGo) Release() error {
	if it.sesIter != nil {
		it.sesIter.Done()
	}
	if it.iterator != nil {
		it.iterator.Done()
	}
	return nil
}

// function for compute_pipes.RdfNode
// -------------------------------------------

func (n *RdfNodeGo) Hdle() any {
	return n
}

func (n *RdfNodeGo) IsNil() bool {
	return n.node == rdf.Null()
}

func (n *RdfNodeGo) Value() any {
	if n == nil || n.node == nil {
		return nil
	}
	return n.node.Value
}

func (n *RdfNodeGo) String() string {
	return n.node.String()
}

func (n *RdfNodeGo) Equals(other compute_pipes.RdfNode) bool {
	otherNodeGo, ok := other.Hdle().(*RdfNodeGo)
	if !ok {
		return false
	}
	return n.node.EQ(otherNodeGo.node) == rdf.TRUE()
}
