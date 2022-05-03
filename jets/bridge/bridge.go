package bridge

import (
	"errors"
	"fmt"
	"strconv"
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
}

type ReteSession struct {
	hdl C.HJRETE
	jetrules_name string
}
type RSIterator struct {
	hdl C.HJITERATOR
}

type Resource struct {
	hdl C.HJR
}

var (
	NotDate = errors.New("not a date")
	NotValidDate = errors.New("not a valid date")
	NotValidDateTime = errors.New("not a valid datetime")
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

func LoadJetRules(rete_db_path string, lookup_db_path string) (*JetStore, error) {
	var js JetStore
	cstr := C.CString(rete_db_path)
	lk_cstr := C.CString(lookup_db_path)
	ret := int(C.create_jetstore_hdl(cstr, lk_cstr, &js.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in LoadJetRules, ret code", ret)
		return &js, errors.New("ERROR calling LoadJetRules()! ")
	}
	C.free(unsafe.Pointer(cstr)) 
	C.free(unsafe.Pointer(lk_cstr)) 
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

func (jr *JetStore) NewReteSession(jetrules_name string) (*ReteSession, error) {
	var rs ReteSession
	cstr := C.CString(jetrules_name)
	ret := int(C.create_rete_session(jr.hdl, cstr, &rs.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewReteSession ret code", ret)
		return &rs, errors.New("ERROR calling NewReteSession(), ret code: " + fmt.Sprint(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
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
	ret := int(C.create_null(js.hdl, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in mete_graph.createResource ret code", ret)
		return &r, errors.New("ERROR calling meta createResource(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewBlankNode(v int) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_blanknode(js.hdl, C.int(v), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in mete_graph.createResource ret code", ret)
		return &r, errors.New("ERROR calling meta createResource(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewResource(resource_name string) (*Resource, error) {
	var r Resource
	cstr := C.CString(resource_name)
	ret := int(C.create_meta_resource(js.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in mete_graph.createResource ret code", ret)
		return &r, errors.New("ERROR calling meta createResource(), ret code: "+fmt.Sprint(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return &r, nil
}
func (js *JetStore) GetResource(resource_name string) (*Resource, error) {
	var r Resource
	cstr := C.CString(resource_name)
	ret := int(C.get_meta_resource(js.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in mete_graph.getResource ret code", ret)
		return &r, errors.New("ERROR calling meta getResource(), ret code: "+fmt.Sprint(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return &r, nil
}
func (js *JetStore) NewTextLiteral(txt string) (*Resource, error) {
	var r Resource
	cstr := C.CString(txt)
	ret := int(C.create_meta_text(js.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewTextLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewTextLiteral(), ret code: "+fmt.Sprint(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return &r, nil
}
func (js *JetStore) NewIntLiteral(value int) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_int(js.hdl, C.int(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewIntLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewIntLiteral(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewUIntLiteral(value uint) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_uint(js.hdl, C.uint(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewUIntLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewUIntLiteral(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewLongLiteral(value int) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_long(js.hdl, C.long(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewLongLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewLongLiteral(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewULongLiteral(value uint) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_ulong(js.hdl, C.ulong(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewULongLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewULongLiteral(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewDoubleLiteral(value float64) (*Resource, error) {
	var r Resource
	ret := int(C.create_meta_double(js.hdl, C.double(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewDoubleLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewDoubleLiteral(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (js *JetStore) NewDateLiteral(value string) (*Resource, error) {
	var r Resource
	cstr := C.CString(value)
	ret := int(C.create_meta_date(js.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewDateLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewDateLiteral(), ret code: "+fmt.Sprint(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return &r, nil
}
func (js *JetStore) NewDatetimeLiteral(value string) (*Resource, error) {
	var r Resource
	cstr := C.CString(value)
	ret := int(C.create_meta_datetime(js.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewDatetimeLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewDatetimeLiteral(), ret code: "+fmt.Sprint(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return &r, nil
}

// assert triple in meta graph
func (js *JetStore) InsertRuleConfig(s *Resource, p *Resource, o *Resource) (int, error) {
	if s==nil || p==nil || o==nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling InsertRuleConfig")
	}
	ret := int(C.insert_meta_graph(js.hdl, s.hdl, p.hdl, o.hdl))
	if ret < 0 {
		fmt.Println("ERROR in JetStore.InsertRuleConfig ret code", ret)
		return ret, errors.New("ERROR calling Insert(), ret code: "+fmt.Sprint(ret))
	}
	return ret, nil
}


// New session-based Resource & Literals
func (rs *ReteSession) NewResource(resource_name string) (*Resource, error) {
	var r Resource
	cstr := C.CString(resource_name)
	ret := int(C.create_resource(rs.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewResource ret code", ret)
		return &r, errors.New("ERROR calling NewResource(), ret code: "+fmt.Sprint(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return &r, nil
}
func (rs *ReteSession) GetResource(resource_name string) (*Resource, error) {
	var r Resource
	cstr := C.CString(resource_name)
	ret := int(C.get_resource(rs.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in GetResource ret code", ret)
		return &r, errors.New("ERROR calling GetResource(), ret code: "+fmt.Sprint(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return &r, nil
}
func (rs *ReteSession) NewTextLiteral(txt string) (*Resource, error) {
	var r Resource
	cstr := C.CString(txt)
	ret := int(C.create_text(rs.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewTextLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewTextLiteral(), ret code: "+fmt.Sprint(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return &r, nil
}
func (rs *ReteSession) NewIntLiteral(value int) (*Resource, error) {
	var r Resource
	ret := int(C.create_int(rs.hdl, C.int(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewIntLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewIntLiteral(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewUIntLiteral(value uint) (*Resource, error) {
	var r Resource
	ret := int(C.create_uint(rs.hdl, C.uint(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewUIntLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewUIntLiteral(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewLongLiteral(value int) (*Resource, error) {
	var r Resource
	ret := int(C.create_long(rs.hdl, C.long(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewLongLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewLongLiteral(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewULongLiteral(value uint) (*Resource, error) {
	var r Resource
	ret := int(C.create_ulong(rs.hdl, C.ulong(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewULongLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewULongLiteral(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewDoubleLiteral(value float64) (*Resource, error) {
	var r Resource
	ret := int(C.create_double(rs.hdl, C.double(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewDoubleLiteral ret code", ret)
		return &r, errors.New("ERROR calling NewDoubleLiteral(), ret code: "+fmt.Sprint(ret))
	}
	return &r, nil
}
func (rs *ReteSession) NewDateLiteral(value string) (*Resource, error) {
	var r Resource
	cstr := C.CString(value)
	ret := int(C.create_date(rs.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in create_date ret code", ret)
		return &r, errors.New("ERROR calling create_date(), ret code: "+fmt.Sprint(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return &r, nil
}
func (rs *ReteSession) NewDatetimeLiteral(value string) (*Resource, error) {
	var r Resource
	cstr := C.CString(value)
	ret := int(C.create_datetime(rs.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in create_datetime ret code", ret)
		return &r, errors.New("ERROR calling create_datetime(), ret code: "+fmt.Sprint(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
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

func (r *Resource) GetName() (string, error) {
	// rdf_named_resource_t
	if r.GetType() != 2 {
		return "", errors.New("ERROR GetName applies to resources only")
	}
	name := C.GoString(C.go_get_resource_name(r.hdl))
	return name, nil
}

func (r *Resource) GetInt() (int, error) {
	// rdf_literal_int32_t
	if r.GetType() != 3 {
		return 0, errors.New("ERROR GetInt applies to resources only")
	}
	var cint C.int
	ret := int(C.get_int_literal(r.hdl, &cint))
	if ret != 0 {
		fmt.Println("ERROR in GetInt ret code", ret)
		return 0, errors.New("ERROR calling GetInt(), ret code: "+fmt.Sprint(ret))
	}
	return int(cint), nil
}

func (r *Resource) GetDateIsoString() string {
	// rdf_literal_date_t
	if r.GetType() != 9 {
		return ""
	}
	return C.GoString(C.go_date_iso_string(r.hdl))
}

func (r *Resource) GetDatetimeIsoString() string {
	// rdf_literal_date_t
	if r.GetType() != 10 {
		return ""
	}
	return C.GoString(C.go_datetime_iso_string(r.hdl))
}

func (r *Resource) GetDateDetails() (y int, m int, d int, err error) {
	// rdf_literal_date_t
	if r.GetType() != 9 {
		err = NotDate
		return 
	}
	var yptr, mptr, dptr *C.int
	ret := int(C.get_date_details(r.hdl, yptr, mptr, dptr))
	if ret == -2 {
		fmt.Println("ERROR in GetDateDetails: date is not a valid date")
		err = NotValidDate
		return 
	}
	y = int(*yptr)
	m = int(*mptr)
	d = int(*dptr)
	return 
}

func (r *Resource) GetText() (string, error) {
	// rdf_literal_string_t
	if r.GetType() != 8 {
		return "", errors.New("ERROR GetText applies to text literal only")
	}
	return C.GoString(C.go_get_text_literal(r.hdl)), nil
}

func (r *Resource) AsText() string {
	if r == nil {
		return "NULL"
	}
	switch rtype := r.GetType(); rtype {
	case 0: return "NULL"
	case 1: return "BN:"
	case 2: 
		v, err := r.GetName()
		if err != nil {
			fmt.Println("ERROR Can't GetName", err)
			return "ERROR!"
		}
		return v
	case 3: 
		v, err := r.GetInt()
		if err != nil {
			fmt.Println("ERROR Can't GetInt", err)
		}
		return strconv.Itoa(v)
	case 8: 
		v, err := r.GetText()
		if err != nil {
			fmt.Println("ERROR Can't GetText", err)
		}
		return v
	case 9: 
		// y, m, d, err := r.GetDateDetails()
		// if err == NotDate {
		// 	return "not a date"
		// }
		// if err == NotValidDate {
		// 	return "not a valid date"
		// }
		// return fmt.Sprintf("%d-%d-%d", y, m, d)
		return r.GetDateIsoString()
	case 10: 
		return r.GetDatetimeIsoString()
	default:
		fmt.Printf("ERROR, Unexpected Resource type: %d\n", rtype)
		return "ERROR!"
	}
}

func (r *Resource) AsInterface() interface{} {
	if r == nil {
		return nil
	}
	switch rtype := r.GetType(); rtype {
	case 0: return nil
	case 1: return "BN:"
	case 2: 
		v, err := r.GetName()
		if err != nil {
			fmt.Println("ERROR Can't GetName", err)
			return "ERROR!"
		}
		return v
	case 3: 
		v, err := r.GetInt()
		if err != nil {
			fmt.Println("ERROR Can't GetInt", err)
		}
		return v
	case 8: 
		v, err := r.GetText()
		if err != nil {
			fmt.Println("ERROR Can't GetText", err)
		}
		return v
	case 9: 
		return r.GetDateIsoString()
	case 10: 
		return r.GetDatetimeIsoString()
	default:
		fmt.Printf("ERROR, Unexpected Resource type: %d\n", rtype)
		return "ERROR!"
	}
}

// ReteSession Insert
func (rs *ReteSession) Insert(s *Resource, p *Resource, o *Resource) (int, error) {
	if s==nil || p==nil || o==nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Insert")
	}
	ret := int(C.insert(rs.hdl, s.hdl, p.hdl, o.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ReteSession.Insert ret code", ret)
		return ret, errors.New("ERROR calling Insert(), ret code: "+fmt.Sprint(ret))
	}
	return ret, nil
}
// ReteSession Contains
func (rs *ReteSession) Contains(s *Resource, p *Resource, o *Resource) (int, error) {
	if s==nil || p==nil || o==nil {
		return 0, fmt.Errorf("ERROR cannot have null args when calling Contains")
	}
	ret := int(C.contains(rs.hdl, s.hdl, p.hdl, o.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ReteSession.Contains ret code", ret)
		return ret, errors.New("ERROR calling Contains(), ret code: "+fmt.Sprint(ret))
	}
	return ret, nil
}
// ReteSession ExecuteRules
func (rs *ReteSession) ExecuteRules() error {
	ret := int(C.execute_rules(rs.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ReteSession.ExecuteRules ret code", ret)
		return errors.New("ERROR calling ExecuteRules(), ret code: "+fmt.Sprint(ret))
	}
	return nil
}
// ReteSession DumpRdfGraph
func (rs *ReteSession) DumpRdfGraph() error {
	ret := int(C.dump_rdf_graph(rs.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ReteSession.DumpRdfGraph ret code", ret)
		return errors.New("ERROR calling DumpRdfGraph(), ret code: "+fmt.Sprint(ret))
	}
	return nil
}
// ReteSession FindAll
func (rs *ReteSession) FindAll() (*RSIterator, error) {
	var itor RSIterator
	ret := int(C.find_all(rs.hdl, &itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ReteSession.FindAll ret code", ret)
		return &itor, errors.New("ERROR calling FindAll(), ret code: "+string(rune(ret)))
	}
	return &itor, nil
}
// ReteSession Find
func (rs *ReteSession) Find(s *Resource, p *Resource, o *Resource) (*RSIterator, error) {
	var itor RSIterator
	var cs, cp, co C.HJR
	if s != nil {cs = s.hdl}
	if p != nil {cp = p.hdl}
	if o != nil {co = o.hdl}

	ret := int(C.find(rs.hdl, cs, cp, co, &itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ReteSession.Find ret code", ret)
		return &itor, errors.New("ERROR calling Find(), ret code: "+string(rune(ret)))
	}
	return &itor, nil
}
func (rs *ReteSession) Find_s(s *Resource) (*RSIterator, error) {
	var itor RSIterator
	if s == nil {
		return &itor, fmt.Errorf("ERROR cannot have null args when calling Find_s")
	}

	ret := int(C.find_s(rs.hdl, s.hdl, &itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ReteSession.Find ret code", ret)
		return &itor, errors.New("ERROR calling Find(), ret code: "+string(rune(ret)))
	}
	return &itor, nil
}
func (rs *ReteSession) Find_sp(s *Resource, p *Resource) (*RSIterator, error) {
	var itor RSIterator
	if s == nil || p == nil {
		return &itor, fmt.Errorf("ERROR cannot have null args when calling Find_sp")
	}

	ret := int(C.find_sp(rs.hdl, s.hdl, p.hdl, &itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ReteSession.Find_sp ret code", ret)
		return &itor, errors.New("ERROR calling Find_sp(), ret code: "+string(rune(ret)))
	}
	return &itor, nil
}
func (rs *ReteSession) GetObject(s *Resource, p *Resource) (*Resource, error) {
	var obj Resource
	if s == nil || p == nil {
		return &obj, fmt.Errorf("ERROR cannot have null args when calling GetObject")
	}
	ret := int(C.find_object(rs.hdl, s.hdl, p.hdl, &obj.hdl));
	if ret < 0 {
		fmt.Println("ERROR in GetObject ret code", ret)
		return &obj, errors.New("ERROR calling GetObject(), ret code: "+string(rune(ret)))
	}
	if obj.hdl == nil {
		return nil, nil
	}
	return &obj, nil
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
func (itor *RSIterator)ReleaseIterator() error {
	ret := int(C.dispose(itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ReleaseIterator ret code", ret)
		return errors.New("ERROR calling ReleaseIterator(), ret code: "+fmt.Sprint(ret))
	}
	return nil
}
