package rdf

import (
	"fmt"
	"log"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Package to define the rdf data model as an pragmatic ast

var globalNull *Node
var globalNan *Node
var globalInf *Node

func init() {
	globalNull = &Node{Value: RdfNull{}}
	globalNan = &Node{Value: math.NaN()}
	globalInf = &Node{Value: math.Inf(0)}
}

type Node struct {
	Value interface{}
}

func (v *Node) Key() int {
	if v == nil {
		return 0
	}
	vv, ok := v.Value.(BlankNode)
	if ok {
		return vv.Key
	}
	return 0
}

func (v *Node) Name() string {
	if v == nil {
		return ""
	}
	switch vv := v.Value.(type) {
	case BlankNode:
		return fmt.Sprintf("BN%d", vv.Key)
	case NamedResource:
		return vv.Name
	default:
		return ""
	}
}

func (v *Node) IsNull() bool {
	if v == nil {
		return true
	}
	_, ok := v.Value.(RdfNull)
	return ok
}

func (v *Node) IsLiteral() bool {
	if v == nil {
		return false
	}
	switch reflect.TypeOf(v.Value).Kind() {
	case reflect.Int, reflect.Float64, reflect.String:
		return true
	default:
		return false
	}
}

// returns true if v is a Resource or BlankNode
func (v *Node) IsResource() bool {
	if v == nil {
		return false
	}
	switch reflect.TypeOf(v.Value) {
	case reflect.TypeOf(NamedResource{}), reflect.TypeOf(BlankNode{}):
		return true
	default:
		return false
	}
}

func (v *Node) GetType() int {
	if v == nil {
		return 0
	}
	switch v.Value.(type) {
	case RdfNull:
		return 0
	case BlankNode:
		return 1
	case NamedResource:
		return 2
	case LDate:
		return 9
	case LDatetime:
		return 10
	case int:
		return 5
	case uint:
		return 6
	case float64:
		return 7
	case string:
		return 8
	default:
		return 0
	}
}

func (v *Node) GetTypeName() string {
	if v == nil {
		return "null"
	}
	switch v.Value.(type) {
	case RdfNull:
		return "rdf_null_type"
	case BlankNode:
		return "blank_node"
	case NamedResource:
		return "named_resource"
	case LDate:
		return "date"
	case LDatetime:
		return "datetime"
	case int:
		return "int"
	case uint:
		return "uint"
	case float64:
		return "double"
	case string:
		return "string"
	default:
		return "unknown"
	}
}

func (v *Node) Bool() bool {
	if v == nil {
		return false
	}
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
	case uint:
		return vv != 0
	case float64:
		return !NearlyEqual(vv, 0)
	case string:
		return vv != ""
	default:
		return false
	}
}

func (v *Node) String() string {
	if v == nil {
		return "null"
	}
	switch vv := v.Value.(type) {
	case BlankNode:
		return fmt.Sprintf("BN%d", vv.Key)
	case NamedResource:
		return vv.Name
	case LDate:
		return fmt.Sprintf("%v", vv)
	case LDatetime:
		return fmt.Sprintf("%v", vv)
	case int:
		return strconv.Itoa(vv)
	case uint:
		return strconv.FormatUint(uint64(vv), 10)
	case float64:
		return fmt.Sprintf("%v", vv)
	case string:
		return vv
	case RdfNull:
		return "rdfNull"
	default:
		return fmt.Sprintf("<invalid type:%v>", reflect.TypeOf(v.Value))
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
			byte(vv.Key >> 56),
			byte(vv.Key >> 48),
			byte(vv.Key >> 40),
			byte(vv.Key >> 32),
			byte(vv.Key >> 24),
			byte(vv.Key >> 16),
			byte(vv.Key >> 8),
			byte(vv.Key),
		}, nil
	case NamedResource:
		return append([]byte(vv.Name), 'R'), nil
	case LDate:
		md, err := vv.Date.MarshalBinary()
		if err == nil {
			md = append(md, 'D')
		}
		return md, err
	case LDatetime:
		mt, err := vv.Datetime.MarshalBinary()
		if err == nil {
			mt = append(mt, 'T')
		}
		return mt, err
	case int:
		// int is 8 bytes
		return []byte{
			'I', '0', '0',
			byte(vv >> 56),
			byte(vv >> 48),
			byte(vv >> 40),
			byte(vv >> 32),
			byte(vv >> 24),
			byte(vv >> 16),
			byte(vv >> 8),
			byte(vv),
		}, nil
	case uint:
		// int is 8 bytes
		return []byte{
			'U', '0', '0',
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
			'F', '6', '4',
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
	case RdfNull:
		return []byte{'R', 'D', 'F', 'N', 'U', 'L', 'L'}, nil
	default:
		return nil, fmt.Errorf("error: unknown type for rdf.Node in MarshalBinary: %v",
			reflect.TypeOf(v.Value))
	}
}

type Triple = [3]*Node

// func (t *Triple) String() string {
// 	return fmt.Sprintf("(%s, %s, %s)", (*t)[0], (*t)[1], (*t)[2])
// }

func ToString(t3 *Triple) string {
	if t3 == nil {
		return "<nil>"
	}
	return fmt.Sprintf("(%v, %v, %v)", (*t3)[0], (*t3)[1], (*t3)[2])
}

func T3(s, p, o *Node) Triple {
	return Triple{s, p, o}
}

func NilTriple() Triple {
	return Triple{Null(), Null(), Null()}
}

func Null() *Node {
	return globalNull
}

func BN(k int) *Node {
	return &Node{Value: BlankNode{Key: k}}
}

func R(name string) *Node {
	return &Node{Value: NamedResource{Name: name}}
}

func D(date string) (*Node, error) {
	t, err := ParseDate(date)
	return &Node{Value: LDate{Date: t}}, err
}

func DD(date string) *Node {
	t, err := ParseDate(date)
	if err != nil {
		log.Printf("error parsing date: %v", err)
		return nil
	}
	return &Node{Value: LDate{Date: t}}
}

func DT(datetime string) (*Node, error) {
	t, err := ParseDatetime(datetime)
	return &Node{Value: LDatetime{Datetime: t}}, err
}

func DDT(datetime string) *Node {
	t, err := ParseDatetime(datetime)
	if err != nil {
		log.Printf("error parsing datetime: %v", err)
		return nil
	}
	return &Node{Value: LDatetime{Datetime: t}}
}

func I(v int) *Node {
	return &Node{Value: v}
}

func UI(v uint) *Node {
	return &Node{Value: v}
}

func B(b bool) *Node {
	if b {
		return TRUE()
	}
	return FALSE()
}

func TRUE() *Node {
	return &Node{Value: 1}
}

func FALSE() *Node {
	return &Node{Value: 0}
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
	Key int
}

type NamedResource struct {
	Name string
}

type LDate struct {
	Date *time.Time
}

func NewLDate(date string) (LDate, error) {
	t, err := ParseDate(date)
	return LDate{Date: t}, err
}

func (lhs LDate) Add(days int) LDate {
	t := lhs.Date.Add(time.Duration(days) * 24 * time.Hour)
	return LDate{Date: &t}
}

type LDatetime struct {
	Datetime *time.Time
}

func NewLDatetime(datetime string) (LDatetime, error) {
	t, err := ParseDatetime(datetime)
	return LDatetime{Datetime: t}, err
}

func (lhs LDatetime) Add(days int) LDatetime {
	t := lhs.Datetime.Add(time.Duration(days) * 24 * time.Hour)
	return LDatetime{Datetime: &t}
}

func ParseBool(value string) int {
	switch strings.ToLower(value) {
	case "true", "t", "1":
		return 1
	default:
		return 0
	}
}

func NearlyEqual(a, b float64) bool {

	// already equal?
	if a == b {
		return true
	}

	diff := math.Abs(a - b)
	if a == 0.0 || b == 0.0 || diff < math.SmallestNonzeroFloat64 {
		return diff < 1e-10*math.SmallestNonzeroFloat64
	}

	return diff/(math.Abs(a)+math.Abs(b)) < 1e-10
}
