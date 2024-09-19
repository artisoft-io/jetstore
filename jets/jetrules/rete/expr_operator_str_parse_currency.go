package rete

import (
	"unicode"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// PARSE_CURRENCY unary operator
type ParseCurrencyOp struct {
}

func NewParseCurrencyOp() UnaryOperator {
	return &ParseCurrencyOp{}
}

func (op *ParseCurrencyOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *ParseCurrencyOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *ParseCurrencyOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	rhsv, ok := rhs.Value.(string)
	if !ok {
		return nil
	}
	out := make([]rune, 0)
	for _, r := range rhsv {
		switch {
		case r == '(' || r == '-':
			out = append(out, '-')
		case unicode.IsDigit(r) || r == '.':
			out = append(out, r)
		}
	}
	return rdf.S(string(out))
}
