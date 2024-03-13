package compute_pipes

import "fmt"

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
	index int
}

func (node *expressionSelectLeaf) eval(input *[]interface{}) (interface{}, error) {
	if node.index >= len(*input) {
		return nil, fmt.Errorf("error expressionSelectLeaf index %d >= len(*input) %d", node.index, len(*input))
	}
	return (*input)[node.index], nil
}

type expressionValueLeaf struct {
	value interface{}
}

func (node *expressionValueLeaf) eval(_ *[]interface{}) (interface{}, error) {
	return node.value, nil
}

// main builder, builds expression evaluator

func (ctx *BuilderContext) buildExprNodeEvaluator(source *InputChannel, outCh *OutputChannel, spec *ExpressionNode) (evalExpression, error) {
	switch {
	case spec.Arg != nil:
		// Case of unary operator node
		if spec.Op == nil {
			return nil, fmt.Errorf("error: case unary operator node, must have arg, and op != nil")
		}
		arg, err := ctx.buildExprNodeEvaluator(source, outCh, spec.Arg)
		if err != nil {
			return nil, err
		}
		op, err := ctx.buildEvalOperator(*spec.Op)
		if err != nil {
			return nil, err
		}
		return &expressionNodeEvaluator{
			lhs: arg,
			op:  op,
		}, nil

	case spec.Lhs != nil:
		// Case of binary node
		if spec.Rhs == nil || spec.Op == nil {
			return nil, fmt.Errorf("error: case node, must have lhs, rhs, and op != nil")
		}
		lhs, err := ctx.buildExprNodeEvaluator(source, outCh, spec.Lhs)
		if err != nil {
			return nil, err
		}
		rhs, err := ctx.buildExprNodeEvaluator(source, outCh, spec.Rhs)
		if err != nil {
			return nil, err
		}
		op, err := ctx.buildEvalOperator(*spec.Op)
		if err != nil {
			return nil, err
		}
		return &expressionNodeEvaluator{
			lhs: lhs,
			op:  op,
			rhs: rhs,
		}, nil

	case spec.Type != nil:
		// Case leaf node
		switch *spec.Type {
		case "value":
			if spec.Expr == nil {
				return nil, fmt.Errorf("error: Type value must have Expr != nil")
			}
			value, err := ctx.parseValue(spec.Expr)
			if err != nil {
				return nil, err
			}
			return &expressionValueLeaf{
				value: value,
			}, nil

		case "select":
			if spec.Expr == nil {
				return nil, fmt.Errorf("error: Type select must have Expr != nil")
			}
			inputPos, ok := source.columns[*spec.Expr]
			var err error
			if !ok {
				err = fmt.Errorf("error column %s not found in input source %s", *spec.Expr, source.config.Name)
			}
			return &expressionSelectLeaf{
				index: inputPos,
			}, err

		case "eval":
			if spec.EvalExpr == nil {
				return nil, fmt.Errorf("error: Type eval must have EvalExpr != nil")
			}
			return ctx.buildExprNodeEvaluator(source, outCh, spec.EvalExpr)
		}
	}
	return nil, fmt.Errorf("error buildExprNodeEvaluator: cannot determine if expr is node or leaf? spec type %v", *spec.Type)
}
