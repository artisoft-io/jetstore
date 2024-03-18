package compute_pipes

import (
	"fmt"
	"time"
)

// TransformationColumnSpec Type count
type countColumnEval struct {
	inputPos int
	outputPos int
	where evalExpression
}
func (ctx *countColumnEval) initializeCurrentValue(currentValue *[]interface{}) {
	if currentValue == nil {
		return
	}
	(*currentValue)[ctx.outputPos] = int64(0)
}
func (ctx *countColumnEval) update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error countColumnEval.update cannot have nil currentValue or input")
	}
	// if count(column_name), make sure the column is not nil
	if ctx.inputPos >= 0 && (*input)[ctx.inputPos] == nil {
		return nil
	}
	if ctx.where != nil {
		w, err := ctx.where.eval(input)
		if err != nil {
			return fmt.Errorf("while evaluating where on count aggregate: %v", err)
		}
		if w == nil || w.(int) != 1 {
			return nil
		}
	}
	var count int64
	c := (*currentValue)[ctx.outputPos]
	if c != nil {
		count = c.(int64)
	}
	(*currentValue)[ctx.outputPos] = count + 1
	return nil
}
func (ctx *countColumnEval) done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *BuilderContext) buildCountEvaluator(source *InputChannel, outCh *OutputChannel,  spec *TransformationColumnSpec) (*countColumnEval, error) {
	if spec == nil || spec.Expr == nil {
		return nil, fmt.Errorf("error: Type count must have Expr != nil")
	}
	inputPos := -1
	var ok bool
	if *spec.Expr != "*" {
		inputPos, ok = source.columns[*spec.Expr]
		if !ok {
			return nil, fmt.Errorf("error: count needs a valid column name or * to count all rows")
		}
	}
	var where evalExpression
	var err error
	if spec.Where != nil {
		where, err = ctx.buildExprNodeEvaluator(source, outCh, spec.Where)
		if err != nil {
			return nil, fmt.Errorf("while building where expression: %v", err)
		}
	}
	outputPos, ok := outCh.columns[spec.Name]
	if !ok {
		err = fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.config.Name)
	}
	return &countColumnEval{
		inputPos: inputPos,
		outputPos: outputPos,
		where: where,
	}, err
}


// TransformationColumnSpec Type distinct_count
type distinctCountColumnEval struct {
	inputPos int
	outputPos int
	distinctValues map[string]bool
	where evalExpression
}
func (ctx *distinctCountColumnEval) initializeCurrentValue(currentValue *[]interface{}) {
	if currentValue == nil {
		return
	}
	(*currentValue)[ctx.outputPos] = int64(0)
}
func (ctx *distinctCountColumnEval) update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error countColumnEval.update cannot have nil currentValue or input")
	}
	// if count(column_name), make sure the column is not nil
	value := (*input)[ctx.inputPos]
	if value == nil {
		return nil
	}
	if ctx.where != nil {
		w, err := ctx.where.eval(input)
		if err != nil {
			return fmt.Errorf("while evaluating where on distinct_count aggregate: %v", err)
		}
		if w == nil || w.(int) != 1 {
			return nil
		}
	}
	//* TODO Currently distinct_count works only on string column, todo convert to string when column is not of type string
	switch vv := value.(type) {
	case string:
		ctx.distinctValues[vv] = true
		(*currentValue)[ctx.outputPos] = int64(len(ctx.distinctValues))

	default:
		return fmt.Errorf("error: distinct_count currently support only string columns")
	}
	return nil
}
func (ctx *distinctCountColumnEval) done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *BuilderContext) buildDistinctCountEvaluator(source *InputChannel, outCh *OutputChannel,  spec *TransformationColumnSpec) (*distinctCountColumnEval, error) {
	if spec == nil || spec.Expr == nil {
		return nil, fmt.Errorf("error: Type distinct_count must have Expr != nil")
	}
	inputPos, ok := source.columns[*spec.Expr]
	if !ok {
		return nil, fmt.Errorf("error, count_distinct needs a valid column name")
	}
	var where evalExpression
	var err error
	if spec.Where != nil {
		where, err = ctx.buildExprNodeEvaluator(source, outCh, spec.Where)
		if err != nil {
			return nil, fmt.Errorf("while building where expression: %v", err)
		}
	}
	outputPos, ok := outCh.columns[spec.Name]
	if !ok {
		err = fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.config.Name)
	}
	return &distinctCountColumnEval{
		inputPos: inputPos,
		outputPos: outputPos,
		where: where,
		distinctValues: make(map[string]bool),
	}, err
}

// add function used for aggregates, supports int, int64, float64
func add(lhs interface{}, rhs interface{}) (interface{}, error) {
	if rhs == nil {
		return lhs, nil
	}
	if lhs == nil {
		lhs = rhs
		return lhs, nil
	}
	switch lhsv := lhs.(type) {
	// case string:
	// 	switch rhsv := rhs.(type) {
	// 	case string:
	// 	case int:
	// 	case int64:
	// 	case float64:
	// 	case time.Time:
	// 	}
	
	case int:
		switch rhsv := rhs.(type) {
		// case string:
		// 	if strconv.Itoa(lhsv) == rhsv {
		// 		return 1, nil
		// 	}
		// 	return 0, nil
		case int:
			return lhsv + rhsv, nil

		case int64:
			return int64(lhsv) + rhsv, nil

		case float64:
				return float64(lhsv) + rhsv, nil
		// case time.Time:
		}

	case int64:
		switch rhsv := rhs.(type) {
		// case string:
		case int:
			return lhsv + int64(rhsv), nil
		case int64:
			return lhsv + rhsv, nil

		case float64:
			return float64(lhsv) + rhsv, nil
		// case time.Time:
		}

	case float64:
		switch rhsv := rhs.(type) {
		// case string:
		case int:
			return lhsv + float64(rhsv), nil
		case int64:
			return lhsv + float64(rhsv), nil

		case float64:
			return lhsv + rhsv, nil
		// case time.Time:
		}

	// case time.Time:
	}
	return nil, fmt.Errorf("add called with unsupported types: (%T, %T)",lhs, rhs)
}

// TransformationColumnSpec Type sum
type sumColumnEval struct {
	inputPos int
	outputPos int
	where evalExpression
}
func (ctx *sumColumnEval) initializeCurrentValue(currentValue *[]interface{}) {
	// by default use int64, may change to float64 based on data
	(*currentValue)[ctx.outputPos] = int64(0)
}
func (ctx *sumColumnEval) update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error sumColumnEval.update cannot have nil currentValue or input")
	}
	// sum(column_name), make sure the column is not nil
	value := (*input)[ctx.inputPos]
	if value == nil {
		return nil
	}
	if ctx.where != nil {
		w, err := ctx.where.eval(input)
		if err != nil {
			return fmt.Errorf("while evaluating where on sum aggregate: %v", err)
		}
		if w == nil || w.(int) != 1 {
			return nil
		}
	}
	//* TODO Sum start with int64 as result type, upgrades to float64 if needed - update to use data model for rdf:type
	var err error
	cv := (*currentValue)[ctx.outputPos]
	cv, err = add(cv, (*input)[ctx.inputPos])
	if err != nil {
		return err
	}
	(*currentValue)[ctx.outputPos] = cv
	return nil
}
func (ctx *sumColumnEval) done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *BuilderContext) buildSumEvaluator(source *InputChannel, outCh *OutputChannel,  spec *TransformationColumnSpec) (*sumColumnEval, error) {
	if spec == nil || spec.Expr == nil {
		return nil, fmt.Errorf("error: Type sum must have Expr != nil")
	}
	inputPos, ok := source.columns[*spec.Expr]
	if !ok {
		return nil, fmt.Errorf("error, sum needs a valid column name")
	}
	var where evalExpression
	var err error
	if spec.Where != nil {
		where, err = ctx.buildExprNodeEvaluator(source, outCh, spec.Where)
		if err != nil {
			return nil, fmt.Errorf("while building where expression for sum aggregate: %v", err)
		}
	}
	outputPos, ok := outCh.columns[spec.Name]
	if !ok {
		err = fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.config.Name)
	}
	return &sumColumnEval{
		inputPos: inputPos,
		outputPos: outputPos,
		where: where,
	}, err
}


// min function used for aggregates, supports int, int64, float64, time
func minAgg(lhs interface{}, rhs interface{}) (interface{}, error) {
	if rhs == nil {
		return lhs, nil
	}
	if lhs == nil {
		return rhs, nil
	}
	switch lhsv := lhs.(type) {
	case int:
		switch rhsv := rhs.(type) {
		case int:
			return min(lhsv, rhsv), nil
		case int64:
			return min(int64(lhsv), rhsv), nil
		case float64:
				return min(float64(lhsv),  rhsv), nil
		}

	case int64:
		switch rhsv := rhs.(type) {
		case int:
			return min(lhsv, int64(rhsv)), nil
		case int64:
			return min(lhsv, rhsv), nil
		case float64:
			return min(float64(lhsv), rhsv), nil
		}

	case float64:
		switch rhsv := rhs.(type) {
		case int:
			return min(lhsv, float64(rhsv)), nil
		case int64:
			return min(lhsv, float64(rhsv)), nil
		case float64:
			return min(lhsv, rhsv), nil
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case time.Time:
			if lhsv.Before(rhsv) {
				return lhsv, nil
			}
			return rhsv, nil
		}
	}
	return nil, fmt.Errorf("minAgg called with unsupported types: (%T, %T)",lhs, rhs)
}

// TransformationColumnSpec Type min
type minColumnEval struct {
	inputPos int
	outputPos int
	where evalExpression
}
func (ctx *minColumnEval) initializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *minColumnEval) update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error minColumnEval.update cannot have nil currentValue or input")
	}
	value := (*input)[ctx.inputPos]
	if value == nil {
		return nil
	}
	if ctx.where != nil {
		w, err := ctx.where.eval(input)
		if err != nil {
			return fmt.Errorf("while evaluating where on min aggregate: %v", err)
		}
		if w == nil || w.(int) != 1 {
			return nil
		}
	}
	var err error
	(*currentValue)[ctx.outputPos], err = minAgg((*currentValue)[ctx.outputPos], (*input)[ctx.inputPos])
	if err != nil {
		return err
	}
	return nil
}
func (ctx *minColumnEval) done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *BuilderContext) buildMinEvaluator(source *InputChannel, outCh *OutputChannel,  spec *TransformationColumnSpec) (*minColumnEval, error) {
	if spec == nil || spec.Expr == nil {
		return nil, fmt.Errorf("error: Type min must have Expr != nil")
	}
	inputPos, ok := source.columns[*spec.Expr]
	if !ok {
		return nil, fmt.Errorf("error, min needs a valid column name")
	}
	var where evalExpression
	var err error
	if spec.Where != nil {
		where, err = ctx.buildExprNodeEvaluator(source, outCh, spec.Where)
		if err != nil {
			return nil, fmt.Errorf("while building where expression for min aggregate: %v", err)
		}
	}
	outputPos, ok := outCh.columns[spec.Name]
	if !ok {
		err = fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.config.Name)
	}
	return &minColumnEval{
		inputPos: inputPos,
		outputPos: outputPos,
		where: where,
	}, err
}
