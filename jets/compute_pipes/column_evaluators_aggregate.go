package compute_pipes

import (
	"fmt"
	"strconv"
	"time"
)

// TransformationColumnSpec Type count
type countColumnEval struct {
	inputPos  int
	outputPos int
	where     evalExpression
}

func (ctx *countColumnEval) Update(currentValue *[]any, input *[]any) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error countColumnEval.update cannot have nil currentValue or input")
	}
	// if count(column_name), make sure the column is not nil
	if ctx.inputPos >= 0 && (*input)[ctx.inputPos] == nil {
		return nil
	}
	if ctx.where != nil {
		w, err := ctx.where.Eval( *input)
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
func (ctx *countColumnEval) Done(currentValue *[]any) error {
	return nil
}

func (ctx *BuilderContext) BuildCountTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.Expr == nil {
		return nil, fmt.Errorf("error: Type count must have Expr != nil")
	}
	inputPos := -1
	var ok bool
	if *spec.Expr != "*" {
		inputPos, ok = (*source.Columns)[*spec.Expr]
		if !ok {
			return nil, fmt.Errorf("error: count needs a valid column name or * to count all rows")
		}
	}
	var where evalExpression
	var err error
	if spec.Where != nil {
		where, err = ctx.BuildExprNodeEvaluator(source.Name, *source.Columns, spec.Where)
		if err != nil {
			return nil, fmt.Errorf("while building where expression: %v", err)
		}
	}
	outputPos, ok := (*outCh.Columns)[spec.Name]
	if !ok {
		return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.Name)
	}
	return &countColumnEval{
		inputPos:  inputPos,
		outputPos: outputPos,
		where:     where,
	}, nil
}

// TransformationColumnSpec Type distinct_count
type distinctCountColumnEval struct {
	inputPos  int
	outputPos int
	where     evalExpression
}

func (ctx *distinctCountColumnEval) Update(currentValue *[]any, input *[]any) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error countColumnEval.update cannot have nil currentValue or input")
	}
	// if count(column_name), make sure the column is not nil
	value := (*input)[ctx.inputPos]
	if value == nil {
		return nil
	}
	if ctx.where != nil {
		w, err := ctx.where.Eval( *input)
		if err != nil {
			return fmt.Errorf("while evaluating where on distinct_count aggregate: %v", err)
		}
		if w == nil || w.(int) != 1 {
			return nil
		}
	}
	var valuesTxt string
	switch vv := value.(type) {
	case string:
		valuesTxt = vv
	case int:
		valuesTxt = strconv.Itoa(vv)
	case int64:
		valuesTxt = strconv.FormatInt(vv, 10)
	case float64:
		valuesTxt = strconv.FormatFloat(vv, 'f', -1, 64)
	default:
		valuesTxt = fmt.Sprintf("%v", vv)
	}
	// The operator must be stateless, keep the distinct values in currentValue
	var distinctValues map[string]bool
	m := (*currentValue)[ctx.outputPos]
	if m == nil {
		distinctValues = make(map[string]bool)
		(*currentValue)[ctx.outputPos] = distinctValues
	} else {
		distinctValues = m.(map[string]bool)
	}
	distinctValues[valuesTxt] = true
	return nil
}
func (ctx *distinctCountColumnEval) Done(currentValue *[]any) error {
	if currentValue == nil {
		return nil
	}
	m := (*currentValue)[ctx.outputPos]
	if m == nil {
		(*currentValue)[ctx.outputPos] = int64(0)
	} else {
		distinctValues := m.(map[string]bool)
		(*currentValue)[ctx.outputPos] = int64(len(distinctValues))
	}
	return nil
}

func (ctx *BuilderContext) BuildDistinctCountTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.Expr == nil {
		return nil, fmt.Errorf("error: Type distinct_count must have Expr != nil")
	}
	inputPos, ok := (*source.Columns)[*spec.Expr]
	if !ok {
		return nil, fmt.Errorf("error, count_distinct needs a valid column name")
	}
	var where evalExpression
	var err error
	if spec.Where != nil {
		where, err = ctx.BuildExprNodeEvaluator(source.Name, *source.Columns, spec.Where)
		if err != nil {
			return nil, fmt.Errorf("while building where expression: %v", err)
		}
	}
	outputPos, ok := (*outCh.Columns)[spec.Name]
	if !ok {
		return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.Name)
	}
	return &distinctCountColumnEval{
		inputPos:  inputPos,
		outputPos: outputPos,
		where:     where,
	}, nil
}

// add function used for aggregates, supports int, int64, float64
func add(lhs any, rhs any, castFnc *CastToRdfFnc) (any, error) {
	var err error
	if rhs == nil {
		return lhs, nil
	}
	if lhs == nil {
		if castFnc != nil {
			lhs, err = castFnc.Cast(rhs)
		} else {
			lhs = rhs
		}
		return lhs, err
	}

	switch lhsv := lhs.(type) {
	case int:
		switch rhsv := rhs.(type) {
		case int:
			return lhsv + rhsv, nil
		case int64:
			return int64(lhsv) + rhsv, nil
		case float64:
			return float64(lhsv) + rhsv, nil
		case string:
			rhsvInt, err := strconv.Atoi(rhsv)
			return lhsv + rhsvInt, err
		}

	case int64:
		switch rhsv := rhs.(type) {
		case int:
			return lhsv + int64(rhsv), nil
		case int64:
			return lhsv + rhsv, nil
		case float64:
			return float64(lhsv) + rhsv, nil
		case string:
			rhsvInt, err := strconv.ParseInt(rhsv, 10, 64)
			return lhsv + rhsvInt, err
		}

	case float64:
		switch rhsv := rhs.(type) {
		case int:
			return lhsv + float64(rhsv), nil
		case int64:
			return lhsv + float64(rhsv), nil
		case float64:
			return lhsv + rhsv, nil
		case string:
			rhsvFloat, err := strconv.ParseFloat(rhsv, 64)
			return lhsv + rhsvFloat, err
		}
	}
	return nil, fmt.Errorf("add called with unsupported types: (%T, %T)", lhs, rhs)
}

// TransformationColumnSpec Type sum
type sumColumnEval struct {
	inputPos     int
	outputPos    int
	where        evalExpression
	cast2RdfType *CastToRdfFnc
}

func (ctx *sumColumnEval) Update(currentValue *[]any, input *[]any) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error sumColumnEval.update cannot have nil currentValue or input")
	}
	// sum(column_name), make sure the column is not nil
	value := (*input)[ctx.inputPos]
	if value == nil {
		return nil
	}
	if ctx.where != nil {
		w, err := ctx.where.Eval( *input)
		if err != nil {
			return fmt.Errorf("while evaluating where on sum aggregate: %v", err)
		}
		if w == nil || w.(int) != 1 {
			return nil
		}
	}
	var err error
	cv := (*currentValue)[ctx.outputPos]
	cv, err = add(cv, (*input)[ctx.inputPos], ctx.cast2RdfType)
	if err != nil {
		return err
	}
	(*currentValue)[ctx.outputPos] = cv
	return nil
}
func (ctx *sumColumnEval) Done(currentValue *[]any) error {
	return nil
}

func (ctx *BuilderContext) BuildSumTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.Expr == nil {
		return nil, fmt.Errorf("error: Type sum must have Expr != nil")
	}
	inputPos, ok := (*source.Columns)[*spec.Expr]
	if !ok {
		return nil, fmt.Errorf("error, sum needs a valid column name")
	}
	var where evalExpression
	var err error
	if spec.Where != nil {
		where, err = ctx.BuildExprNodeEvaluator(source.Name, *source.Columns, spec.Where)
		if err != nil {
			return nil, fmt.Errorf("while building where expression for sum aggregate: %v", err)
		}
	}
	outputPos, ok := (*outCh.Columns)[spec.Name]
	if !ok {
		return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.Name)
	}
	var cast2RdfType *CastToRdfFnc
	if spec.AsRdfType != "" {
		cast2RdfType = NewCastToRdfFnc("", spec.AsRdfType, false)
	}
	return &sumColumnEval{
		inputPos:     inputPos,
		outputPos:    outputPos,
		where:        where,
		cast2RdfType: cast2RdfType,
	}, nil
}

// min function used for aggregates, supports int, int64, float64, time
func minMaxAgg(lhs any, rhs any, castFnc *CastToRdfFnc, isMin bool) (any, error) {
	var err error
	if rhs == nil {
		return lhs, nil
	}
	if lhs == nil {
		if castFnc != nil {
			lhs, err = castFnc.Cast(rhs)
		} else {
			lhs = rhs
		}
		return lhs, err
	}
	switch lhsv := lhs.(type) {
	case int:
		switch rhsv := rhs.(type) {
		case int:
			if isMin {
				return min(lhsv, rhsv), nil
			}
			return max(lhsv, rhsv), nil
		case int64:
			if isMin {
				return min(int64(lhsv), rhsv), nil
			}
			return max(int64(lhsv), rhsv), nil
		case float64:
			if isMin {
				return min(float64(lhsv), rhsv), nil
			}
			return max(float64(lhsv), rhsv), nil
		case string:
			rhsvInt, err := strconv.Atoi(rhsv)
			if isMin {
				return min(lhsv, rhsvInt), err
			}
			return max(lhsv, rhsvInt), err
		}

	case int64:
		switch rhsv := rhs.(type) {
		case int:
			if isMin {
				return min(lhsv, int64(rhsv)), nil
			}
			return max(lhsv, int64(rhsv)), nil
		case int64:
			if isMin {
				return min(lhsv, rhsv), nil
			}
			return max(lhsv, rhsv), nil
		case float64:
			if isMin {
				return min(float64(lhsv), rhsv), nil
			}
			return max(float64(lhsv), rhsv), nil
		case string:
			rhsvInt, err := strconv.ParseInt(rhsv, 10, 64)
			if isMin {
				return min(lhsv, rhsvInt), err
			}
			return max(lhsv, rhsvInt), err
		}

	case float64:
		switch rhsv := rhs.(type) {
		case int:
			if isMin {
				return min(lhsv, float64(rhsv)), nil
			}
			return max(lhsv, float64(rhsv)), nil
		case int64:
			if isMin {
				return min(lhsv, float64(rhsv)), nil
			}
			return max(lhsv, float64(rhsv)), nil
		case float64:
			if isMin {
				return min(lhsv, rhsv), nil
			}
			return max(lhsv, rhsv), nil
		case string:
			rhsvFloat, err := strconv.ParseFloat(rhsv, 64)
			if isMin {
				return min(lhsv, rhsvFloat), err
			}
			return max(lhsv, rhsvFloat), err
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case time.Time:
			if isMin {
				if lhsv.Before(rhsv) {
					return lhsv, nil
				}
				return rhsv, nil
			} else {
				if lhsv.After(rhsv) {
					return lhsv, nil
				}
				return rhsv, nil
			}
		case string:
			rhsvTime, err := ParseDate(rhsv)
			if err != nil {
				return nil, err
			}
			if isMin {
				if lhsv.Before(*rhsvTime) {
					return lhsv, nil
				}
				return *rhsvTime, nil
			} else {
				if lhsv.After(*rhsvTime) {
					return lhsv, nil
				}
				return *rhsvTime, nil
			}
		}
	}
	return nil, fmt.Errorf("minAgg called with unsupported types: (%T, %T)", lhs, rhs)
}

// TransformationColumnSpec Type min
type minMaxColumnEval struct {
	inputPos     int
	outputPos    int
	where        evalExpression
	isMin        bool
	cast2RdfType *CastToRdfFnc
}

func (ctx *minMaxColumnEval) Update(currentValue *[]any, input *[]any) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error minColumnEval.update cannot have nil currentValue or input")
	}
	value := (*input)[ctx.inputPos]
	if value == nil {
		return nil
	}
	if ctx.where != nil {
		w, err := ctx.where.Eval( *input)
		if err != nil {
			return fmt.Errorf("while evaluating where on min aggregate: %v", err)
		}
		if w == nil || w.(int) != 1 {
			return nil
		}
	}
	var err error
	(*currentValue)[ctx.outputPos], err = minMaxAgg((*currentValue)[ctx.outputPos], (*input)[ctx.inputPos],
		ctx.cast2RdfType, ctx.isMin)
	if err != nil {
		return err
	}
	return nil
}
func (ctx *minMaxColumnEval) Done(currentValue *[]any) error {
	return nil
}

func (ctx *BuilderContext) BuildMinMaxTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec, isMin bool) (TransformationColumnEvaluator, error) {
	if spec == nil || spec.Expr == nil {
		if isMin {
			return nil, fmt.Errorf("error: Type min must have Expr != nil")
		} else {
			return nil, fmt.Errorf("error: Type max must have Expr != nil")
		}
	}
	inputPos, ok := (*source.Columns)[*spec.Expr]
	if !ok {
		return nil, fmt.Errorf("error, min/max needs a valid column name")
	}
	var where evalExpression
	var err error
	if spec.Where != nil {
		where, err = ctx.BuildExprNodeEvaluator(source.Name, *source.Columns, spec.Where)
		if err != nil {
			return nil, fmt.Errorf("while building where expression for min/max aggregate: %v", err)
		}
	}
	outputPos, ok := (*outCh.Columns)[spec.Name]
	if !ok {
		return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.Name)
	}
	var cast2RdfType *CastToRdfFnc
	if spec.AsRdfType != "" {
		cast2RdfType = NewCastToRdfFnc("", spec.AsRdfType, false)
	}
	return &minMaxColumnEval{
		inputPos:     inputPos,
		outputPos:    outputPos,
		where:        where,
		isMin:        isMin,
		cast2RdfType: cast2RdfType,
	}, nil
}

// TransformationColumnSpec Type avrg
type avrgColumnEval struct {
	inputPos     int
	outputPos    int
	where        evalExpression
	nbrSamples   int64
	cast2RdfType *CastToRdfFnc
}

func (ctx *avrgColumnEval) alphaFactor() float64 {
	if ctx.nbrSamples <= 1 {
		return 1.0
	}
	return 2.0 / (float64(ctx.nbrSamples) + 1.0)
}

func (ctx *avrgColumnEval) calcAvrg(lhs, rhs float64) float64 {
	alpha := ctx.alphaFactor()
	return alpha*rhs + (1.0-alpha)*lhs
}

// avrg function used for aggregates, supports int, int64, float64
func (ctx *avrgColumnEval) avrg(lhs any, rhs any, castFnc *CastToRdfFnc) (any, error) {
	var err error
	if rhs == nil {
		return lhs, nil
	}
	if lhs == nil {
		if castFnc != nil {
			lhs, err = castFnc.Cast(rhs)
		} else {
			lhs = rhs
		}
		return lhs, err
	}

	switch lhsv := lhs.(type) {

	case float64:
		switch rhsv := rhs.(type) {
		case float64:
			return ctx.calcAvrg(lhsv, rhsv), nil
		case string:
			rhsvFloat, err := strconv.ParseFloat(rhsv, 64)
			return ctx.calcAvrg(lhsv, rhsvFloat), err
		}
	}
	return nil, fmt.Errorf("avrg called with unsupported types: (%T, %T)", lhs, rhs)
}

func (ctx *avrgColumnEval) Update(currentValue *[]any, input *[]any) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error avrgColumnEval.update cannot have nil currentValue or input")
	}
	// avrg(column_name), make sure the column is not nil
	value := (*input)[ctx.inputPos]
	if value == nil {
		return nil
	}
	if ctx.where != nil {
		w, err := ctx.where.Eval( *input)
		if err != nil {
			return fmt.Errorf("while evaluating where on avrg aggregate: %v", err)
		}
		if w == nil || w.(int) != 1 {
			return nil
		}
	}
	var err error
	ctx.nbrSamples++
	cv := (*currentValue)[ctx.outputPos]
	cv, err = ctx.avrg(cv, value, ctx.cast2RdfType)
	if err != nil {
		return err
	}
	(*currentValue)[ctx.outputPos] = cv
	return nil
}
func (ctx *avrgColumnEval) Done(currentValue *[]any) error {
	return nil
}

func (ctx *BuilderContext) BuildAvrgTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.Expr == nil {
		return nil, fmt.Errorf("error: Type avrg must have Expr != nil")
	}
	inputPos, ok := (*source.Columns)[*spec.Expr]
	if !ok {
		return nil, fmt.Errorf("error, avrg needs a valid column name")
	}
	var where evalExpression
	var err error
	if spec.Where != nil {
		where, err = ctx.BuildExprNodeEvaluator(source.Name, *source.Columns, spec.Where)
		if err != nil {
			return nil, fmt.Errorf("while building where expression for avrg aggregate: %v", err)
		}
	}
	outputPos, ok := (*outCh.Columns)[spec.Name]
	if !ok {
		return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.Name)
	}
	var cast2RdfType *CastToRdfFnc
	if spec.AsRdfType != "" {
		cast2RdfType = NewCastToRdfFnc("", spec.AsRdfType, false)
	}
	return &avrgColumnEval{
		inputPos:     inputPos,
		outputPos:    outputPos,
		where:        where,
		cast2RdfType: cast2RdfType,
	}, nil
}
