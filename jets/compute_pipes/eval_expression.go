package compute_pipes

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// input is the whole input row as []any or map[string]any depending on the context.
type evalExpression interface {
	Eval(input any) (any, error)
}
type evalOperator interface {
	Eval(lhs any, rhs any) (any, error)
}

type expressionNodeEvaluator struct {
	Lhs     evalExpression
	Op      evalOperator
	Rhs     evalExpression
	Default evalExpression
}

func (node *expressionNodeEvaluator) Eval(input any) (any, error) {
	lhs, err := node.Lhs.Eval(input)
	if err != nil {
		return nil, err
	}
	// Special consideration for short-circuit operators AND and OR,
	// if the operator is AND and lhs is false, we do not evaluate rhs and return false,
	// if the operator is OR and lhs is true, we do not evaluate rhs and return true.
	switch node.Op.(type) {
	case *opAND:
		if !ToBool(lhs) {
			return 0, nil
		}
	case *opOR:
		if ToBool(lhs) {
			return 1, nil
		}
	}
	var rhs any
	if node.Rhs != nil {
		rhs, err = node.Rhs.Eval(input)
		if err != nil {
			return nil, err
		}
	}
	result, err := node.Op.Eval(lhs, rhs)
	if err != nil && node.Default != nil {
		return node.Default.Eval(input)
	}
	return result, err
}

type expressionFunctionEvaluator struct {
	Args    []evalExpression
	Fnc     evalFunction
	Default evalExpression
}

func (node *expressionFunctionEvaluator) Eval(input any) (any, error) {
	args := make([]any, len(node.Args))
	for i, arg := range node.Args {
		var err error
		args[i], err = arg.Eval(input)
		if err != nil {
			return nil, err
		}
	}
	result, err := node.Fnc(args)
	if err != nil && node.Default != nil {
		return node.Default.Eval(input)
	}
	return result, err
}

type evalFunction func(args []any) (any, error)

type expressionSelectLeaf struct {
	index   int
	colName string
	rdfType string
	Default evalExpression
}

func (node *expressionSelectLeaf) Eval(in any) (any, error) {
	var value any
	switch input := in.(type) {
	case []any:
		if node.index >= len(input) {
			return nil, fmt.Errorf("error expressionSelectLeaf index %d >= len(input) %d", node.index, len(input))
		}
		value = input[node.index]
	case map[string]any:
		value = input[node.colName]
	default:
		if node.Default != nil {
			return node.Default.Eval(in)
		}
		return nil, fmt.Errorf("error: invalid type passed to expression.Eval for input: %v", in)
	}
	if node.rdfType != "" {
		r, err := CastToRdfType(value, node.rdfType, nil)
		if err != nil && node.Default != nil {
			return node.Default.Eval(in)
		}
		return r, err
	}
	return value, nil
}

type expressionValueLeaf struct {
	value any
}

func (node *expressionValueLeaf) Eval(_ any) (any, error) {
	return node.value, nil
}

type expressionStaticListLeaf struct {
	values map[any]bool
}

func (node *expressionStaticListLeaf) Eval(_ any) (any, error) {
	return node.values, nil
}

// main builder, builds expression evaluator
type ExprBuilderContext map[string]any

func (ctx ExprBuilderContext) parseValue(expr *string, maxSubstitutions int) (any, error) {
	var value any
	var err error
	if maxSubstitutions <= 0 {
		maxSubstitutions = 3
	}
	switch {
	case *expr == "NULL" || *expr == "null":
		value = nil
	case *expr == "NaN" || *expr == "NAN":
		value = math.NaN()

	case strings.HasPrefix(*expr, "'") && strings.HasSuffix(*expr, "'"):
		// value is a string
		value = strings.TrimSuffix(strings.TrimPrefix(*expr, "'"), "'")

	case strings.Contains(*expr, "$"):
		// value contains an env var, e.g. $DATE_FILE_KEY
		valueStr := *expr
		if maxSubstitutions == 1 {
			// Special case, valueStr is directly the env var key, e.g. $DATE_FILE_KEY, in this case we do not want to do
			// further substitution than the value of the env var.
			if v, ok := ctx[valueStr]; ok {
				return v, nil
			} else {
				return nil, fmt.Errorf("error: env var %s not found in context for value %s", valueStr, *expr)
			}
		}
		lc := 0
		for strings.Contains(valueStr, "$") && lc < maxSubstitutions {
			lc += 1
			for k, v := range ctx {
				vstr, ok := v.(string)
				if ok {
					valueStr = strings.ReplaceAll(valueStr, k, vstr)
				} else {
					if strings.Contains(valueStr, k) {
						// the value is not a string, no further replacement
						value = v
						goto Substitution_Done
					}
				}
			}
		}
		value = valueStr
	Substitution_Done:
		;

	case strings.Contains(*expr, "."):
		// value is double
		value, err = strconv.ParseFloat(*expr, 64)
		if err != nil {
			return nil, fmt.Errorf("error: expecting a double: %s", *expr)
		}
	default:
		// default to int
		value, err = strconv.ParseInt(*expr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error: expecting an int: %s", *expr)
		}
	}
	// fmt.Printf("**!@@ PARSEVALUE: %s => = %v of type %T\n", *expr, value, value)
	return value, err
}

// Note that columns can be nil when evalExtression is having map[string]any as argument.
func (ctx ExprBuilderContext) BuildExprNodeEvaluator(sourceName string, columns map[string]int, spec *ExpressionNode) (evalExpression, error) {
	if spec == nil {
		return nil, nil
	}
	defaultExpr, err := ctx.BuildExprNodeEvaluator(sourceName, columns, spec.Default)
	if err != nil {
		return nil, fmt.Errorf("while building default expression: %v", err)
	}
	switch {
	case spec.Arg != nil:
		// Case of unary operator node
		if spec.Op == "" {
			return nil, fmt.Errorf("error: case unary operator node, must have arg, and op != nil")
		}
		arg, err := ctx.BuildExprNodeEvaluator(sourceName, columns, spec.Arg)
		if err != nil {
			return nil, err
		}
		op, err := BuildEvalOperator(spec.Op)
		if err != nil {
			return nil, err
		}
		return &expressionNodeEvaluator{
			Lhs:     arg,
			Op:      op,
			Default: defaultExpr,
		}, nil

	case spec.Lhs != nil:
		// Case of binary node
		if spec.Rhs == nil || spec.Op == "" {
			return nil, fmt.Errorf("error: case node, must have lhs, rhs, and op != nil")
		}
		// Check for special IN IN_NO_CASE operator who must have a static_list as rhs
		if strings.ToUpper(spec.Op) == "IN" && strings.ToUpper(spec.Rhs.Type) != "STATIC_LIST" {
			return nil, fmt.Errorf("error: operator IN must have static_list as rhs argument")
		}
		if strings.ToUpper(spec.Op) == "IN_NO_CASE" {
			if strings.ToUpper(spec.Rhs.Type) != "STATIC_LIST" {
				return nil, fmt.Errorf("error: operator IN_NO_CASE must have static_list as rhs argument")
			}
			// make the static list in upper case for case-insensitive comparison in opIn.Eval
			for i, v := range spec.Rhs.ExprList {
				spec.Rhs.ExprList[i] = strings.ToUpper(v)
			}
		}
		lhs, err := ctx.BuildExprNodeEvaluator(sourceName, columns, spec.Lhs)
		if err != nil {
			return nil, err
		}
		rhs, err := ctx.BuildExprNodeEvaluator(sourceName, columns, spec.Rhs)
		if err != nil {
			return nil, err
		}
		op, err := BuildEvalOperator(spec.Op)
		if err != nil {
			return nil, err
		}
		return &expressionNodeEvaluator{
			Lhs:     lhs,
			Op:      op,
			Rhs:     rhs,
			Default: defaultExpr,
		}, nil

	case spec.Type != "":
		// Case leaf node
		switch strings.ToUpper(spec.Type) {
		case "VALUE":
			if spec.Expr == "" {
				return nil, fmt.Errorf("error: Type value must have Expr != nil")
			}
			value, err := ctx.parseValue(&spec.Expr, spec.MaxEnvVarSubstitution)
			if err != nil {
				return nil, err
			}
			if spec.AsRdfType != "" {
				value, err = CastToRdfType(value, spec.AsRdfType, nil)
			}
			return &expressionValueLeaf{
				value: value,
			}, err

		case "SELECT":
			if spec.Expr == "" && spec.ExprPos == nil {
				return nil, fmt.Errorf("error: Type select must have Expr or ExprPos not nil")
			}
			switch {
			case spec.Expr != "":
				// Select by column name
				// Special case when spec.Expr starts with '$', in this case we consider
				// that spec.Expr is an env var key whose value is the actual column name to select.
				if strings.HasPrefix(spec.Expr, "$") {
					if v, ok := ctx[spec.Expr]; ok {
						if colName, ok := v.(string); ok {
							spec.Expr = colName
						} else {
							return nil, fmt.Errorf("error: env var %s does not contain a valid string for column name", spec.Expr)
						}
					}
					// Note that if the env var is not found in context, we do not return an error, we just keep spec.Expr as is,
					// and let it be handled as a regular column name,
					// this allows to the odd case when a column name actually strats with '$'
				}
				if columns == nil {
					return &expressionSelectLeaf{
						colName: spec.Expr,
						rdfType: spec.AsRdfType,
						Default: defaultExpr,
					}, nil
				}
				inputPos, ok := columns[spec.Expr]
				var err error
				if !ok {
					if defaultExpr == nil {
						err = fmt.Errorf("error column %s not found in input source %s", spec.Expr, sourceName)
					} else {
						return defaultExpr, nil
					}
				}
				return &expressionSelectLeaf{
					index:   inputPos,
					rdfType: spec.AsRdfType,
					Default: defaultExpr,
				}, err

			case spec.ExprPos != nil:
				// Select by column position
				return &expressionSelectLeaf{
					index:   *spec.ExprPos,
					rdfType: spec.AsRdfType,
					Default: defaultExpr,
				}, nil
			}

		case "STATIC_LIST":
			if len(spec.ExprList) == 0 {
				return nil, fmt.Errorf("error: Type select must have non empty expr_list")
			}
			values := make(map[any]bool, len(spec.ExprList))
			for _, v := range spec.ExprList {
				value, err := ctx.parseValue(&v, spec.MaxEnvVarSubstitution)
				if err != nil {
					return nil, fmt.Errorf("while parsing value of static_list: %v", err)
				}
				values[value] = true
			}
			return &expressionStaticListLeaf{
				values: values,
			}, nil

		case "EXPR_PROXY":
			// special case of expression proxy, the actual expression is specified by one of:
			// - ExprEnvVarProxy: the expression is specified by an env var, the value of the
			//   env var is the actual expression as a json string to evaluate.
			if spec.ExprEnvVarProxy == "" {
				return nil, fmt.Errorf("error: Type expr_proxy must have ExprEnvVarProxy not nil")
			}
			v, ok := ctx[spec.ExprEnvVarProxy]
			if !ok {
				return nil, fmt.Errorf("error: env var %s not found in context for expr_proxy", spec.ExprEnvVarProxy)
			}
			exprStr, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("error: env var %s does not contain a valid string for expr_proxy", spec.ExprEnvVarProxy)
			}
			// parse the exprStr as an ExpressionNode
			var exprNode ExpressionNode
			err := json.Unmarshal([]byte(exprStr), &exprNode)
			if err != nil {
				return nil, fmt.Errorf("error: failed to parse expr_proxy env var %s value as ExpressionNode: %v", spec.ExprEnvVarProxy, err)
			}
			// build the expression evaluator for the parsed ExpressionNode
			return ctx.BuildExprNodeEvaluator(sourceName, columns, &exprNode)

		case "FUNCTION":
			if spec.Expr == "" {
				return nil, fmt.Errorf("error: Type function must have Expr not nil")
			}
			fnc, err := BuildFncEvaluator(spec.Expr)
			if err != nil {
				return nil, fmt.Errorf("error: failed to build operator for function type expression with expr %s: %v", spec.Expr, err)
			}
			args := make([]evalExpression, len(spec.Farg))
			for i, argSpec := range spec.Farg {
				argEval, err := ctx.BuildExprNodeEvaluator(sourceName, columns, &argSpec)
				if err != nil {
					return nil, fmt.Errorf("error: failed to build evaluator for argument %d of function type expression with expr %s: %v", i, spec.Expr, err)
				}
				args[i] = argEval
			}
			return &expressionFunctionEvaluator{
				Fnc:     fnc,
				Args:    args,
				Default: defaultExpr,
			}, nil
		default:
			return nil, fmt.Errorf("error: unknown expression leaf node type: %s", spec.Type)
		}
	}
	return nil, fmt.Errorf("error BuildExprNodeEvaluator: cannot determine if expr is node or leaf? spec type %v", spec.Type)
}
