package op

import (
	"fmt"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Max operator - with truth maintenance
type MinMaxOp struct {
	isMin bool
}

func NewMinMaxOp(isMin bool) rete.BinaryOperator {
	return &MinMaxOp{
		isMin: isMin,
	}
}

// Add truth maintenance 
func (op *MinMaxOp) RegisterCallback(reteSession *rete.ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	if reteSession == nil {
		return nil
	}
	// Register the callback with the rhs domain property
	rdfSession := reteSession.RdfSession
	jr := rdfSession.ResourceMgr.JetsResources
	obj := rdfSession.GetObject(rhs, jr.Jets__entity_property)
  // if objp == null then mode is min/max of a multi value property
	var cb rdf.NotificationCallback
	if obj != nil {
		// obj is the domain property
		cb = rete.NewReteCallbackForFilter(reteSession, vertex, obj)
	} else {
		// rhs is the domain property
		cb = rete.NewReteCallbackForFilter(reteSession, vertex, rhs)
	}
	rdfSession.AssertedGraph.CallbackMgr.AddCallback(cb)
	rdfSession.InferredGraph.CallbackMgr.AddCallback(cb)
	return nil
}

func (op *MinMaxOp) Eval(reteSession *rete.ReteSession, row *rete.BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
XXX
	switch lhsv := lhs.Value.(type) {
	case rdf.BlankNode:
		return nil
	case rdf.NamedResource:
		return nil
	case rdf.LDate:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		case int64:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		case float64:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		default:
			return nil
		}
	
	case rdf.LDatetime:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		case int64:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		case float64:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		default:
			return nil
		}

	case int32:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv + rhsv}
		case int64:
			return &rdf.Node{Value: int64(lhsv) + rhsv}
		case float64:
			return &rdf.Node{Value: float64(lhsv) + rhsv}
		default:
			return nil
		}
	case int64:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv + int64(rhsv)}
		case int64:
			return &rdf.Node{Value: int64(lhsv) + rhsv}
		case float64:
			return &rdf.Node{Value: float64(lhsv) + rhsv}
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv + float64(rhsv)}
		case int64:
			return &rdf.Node{Value: lhsv + float64(rhsv)}
		case float64:
			return &rdf.Node{Value: lhsv + rhsv}
		default:
			return nil
		}
	case string:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: fmt.Sprintf("%v%v", rhsv, rhsv)}
		case int64:
			return &rdf.Node{Value: fmt.Sprintf("%v%v", rhsv, rhsv)}
		case float64:
			return &rdf.Node{Value: fmt.Sprintf("%v%v", rhsv, rhsv)}
		case string:
			return &rdf.Node{Value: rhsv + rhsv}
		default:
			return nil
		}
	default:
		return nil
	}
}