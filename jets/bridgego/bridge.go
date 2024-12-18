package bridgego

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

type JetStore struct {
	Factory         *rete.ReteMetaStoreFactory
	MetaStore       *rete.ReteMetaStore
	MetaMgr         *rdf.ResourceManager
	MetaGraph       *rdf.RdfGraph
	ProcessName     string
	MainRuleName    string
	LookupDbPath    string
}

type RDFSession struct {
	js         *JetStore
	rdfSession *rdf.RdfSession
}

type ReteSession struct {
	js          *JetStore
	rdfSession  *RDFSession
	reteSession *rete.ReteSession
}
type RSIterator = rdf.RdfSessionIterator

type Resource struct {
	r *rdf.Node
}

func NewResource(r *rdf.Node) *Resource {
	return &Resource{r: r}
}

var (
	ErrNotValidDate      = errors.New("not a valid date")
	ErrNotValidDateTime  = errors.New("not a valid datetime")
	ErrNullValue         = errors.New("null value")
	ErrUnexpectedRdfType = errors.New("value with unexpected rdf type")
	ErrLookupTable       = errors.New("error loading lookup tables")
)

// ResourceType
// switch (r->which()) {
//   case rdf_null_t             :0 return rdf_null_t;
//   case rdf_blank_node_t       :1 return rdf_blank_node_t;
//   case rdf_named_resource_t   :2 return rdf_named_resource_t;
//   case rdf_literal_int32_t    :3 return rdf_literal_int32_t;
//   case rdf_literal_uint32_t   :4 return rdf_literal_uint32_t;
//   case rdf_literal_int64_t    :5 return rdf_literal_int64_t;
//   case rdf_literal_uint64_t   :6 return rdf_literal_uint64_t;
//   case rdf_literal_double_t   :7 return rdf_literal_double_t;
//   case rdf_literal_string_t   :8 return rdf_literal_string_t;
//   case rdf_literal_date_t     :9 return rdf_literal_date_t;
//   case rdf_literal_datetime_t :10 return rdf_literal_datetime_t;

func getTypeName(dtype int) string {
	switch dtype {
	case 0:
		return "null"
	case 1:
		return "blank_node"
	case 2:
		return "named_resource"
	case 3:
		return "int"
	case 4:
		return "int"
	case 5:
		return "int"
	case 6:
		return "int"
	case 7:
		return "double"
	case 8:
		return "string"
	case 9:
		return "date"
	case 10:
		return "datetime"
	}
	return "unknown"
}

func GetTypeName(dtype int) string {
	return getTypeName(dtype)
}

// mainRuleName correspond to either a MainRuleFile (aka rule set) or a rule sequece (aka sequence of rule set)
func LoadJetRules(processName string, mainRuleName string, lookup_db_path string) (*JetStore, error) {
	log.Printf("** LoadJetRules Called process: %s, mainRule: %s", processName, mainRuleName)
	js := &JetStore{
		ProcessName:     processName,
		MainRuleName:    mainRuleName,
		LookupDbPath:    lookup_db_path,
	}
	var err error
	js.Factory, err = rete.NewReteMetaStoreFactory(js.MainRuleName)
	if err != nil {
		return nil, fmt.Errorf("while calling NewReteMetaStoreFactory(%s): %v", js.MainRuleName, err)
	}
	// NOTE: js.MetaStore is set when the rete session is created (in NewReteSession)
	// setting a default value to access Classes and Table information when preparing
	// the server process
	js.MetaStore = js.Factory.MetaStoreLookup[js.Factory.MainRuleFileNames[0]]
	js.MetaMgr = js.Factory.ResourceMgr
	js.MetaGraph = js.Factory.MetaGraph

	return js, nil
}

func (jr *JetStore) ReleaseJetRules() error {
	return nil
}

func (js *JetStore) NewRDFSession() (*RDFSession, error) {
	return &RDFSession{
		js:         js,
		rdfSession: rdf.NewRdfSession(js.MetaMgr, js.MetaGraph),
	}, nil
}

func (rdfs *RDFSession) ReleaseRDFSession() error {
	return nil
}

func (js *JetStore) NewReteSession(rdfSession *RDFSession, jetrulesName string) (*ReteSession, error) {

	reteSession := rete.NewReteSession(rdfSession.rdfSession)
	// Set the current meta store
	js.MetaStore = js.Factory.MetaStoreLookup[jetrulesName]
	if js.MetaStore == nil {
		return nil, fmt.Errorf("error: Rete Network for main rule %s not found", js.MainRuleName)
	}
	reteSession.Initialize(js.MetaStore)
	return &ReteSession{
		js:         js,
		rdfSession: rdfSession,
		reteSession: reteSession,
	}, nil
}

func (rs *ReteSession) ReleaseReteSession() error {
	rs.reteSession.Done()
	return nil
}

// create resources and literals from meta_graph
func (js *JetStore) NewNull() (*Resource, error) {
	return &Resource{
		r: rdf.Null(),
	}, nil
}
func (js *JetStore) NewBlankNode(v int) (*Resource, error) {
	return &Resource{
		r: js.MetaMgr.CreateBNode(v),
	}, nil
}
func (js *JetStore) NewResource(resource_name string) (*Resource, error) {
	return &Resource{
		r: js.MetaMgr.NewResource(resource_name),
	}, nil
}
func (js *JetStore) GetResource(resource_name string) (*Resource, error) {
	return &Resource{
		r: js.MetaMgr.GetResource(resource_name),
	}, nil
}
func (js *JetStore) NewTextLiteral(txt string) (*Resource, error) {
	return &Resource{
		r: js.MetaMgr.NewTextLiteral(txt),
	}, nil
}
func (js *JetStore) NewIntLiteral(value int) (*Resource, error) {
	return &Resource{
		r: js.MetaMgr.NewIntLiteral(value),
	}, nil
}
func (js *JetStore) NewUIntLiteral(value uint) (*Resource, error) {
	return &Resource{
		r: js.MetaMgr.NewIntLiteral(int(value)),
	}, nil
}
func (js *JetStore) NewLongLiteral(value int) (*Resource, error) {
	return &Resource{
		r: js.MetaMgr.NewIntLiteral(int(value)),
	}, nil
}
func (js *JetStore) NewULongLiteral(value uint) (*Resource, error) {
	return &Resource{
		r: js.MetaMgr.NewIntLiteral(int(value)),
	}, nil
}
func (js *JetStore) NewDoubleLiteral(value float64) (*Resource, error) {
	return &Resource{
		r: js.MetaMgr.NewDoubleLiteral(value),
	}, nil
}
func (js *JetStore) NewDateLiteral(value string) (*Resource, error) {
	ld, err := rdf.NewLDate(value)
	return &Resource{
		r: js.MetaMgr.NewDateLiteral(ld),
	}, err
}
func (js *JetStore) NewDatetimeLiteral(value string) (*Resource, error) {
	ld, err := rdf.NewLDatetime(value)
	return &Resource{
		r: js.MetaMgr.NewDatetimeLiteral(ld),
	}, err
}

// load process meta triples in meta graph
func (js *JetStore) LoadProcessMetaTriples(jetrules_name string, is_rule_set int) (int, error) {
	return 0, nil
}

// assert triple in meta graph
func (js *JetStore) InsertRuleConfig(s *Resource, p *Resource, o *Resource) (int, error) {
	if s == nil || p == nil || o == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling InsertRuleConfig")
	}
	_, err := js.MetaGraph.Insert(s.r, p.r, o.r)
	return 0, err
}

// New session-based Resource & Literals
// ------------------------------------
func (rs *ReteSession) NewNull() (*Resource, error) {
	return &Resource{
		r: rdf.Null(),
	}, nil
}
func (rs *ReteSession) NewResource(resource_name string) (*Resource, error) {
	return &Resource{
		r: rs.rdfSession.rdfSession.ResourceMgr.NewResource(resource_name),
	}, nil
}
func (rs *ReteSession) GetResource(resource_name string) (*Resource, error) {
	return &Resource{
		r: rs.rdfSession.rdfSession.ResourceMgr.GetResource(resource_name),
	}, nil
}
func (rs *ReteSession) NewTextLiteral(txt string) (*Resource, error) {
	return &Resource{
		r: rs.rdfSession.rdfSession.ResourceMgr.NewTextLiteral(txt),
	}, nil
}
func (rs *ReteSession) NewIntLiteral(value int) (*Resource, error) {
	return &Resource{
		r: rs.rdfSession.rdfSession.ResourceMgr.NewIntLiteral(value),
	}, nil
}
func (rs *ReteSession) NewUIntLiteral(value uint) (*Resource, error) {
	return &Resource{
		r: rs.rdfSession.rdfSession.ResourceMgr.NewIntLiteral(int(value)),
	}, nil
}
func (rs *ReteSession) NewLongLiteral(value int64) (*Resource, error) {
	return &Resource{
		r: rs.rdfSession.rdfSession.ResourceMgr.NewIntLiteral(int(value)),
	}, nil
}
func (rs *ReteSession) NewULongLiteral(value uint64) (*Resource, error) {
	return &Resource{
		r: rs.rdfSession.rdfSession.ResourceMgr.NewIntLiteral(int(value)),
	}, nil
}
func (rs *ReteSession) NewDoubleLiteral(value float64) (*Resource, error) {
	return &Resource{
		r: rs.rdfSession.rdfSession.ResourceMgr.NewDoubleLiteral(value),
	}, nil
}
func (rs *ReteSession) NewDateLiteral(value string) (*Resource, error) {
	ld, err := rdf.NewLDate(value)
	return &Resource{
		r: rs.rdfSession.rdfSession.ResourceMgr.NewDateLiteral(ld),
	}, err
}
func (rs *ReteSession) NewDatetimeLiteral(value string) (*Resource, error) {
	ld, err := rdf.NewLDatetime(value)
	return &Resource{
		r: rs.rdfSession.rdfSession.ResourceMgr.NewDatetimeLiteral(ld),
	}, err
}

// Get Resource & Literals properties
func (r *Resource) GetType() int {
	return r.r.GetType()
}

func (r *Resource) GetTypeName() string {
	return r.r.GetTypeName()
}

func (r *Resource) GetName() (string, error) {
	return r.r.Name(), nil
}

func (r *Resource) GetInt() (int, error) {
	v, ok := r.r.Value.(int)
	if !ok {
		return 0, ErrUnexpectedRdfType
	}
	return v, nil
}

func (r *Resource) GetDouble() (float64, error) {
	v, ok := r.r.Value.(float64)
	if !ok {
		return 0, ErrUnexpectedRdfType
	}
	return v, nil
}

func (r *Resource) GetDateIsoString() (string, error) {
	v, ok := r.r.Value.(rdf.LDate)
	if !ok {
		return "", ErrUnexpectedRdfType
	}	
	return v.Date.Format("2006-01-02T15:04:05Z"), nil
}

func (r *Resource) GetDatetimeIsoString() (string, error) {
	return r.GetDateIsoString()
}

func (r *Resource) GetDateDetails() (y int, m int, d int, err error) {
	v, ok := r.r.Value.(rdf.LDate)
	if !ok {
		return 0, 0, 0, ErrUnexpectedRdfType
	}	
	y = v.Date.Year()
	m = int(v.Date.Month())
	d = v.Date.Day()
	return
}

func (r *Resource) GetDatetimeDetails() (y, m, d, hr, min, sec, frac int, err error) {
	v, ok := r.r.Value.(rdf.LDate)
	if !ok {
		return 0, 0, 0, 0, 0, 0, 0, ErrUnexpectedRdfType
	}	
	y = v.Date.Year()
	m = int(v.Date.Month())
	d = v.Date.Day()
	hr = v.Date.Hour()
	min = v.Date.Minute()
	sec = v.Date.Second()
	frac = v.Date.Nanosecond()
	return
}
func (r *Resource) GetText() (string, error) {
	v, ok := r.r.Value.(string)
	if !ok {
		return "", ErrUnexpectedRdfType
	}	
	return v, nil
}

func (r *Resource) AsTextSilent() string {
	v, _ := r.r.Value.(string)
	return v
}

func (r *Resource) AsText() (string, error) {
	return r.r.String(), nil
}

func reportTypeError(r *Resource, columnType string) (ret interface{}, err error) {
	err = fmt.Errorf(
		"error: while saving entity, got a data property of type %s but the db schema is expecting %s",
		r.GetTypeName(), columnType)
	fmt.Println(err)
	return
}

func (r *Resource) AsInterface(columnType string) (ret interface{}, err error) {
	if r == nil {
		return ret, fmt.Errorf("error: null resource call AsInterface{}")
	}
	switch rtype := r.GetType(); rtype {
	case 0:
		return nil, nil
	// case 1:
	// 	return "BN:", nil
	case 2:
		v, err := r.GetName()
		if err != nil {
			fmt.Println("ERROR Can't get resource name", err)
			return ret, fmt.Errorf("while getting name of resource for AsInterface: %v", err)
		}
		if columnType != "text" {
			return reportTypeError(r, columnType)
		}
		return v, nil
	case 3:
		v, err := r.GetInt()
		if err != nil {
			fmt.Println("ERROR Can't GetInt", err)
			return ret, fmt.Errorf("while getting int value of literal for AsInterface: %v", err)
		}
		if columnType != "integer" && columnType != "int" {
			return reportTypeError(r, columnType)
		}
		return v, nil
	case 7:
		v, err := r.GetDouble()
		if err != nil {
			fmt.Println("ERROR Can't GetDouble", err)
			return ret, fmt.Errorf("while getting double value of literal for AsInterface: %v", err)
		}
		if columnType != "double precision" && columnType != "double" {
			return reportTypeError(r, columnType)
		}
		return v, nil
	case 8:
		v, err := r.GetText()
		if err != nil {
			fmt.Println("ERROR Can't GetText", err)
			return ret, fmt.Errorf("while getting text of literal for AsInterface: %v", err)
		}
		if columnType != "text" && columnType != "string" {
			return reportTypeError(r, columnType)
		}
		return v, nil
	case 9:
		y, m, d, err := r.GetDateDetails()
		if err != nil {
			return ret, fmt.Errorf("while getting date details: %v", err)
		}
		if columnType == "text" || columnType == "string" {
			return fmt.Sprintf("%d-%d-%d", y, m, d), nil
		}
		if columnType == "date" {
			return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC), nil
		}
		return reportTypeError(r, columnType)
	case 10:
		if columnType == "text" || columnType == "string" {
			v, err := r.GetDatetimeIsoString()
			if err != nil {
				return ret, fmt.Errorf("while getting datetime literal for AsInterface: %v", err)
			}
			return v, nil
		}
		if columnType == "datetime" {
			y, m, d, hr, min, sec, frac, err := r.GetDatetimeDetails()
			if err != nil {
				return ret, fmt.Errorf("while getting datetime details: %v", err)
			}
			return time.Date(y, time.Month(m), d, hr, min, sec, frac, time.UTC), nil
		}
		return reportTypeError(r, columnType)
	default:
		fmt.Printf("ERROR, Unexpected Resource type: %d\n", rtype)
		return ret, fmt.Errorf("error, unexpected resource type: %d", rtype)
	}
}

// ReteSession Insert
func (rs *RDFSession) Insert(s *Resource, p *Resource, o *Resource) (int, error) {
	if s == nil || p == nil || o == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Insert")
	}
	_, err := rs.rdfSession.Insert(s.r, p.r, o.r)
	return 0, err
}
func (rs *ReteSession) Insert(s *Resource, p *Resource, o *Resource) (int, error) {
	if s == nil || p == nil || o == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Insert")
	}
	_, err := rs.rdfSession.rdfSession.Insert(s.r, p.r, o.r)
	return 0, err
}

// ReteSession Contains
func (rs *RDFSession) Contains(s *Resource, p *Resource, o *Resource) (int, error) {
	if s == nil || p == nil || o == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Contains")
	}
	if rs.rdfSession.Contains(s.r, p.r, o.r) {
		return 1, nil
	}
	return 0, nil
}
func (rs *ReteSession) Contains(s *Resource, p *Resource, o *Resource) (int, error) {
	if s == nil || p == nil || o == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Contains")
	}
	if rs.rdfSession.rdfSession.Contains(s.r, p.r, o.r) {
		return 1, nil
	}
	return 0, nil
}
func (rs *RDFSession) ContainsSP(s *Resource, p *Resource) (int, error) {
	if s == nil || p == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Contains")
	}
	if rs.rdfSession.ContainsSP(s.r, p.r) {
		return 1, nil
	}
	return 0, nil
}
func (rs *ReteSession) ContainsSP(s *Resource, p *Resource) (int, error) {
	if s == nil || p == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Contains")
	}
	if rs.rdfSession.rdfSession.ContainsSP(s.r, p.r) {
		return 1, nil
	}
	return 0, nil
}

// ReteSession Erase
func (rs *RDFSession) Erase(s *Resource, p *Resource, o *Resource) (int, error) {
	var u, v, w *rdf.Node
	if s != nil {
		u = s.r
	}
	if p != nil {
		v = p.r
	}
	if o != nil {
		w = o.r
	}
	b, err := rs.rdfSession.Erase(u, v, w)
	if err != nil {
		return 0, err
	}
	if b {
		return 1, nil
	}
	return 0, nil
}
func (rs *ReteSession) Erase(s *Resource, p *Resource, o *Resource) (int, error) {
	return rs.rdfSession.Erase(s, p, o)
}

// ReteSession ExecuteRules
func (rs *ReteSession) ExecuteRules() (string, error) {
	err := rs.reteSession.ExecuteRules()
	if err != nil {
		return err.Error(), err
	}
	return "", nil
}

// RDFSession GetRdfGraph as text
// see c++: jets::rdf::RDFSession::get_graph_buf
// returns the rdf graph as list of triples (text buffer)
//*TODO rdf graph as list of triples (text buffer)
func (rs *RDFSession) GetRdfGraph() string {
	return ""
}
// RDFSession DumpRdfGraph
func DumpGraph(g *rdf.RdfGraph) {
	triples := make([]string, 0)
	t3Itor := g.Find()
	for t3 := range t3Itor.Itor {
		triples = append(triples, fmt.Sprintf("(%s, %s, %s)", t3[0], t3[1], t3[2]))
	}
	t3Itor.Done()
	sort.Slice(triples, func(i, j int) bool { return triples[i] < triples[j] })
	count := 0
	for i := range triples {
		log.Println(triples[i])
		count += 1
	}
}
func (rs *RDFSession) DumpRdfGraph() error {
	// log.Printf("Meta Graph Contains %d triples (go version):\n", rs.rdfSession.MetaGraph.Size())
	// DumpGraph(rs.rdfSession.MetaGraph)
	log.Printf("Asserted Graph Contains %d triples (go version):", rs.rdfSession.AssertedGraph.Size())
	DumpGraph(rs.rdfSession.AssertedGraph)
	log.Printf("Inferred Graph Contains %d triples (go version):", rs.rdfSession.InferredGraph.Size())
	DumpGraph(rs.rdfSession.InferredGraph)
	log.Printf("The Meta Graph contains %d triples",rs.rdfSession.MetaGraph.Size())
	return nil
}
func (rs *ReteSession) DumpRdfGraph() error {
	return rs.rdfSession.DumpRdfGraph()
}

func (rs *ReteSession) DumpVertexVisit() {
	for i := range rs.reteSession.VertexVisits {
		vv := &rs.reteSession.VertexVisits[i]
		if len(vv.Label) > 0 && vv.InferCount > 0 {
			log.Printf("Rules %s, inferred: %d, retracted: %d", vv.Label, vv.InferCount, vv.RetractCount)
		}
	}
}

// ReteSession FindAll
func (rs *RDFSession) FindAll() (*RSIterator, error) {
	return rs.rdfSession.Find(), nil
}
func (rs *ReteSession) FindAll() (*RSIterator, error) {
	return rs.rdfSession.FindAll()
}

// ReteSession Find
func (rs *RDFSession) Find(s *Resource, p *Resource, o *Resource) (*RSIterator, error) {
	var u, v, w *rdf.Node
	if s != nil {
		u = s.r
	}
	if p != nil {
		v = p.r
	}
	if o != nil {
		w = o.r
	}
	return rs.rdfSession.FindSPO(u, v, w), nil
}
func (rs *ReteSession) Find(s *Resource, p *Resource, o *Resource) (*RSIterator, error) {
	return rs.rdfSession.Find(s, p, o)
}

func (rs *RDFSession) Find_s(s *Resource) (*RSIterator, error) {
	return rs.rdfSession.FindS(s.r), nil
}
func (rs *ReteSession) Find_s(s *Resource) (*RSIterator, error) {
	return rs.rdfSession.Find_s(s)
}

func (rs *RDFSession) Find_sp(s *Resource, p *Resource) (*RSIterator, error) {
	return rs.rdfSession.FindSP(s.r, p.r), nil
}
func (rs *ReteSession) Find_sp(s *Resource, p *Resource) (*RSIterator, error) {
	return rs.rdfSession.Find_sp(s, p)
}

func (rs *RDFSession) GetObject(s *Resource, p *Resource) (*Resource, error) {
	r := rs.rdfSession.GetObject(s.r, p.r)
	if r == nil {
		return nil, nil
	}
	return &Resource{
		r: r,
	}, nil
}
func (rs *ReteSession) GetObject(s *Resource, p *Resource) (*Resource, error) {
	return rs.rdfSession.GetObject(s, p)
}
