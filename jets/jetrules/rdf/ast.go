package rdf

import (
	"fmt"
	"time"
)

// Package to define the rdf data model as an pragmatic ast

type Node struct {
	Value interface{}
}

func (v *Node) Key() int {
	vv, ok := v.Value.(BlankNode)
	if ok {
		return vv.key
	}
	return 0
}

func (v *Node) Name() string {
	switch vv := v.Value.(type) {
	case BlankNode:
		return fmt.Sprintf("BN%d", vv.key)
	case NamedResource:
		return vv.name
	default:
		return ""
	}
}

func (v *Node) IsNull() bool {
	_, ok := v.Value.(RdfNull)
	if ok {
		return true
	}
	return false
}

func (v *Node) Bool() bool {
	switch vv := v.Value.(type) {
	case BlankNode:
		return true
	case NamedResource:
		return true
	case LDate:
		return true
	case LDatetime:
		return true
	case int32:
		return vv != 0
	case int64:
		return vv != 0
	case float64:
		return vv != 0
	case string:
		return vv != ""
	default:
		return false
	}
}

func (v *Node) String() string {
	switch vv := v.Value.(type) {
	case BlankNode:
		return fmt.Sprintf("BN%d", vv.key)
	case NamedResource:
		return vv.name
	case LDate:
		return fmt.Sprintf("%v", vv)
	case LDatetime:
		return fmt.Sprintf("%v", vv)
	case int:
		return fmt.Sprintf("%v", vv)
	case int32:
		return fmt.Sprintf("%v", vv)
	case int64:
		return fmt.Sprintf("%v", vv)
	case float64:
		return fmt.Sprintf("%v", vv)
	case string:
		return vv
	default:
		return "??"
	}
}

// type Node interface {
// 	Key() int
// 	Name() string
// 	Bool() bool
// 	Value() interface{}
// }

// From c++ implementation:
// bool operator()(RDFNull       const& )const{return false;}
// bool operator()(BlankNode     const&v)const{return true;}
// bool operator()(NamedResource const&v)const{return true;}
// bool operator()(LDate         const&v)const{return true;}
// bool operator()(LDatetime     const&v)const{return true;}
// bool operator()(LInt32        const&v)const{return v.data;}
// bool operator()(LInt64        const&v)const{return v.data;}
// bool operator()(LDouble       const&v)const{return v.data;}
// bool operator()(LString       const&v)const
// Omit unsigned integrals:
// bool operator()(LUInt32       const&v)const{return v.data;}
// bool operator()(LUInt64       const&v)const{return v.data;}

func Null() *Node {
	return &Node{Value: &RdfNull{}}
}

func BN(k int) *Node {
	return &Node{Value: BlankNode{key: k}}
}

func R(name string) *Node {
	return &Node{Value: NamedResource{name: name}}
}

func D(date string) (*Node, error) {
	t, err := ParseDate(date)
	return &Node{Value: LDate{date: t}}, err
}

func DT(datetime string) (*Node, error) {
	t, err := ParseDatetime(datetime)
	return &Node{Value: LDatetime{datetime: t}}, err
}

func I(v int32) *Node {
	return &Node{Value: v}
}

func L(v int64) *Node {
	return &Node{Value: v}
}

func S(v string) *Node {
	return &Node{Value: v}
}

type RdfNull struct{}

// func (r *RdfNull) Key() int {
// 	return 0
// }
// func (r *RdfNull) Name() string {
// 	return ""
// }
// func (r *RdfNull) Bool() bool {
// 	return false
// }
// func (r *RdfNull) Value() interface{} {
// 	return r
// }

type BlankNode struct {
	key int
}

// func (r *BlankNode) Key() int {
// 	return r.key
// }
// func (r *BlankNode) Name() string {
// 	return ""
// }
// func (r *BlankNode) Bool() bool {
// 	return r.key != 0
// }
// func (r *BlankNode) Value() interface{} {
// 	return r
// }

type NamedResource struct {
	name string
}

// func (r *NamedResource) Key() int {
// 	return 0
// }
// func (r *NamedResource) Name() string {
// 	return r.name
// }
// func (r *NamedResource) Bool() bool {
// 	return r.name != ""
// }
// func (r *NamedResource) Value() interface{} {
// 	return r
// }

type LDate struct {
	date *time.Time
}

// func (r *LDate) Key() int {
// 	return 0
// }
// func (r *LDate) Name() string {
// 	return ""
// }
// func (r *LDate) Bool() bool {
// 	return r != nil
// }
// func (r *LDate) Value() interface{} {
// 	return r.date
// }

type LDatetime struct {
	datetime *time.Time
}

// func (r *LDatetime) Key() int {
// 	return 0
// }
// func (r *LDatetime) Name() string {
// 	return ""
// }
// func (r *LDatetime) Bool() bool {
// 	return r != nil
// }
// func (r *LDatetime) Value() interface{} {
// 	return r.datetime
// }

// type LInt32 struct{
// 	data int32
// }
// func (r *LInt32) Key() int {
// 	return 0
// }
// func (r *LInt32) Name() string {
// 	return ""
// }
// func (r *LInt32) Bool() bool {
// 	return r.data != 0
// }
// func (r *LInt32) Value() interface{} {
// 	return r.data
// }

// type LInt64 struct{
// 	data int64
// }
// func (r *LInt64) Key() int {
// 	return 0
// }
// func (r *LInt64) Name() string {
// 	return ""
// }
// func (r *LInt64) Bool() bool {
// 	return r.data != 0
// }
// func (r *LInt64) Value() interface{} {
// 	return r.data
// }

// type LDouble struct{
// 	data float64
// }
// func (r *LDouble) Key() int {
// 	return 0
// }
// func (r *LDouble) Name() string {
// 	return ""
// }
// func (r *LDouble) Bool() bool {
// 	return r.data != 0
// }
// func (r *LDouble) Value() interface{} {
// 	return r.data
// }

// type LString struct{
// 	data string
// }
// func (r *LString) Key() int {
// 	return 0
// }
// func (r *LString) Name() string {
// 	return ""
// }
// func (r *LString) Bool() bool {
// 	return r.data != ""
// }
// func (r *LString) Value() interface{} {
// 	return r.data
// }
