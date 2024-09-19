package rete

import (
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/google/uuid"
)

var seedUuid uuid.UUID

func init() {
	seed := os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")
	if seed == "" {
		seed = "03847036-1ef4-4c24-8815-7aab5064c3ce"
	}
	var err error
	seedUuid, err = uuid.Parse(seed)
	if err != nil {
		log.Println("error: JETS_DOMAIN_KEY_HASH_SEED has an invalid uuid, will use the default value")
		seedUuid = uuid.MustParse("03847036-1ef4-4c24-8815-7aab5064c3ce")
	}
}

// UUID_MD5 unary operator
type UuidMd5Op struct {
}

func NewUuidMd5Op() UnaryOperator {
	return &UuidMd5Op{}
}

func (op *UuidMd5Op) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *UuidMd5Op) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *UuidMd5Op) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	rhsv, ok := rhs.Value.(string)
	if !ok {
		return nil
	}
	return rdf.S(uuid.NewMD5(seedUuid, []byte(rhsv)).String())
}
