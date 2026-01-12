package jetrules_go_adaptor

import (
	"fmt"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains the adaptor code to implement JetrulesInterface for the Go rule engine.

// function for compute_pipes.JetRulesFactory
// -------------------------------------------

func (factory *JetRulesFactoryGo) ClearCache() bool {
	// No cache to clear for Go rules engine
	return true
}

func (factory *JetRulesFactoryGo) NewJetRuleEngine(_ *pgxpool.Pool, processName string) (compute_pipes.JetRuleEngine, error) {
	f, err := rete.NewReteMetaStoreFactory(processName)
	if err != nil {
		return nil, err
	}
	ruleEngine := &JetRuleEngineGo{
		processName:          processName,
		factory:              factory,
		reteMetaStoreFactory: f,
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
	err := engine.Release()
	if err != nil {
		return err
	}
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
