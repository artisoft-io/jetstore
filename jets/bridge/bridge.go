package bridge

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	// "strings"
	"unsafe"
)

// #cgo CFLAGS: -I/home/michel/projects/repos/jetstore/jets -I/usr/local/go/src/jetstore/jets
// #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/build/jets -L/usr/local/go/build/jets -ljets -lsqlite3
// #cgo LDFLAGS: -labsl_city -labsl_low_level_hash -labsl_raw_hash_set
// #include "rete/jets_rete_cwrapper.h"
import "C"

type JetStore struct {
	hdl C.HJETS
	process_name string
}

type RDFSession struct {
	hdl C.HJRDF
}

type ReteSession struct {
	hdl           C.HJRETE
	rdfs          *RDFSession
	jetrules_name string
}
type RSIterator struct {
	hdl C.HJITERATOR
}

type Resource struct {
	hdl C.HJR
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
	switch (dtype) {
		case 0  : return "null";
		case 1  : return "blank_node";
		case 2  : return "named_resource";
		case 3  : return "int32";
		case 4  : return "uint32";
		case 5  : return "int64";
		case 6  : return "uint64";
		case 7  : return "double";
		case 8  : return "string";
		case 9  : return "date";
		case 10 : return "datetime";
	}
	return "unknown";
}

func LoadJetRules(process_name string, rete_db_path string, lookup_db_path string) (*JetStore, error) {
	var js JetStore
	js.process_name = process_name
	cstr := C.CString(rete_db_path)
	defer C.free(unsafe.Pointer(cstr))
	lk_cstr := C.CString(lookup_db_path)
	defer C.free(unsafe.Pointer(lk_cstr))
	ret := int(C.create_jetstore_hdl(cstr, lk_cstr, &js.hdl))
	if ret != 0 {
		fmt.Println("Error in LoadJetRules, ret code", ret)
		if ret < -99 && ret > -200 {
			return &js, ErrLookupTable
		}
		return &js, errors.New("error loading workspace, see logs")
	}
	return &js, nil
}

func (jr *JetStore) ReleaseJetRules() error {
	ret := int(C.delete_jetstore_hdl(jr.hdl))
	if ret != 0 {
		fmt.Println("OOps got error in c++ ReleaseJetRules!!")
		return errors.New("error calling ReleaseJetRules(), ret code: " + fmt.Sprint(ret))
	}
	return nil
}

func (jr *JetStore) NewRDFSession() (*RDFSession, error) {
	var rdfs RDFSession
	ret := int(C.create_rdf_session(jr.hdl, &rdfs.hdl))
	if ret != 0 {
		fmt.Println("Got error in NewRDFSession ret code", ret)
		return &rdfs, errors.New("ERROR calling NewRDFSession(), ret code: " + fmt.Sprint(ret))
	}
	return &rdfs, nil
}

func (rdfs *RDFSession) ReleaseRDFSession() error {
	ret := int(C.delete_rdf_session(rdfs.hdl))
	if ret != 0 {
		fmt.Println("OOps got error in c++ ReleaseRDFSession!! ret code", ret)
		return errors.New("error calling ReleaseRDFSession(), ret code: " + fmt.Sprint(ret))
	}
	return nil
}

func (jr *JetStore) NewReteSession(rdfSession *RDFSession, jetrules_name string) (*ReteSession, error) {
	var rs ReteSession
	cstr := C.CString(jetrules_name)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.create_rete_session(jr.hdl, rdfSession.hdl, cstr, &rs.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewReteSession ret code", ret)
		return &rs, errors.New("ERROR calling NewReteSession(), ret code: " + fmt.Sprint(ret))
	}
	rs.rdfs = rdfSession
	return &rs, nil
}

func (rs *ReteSession) ReleaseReteSession() error {
	ret := int(C.delete_rete_session(rs.hdl))
	if ret != 0 {
		fmt.Println("OOps got error in c++ ReleaseReteSession!! ret code", ret)
		return errors.New("error calling ReleaseReteSession(), ret code: " + fmt.Sprint(ret))
	}
	return nil
}

// create resources and literals from meta_graph
func (js *JetStore) NewNull() (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_null(js.hdl, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in mete_graph.createResource ret code", ret)
		return &r, errors.New("ERROR calling meta createResource(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewBlankNode(v int) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_blanknode(js.hdl, C.int(v), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in mete_graph.createResource ret code", ret)
		return &r, errors.New("ERROR calling meta createResource(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewResource(resource_name string) (*Resource, error) {
	var r Resource
	cstr := C.CString(resource_name)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.create_meta_resource(js.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in mete_graph.createResource ret code", ret)
		return &r, errors.New("ERROR calling meta createResource(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) GetResource(resource_name string) (*Resource, error) {
	var r Resource
	cstr := C.CString(resource_name)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.get_meta_resource(js.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in mete_graph.getResource ret code", ret)
		return &r, errors.New("ERROR calling meta getResource(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewTextLiteral(txt string) (*Resource, error) {
	var r Resource
	cstr := C.CString(txt)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.create_meta_text(js.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewTextLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewTextLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewIntLiteral(value int) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_int(js.hdl, C.int(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewIntLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewIntLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewUIntLiteral(value uint) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_uint(js.hdl, C.uint(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewUIntLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewUIntLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewLongLiteral(value int) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_long(js.hdl, C.long(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewLongLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewLongLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewULongLiteral(value uint) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_ulong(js.hdl, C.ulong(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewULongLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewULongLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewDoubleLiteral(value float64) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_double(js.hdl, C.double(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewDoubleLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewDoubleLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewDateLiteral(value string) (*Resource, error) {
	var r Resource
	cstr := C.CString(value)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.create_meta_date(js.hdl, cstr, &r.hdl))
	if ret == -2 {
		return &r, ErrNotValidDate
	}
	if ret != 0 {
		fmt.Println("Yikes got error in NewDateLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewDateLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewDatetimeLiteral(value string) (*Resource, error) {
	var r Resource
	cstr := C.CString(value)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.create_meta_datetime(js.hdl, cstr, &r.hdl))
	if ret == -2 {
		return &r, ErrNotValidDateTime
	}
	if ret != 0 {
		fmt.Println("Yikes got error in NewDatetimeLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewDatetimeLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}

// load process meta triples in meta graph
func (js *JetStore) LoadProcessMetaTriples(jetrules_name string, is_rule_set int) (int, error) {
	cstr := C.CString(jetrules_name)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.load_process_meta_triples(cstr, C.int(is_rule_set), js.hdl))
	if ret < 0 {
		fmt.Println("ERROR in JetStore.LoadProcessMetaTriples ret code", ret)
		return ret, errors.New("ERROR calling LoadProcessMetaTriples(), ret code: " + fmt.Sprint(ret))
	}
	return ret, nil
}

// assert triple in meta graph
func (js *JetStore) InsertRuleConfig(s *Resource, p *Resource, o *Resource) (int, error) {
	if s == nil || p == nil || o == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling InsertRuleConfig")
	}
	ret := int(C.insert_meta_graph(js.hdl, s.hdl, p.hdl, o.hdl))
	if ret < 0 {
		fmt.Println("ERROR in JetStore.InsertRuleConfig ret code", ret)
		return ret, errors.New("ERROR calling InsertRuleConfig(), ret code: " + fmt.Sprint(ret))
	}
	return ret, nil
}

// New session-based Resource & Literals
func (rs *ReteSession) NewNull() (*Resource, error) {
	var r Resource
	ret := int(C.create_null(rs.rdfs.hdl, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewResource ret code", ret)
		return &r, errors.New("ERROR calling NewResource(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewResource(resource_name string) (*Resource, error) {
	var r Resource
	cstr := C.CString(resource_name)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.create_resource(rs.rdfs.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewResource ret code", ret)
		return &r, errors.New("ERROR calling NewResource(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) GetResource(resource_name string) (*Resource, error) {
	var r Resource
	cstr := C.CString(resource_name)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.get_resource(rs.rdfs.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in GetResource ret code", ret)
		return &r, errors.New("ERROR calling GetResource(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewTextLiteral(txt string) (*Resource, error) {
	var r Resource
	cstr := C.CString(txt)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.create_text(rs.rdfs.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewTextLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewTextLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewIntLiteral(value int) (*Resource, error) {
	var r Resource
	ret := int(C.create_int(rs.rdfs.hdl, C.int(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewIntLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewIntLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewUIntLiteral(value uint) (*Resource, error) {
	var r Resource
	ret := int(C.create_uint(rs.rdfs.hdl, C.uint(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewUIntLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewUIntLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewLongLiteral(value int64) (*Resource, error) {
	var r Resource
	ret := int(C.create_long(rs.rdfs.hdl, C.long(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewLongLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewLongLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewULongLiteral(value uint64) (*Resource, error) {
	var r Resource
	ret := int(C.create_ulong(rs.rdfs.hdl, C.ulong(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewULongLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewULongLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewDoubleLiteral(value float64) (*Resource, error) {
	var r Resource
	ret := int(C.create_double(rs.rdfs.hdl, C.double(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewDoubleLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewDoubleLiteral(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewDateLiteral(value string) (*Resource, error) {
	var r Resource
	cstr := C.CString(value)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.create_date(rs.rdfs.hdl, cstr, &r.hdl))
	if ret == -2 {
		// fmt.Println("ERROR in NewDateLiteral: date is not a valid date")
		return &r, ErrNotValidDate
	}
	if ret != 0 {
		fmt.Println("Yikes got error in create_date ret code", ret)
		return &r, errors.New("ERROR calling create_date(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewDatetimeLiteral(value string) (*Resource, error) {
	var r Resource
	cstr := C.CString(value)
	defer C.free(unsafe.Pointer(cstr))
	ret := int(C.create_datetime(rs.rdfs.hdl, cstr, &r.hdl))
	if ret == -2 {
		// fmt.Println("ERROR in NewDatetimeLiteral: datetime is not valid")
		return &r, ErrNotValidDateTime
	}
	if ret != 0 {
		fmt.Println("Yikes got error in create_datetime ret code", ret)
		return &r, errors.New("ERROR calling create_datetime(), ret code: " + fmt.Sprint(ret))
	}
	return &r, nil
}

// Get Resource & Literals properties
func (r *Resource) GetType() int {
	ret := int(C.get_resource_type(r.hdl))
	if ret < 0 {
		fmt.Println("ERROR calling GetType(), ret code:", ret)
		return ret
	}
	return ret
}

func (r *Resource) GetTypeName() string {
	ret := int(C.get_resource_type(r.hdl))
	if ret < 0 {
		fmt.Println("ERROR calling GetType(), ret code:", ret)
		return ""
	}
	return getTypeName(ret)
}

func (r *Resource) GetName() (string, error) {
	// rdf_named_resource_t
	if r.GetType() != 2 {
		return "", ErrUnexpectedRdfType
	}
	var cret C.int
	sp := C.get_resource_name2(r.hdl, &cret)
	if int(cret) != 0 {
		fmt.Println("ERROR getting resource name", int(cret))
		return "", fmt.Errorf("error while getting resource name: %v", int(cret))
	}
	return C.GoString(sp), nil
}

func (r *Resource) GetInt() (int, error) {
	// rdf_literal_int32_t
	if r.GetType() != 3 {
		return 0, errors.New("ERROR GetInt applies to int literal only")
	}
	var cint C.int
	ret := int(C.get_int_literal(r.hdl, &cint))
	if ret != 0 {
		fmt.Println("ERROR in GetInt ret code", ret)
		return 0, errors.New("ERROR calling GetInt(), ret code: " + fmt.Sprint(ret))
	}
	return int(cint), nil
}

func (r *Resource) GetDouble() (float64, error) {
	// rdf_literal_double_t
	if r.GetType() != 7 {
		return 0, errors.New("ERROR GetDouble applies to double literal only")
	}
	var cdbl C.double
	ret := int(C.get_double_literal(r.hdl, &cdbl))
	if ret != 0 {
		fmt.Println("ERROR in GetInt ret code", ret)
		return 0, errors.New("ERROR calling GetDouble(), ret code: " + fmt.Sprint(ret))
	}
	return float64(cdbl), nil
}

func (r *Resource) GetDateIsoString() (string, error) {
	// rdf_literal_date_t
	if r.GetType() != 9 {
		return "", ErrUnexpectedRdfType
	}
	var cret C.int
	sp := C.get_date_iso_string2(r.hdl, &cret)
	ret := int(cret)
	if ret == -2 {
		return "", ErrNotValidDate
	}
	if ret != 0 {
		fmt.Println("ERROR getting date in iso str format", ret)
		return "", fmt.Errorf("error while date in iso str format: %v", ret)
	}
	return C.GoString(sp), nil
}

func (r *Resource) GetDatetimeIsoString() (string, error) {
	// rdf_literal_date_t
	if r.GetType() != 10 {
		return "", ErrUnexpectedRdfType
	}
	var cret C.int
	sp := C.get_datetime_iso_string2(r.hdl, &cret)
	ret := int(cret)
	if ret == -2 {
		return "", ErrNotValidDateTime
	}
	if ret != 0 {
		fmt.Println("ERROR getting datetime in iso str format", ret)
		return "", fmt.Errorf("error while datetime in iso str format: %v", ret)
	}
	return C.GoString(sp), nil
}

func (r *Resource) GetDateDetails() (y int, m int, d int, err error) {
	// rdf_literal_date_t
	if r.GetType() != 9 {
		err = ErrNotValidDate
		return
	}
	var cy, cm, cd C.int
	ret := int(C.get_date_details(r.hdl, &cy, &cm, &cd))
	if ret == -2 {
		// fmt.Println("ERROR in GetDateDetails: date is not a valid date")
		err = ErrNotValidDate
		return
	}
	y = int(cy)
	m = int(cm)
	d = int(cd)
	return
}

func (r *Resource) GetDatetimeDetails() (y, m, d, hr, min, sec, frac int, err error) {
	// rdf_literal_datetime_t
	if r.GetType() != 10 {
		err = ErrNotValidDateTime
		return
	}
	var cy, cm, cd, chr, cmin, csec, cfrac C.int
	ret := int(C.get_datetime_details(r.hdl, &cy, &cm, &cd, &chr, &cmin, &csec, &cfrac))
	if ret == -2 {
		// fmt.Println("ERROR in GetDatetimeDetails: date is not a valid date")
		err = ErrNotValidDate
		return
	}
	y = int(cy)
	m = int(cm)
	d = int(cd)
	hr = int(hr)
	min = int(cmin)
	sec = int(csec)
	frac = int(cfrac)
	return
}
func (r *Resource) GetText() (string, error) {
	// rdf_literal_string_t
	if r.GetType() != 8 {
		return "", errors.New("ERROR GetText applies to text literal only")
	}
	var cret C.int
	sp := C.get_text_literal2(r.hdl, &cret)
	ret := int(cret)
	if ret != 0 {
		fmt.Println("ERROR getting literal text value:", ret)
		return "", fmt.Errorf("error getting literal text value: %v", ret)
	}
	return C.GoString(sp), nil
}

func (r *Resource) AsTextSilent() string {
	txt,_ := r.AsText()
	return txt
}

func (r *Resource) AsText() (string, error) {
	if r == nil {
		return "NULL", nil
	}
	switch rtype := r.GetType(); rtype {
	case 0:
		return "NULL", nil
	case 1:
		return "BN:", nil
	case 2:
		v, err := r.GetName()
		if err != nil {
			fmt.Println("ERROR Can't GetName", err)
			return "", fmt.Errorf("error getting resource name: %v", err)
		}
		return v, nil
	case 3:
		v, err := r.GetInt()
		if err != nil {
			fmt.Println("ERROR Can't GetInt", err)
			return "", fmt.Errorf("error getting literal int value: %v", err)
		}
		return strconv.Itoa(v), nil
	case 7:
		v, err := r.GetDouble()
		if err != nil {
			fmt.Println("ERROR Can't GetDouble", err)
			return "", fmt.Errorf("error getting literal double value: %v", err)
		}
		return strconv.FormatFloat(v, 'f', 2, 64), nil
	case 8:
		v, err := r.GetText()
		if err != nil {
			fmt.Println("ERROR Can't GetText", err)
			return "", fmt.Errorf("error getting literal text value: %v", err)
		}
		return v, nil
	case 9:
		return r.GetDateIsoString()
	case 10:
		return r.GetDatetimeIsoString()
	default:
		fmt.Printf("ERROR, Unexpected Resource type: %d\n", rtype)
		return "", fmt.Errorf("error unexpected resource type: %v", rtype)
	}
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
		if columnType != "integer" {
			return reportTypeError(r, columnType)
		}
		return v, nil
	case 7:
		v, err := r.GetDouble()
		if err != nil {
			fmt.Println("ERROR Can't GetDouble", err)
			return ret, fmt.Errorf("while getting double value of literal for AsInterface: %v", err)
		}
		if columnType != "double precision" {
			return reportTypeError(r, columnType)
		}
		return v, nil
	case 8:
		v, err := r.GetText()
		if err != nil {
			fmt.Println("ERROR Can't GetText", err)
			return ret, fmt.Errorf("while getting text of literal for AsInterface: %v", err)
		}
		if columnType != "text" {
			return reportTypeError(r, columnType)
		}
		return v, nil
	case 9:
		y, m, d, err := r.GetDateDetails()
		if err != nil {
			return ret, fmt.Errorf("while getting date details: %v", err)
		}
		if columnType == "text" {
			return fmt.Sprintf("%d-%d-%d", y, m, d), nil
		}
		if columnType == "date" {
			return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC), nil
		}
		return reportTypeError(r, columnType)
	case 10:
		if columnType == "text" {
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
	ret := int(C.insert(rs.hdl, s.hdl, p.hdl, o.hdl))
	if ret < 0 {
		fmt.Println("ERROR in RDFSession.Insert ret code", ret)
		return ret, errors.New("ERROR calling Insert(), ret code: " + fmt.Sprint(ret))
	}
	return ret, nil
}
func (rs *ReteSession) Insert(s *Resource, p *Resource, o *Resource) (int, error) {
	if s == nil || p == nil || o == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Insert")
	}
	return rs.rdfs.Insert(s, p, o)
}

// ReteSession Contains
func (rs *RDFSession) Contains(s *Resource, p *Resource, o *Resource) (int, error) {
	if s == nil || p == nil || o == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Contains")
	}
	ret := int(C.contains(rs.hdl, s.hdl, p.hdl, o.hdl))
	if ret < 0 {
		fmt.Println("ERROR in RDFSession.Contains ret code", ret)
		return ret, errors.New("ERROR calling Contains(), ret code: " + fmt.Sprint(ret))
	}
	return ret, nil
}
func (rs *ReteSession) Contains(s *Resource, p *Resource, o *Resource) (int, error) {
	if s == nil || p == nil || o == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Contains")
	}
	return rs.rdfs.Contains(s, p, o)
}
func (rs *RDFSession) ContainsSP(s *Resource, p *Resource) (int, error) {
	if s == nil || p == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Contains")
	}
	ret := int(C.contains_sp(rs.hdl, s.hdl, p.hdl))
	if ret < 0 {
		fmt.Println("ERROR in RDFSession.ContainsSP ret code", ret)
		return ret, errors.New("ERROR calling Contains(), ret code: " + fmt.Sprint(ret))
	}
	return ret, nil
}
func (rs *ReteSession) ContainsSP(s *Resource, p *Resource) (int, error) {
	if s == nil || p == nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Contains")
	}
	return rs.rdfs.ContainsSP(s, p)
}

// ReteSession Erase
func (rs *RDFSession) Erase(s *Resource, p *Resource, o *Resource) (int, error) {
	var s_hdl, p_hdl, o_hdl C.HJR
	if s != nil {
		s_hdl = s.hdl
	}
	if p != nil {
		p_hdl = p.hdl
	}
	if o != nil {
		o_hdl = o.hdl
	}
	ret := int(C.erase(rs.hdl, s_hdl, p_hdl, o_hdl))
	if ret < 0 {
		fmt.Println("ERROR in RDFSession.Erase ret code", ret)
		return ret, errors.New("ERROR calling Erase(), ret code: " + fmt.Sprint(ret))
	}
	return ret, nil
}
func (rs *ReteSession) Erase(s *Resource, p *Resource, o *Resource) (int, error) {
	return rs.rdfs.Erase(s, p, o)
}

// ReteSession ExecuteRules
func (rs *ReteSession) ExecuteRules() (string, error) {
	var cret C.int
	sp := C.execute_rules2(rs.hdl, &cret)
	ret := int(cret)
	if ret != 0 {
		fmt.Println("ERROR calling execute rules:", ret)
		return C.GoString(sp), fmt.Errorf("error during execute rules in c: %v", ret)
	}
	return "", nil
}

// RDFSession GetRdfGraph as text
func (rs *RDFSession) GetRdfGraph() string {
	var cret C.int
	sp := C.get_rdf_graph_txt(rs.hdl, &cret)
	ret := int(cret)
	if ret != 0 {
		fmt.Println("ERROR calling C.get_rdf_graph_txt:", ret)
		return ""
	}
	return C.GoString(sp)
}

// RDFSession DumpRdfGraph
func (rs *RDFSession) DumpRdfGraph() error {
	ret := int(C.dump_rdf_graph(rs.hdl))
	if ret < 0 {
		fmt.Println("ERROR in RDFSession.DumpRdfGraph ret code", ret)
		return errors.New("ERROR calling DumpRdfGraph(), ret code: " + fmt.Sprint(ret))
	}
	return nil
}
func (rs *ReteSession) DumpRdfGraph() error {
	return rs.rdfs.DumpRdfGraph()
}

// ReteSession FindAll
func (rs *RDFSession) FindAll() (*RSIterator, error) {
	var itor RSIterator
	ret := int(C.find_all(rs.hdl, &itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in RDFSession.FindAll ret code", ret)
		return &itor, errors.New("ERROR calling FindAll(), ret code: " + string(rune(ret)))
	}
	return &itor, nil
}
func (rs *ReteSession) FindAll() (*RSIterator, error) {
	return rs.rdfs.FindAll()
}

// ReteSession Find
func (rs *RDFSession) Find(s *Resource, p *Resource, o *Resource) (*RSIterator, error) {
	var itor RSIterator
	var cs, cp, co C.HJR
	if s != nil {
		cs = s.hdl
	}
	if p != nil {
		cp = p.hdl
	}
	if o != nil {
		co = o.hdl
	}
	ret := int(C.find(rs.hdl, cs, cp, co, &itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in RDFSession.Find ret code", ret)
		return &itor, errors.New("ERROR calling Find(), ret code: " + string(rune(ret)))
	}
	return &itor, nil
}
func (rs *ReteSession) Find(s *Resource, p *Resource, o *Resource) (*RSIterator, error) {
	return rs.rdfs.Find(s, p, o)
}

func (rs *RDFSession) Find_s(s *Resource) (*RSIterator, error) {
	var itor RSIterator
	if s == nil {
		return &itor, fmt.Errorf("ERROR cannot have null args when calling Find_s")
	}
	ret := int(C.find_s(rs.hdl, s.hdl, &itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in RDFSession.Find ret code", ret)
		return &itor, errors.New("ERROR calling Find(), ret code: " + string(rune(ret)))
	}
	return &itor, nil
}
func (rs *ReteSession) Find_s(s *Resource) (*RSIterator, error) {
	return rs.rdfs.Find_s(s)
}

func (rs *RDFSession) Find_sp(s *Resource, p *Resource) (*RSIterator, error) {
	var itor RSIterator
	if s == nil || p == nil {
		return &itor, fmt.Errorf("ERROR cannot have null args when calling Find_sp")
	}
	ret := int(C.find_sp(rs.hdl, s.hdl, p.hdl, &itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in RDFSession.Find_sp ret code", ret)
		return &itor, errors.New("ERROR calling Find_sp(), ret code: " + string(rune(ret)))
	}
	return &itor, nil
}
func (rs *ReteSession) Find_sp(s *Resource, p *Resource) (*RSIterator, error) {
	return rs.rdfs.Find_sp(s, p)
}

func (rs *RDFSession) GetObject(s *Resource, p *Resource) (*Resource, error) {
	var obj Resource
	if s == nil || p == nil {
		return &obj, fmt.Errorf("ERROR cannot have null args when calling GetObject")
	}
	ret := int(C.find_object(rs.hdl, s.hdl, p.hdl, &obj.hdl))
	if ret < 0 {
		fmt.Println("ERROR in GetObject ret code", ret)
		return &obj, errors.New("ERROR calling GetObject(), ret code: " + string(rune(ret)))
	}
	if obj.hdl == nil {
		return nil, nil
	}
	return &obj, nil
}
func (rs *ReteSession) GetObject(s *Resource, p *Resource) (*Resource, error) {
	return rs.rdfs.GetObject(s, p)
}

// RSIterator IsEnd
func (itor *RSIterator) IsEnd() bool {
	ret := int(C.is_end(itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in IsEnd ret code", ret)
		return false
	}
	return ret > 0
}

// RSIterator Next
func (itor *RSIterator) Next() bool {
	ret := int(C.next(itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in Next ret code", ret)
		return false
	}
	return ret > 0
}

// RSIterator GetSubject
func (itor *RSIterator) GetSubject() *Resource {
	var subject Resource
	ret := int(C.get_subject(itor.hdl, &subject.hdl))
	if ret < 0 {
		fmt.Println("ERROR in GetSubject ret code", ret)
		return &subject
	}
	return &subject
}

// RSIterator GetPredicate
func (itor *RSIterator) GetPredicate() *Resource {
	var predicate Resource
	ret := int(C.get_predicate(itor.hdl, &predicate.hdl))
	if ret < 0 {
		fmt.Println("ERROR in GetPredicate ret code", ret)
		return &predicate
	}
	return &predicate
}

// RSIterator GetObject
func (itor *RSIterator) GetObject() *Resource {
	var object Resource
	ret := int(C.get_object(itor.hdl, &object.hdl))
	if ret < 0 {
		fmt.Println("ERROR in GetObject ret code", ret)
		return &object
	}
	return &object
}

// ReteSession ReleaseIterator
func (itor *RSIterator) ReleaseIterator() error {
	ret := int(C.dispose(itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ReleaseIterator ret code", ret)
		return errors.New("ERROR calling ReleaseIterator(), ret code: " + fmt.Sprint(ret))
	}
	return nil
}
