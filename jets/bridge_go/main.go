package bridge

import (
	"errors"
	"fmt"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

type JetStore struct {
	factory         *rete.ReteMetaStoreFactory
	metaStore       *rete.ReteMetaStore
	metaMgr         *rdf.ResourceManager
	metaGraph       *rdf.RdfGraph
	processName     string
	mainRuleName    string
	workspaceDbPath string
	lookupDbPath    string
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
type RSIterator struct {
	t3Itor *rdf.RdfSessionIteratorAdaptor
}

type Resource struct {
	r *rdf.Node
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

func LoadJetRules(processName string, mainRuleName string, rete_db_path string, lookup_db_path string) (*JetStore, error) {
	js := &JetStore{
		processName:     processName,
		mainRuleName:    mainRuleName,
		workspaceDbPath: rete_db_path,
		lookupDbPath:    lookup_db_path,
	}
	var err error
	js.factory, err = rete.NewReteMetaStoreFactory(js.mainRuleName)
	if err != nil {
		return nil, fmt.Errorf("while calling NewReteMetaStoreFactory(%s): %v", js.mainRuleName, err)
	}
	js.metaStore = js.factory.MetaStoreLookup[js.mainRuleName]
	if js.metaStore == nil {
		return nil, fmt.Errorf("error: Rete Network for main rule %s not found", js.mainRuleName)
	}
	return js, nil
}

func (jr *JetStore) ReleaseJetRules() error {
	return nil
}

func (js *JetStore) NewRDFSession() (*RDFSession, error) {
	return &RDFSession{
		js:         js,
		rdfSession: rdf.NewRdfSession(js.metaMgr, js.metaGraph),
	}, nil
}

func (rdfs *RDFSession) ReleaseRDFSession() error {
	return nil
}

func (js *JetStore) NewReteSession(rdfSession *RDFSession, jetrules_name string) (*ReteSession, error) {

	return &ReteSession{
		js:         js,
		rdfSession: rdfSession,
		reteSession: rete.NewReteSession(rdfSession.rdfSession),
	}, nil
}

func (rs *ReteSession) ReleaseReteSession() error {
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
		r: js.metaMgr.CreateBNode(v),
	}, nil
}
func (js *JetStore) NewResource(resource_name string) (*Resource, error) {
	return &Resource{
		r: js.metaMgr.NewResource(resource_name),
	}, nil
}
func (js *JetStore) GetResource(resource_name string) (*Resource, error) {
	return &Resource{
		r: js.metaMgr.GetResource(resource_name),
	}, nil
}
func (js *JetStore) NewTextLiteral(txt string) (*Resource, error) {
	return &Resource{
		r: js.metaMgr.NewTextLiteral(txt),
	}, nil
}
func (js *JetStore) NewIntLiteral(value int) (*Resource, error) {
	return &Resource{
		r: js.metaMgr.NewIntLiteral(value),
	}, nil
}
func (js *JetStore) NewUIntLiteral(value uint) (*Resource, error) {
	return &Resource{
		r: js.metaMgr.NewIntLiteral(int(value)),
	}, nil
}
func (js *JetStore) NewLongLiteral(value int) (*Resource, error) {
	return &Resource{
		r: js.metaMgr.NewIntLiteral(int(value)),
	}, nil
}
func (js *JetStore) NewULongLiteral(value uint) (*Resource, error) {
	return &Resource{
		r: js.metaMgr.NewIntLiteral(int(value)),
	}, nil
}
func (js *JetStore) NewDoubleLiteral(value float64) (*Resource, error) {
	return &Resource{
		r: js.metaMgr.NewDoubleLiteral(value),
	}, nil
}
func (js *JetStore) NewDateLiteral(value string) (*Resource, error) {
	ld, err := rdf.NewLDate(value)
	return &Resource{
		r: js.metaMgr.NewDateLiteral(ld),
	}, err
}
func (js *JetStore) NewDatetimeLiteral(value string) (*Resource, error) {
	ld, err := rdf.NewLDatetime(value)
	return &Resource{
		r: js.metaMgr.NewDatetimeLiteral(ld),
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
	_, err := js.metaGraph.Insert(s.r, p.r, o.r)
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
	b, err := rs.rdfSession.Erase(s.r, p.r, o.r)
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
func (rs *RDFSession) DumpRdfGraph() error {
	//*TODO dumpt to output the rdf graph
	return nil
}
func (rs *ReteSession) DumpRdfGraph() error {
	return rs.rdfSession.DumpRdfGraph()
}

// ReteSession FindAll
func (rs *RDFSession) FindAll() (*RSIterator, error) {
	return &RSIterator{
		t3Itor: rdf.NewRdfSessionIteratorAdaptor(rs.rdfSession.Find()),
	}, nil
}
func (rs *ReteSession) FindAll() (*RSIterator, error) {
	return rs.rdfSession.FindAll()
}

// ReteSession Find
func (rs *RDFSession) Find(s *Resource, p *Resource, o *Resource) (*RSIterator, error) {
	return &RSIterator{
		t3Itor: rdf.NewRdfSessionIteratorAdaptor(rs.rdfSession.FindSPO(s.r, p.r, o.r)),
	}, nil
}
func (rs *ReteSession) Find(s *Resource, p *Resource, o *Resource) (*RSIterator, error) {
	return rs.rdfSession.Find(s, p, o)
}

func (rs *RDFSession) Find_s(s *Resource) (*RSIterator, error) {
	return &RSIterator{
		t3Itor: rdf.NewRdfSessionIteratorAdaptor(rs.rdfSession.FindS(s.r)),
	}, nil
}
func (rs *ReteSession) Find_s(s *Resource) (*RSIterator, error) {
	return rs.rdfSession.Find_s(s)
}

func (rs *RDFSession) Find_sp(s *Resource, p *Resource) (*RSIterator, error) {
	return &RSIterator{
		t3Itor: rdf.NewRdfSessionIteratorAdaptor(rs.rdfSession.FindSP(s.r, p.r)),
	}, nil
}
func (rs *ReteSession) Find_sp(s *Resource, p *Resource) (*RSIterator, error) {
	return rs.rdfSession.Find_sp(s, p)
}

func (rs *RDFSession) GetObject(s *Resource, p *Resource) (*Resource, error) {
	return &Resource{
		r: rs.rdfSession.GetObject(s.r, p.r),
	}, nil
}
func (rs *ReteSession) GetObject(s *Resource, p *Resource) (*Resource, error) {
	return rs.rdfSession.GetObject(s, p)
}

// RSIterator IsEnd
func (itor *RSIterator) IsEnd() bool {	
	return itor.t3Itor.IsEnd()
}

// RSIterator Next
func (itor *RSIterator) Next() bool {
	return itor.t3Itor.Next()
}

// RSIterator GetSubject
func (itor *RSIterator) GetSubject() *Resource {
	t3 := itor.t3Itor.Triple()
	return &Resource{
		r: (*t3)[0],
	}
}

// RSIterator GetPredicate
func (itor *RSIterator) GetPredicate() *Resource {
	t3 := itor.t3Itor.Triple()
	return &Resource{
		r: (*t3)[1],
	}
}

// RSIterator GetObject
func (itor *RSIterator) GetObject() *Resource {
	t3 := itor.t3Itor.Triple()
	return &Resource{
		r: (*t3)[2],
	}
}

// ReteSession ReleaseIterator
func (itor *RSIterator) ReleaseIterator() error {
	itor.t3Itor.Done()
	return nil
}
