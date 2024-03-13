package compute_pipes

import (
	"fmt"
	"strconv"
)

// TransformationColumnSpec Type case_expr
type mapReduceColumnEval struct {
	intermediateColumns map[string]int
	mapOnColumnIdx int
	currentIntermediateValues map[string][]interface{}
	mapColumnEval []TransformationColumnEvaluator
	reduceColumnEval []TransformationColumnEvaluator
}

func (ctx *mapReduceColumnEval) initializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *mapReduceColumnEval) update(_ *[]interface{}, input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error mapReduceColumnEval.update cannot have nil currentValue or input")
	}
	var key string
	v := (*input)[ctx.mapOnColumnIdx]
	if v != nil {
		switch vv := v.(type) {
		case string:
			key = vv
		case int:
			key = strconv.Itoa(vv)
		}
		if len(key) > 0 {
			intermediateValues := ctx.currentIntermediateValues[key]
			if intermediateValues == nil {
				intermediateValues = make([]interface{}, len(ctx.intermediateColumns))
				ctx.currentIntermediateValues[key] = intermediateValues
			}
			for i := range ctx.mapColumnEval {
				err := ctx.mapColumnEval[i].update(&intermediateValues, input)
				if err != nil {
					return fmt.Errorf("while calling update on TransformationColumnEvaluator (map of map_reduce): %v", err)
				}		
			}
		}
	}
	return nil
}
func (ctx *mapReduceColumnEval) done(currentValue *[]interface{}) error {
	for i := range ctx.reduceColumnEval {
		ctx.reduceColumnEval[i].initializeCurrentValue(currentValue)
	}
	for _, intermediateInput := range ctx.currentIntermediateValues {
		for i := range ctx.reduceColumnEval {
			err := ctx.reduceColumnEval[i].update(currentValue, &intermediateInput)
			if err != nil {
				return fmt.Errorf("while calling update on TransformationColumnEvaluator (reduce of map_reduce): %v", err)
			}
		}
	}
	for i := range ctx.reduceColumnEval {
		err := ctx.reduceColumnEval[i].done(currentValue)
		if err != nil {
			return fmt.Errorf("while calling done on TransformationColumnEvaluator (reduce of map_reduce): %v", err)
		}
	}
	// for k,v := range ctx.currentIntermediateValues {
	// 	fmt.Println("**! MAP REDUCE intermediate values by key",k,": ",v)
	// }
	return nil
}

func (ctx *BuilderContext) buildMapReduceEvaluator(source *InputChannel, outCh *OutputChannel,  spec *TransformationColumnSpec) (*mapReduceColumnEval, error) {
	if spec == nil || spec.MapOn == nil || spec.ApplyMap == nil || spec.ApplyReduce == nil {
		return nil, fmt.Errorf("error: Type map_reduce must have MapOn, ApplyMap and ApplyReduce not nil")
	}
	var err error
	mapOnColumnIdx, ok := source.columns[*spec.MapOn]
	if !ok {
		err = fmt.Errorf("error column %s not found in input source %s", *spec.MapOn, source.config.Name)
	}
	intermediateColumns := make(map[string]int)
	for i := range *spec.ApplyMap {
		intermediateColumns[(*spec.ApplyMap)[i].Name] = i
	}

	mapColumnEval := make([]TransformationColumnEvaluator, len(*spec.ApplyMap))
	intermediateOutputChannel := &OutputChannel{
		columns: intermediateColumns,
		config: &ChannelSpec{Name: "map_reduce.intermediateOutputChannel"},
	}
	for i := range *spec.ApplyMap {
		mapColumnEval[i], err = ctx.buildTransformationColumnEvaluator(source, intermediateOutputChannel, &(*spec.ApplyMap)[i])
		if err != nil {
			return nil, 
				fmt.Errorf("while building Column Transformation Evaluator (map of map_reduce) for column %s: %v", (*spec.ApplyMap)[i].Name, err)
		}
	}

	reduceColumnEval := make([]TransformationColumnEvaluator, len(*spec.ApplyReduce))
	intermediateInputChannel := &InputChannel{
		columns: intermediateColumns,
		config: &ChannelSpec{Name: "map_reduce.intermediateInputChannel"},
	}
	for i := range *spec.ApplyReduce {
		reduceColumnEval[i], err = ctx.buildTransformationColumnEvaluator(intermediateInputChannel, outCh, &(*spec.ApplyReduce)[i])
		if err != nil {
			return nil, 
				fmt.Errorf("while building Column Transformation Evaluator (reduce of map_reduce) for column %s: %v", (*spec.ApplyReduce)[i].Name, err)
		}
	}

	return &mapReduceColumnEval{
		mapOnColumnIdx: mapOnColumnIdx,
		intermediateColumns: intermediateColumns,
		currentIntermediateValues: make(map[string][]interface{}),
		mapColumnEval: mapColumnEval,
		reduceColumnEval: reduceColumnEval,
	}, err
}

