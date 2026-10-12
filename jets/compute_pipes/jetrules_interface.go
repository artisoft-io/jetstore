package compute_pipes

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This file contains the definition of the interface for jetrules native and go versions integration.

type JetRulesProxy interface {
	GetDefaultFactory() JetRulesFactory
	GetGoFactory() JetRulesFactory
	GetNativeFactory() JetRulesFactory
}

type JetRulesFactory interface {
	JetRulesName() string
	// NewJetRuleEngine builds the rule networks of processName. opts are applied before
	// the rules load; a pipeline passes none and gets the legacy engine.
	NewJetRuleEngine(dbpool *pgxpool.Pool, processName string, isDebug bool, opts ...EngineOption) (JetRuleEngine, error)
	ClearCache() bool
}

// RetractionMode is the LEGACY retraction mode a rule engine is built in: whether a
// graph-reading consequent is replayed rather than recomputed on retraction (defect 1
// off) and whether a withdrawn row is erased (defect 2 off). The zero value is legacy
// for both, which is what every pipeline runs. See jets/jetrules/rete/README.md,
// 2026-10-09 and 2026-10-11; the C++ engine's twin is jets::rete::RetractionMode.
type RetractionMode = rete.RetractionMode

// EngineOptions are what NewJetRuleEngine's options set.
type EngineOptions struct {
	RetractionMode RetractionMode
}

// EngineOption configures one NewJetRuleEngine call.
type EngineOption func(*EngineOptions)

// WithRetractionMode builds the engine in mode, in both adaptors. Only the rules unit
// test harness passes it, from a test definition's legacy object: there is
// deliberately no JetrulesSpec field and no rule_config key that reaches it, so a
// pipeline cannot turn a LEGACY defect off.
func WithRetractionMode(mode RetractionMode) EngineOption {
	return func(o *EngineOptions) {
		o.RetractionMode = mode
	}
}

// ApplyEngineOptions folds opts into EngineOptions, for the adaptors.
func ApplyEngineOptions(opts ...EngineOption) EngineOptions {
	var o EngineOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

type JetRuleEngine interface {
	MainRuleFile() string
	JetResources() *JetResources
	GetMetaGraphTriples() []string

	GetMetaResourceManager() JetResourceManager
	Insert(s, p RdfNode, o RdfNode) error

	NewRdfSession() (JetRdfSession, error)
	Release() error
}

type JetResourceManager interface {
	RdfNull() RdfNode
	CreateBNode(key int) RdfNode
	NewDateLiteral(data string) RdfNode
	NewDateDetails(year, month, day int) RdfNode
	NewDatetimeLiteral(data string) RdfNode
	NewDatetimeDetails(year, month, day, hour, min, sec int) RdfNode
	NewDoubleLiteral(x float64) RdfNode
	NewIntLiteral(data int) RdfNode
	NewUIntLiteral(data uint) RdfNode
	NewResource(name string) RdfNode
	NewTextLiteral(data string) RdfNode
}

type JetRdfSession interface {
	GetResourceManager() JetResourceManager
	JetResources() *JetResources

	Insert(s, p RdfNode, o RdfNode) error
	Erase(s, p RdfNode, o RdfNode) (bool, error)
	Retract(s, p RdfNode, o RdfNode) (bool, error)
	Contains(s, p RdfNode, o RdfNode) bool
	ContainsSP(s, p RdfNode) bool
	GetObject(s, p RdfNode) RdfNode

	FindSPO(s, p, o RdfNode) TripleIterator
	FindSP(s, p RdfNode) TripleIterator
	FindS(s RdfNode) TripleIterator
	Find() TripleIterator

	NewReteSession(ruleset string) (JetReteSession, error)
	EncodeRdfSession() string
	Release() error
}

type JetReteSession interface {
	ExecuteRules() error
	Release() error
}

type TripleIterator interface {
	IsEnd() bool
	Next() bool
	GetSubject() RdfNode
	GetPredicate() RdfNode
	GetObject() RdfNode
	Release() error
}

type RdfNode interface {
	Hdle() any
	IsNil() bool
	Value() any
	Type() string
	// Equals reports whether both are the same RDF node, the identity that
	// Contains and FindSPO use: the same resource, or a literal of the same
	// type and value. No coercion, so int 1 and double 1 are not equal.
	Equals(other RdfNode) bool
	String() string
}

func GetRdfNodeValue(r RdfNode) any {
	switch vv := r.Value().(type) {
	case int, uint, float64, string:
		return r.Value()
	case rdf.LDate:
		return *vv.Date
	case rdf.NamedResource:
		return vv.Name
	case rdf.LDatetime:
		return *vv.Datetime
	case rdf.RdfNull:
		return nil
	case rdf.BlankNode:
		return fmt.Sprintf("BN%d", vv.Key)
	case int64:
		return int(vv)
	case int32:
		return int(vv)
	case uint64:
		return uint(vv)
	case uint32:
		return uint(vv)
	default:
		return nil
	}
}

func ParseRdfNodeValue(re JetResourceManager, value, rdfType string) (node RdfNode, err error) {
	var key int
	// log.Println("**PARSE OBJECT:",object,"TO TYPE:",rdfType)
	switch strings.TrimSpace(rdfType) {
	case "null":
		node = re.RdfNull()
	case "bn":
		key, err = strconv.Atoi(value)
		if err != nil {
			return
		}
		node = re.CreateBNode(key)
	case "resource":
		node = re.NewResource(value)
	case "int":
		var v int
		_, err = fmt.Sscan(value, &v)
		if err != nil {
			return nil, fmt.Errorf("while asserting rule config: %v", err)
		}
		node = re.NewIntLiteral(v)
	case "bool":
		// The accepted spellings and nothing else; empty is null (I-45). This
		// read the first letter until 2026-10-11, so "tomato" was true.
		b, err := rdf.ParseBool(value)
		switch {
		case errors.Is(err, rdf.ErrNullBool):
			node = re.RdfNull()
		case err != nil:
			return nil, fmt.Errorf("while rule config triple; %v", err)
		case b:
			node = re.NewIntLiteral(1)
		default:
			node = re.NewIntLiteral(0)
		}
	case "long":
		var v int
		_, err = fmt.Sscan(value, &v)
		if err != nil {
			return nil, fmt.Errorf("while asserting rule config: %v", err)
		}
		node = re.NewIntLiteral(v)
	case "uint", "ulong":
		// The rule language has both unsigned types (JetRule.g4) and the engines one
		// unsigned literal, as castToRdfTypeFromTxt maps both to uint.
		var v uint
		_, err = fmt.Sscan(value, &v)
		if err != nil {
			return nil, fmt.Errorf("while asserting rule config: value is not %s: %q: %v", rdfType, value, err)
		}
		node = re.NewUIntLiteral(v)
	case "double":
		var v float64
		_, err = fmt.Sscan(value, &v)
		if err != nil {
			return nil, fmt.Errorf("while asserting rule config: %v", err)
		}
		node = re.NewDoubleLiteral(v)
	case "text":
		node = re.NewTextLiteral(value)
	case "date":
		node = re.NewDateLiteral(value)
	case "datetime":
		node = re.NewDatetimeLiteral(value)
	default:
		err = fmt.Errorf("ERROR ParseObject: unknown rdf type for object: %s", rdfType)
	}
	return
}

type JetResources struct {
	Jets__client                  RdfNode
	Jets__completed               RdfNode
	Jets__currentSourcePeriod     RdfNode
	Jets__currentSourcePeriodDate RdfNode
	Jets__entity_property         RdfNode
	Jets__exception               RdfNode
	Jets__from                    RdfNode
	Jets__input_record            RdfNode
	Jets__istate                  RdfNode
	Jets__key                     RdfNode
	Jets__length                  RdfNode
	Jets__lookup_multi_rows       RdfNode
	Jets__lookup_row              RdfNode
	Jets__loop                    RdfNode
	Jets__max_vertex_visits       RdfNode
	Jets__operator                RdfNode
	Jets__org                     RdfNode
	Jets__range_value             RdfNode
	Jets__replace_chars           RdfNode
	Jets__replace_with            RdfNode
	Jets__row_id                  RdfNode
	Jets__source_period_sequence  RdfNode
	Jets__sourcePeriodType        RdfNode
	Jets__state                   RdfNode
	Jets__value_property          RdfNode
	Rdf__type                     RdfNode
}

func NewJetResources(je JetResourceManager) *JetResources {
	jr := &JetResources{}
	jr.Initialize(je)
	return jr
}

func (jr *JetResources) Initialize(je JetResourceManager) {
	if je == nil {
		return
	}
	// Create the resources
	jr.Jets__client = je.NewResource("jets:client")
	jr.Jets__completed = je.NewResource("jets:completed")
	jr.Jets__currentSourcePeriod = je.NewResource("jets:currentSourcePeriod")
	jr.Jets__currentSourcePeriodDate = je.NewResource("jets:currentSourcePeriodDate")
	jr.Jets__entity_property = je.NewResource("jets:entity_property")
	jr.Jets__exception = je.NewResource("jets:exception")
	jr.Jets__from = je.NewResource("jets:from")
	jr.Jets__input_record = je.NewResource("jets:InputRecord")
	jr.Jets__istate = je.NewResource("jets:iState")
	jr.Jets__key = je.NewResource("jets:key")
	jr.Jets__length = je.NewResource("jets:length")
	jr.Jets__lookup_multi_rows = je.NewResource("jets:lookup_multi_rows")
	jr.Jets__lookup_row = je.NewResource("jets:lookup_row")
	jr.Jets__loop = je.NewResource("jets:loop")
	jr.Jets__max_vertex_visits = je.NewResource("jets:max_vertex_visits")
	jr.Jets__operator = je.NewResource("jets:operator")
	jr.Jets__org = je.NewResource("jets:org")
	jr.Jets__range_value = je.NewResource("jets:range_value")
	jr.Jets__replace_chars = je.NewResource("jets:replace_chars")
	jr.Jets__replace_with = je.NewResource("jets:replace_with")
	jr.Jets__row_id = je.NewResource("jets:row_id")
	jr.Jets__source_period_sequence = je.NewResource("jets:source_period_sequence")
	jr.Jets__sourcePeriodType = je.NewResource("jets:sourcePeriodType")
	jr.Jets__state = je.NewResource("jets:State")
	jr.Jets__value_property = je.NewResource("jets:value_property")
	jr.Rdf__type = je.NewResource("rdf:type")
}
