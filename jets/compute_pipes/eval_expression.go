package compute_pipes

import (
	"fmt"
	"strings"
)

type evalExpression interface {
	eval(input *[]interface{}) (interface{}, error)
}
type evalOperator interface {
	eval(lhs interface{}, rhs interface{}) (interface{}, error)
}

type expressionNodeEvaluator struct {
	lhs evalExpression
	op  evalOperator
	rhs evalExpression
}

func (node *expressionNodeEvaluator) eval(input *[]interface{}) (interface{}, error) {
	lhs, err := node.lhs.eval(input)
	if err != nil {
		return nil, err
	}
	var rhs interface{}
	if node.rhs != nil {
		rhs, err = node.rhs.eval(input)
		if err != nil {
			return nil, err
		}
	}
	return node.op.eval(lhs, rhs)
}

type expressionSelectLeaf struct {
	index   int
	rdfType string
}

func (node *expressionSelectLeaf) eval(input *[]interface{}) (interface{}, error) {
	if node.index >= len(*input) {
		return nil, fmt.Errorf("error expressionSelectLeaf index %d >= len(*input) %d", node.index, len(*input))
	}
	if node.rdfType != "" {
		return CastToRdfType((*input)[node.index], node.rdfType)
	}
	return (*input)[node.index], nil
}

type expressionValueLeaf struct {
	value interface{}
}

func (node *expressionValueLeaf) eval(_ *[]interface{}) (interface{}, error) {
	return node.value, nil
}

type expressionStaticListLeaf struct {
	values map[any]bool
}

func (node *expressionStaticListLeaf) eval(_ *[]interface{}) (interface{}, error) {
	return node.values, nil
}

// main builder, builds expression evaluator

func (ctx *BuilderContext) buildExprNodeEvaluator(source *InputChannel, outCh *OutputChannel, spec *ExpressionNode) (evalExpression, error) {
	switch {
	case spec.Arg != nil:
		// Case of unary operator node
		if spec.Op == "" {
			return nil, fmt.Errorf("error: case unary operator node, must have arg, and op != nil")
		}
		arg, err := ctx.buildExprNodeEvaluator(source, outCh, spec.Arg)
		if err != nil {
			return nil, err
		}
		op, err := ctx.buildEvalOperator(spec.Op)
		if err != nil {
			return nil, err
		}
		return &expressionNodeEvaluator{
			lhs: arg,
			op:  op,
		}, nil

	case spec.Lhs != nil:
		// Case of binary node
		if spec.Rhs == nil || spec.Op == "" {
			return nil, fmt.Errorf("error: case node, must have lhs, rhs, and op != nil")
		}
		// Check for special IN operator who must have a static_list as rhs
		if strings.ToUpper(spec.Op) == "IN" && spec.Rhs.Type != "static_list" {
			return nil, fmt.Errorf("error: operator IN must have static_list as rhs argument")
		}
		lhs, err := ctx.buildExprNodeEvaluator(source, outCh, spec.Lhs)
		if err != nil {
			return nil, err
		}
		rhs, err := ctx.buildExprNodeEvaluator(source, outCh, spec.Rhs)
		if err != nil {
			return nil, err
		}
		op, err := ctx.buildEvalOperator(spec.Op)
		if err != nil {
			return nil, err
		}
		return &expressionNodeEvaluator{
			lhs: lhs,
			op:  op,
			rhs: rhs,
		}, nil

	case spec.Type != "":
		// Case leaf node
		switch spec.Type {
		case "value":
			if spec.Expr == "" {
				return nil, fmt.Errorf("error: Type value must have Expr != nil")
			}
			value, err := ctx.parseValue(&spec.Expr)
			if err != nil {
				return nil, err
			}
			if spec.AsRdfType != "" {
				value, err = CastToRdfType(value, spec.AsRdfType)
			}
			return &expressionValueLeaf{
				value: value,
			}, err

		case "select":
			if spec.Expr == "" {
				return nil, fmt.Errorf("error: Type select must have Expr != nil")
			}
			inputPos, ok := (*source.columns)[spec.Expr]
			var err error
			if !ok {
				err = fmt.Errorf("error column %s not found in input source %s", spec.Expr, source.name)
			}
			return &expressionSelectLeaf{
				index:   inputPos,
				rdfType: spec.AsRdfType,
			}, err

		case "static_list":
			if len(spec.ExprList) == 0 {
				return nil, fmt.Errorf("error: Type select must have non empty expr_list")
			}
			values := make(map[any]bool, len(spec.ExprList))
			for _, v := range spec.ExprList {
				value, err := ctx.parseValue(&v)
				if err != nil {
					return nil, fmt.Errorf("while parsing value of static_list: %v", err)
				}
				values[value] = true
			}
			return &expressionStaticListLeaf{
				values: values,
			}, nil
		}
	}
	return nil, fmt.Errorf("error buildExprNodeEvaluator: cannot determine if expr is node or leaf? spec type %v", spec.Type)
}
