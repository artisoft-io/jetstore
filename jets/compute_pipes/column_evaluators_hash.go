package compute_pipes

import (
	"fmt"
	"hash/fnv"
)

// TransformationColumnSpec Type hash
type hashColumnEval struct {
	inputPos    int
	outputPos   int
	partitions  uint64
	format      string
	defaultExpr *evalExpression
}

func Hash(key []byte, partitions uint64) uint64 {
	h := fnv.New64a()
	h.Write(key)
	keyHash := h.Sum64()
	if partitions > 0 {
		keyHash = keyHash % partitions
	}
	return keyHash
}
func partition(key, partitions uint64) uint64 {
	if partitions > 0 {
		key = key % partitions
	}
	return key
}

func EvalHash(key interface{}, partitions uint64) *uint64 {
	if key == nil {
		return nil
	}
	var hashedValue uint64
	switch vv := key.(type) {
	case string:
		hashedValue = Hash([]byte(vv), partitions)
	case int:
		hashedValue = partition(uint64(vv), partitions)
	case uint:
		hashedValue = partition(uint64(vv), partitions)
	case int32:
		hashedValue = partition(uint64(vv), partitions)
	case uint32:
		hashedValue = partition(uint64(vv), partitions)
	case int64:
		hashedValue = partition(uint64(vv), partitions)
	case uint64:
		hashedValue = partition(uint64(vv), partitions)
	case []byte:
		hashedValue = Hash(vv, partitions)
	case bool:
		if vv {
			hashedValue = uint64(1)
		} else {
			hashedValue = uint64(0)
		}
	default:
		hashedValue = Hash([]byte(fmt.Sprintf("%v", vv)), partitions)
	}
	return &hashedValue
}

func (ctx *hashColumnEval) initializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *hashColumnEval) update(currentValue *[]interface{}, input *[]interface{}) error {
	var err error
	if currentValue == nil || input == nil {
		return fmt.Errorf("error hashColumnEval.update cannot have nil currentValue or input")
	}
	// update currentValue using input applying cleansing function and default value
	inputVal := (*input)[ctx.inputPos]
	var hashedValue interface{}

	if inputVal == nil && ctx.defaultExpr != nil {
		// Apply default
		hashedValue, err = (*ctx.defaultExpr).eval(input)
	} else {
		h := EvalHash(inputVal, ctx.partitions)
		if h != nil {
			hashedValue = *h
			// fmt.Printf("##### # EvalHash k: %v, nbr: %d => %v\n", inputVal, ctx.partitions, hashedValue)
		} else {
			// fmt.Printf("##### # EvalHash k: %v, nbr: %d => NULL\n", inputVal, ctx.partitions)
		}
		if len(ctx.format) > 0 {
			hashedValue = fmt.Sprintf(ctx.format, hashedValue)
		}
	}

	(*currentValue)[ctx.outputPos] = hashedValue
	return err
}
func (ctx *hashColumnEval) done(currentValue *[]interface{}) error {
	return nil
}

// The Hash operator full example (dw_rawfilename is string):
// jets_partition as a string (applies the format):
//
//	{
//		"name": "jets_partition",
//		"type": "hash",
//		"hash_expr": {
//			"expr": "dw_rawfilename",
//			"nbr_jets_partitions": 3,
//			"format": "%04d",
//			"default_expr": {
//				"type": "value",
//				"expr": "1"
//			}
//		}
//
// jets_partition as uint64:
//
//	{
//		"name": "jets_partition",
//		"type": "hash",
//		"hash_expr": {
//			"expr": "dw_rawfilename",
//			"nbr_jets_partitions": 3
//		}
//	},
//
// jets_partition will be of type uint64 if expr column is of integral type.
func (ctx *BuilderContext) buildHashEvaluator(source *InputChannel, outCh *OutputChannel, spec *TransformationColumnSpec) (*hashColumnEval, error) {
	var err error
	if spec == nil || spec.HashExpr == nil {
		return nil, fmt.Errorf("error: Type map must have HashExpr != nil")
	}
	inputPos, ok := source.columns[spec.HashExpr.Expr]
	if !ok {
		err = fmt.Errorf("error column %s not found in input source %s", *spec.Expr, source.config.Name)
	}
	outputPos, ok := outCh.columns[spec.Name]
	if !ok {
		err = fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.config.Name)
	}
	var partitions uint64
	if spec.HashExpr.NbrJetsPartitions != nil {
		partitions = *spec.HashExpr.NbrJetsPartitions
	} else {
		partitions = ctx.cpConfig.ClusterConfig.NbrJetsPartitions
	}
	var format string
	if spec.HashExpr.Format != nil {
		format = *spec.HashExpr.Format
	}
	var defaultExpr evalExpression
	if spec.HashExpr.DefaultExpr != nil {
		defaultExpr, err = ctx.buildExprNodeEvaluator(source, outCh, spec.HashExpr.DefaultExpr)
		if err != nil {
			return nil, fmt.Errorf("while building the default expr in Hash operator: %v", err)
		}
	}
	return &hashColumnEval{
		inputPos:    inputPos,
		outputPos:   outputPos,
		partitions:  partitions,
		format:      format,
		defaultExpr: &defaultExpr,
	}, err
}
