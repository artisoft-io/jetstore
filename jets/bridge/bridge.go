package bridge

import (
	"errors"
	"fmt"
	"strconv"
	"unsafe"
)

// #cgo CFLAGS: -I/home/michel/projects/repos/jetstore/jets
// #cgo LDFLAGS: -L/home/michel/projects/repos/jetstore/build/jets -ljets -lsqlite3
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

func LoadJetRules(rete_db_path string) (JetStore, error) {
	var js JetStore
	cstr := C.CString(rete_db_path)
	ret := int(C.create_jetstore_hdl(cstr, &js.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in LoadJetRules, ret code", ret)
		return js, errors.New("ERROR calling LoadJetRules()! ")
	}
	C.free(unsafe.Pointer(cstr)) 
	return js, nil
}

func ReleaseJetRules(jr JetStore) error {
	ret := int(C.delete_jetstore_hdl(jr.hdl))
	if ret != 0 {
		fmt.Println("OOps got error in c++ ReleaseJetRules!!")
		return errors.New("error calling ReleaseJetRules(), ret code: " + string(ret))
	}
	return nil
}

func NewReteSession(jr JetStore, jetrules_name string) (ReteSession, error) {
	var rs ReteSession
	cstr := C.CString(jetrules_name)
	ret := int(C.create_rete_session(jr.hdl, cstr, &rs.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewReteSession ret code", ret)
		return rs, errors.New("ERROR calling NewReteSession(), ret code: "+string(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return rs, nil
}

func ReleaseReteSession(rs ReteSession) error {
	ret := int(C.delete_rete_session(rs.hdl))
	if ret != 0 {
		fmt.Println("OOps got error in c++ ReleaseReteSession!! ret code", ret)
		return errors.New("error calling ReleaseReteSession(), ret code: " + string(ret))
	}
	return nil
}

// New Resource & Literals
func NewResource(rs ReteSession, resource_name string) (Resource, error) {
	var r Resource
	cstr := C.CString(resource_name)
	ret := int(C.create_resource(rs.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewResource ret code", ret)
		return r, errors.New("ERROR calling NewResource(), ret code: "+string(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return r, nil
}
func NewTextLiteral(rs ReteSession, txt string) (Resource, error) {
	var r Resource
	cstr := C.CString(txt)
	ret := int(C.create_text(rs.hdl, cstr, &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewTextLiteral ret code", ret)
		return r, errors.New("ERROR calling NewTextLiteral(), ret code: "+string(ret))
	}
	C.free(unsafe.Pointer(cstr)) 
	return r, nil
}
func NewIntLiteral(rs ReteSession, value int) (Resource, error) {
	var r Resource
	ret := int(C.create_int(rs.hdl, C.int(value), &r.hdl))
	if ret != 0 {
		fmt.Println("Yikes got error in NewIntLiteral ret code", ret)
		return r, errors.New("ERROR calling NewIntLiteral(), ret code: "+string(ret))
	}
	return r, nil
}

// Get Resource & Literals properties
func (r Resource) GetType() int {
	ret := int(C.get_resource_type(r.hdl))
	if ret < 0 {
		fmt.Println("ERROR calling GetType(), ret code:", ret)
		return ret
	}
	return ret
}

func (r Resource) GetName() (string, error) {
	// rdf_named_resource_t
	if r.GetType() != 2 {
		return "", errors.New("ERROR GetName applies to resources only")
	}
	name := C.GoString(C.go_get_resource_name(r.hdl))
	return name, nil
}

func (r Resource) GetInt() (int, error) {
	// rdf_literal_int32_t
	if r.GetType() != 3 {
		return 0, errors.New("ERROR GetInt applies to resources only")
	}
	var ptr *C.int
	ret := int(C.get_int_literal(r.hdl, ptr))
	if ret != 0 {
		fmt.Println("ERROR in GetInt ret code", ret)
		return 0, errors.New("ERROR calling GetInt(), ret code: "+string(ret))
	}
	return int(*ptr), nil
}

func (r Resource) GetText() (string, error) {
	// rdf_literal_string_t
	if r.GetType() != 8 {
		return "", errors.New("ERROR GetText applies to resources only")
	}
	return C.GoString(C.go_get_text_literal(r.hdl)), nil
}

func (r Resource) AsText() string {
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
	default:
		fmt.Printf("ERROR, Unexpected Resource type: %d.\n", rtype)
		return "ERROR!"
	}
}

// ReteSession Insert
func (rs ReteSession) Insert(s Resource, p Resource, o Resource) (int, error) {
	ret := int(C.insert(rs.hdl, s.hdl, p.hdl, o.hdl))
	if ret < 0 {
		fmt.Println("ERROR in Insert ret code", ret)
		return ret, errors.New("ERROR calling Insert(), ret code: "+string(ret))
	}
	return ret, nil
}
// ReteSession Contains
func (rs ReteSession) Contains(s Resource, p Resource, o Resource) (int, error) {
	ret := int(C.contains(rs.hdl, s.hdl,p.hdl, o.hdl))
	if ret < 0 {
		fmt.Println("ERROR in Contains ret code", ret)
		return ret, errors.New("ERROR calling Contains(), ret code: "+string(ret))
	}
	return ret, nil
}
// ReteSession ExecuteRules
func (rs ReteSession) ExecuteRules() error {
	ret := int(C.execute_rules(rs.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ExecuteRules ret code", ret)
		return errors.New("ERROR calling ExecuteRules(), ret code: "+string(ret))
	}
	return nil
}
// ReteSession FindAll
func (rs ReteSession) FindAll() (RSIterator, error) {
	var itor RSIterator
	ret := int(C.find_all(rs.hdl, &itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in FindAll ret code", ret)
		return itor, errors.New("ERROR calling FindAll(), ret code: "+string(rune(ret)))
	}
	return itor, nil
}
// RSIterator IsEnd
func (itor RSIterator) IsEnd() bool {
	ret := int(C.is_end(itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in IsEnd ret code", ret)
		return false
	}
	return ret > 0
}
// RSIterator Next
func (itor RSIterator) Next() bool {
	ret := int(C.next(itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in Next ret code", ret)
		return false
	}
	return ret > 0
}
// RSIterator GetSubject
func (itor RSIterator) GetSubject() Resource {
	var subject Resource
	ret := int(C.get_subject(itor.hdl, &subject.hdl))
	if ret < 0 {
		fmt.Println("ERROR in GetSubject ret code", ret)
		return subject
	}
	return subject
}
// RSIterator GetPredicate
func (itor RSIterator) GetPredicate() Resource {
	var predicate Resource
	ret := int(C.get_predicate(itor.hdl, &predicate.hdl))
	if ret < 0 {
		fmt.Println("ERROR in GetPredicate ret code", ret)
		return predicate
	}
	return predicate
}
// RSIterator GetObject
func (itor RSIterator) GetObject() Resource {
	var object Resource
	ret := int(C.get_object(itor.hdl, &object.hdl))
	if ret < 0 {
		fmt.Println("ERROR in GetObject ret code", ret)
		return object
	}
	return object
}
// ReteSession ReleaseIterator
func ReleaseIterator(itor RSIterator) error {
	ret := int(C.dispose(itor.hdl))
	if ret < 0 {
		fmt.Println("ERROR in ReleaseIterator ret code", ret)
		return errors.New("ERROR calling ReleaseIterator(), ret code: "+string(ret))
	}
	return nil
}
