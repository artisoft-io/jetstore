package rete

import (
	"log"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// CREATE_RESOURCE unary operator
type CreateResourceOp struct {
}

func NewCreateResourceOp() UnaryOperator {
	return &CreateResourceOp{}
}

func (op *CreateResourceOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *CreateResourceOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case int:
		return rdf.R(strconv.Itoa(rhsv))
	case float64:
		return rdf.R(strconv.FormatFloat(rhsv, 'G', 15, 64))
	case string:
		return rdf.R(rhsv)
	default:
		log.Printf("Argment is not a literal (create_resource): %v", rhsv)
		return nil
	}
}
