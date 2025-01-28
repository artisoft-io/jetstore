package compute_pipes

import (
	"fmt"
	"strconv"
)

// TransformationColumnSpec Type case_expr
type mapReduceColumnEval struct {
	intermediateColumns       map[string]int
	mapOnColumnIdx            int
	altInputKey               []PreprocessingFunction
	currentIntermediateValues map[string][]interface{}
	mapColumnEval             []TransformationColumnEvaluator
	reduceColumnEval          []TransformationColumnEvaluator
}

func (ctx *mapReduceColumnEval) InitializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *mapReduceColumnEval) Update(_ *[]interface{}, input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error mapReduceColumnEval.update cannot have nil currentValue or input")
	}
	var key string
	var err error
	inputVal := (*input)[ctx.mapOnColumnIdx]
	if inputVal == nil && ctx.altInputKey != nil {
		// Make the alternate key to hash
		inputVal, err = makeAlternateKey(&ctx.altInputKey, input)
		// fmt.Printf("##### # mapReduceColumnEval: makeAlternateKey got: %v\n", inputVal)
		if err != nil {
			return err
		}
	}

	if inputVal != nil {
		switch vv := inputVal.(type) {
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
				err := ctx.mapColumnEval[i].Update(&intermediateValues, input)
				if err != nil {
					return fmt.Errorf("while calling update on TransformationColumnEvaluator (map of map_reduce): %v", err)
				}
			}
		}
	}
	return nil
}
func (ctx *mapReduceColumnEval) Done(currentValue *[]interface{}) error {
	for i := range ctx.reduceColumnEval {
		ctx.reduceColumnEval[i].InitializeCurrentValue(currentValue)
	}
	for _, intermediateInput := range ctx.currentIntermediateValues {
		for i := range ctx.reduceColumnEval {
			err := ctx.reduceColumnEval[i].Update(currentValue, &intermediateInput)
			if err != nil {
				return fmt.Errorf("while calling update on TransformationColumnEvaluator (reduce of map_reduce): %v", err)
			}
		}
	}
	for i := range ctx.reduceColumnEval {
		err := ctx.reduceColumnEval[i].Done(currentValue)
		if err != nil {
			return fmt.Errorf("while calling done on TransformationColumnEvaluator (reduce of map_reduce): %v", err)
		}
	}
	// for k,v := range ctx.currentIntermediateValues {
	// 	fmt.Println("**!@@ MAP REDUCE intermediate values by key",k,": ",v)
	// }
	return nil
}

func (ctx *BuilderContext) BuildMapReduceTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.MapOn == nil || len(spec.ApplyMap) == 0 || len(spec.ApplyReduce) == 0 {
		return nil, fmt.Errorf("error: Type map_reduce must have MapOn, ApplyMap and ApplyReduce not empty")
	}
	var err error
	mapOnColumnIdx, ok := (*source.columns)[*spec.MapOn]
	if !ok {
		return nil, fmt.Errorf("error column %s not found in input source %s", *spec.MapOn, source.name)
	}
	var altInputKey []PreprocessingFunction
	if len(spec.AlternateMapOn) > 0 {
		altInputKey, err = ParseAltKeyDefinition(spec.AlternateMapOn, source.columns)
		if err != nil {
			return nil, fmt.Errorf("buildMapReduceEvaluator: %v in source name %s", err, source.name)
		}
	}

	intermediateColumns := make(map[string]int)
	for i := range spec.ApplyMap {
		intermediateColumns[spec.ApplyMap[i].Name] = i
	}

	mapColumnEval := make([]TransformationColumnEvaluator, len(spec.ApplyMap))
	intermediateOutputChannel := &OutputChannel{
		columns: &intermediateColumns,
		config: &ChannelSpec{
			Name:      "map_reduce.intermediateOutputChannel",
			ClassName: source.config.ClassName,
		},
	}
	for i := range spec.ApplyMap {
		mapColumnEval[i], err = ctx.BuildTransformationColumnEvaluator(source, intermediateOutputChannel, &spec.ApplyMap[i])
		if err != nil {
			return nil,
				fmt.Errorf("while building Column Transformation Evaluator (map of map_reduce) for column %s: %v", spec.ApplyMap[i].Name, err)
		}
	}

	reduceColumnEval := make([]TransformationColumnEvaluator, len(spec.ApplyReduce))
	intermediateInputChannel := &InputChannel{
		columns: &intermediateColumns,
		config: &ChannelSpec{
			Name:      "map_reduce.intermediateInputChannel",
			ClassName: source.config.ClassName,
		},
	}
	for i := range spec.ApplyReduce {
		reduceColumnEval[i], err = ctx.BuildTransformationColumnEvaluator(intermediateInputChannel, outCh, &spec.ApplyReduce[i])
		if err != nil {
			return nil,
				fmt.Errorf("while building Column Transformation Evaluator (reduce of map_reduce) for column %s: %v", spec.ApplyReduce[i].Name, err)
		}
	}

	return &mapReduceColumnEval{
		mapOnColumnIdx:            mapOnColumnIdx,
		altInputKey:               altInputKey,
		intermediateColumns:       intermediateColumns,
		currentIntermediateValues: make(map[string][]interface{}),
		mapColumnEval:             mapColumnEval,
		reduceColumnEval:          reduceColumnEval,
	}, nil
}
