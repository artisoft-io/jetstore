package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/google/uuid"
)

// UUID_MD5 unary operator
type UuidSha1Op struct {
}

func NewUuidSha1Op() UnaryOperator {
	return &UuidSha1Op{}
}

func (op *UuidSha1Op) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *UuidSha1Op) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *UuidSha1Op) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	rhsv, ok := rhs.Value.(string)
	if !ok {
		return nil
	}
	return rdf.S(uuid.NewSHA1(seedUuid, []byte(rhsv)).String())
}
