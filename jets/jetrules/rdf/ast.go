package rdf

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Package to define the rdf data model as an pragmatic ast

var globalNull *Node

func init() {
	globalNull = &Node{Value: &RdfNull{}}
}

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
	return ok
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
	case int:
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
	case float64:
		return fmt.Sprintf("%v", vv)
	case string:
		return vv
	default:
		return "??"
	}
}

func (v *Node) MarshalBinary() ([]byte, error) {
	if v == nil {
		return nil, fmt.Errorf("error: MarshalBinary called with null rdf.Node")
	}
	switch vv := v.Value.(type) {
	case BlankNode:
		// int is 8 bytes
		return []byte{
			'B', 
			byte(vv.key >> 56), 
			byte(vv.key >> 48),
			byte(vv.key >> 40),
			byte(vv.key >> 32),
			byte(vv.key >> 24),
			byte(vv.key >> 16),
			byte(vv.key >> 8),
			byte(vv.key),
		}, nil
	case NamedResource:
		return append([]byte(vv.name), 'R'), nil
	case LDate:
		md, err := vv.date.MarshalBinary()
		if err == nil {
			md = append(md, 'D')
		}
		return md, err
	case LDatetime:
		mt, err := vv.datetime.MarshalBinary()
		if err == nil {
			mt = append(mt, 'T')
		}
		return mt, err
	case int:
		// int is 8 bytes
		return []byte{
			'I','0','0',
			byte(vv >> 56), 
			byte(vv >> 48),
			byte(vv >> 40),
			byte(vv >> 32),
			byte(vv >> 24),
			byte(vv >> 16),
			byte(vv >> 8),
			byte(vv),
		}, nil
	case float64:
		// float64 -> uint64 is 8 bytes
		t := math.Float64bits(vv)
		return []byte{
			'F','6','4',
			byte(t >> 56), 
			byte(t >> 48),
			byte(t >> 40),
			byte(t >> 32),
			byte(t >> 24),
			byte(t >> 16),
			byte(t >> 8),
			byte(t),
		}, nil
	case string:
		return append([]byte(vv), 'S'), nil
	default:
		return nil, fmt.Errorf("error: unknown type for rdf.Node in MarshalBinary")
	}
}

type Triple = [3]*Node

func T3(s, p, o *Node) Triple {
	return Triple{s, p, o}
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
// bool operator()(LInt          const&v)const{return v.data;}
// bool operator()(LDouble       const&v)const{return v.data;}
// bool operator()(LString       const&v)const

func Null() *Node {
	return globalNull
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

func I(v int) *Node {
	return &Node{Value: v}
}

func S(v string) *Node {
	return &Node{Value: v}
}

func F(v float64) *Node {
	return &Node{Value: v}
}

type RdfNull struct{}

func NewRdfNull() RdfNull {
	return RdfNull{}
}


type BlankNode struct {
	key int
}


type NamedResource struct {
	name string
}


type LDate struct {
	date *time.Time
}

func NewLDate(date string) (LDate, error) {
	t, err := ParseDate(date)
	return LDate{date: t}, err
}

func (lhs LDate) Add (days int) LDate {
	t := lhs.date.Add(time.Duration(days) * 24 * time.Hour)
	return LDate{date: &t}
}

type LDatetime struct {
	datetime *time.Time
}

func NewLDatetime(datetime string) (LDatetime, error) {
	t, err := ParseDatetime(datetime)
	return LDatetime{datetime: t}, err
}

func (lhs LDatetime) Add (days int) LDatetime {
	t := lhs.datetime.Add(time.Duration(days) * 24 * time.Hour)
	return LDatetime{datetime: &t}
}

func ParseBool(value string) int {
	switch strings.ToLower(value) {
	case "true","t","1":
		return 1
	default:
		return 0
	}
}