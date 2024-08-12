package rete

import "github.com/artisoft-io/jetstore/jets/jetrules/rdf"

type LookupTable interface {
	Lookup(rs *ReteSession, tblName *string, key *string) (*rdf.Node, error)
	MultiLookup(rs *ReteSession, tblName *string, key *string) (*rdf.Node, error)
	LookupRand(rs *ReteSession, tblName *string) (*rdf.Node, error)
	MultiLookupRand(rs *ReteSession, tblName *string) (*rdf.Node, error)
}
