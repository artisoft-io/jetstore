package jetrules_native_adaptor

import (
	"fmt"
	"log"
	"time"

	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains the adaptor code to implement JetrulesInterface for the Native rule engine.

// function for compute_pipes.JetRulesFactory
// -------------------------------------------

func (factory *JetRulesFactoryNative) ClearCache() bool {
	// No cache to clear for Native rules engine
	return true
}

func (factory *JetRulesFactoryNative) NewJetRuleEngine(_ *pgxpool.Pool, processName string, isDebug bool) (
	compute_pipes.JetRuleEngine, error) {

	jsHdhle, err := bridge.LoadJetRules(processName, workspace.WorkspaceDbPath(), workspace.LookupDbPath())
	if err != nil {
		return nil, fmt.Errorf("while loading workspace db: %v", err)
	}
	ruleEngine := &JetRuleEngineNative{
		processName: processName,
		factory:     factory,
		js:          jsHdhle,
		isDebug:     isDebug,
	}
	return ruleEngine, nil
}

// function for compute_pipes.JetRuleEngine
// -------------------------------------------

func (engine *JetRuleEngineNative) MainRuleFile() string {
	return engine.processName
}

func (engine *JetRuleEngineNative) JetResources() *compute_pipes.JetResources {
	if engine.jetResources == nil {
		engine.jetResources = compute_pipes.NewJetResources(engine.GetMetaResourceManager())
	}
	return engine.jetResources
}

func (engine *JetRuleEngineNative) GetMetaGraphTriples() []string {
	return []string{}
}

func (engine *JetRuleEngineNative) Insert(s, p, o compute_pipes.RdfNode) error {
	sNative, pNative, oNative, err := toSPO(s, p, o)
	if err != nil {
		return err
	}
	_, err = engine.js.InsertRuleConfig(sNative, pNative, oNative)
	return err
}

func (engine *JetRuleEngineNative) GetMetaResourceManager() compute_pipes.JetResourceManager {
	return &JetResourceManagerNative{
		js: engine.js,
	}
}

func (engine *JetRuleEngineNative) NewRdfSession() (compute_pipes.JetRdfSession, error) {
	rdfSession, err := engine.js.NewRDFSession()
	if err != nil {
		return nil, err
	}
	return &JetRdfSessionNative{
		re:         engine,
		rdfSession: rdfSession,
	}, nil
}

func (engine *JetRuleEngineNative) Release() error {
	engine.factory.ClearCache()
	return engine.js.ReleaseJetRules()
}

// function for compute_pipes.JetResourceManager
// -------------------------------------------

func (rm *JetResourceManagerNative) RdfNull() compute_pipes.RdfNode {
	var node *bridge.Resource
	if rm.rs != nil {
		node, _ = rm.rs.NewNull()
	} else {
		node, _ = rm.js.NewNull()
	}
	return &RdfNodeNative{
		node: node,
	}
}

func (rm *JetResourceManagerNative) CreateBNode(key int) compute_pipes.RdfNode {
	var node *bridge.Resource
	if rm.rs != nil {
		node, _ = rm.rs.NewBlankNode(key)
	} else {
		node, _ = rm.js.NewBlankNode(key)
	}
	return &RdfNodeNative{
		node: node,
	}
}

func (rm *JetResourceManagerNative) NewDateLiteral(date string) compute_pipes.RdfNode {
	var node *bridge.Resource
	if rm.rs != nil {
		node, _ = rm.rs.NewDateLiteral(date)
	} else {
		node, _ = rm.js.NewDateLiteral(date)
	}
	return &RdfNodeNative{
		node: node,
	}
}

func (rm *JetResourceManagerNative) NewDateDetails(year, month, day int) compute_pipes.RdfNode {
	var node *bridge.Resource
	if rm.rs != nil {
		node, _ = rm.rs.NewDateDetails(year, month, day)
	} else {
		node, _ = rm.js.NewDateDetails(year, month, day)
	}
	return &RdfNodeNative{
		node: node,
	}
}

func (rm *JetResourceManagerNative) NewDatetimeLiteral(date string) compute_pipes.RdfNode {
	var node *bridge.Resource
	if rm.rs != nil {
		node, _ = rm.rs.NewDatetimeLiteral(date)
	} else {
		node, _ = rm.js.NewDatetimeLiteral(date)
	}
	return &RdfNodeNative{
		node: node,
	}
}

func (rm *JetResourceManagerNative) NewDatetimeDetails(year, month, day, hour, min, sec int) compute_pipes.RdfNode {
	var node *bridge.Resource
	if rm.rs != nil {
		node, _ = rm.rs.NewDatetimeDetails(year, month, day, hour, min, sec)
	} else {
		node, _ = rm.js.NewDatetimeDetails(year, month, day, hour, min, sec)
	}
	return &RdfNodeNative{
		node: node,
	}
}

func (rm *JetResourceManagerNative) NewDoubleLiteral(x float64) compute_pipes.RdfNode {
	var node *bridge.Resource
	if rm.rs != nil {
		node, _ = rm.rs.NewDoubleLiteral(x)
	} else {
		node, _ = rm.js.NewDoubleLiteral(x)
	}
	return &RdfNodeNative{
		node: node,
	}
}

func (rm *JetResourceManagerNative) NewIntLiteral(data int) compute_pipes.RdfNode {
	var node *bridge.Resource
	if rm.rs != nil {
		node, _ = rm.rs.NewIntLiteral(data)
	} else {
		node, _ = rm.js.NewIntLiteral(data)
	}
	return &RdfNodeNative{
		node: node,
	}
}

func (rm *JetResourceManagerNative) NewUIntLiteral(data uint) compute_pipes.RdfNode {
	var node *bridge.Resource
	if rm.rs != nil {
		node, _ = rm.rs.NewUIntLiteral(data)
	} else {
		node, _ = rm.js.NewUIntLiteral(data)
	}
	return &RdfNodeNative{
		node: node,
	}
}

func (rm *JetResourceManagerNative) NewResource(name string) compute_pipes.RdfNode {
	var node *bridge.Resource
	if rm.rs != nil {
		node, _ = rm.rs.NewResource(name)
	} else {
		node, _ = rm.js.NewResource(name)
	}
	return &RdfNodeNative{
		node: node,
	}
}

func (rm *JetResourceManagerNative) NewTextLiteral(data string) compute_pipes.RdfNode {
	var node *bridge.Resource
	if rm.rs != nil {
		node, _ = rm.rs.NewTextLiteral(data)
	} else {
		node, _ = rm.js.NewTextLiteral(data)
	}
	return &RdfNodeNative{
		node: node,
	}
}

// function for compute_pipes.JetRdfSession
// -----------------------------------------

func (ses *JetRdfSessionNative) GetResourceManager() compute_pipes.JetResourceManager {
	return &JetResourceManagerNative{
		js: ses.re.js,
		rs: ses.rs,
	}
}

func (ses *JetRdfSessionNative) JetResources() *compute_pipes.JetResources {
	return ses.re.JetResources()
}

func (ses *JetRdfSessionNative) NewReteSession(ruleset string) (compute_pipes.JetReteSession, error) {
	reteSession, err := ses.re.js.NewReteSession(ses.rdfSession, ruleset)
	if err != nil {
		return nil, err
	}
	ses.rs = reteSession
	hdl := &JetReteSessionNative{
		rdfSession:  ses.rdfSession,
		reteSession: reteSession,
		ruleset:     ruleset,
	}
	hdl.rdfSessionHdl = ses
	return hdl, nil
}

func toSPO(s, p, o compute_pipes.RdfNode) (*bridge.Resource, *bridge.Resource, *bridge.Resource, error) {
	var u, v, w *bridge.Resource
	if s != nil {
		u2, ok := s.Hdle().(*RdfNodeNative)
		if !ok {
			return nil, nil, nil, fmt.Errorf("toSPO: invalid subject node")
		}
		u = u2.node
	}
	if p != nil {
		v2, ok := p.Hdle().(*RdfNodeNative)
		if !ok {
			return nil, nil, nil, fmt.Errorf("toSPO: invalid predicate node")
		}
		v = v2.node
	}
	if o != nil {
		w2, ok := o.Hdle().(*RdfNodeNative)
		if !ok {
			return nil, nil, nil, fmt.Errorf("toSPO: invalid object node")
		}
		w = w2.node
	}
	return u, v, w, nil
}

func (ses *JetRdfSessionNative) Insert(s, p, o compute_pipes.RdfNode) error {
	sNative, pNative, oNative, err := toSPO(s, p, o)
	if err != nil {
		return err
	}
	c, err := ses.rdfSession.Insert(sNative, pNative, oNative)
	ses.insertCounter += c
	return err
}

func (ses *JetRdfSessionNative) Erase(s, p, o compute_pipes.RdfNode) (bool, error) {
	sNative, pNative, oNative, err := toSPO(s, p, o)
	if err != nil {
		return false, err
	}
	var b int
	b, err = ses.rdfSession.Erase(sNative, pNative, oNative)
	return b > 0, err
}

func (ses *JetRdfSessionNative) Retract(s, p, o compute_pipes.RdfNode) (bool, error) {
	sNative, pNative, oNative, err := toSPO(s, p, o)
	if err != nil {
		return false, err
	}
	b, err := ses.rdfSession.Erase(sNative, pNative, oNative)
	return b > 0, err
}

func (ses *JetRdfSessionNative) Contains(s, p, o compute_pipes.RdfNode) bool {
	sNative, pNative, oNative, err := toSPO(s, p, o)
	if err != nil {
		return false
	}
	b, err := ses.rdfSession.Contains(sNative, pNative, oNative)
	if err != nil {
		log.Printf("Error in Contains: %v", err)
		return false
	}
	return b > 0
}

func (ses *JetRdfSessionNative) ContainsSP(s, p compute_pipes.RdfNode) bool {
	sNative, pNative, _, err := toSPO(s, p, nil)
	if err != nil {
		return false
	}
	b, err := ses.rdfSession.ContainsSP(sNative, pNative)
	if err != nil {
		log.Printf("Error in ContainsSP: %v", err)
		return false
	}
	return b > 0
}

func (ses *JetRdfSessionNative) GetObject(s, p compute_pipes.RdfNode) compute_pipes.RdfNode {
	sNative, pNative, _, err := toSPO(s, p, nil)
	if err != nil {
		return nil
	}
	node, err := ses.rdfSession.GetObject(sNative, pNative)
	if err != nil {
		return nil
	}
	return &RdfNodeNative{
		node: node,
	}
}

func (ses *JetRdfSessionNative) FindSPO(s, p, o compute_pipes.RdfNode) compute_pipes.TripleIterator {
	sNative, pNative, oNative, err := toSPO(s, p, o)
	if err != nil {
		return nil
	}
	itor, err := ses.rdfSession.Find(sNative, pNative, oNative)
	if err != nil {
		return nil
	}
	null, _ := ses.rs.NewNull()
	return NewTripleIteratorNative(itor, null)
}

func (ses *JetRdfSessionNative) FindSP(s, p compute_pipes.RdfNode) compute_pipes.TripleIterator {
	sNative, pNative, _, err := toSPO(s, p, nil)
	if err != nil {
		return nil
	}
	itor, err := ses.rdfSession.Find_sp(sNative, pNative)
	if err != nil {
		return nil
	}
	null, _ := ses.rs.NewNull()
	return NewTripleIteratorNative(itor, null)
}

func (ses *JetRdfSessionNative) FindS(s compute_pipes.RdfNode) compute_pipes.TripleIterator {
	sNative, _, _, err := toSPO(s, nil, nil)
	if err != nil {
		return nil
	}
	itor, err := ses.rdfSession.Find_s(sNative)
	if err != nil {
		return nil
	}
	null, _ := ses.rs.NewNull()
	return NewTripleIteratorNative(itor, null)
}

func (ses *JetRdfSessionNative) Find() compute_pipes.TripleIterator {
	itor, err := ses.rdfSession.FindAll()
	if err != nil {
		return nil
	}
	null, _ := ses.rs.NewNull()
	return NewTripleIteratorNative(itor, null)
}

func (ses *JetRdfSessionNative) Release() error {
	if ses.rdfSession != nil {
		ses.rdfSession.ReleaseRDFSession()
		ses.rdfSession = nil
	}
	return nil
}

// function for compute_pipes.JetReteSession
// -------------------------------------------

func (ses *JetReteSessionNative) ExecuteRules() error {

	if ses.rdfSessionHdl.re.isDebug {
		log.Printf("%p Calling ExecuteRules, inserted %d triples :: %s", ses,
			ses.rdfSessionHdl.insertCounter, ses.ruleset)
	}
	msg, err := ses.reteSession.ExecuteRules()
	if err != nil {
		ses.executeErrorCounter++
		err = fmt.Errorf("%s: %v", msg, err)
		log.Printf("%p Error in ExecuteRules: %v", ses, err)
		return err
	}
	ses.executeCounter++
	return nil
}

func (ses *JetReteSessionNative) Release() error {
	if ses.reteSession != nil {
		ses.reteSession.ReleaseReteSession()
		ses.reteSession = nil
	}
	return nil
}

// function for compute_pipes.TripleIterator
// -------------------------------------------

func (i *TripleIteratorNative) Next() bool {
	return i.iterator.Next()
}

func (i *TripleIteratorNative) Value() [3]compute_pipes.RdfNode {
	if i.iterator.IsEnd() {
		return [3]compute_pipes.RdfNode{
			&RdfNodeNative{node: i.null},
			&RdfNodeNative{node: i.null},
			&RdfNodeNative{node: i.null},
		}
	}
	return [3]compute_pipes.RdfNode{
		&RdfNodeNative{node: i.iterator.GetSubject()},
		&RdfNodeNative{node: i.iterator.GetPredicate()},
		&RdfNodeNative{node: i.iterator.GetObject()},
	}
}

func (it *TripleIteratorNative) IsEnd() bool {
	return it.iterator.IsEnd()
}

func (it *TripleIteratorNative) GetSubject() compute_pipes.RdfNode {
	return &RdfNodeNative{
		node: it.iterator.GetSubject(),
	}
}

func (it *TripleIteratorNative) GetPredicate() compute_pipes.RdfNode {
	return &RdfNodeNative{
		node: it.iterator.GetPredicate(),
	}
}

func (it *TripleIteratorNative) GetObject() compute_pipes.RdfNode {
	return &RdfNodeNative{
		node: it.iterator.GetObject(),
	}
}

func (it *TripleIteratorNative) Release() error {
	if it.iterator != nil {
		return it.iterator.ReleaseIterator()
	}
	return nil
}

// function for compute_pipes.RdfNode
// -------------------------------------------

func (n *RdfNodeNative) Hdle() any {
	return n
}

func (n *RdfNodeNative) IsNil() bool {
	return n.node.GetType() == 0
}

func (n *RdfNodeNative) Value() any {
	if n == nil || n.node == nil {
		return nil
	}
	value, err := n.node.AsInterface("")
	if err != nil {
		log.Printf("Error getting value from RdfNodeNative: %v", err)
		return nil
	}
	switch n.node.GetType() {
	case 9: // date
		switch v := value.(type) {
		case time.Time:
			value = rdf.LDate{
				Date: &v,
			}
		case *time.Time:
			value = rdf.LDate{
				Date: v,
			}
		case string:
			value = v
			// value, err = rdf.ParseDate(v)
			// if err != nil {
			// 	log.Printf("Error parsing date value from string '%s': %v", v, err)
			// 	return nil
			// }
		default:
			log.Printf("Error converting date value from %T to rdf.LDate", value)
			return nil
		}
	case 10: // datetime
		switch v := value.(type) {
		case time.Time:
			value = rdf.LDatetime{
				Datetime: &v,
			}
		case *time.Time:
			value = rdf.LDatetime{
				Datetime: v,
			}
		case string:
			value = v
			// value, err = rdf.ParseDatetime(v)
			// if err != nil {
			// 	log.Printf("Error parsing datetime value from string '%s': %v", v, err)
			// 	return nil
			// }
		default:
			log.Printf("Error converting datetime value from %T to rdf.LDatetime", value)
			return nil
		}
	}
	return value
}

func (n *RdfNodeNative) String() string {
	v, _ := n.node.AsText()
	return v
}

func (n *RdfNodeNative) Equals(other compute_pipes.RdfNode) bool {
	otherNodeNative, ok := other.Hdle().(*RdfNodeNative)
	if !ok {
		return false
	}
	return n.node == otherNodeNative.node
}
