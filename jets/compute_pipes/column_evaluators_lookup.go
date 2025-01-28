package compute_pipes

import (
	"fmt"
	"strings"
)

// TransformationColumnSpec Type lookup
type lookupColumnTransformationEval struct {
	lookupTable    LookupTable
	keyEvaluator   []lookupColumnEval
	valueEvaluator []lookupColumnEval
}

// type to evaluate the lookup key and returned values
type lookupColumnEval interface {
	EvalKey(input *[]interface{}) (string, error)
	EvalValue(currentValue *[]interface{}, input *[]interface{}) error
}

type lceSelect struct {
	lookupName *string
	inputPos   int
	outputPos  int
}

func (lce *lceSelect) EvalKey(input *[]interface{}) (string, error) {
	if input == nil || len(*input) <= lce.inputPos {
		return "",
			fmt.Errorf("error lceSelect.EvalKey cannot have nil input or invalid column position for lookup %s",
				*lce.lookupName)
	}
	v := (*input)[lce.inputPos]
	key, ok := v.(string)
	if !ok {
		key = fmt.Sprintf("%v", v)
	}
	return key, nil
}
func (lce *lceSelect) EvalValue(output *[]interface{}, input *[]interface{}) error {
	if output == nil || input == nil {
		return fmt.Errorf("error lceSelect.EvalValue cannot have nil output or input for lookup %s",
			*lce.lookupName)
	}
	if lce.outputPos >= len(*output) || lce.inputPos >= len(*input) {
		return fmt.Errorf("error lceSelect.EvalValue invalid column position for lookup %s", *lce.lookupName)
	}
	(*output)[lce.outputPos] = (*input)[lce.inputPos]
	return nil
}

type lceValue struct {
	lookupName *string
	value      interface{}
	outputPos  int
}

func (lce *lceValue) EvalKey(input *[]interface{}) (string, error) {
	if input == nil {
		return "",
			fmt.Errorf("error lceValue.EvalKey cannot have nil input for lookup %s", *lce.lookupName)
	}
	key, ok := lce.value.(string)
	if !ok {
		key = fmt.Sprintf("%v", lce.value)
	}
	return key, nil
}
func (lce *lceValue) EvalValue(output *[]interface{}, _ *[]interface{}) error {
	if output == nil {
		return fmt.Errorf("error lceValue.EvalValue cannot have nil output for lookup %s", *lce.lookupName)
	}
	if lce.outputPos >= len(*output) {
		return fmt.Errorf("error lceValue.EvalValue invalid column position for lookup %s", *lce.lookupName)
	}
	(*output)[lce.outputPos] = lce.value
	return nil
}

func (ctx *lookupColumnTransformationEval) InitializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *lookupColumnTransformationEval) Update(output *[]interface{}, input *[]interface{}) error {
	// lookup update
	// build the lookup key using input row
	// get the lookup record, update output row with lookup values
	if input == nil || output == nil {
		return fmt.Errorf("error lookupColumnTransformationEval.update cannot have nil output or input")
	}
	var err error
	// build the lookup key
	key := make([]string, len(ctx.keyEvaluator))
	for i := range ctx.keyEvaluator {
		k, err := ctx.keyEvaluator[i].EvalKey(input)
		if err != nil {
			return fmt.Errorf("while making the lookup key: %v", err)
		}
		key[i] = k
	}
	// Fetch the lookup row
	var keyStr string
	if len(key) == 1 {
		keyStr = key[0]
	} else {
		keyStr = strings.Join(key, "")
	}
	row, err := ctx.lookupTable.Lookup(&keyStr)
	if err != nil {
		return fmt.Errorf("while fetching the lookup row: %v", err)
	}
	if row == nil {
		// No match in the lookup
		return nil
	}
	// Update the output row
	for i := range ctx.valueEvaluator {
		err = ctx.valueEvaluator[i].EvalValue(output, row)
		if err != nil {
			return err
		}
	}
	return nil
}
func (ctx *lookupColumnTransformationEval) Done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *BuilderContext) BuildLookupTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.LookupName == nil || len(spec.LookupKey) == 0 || len(spec.LookupValues) == 0 {
		return nil, fmt.Errorf("error: Type lookup must have LookupName, LookupKey and LookupValues not empty")
	}
	keyEvaluator := make([]lookupColumnEval, len(spec.LookupKey))
	valueEvaluator := make([]lookupColumnEval, len(spec.LookupValues))

	// build the key evaluators
	for i := range spec.LookupKey {
		columnSpec := &spec.LookupKey[i]
		switch columnSpec.Type {
		case "select":
			keyEvaluator[i] = &lceSelect{
				lookupName: spec.LookupName,
				inputPos:   (*source.columns)[*columnSpec.Expr],
			}
		case "value":
			value, err := ctx.parseValue(columnSpec.Expr)
			if err != nil {
				return nil, fmt.Errorf("while building key evaluator of type 'value' for lookup %s: %v",
					*spec.LookupName, err)
			}
			keyEvaluator[i] = &lceValue{
				lookupName: spec.LookupName,
				value:      value,
			}
		}
	}
	lookupTable, ok := ctx.lookupTableManager.LookupTableMap[*spec.LookupName]
	if !ok {
		return nil, fmt.Errorf("error: lookup table '%s' not found in lookup table manager", *spec.LookupName)
	}
	// If this is an empty lookup (in the sense that it's a s3 file-based lookup but no files were found
	// and the spec indicated that it is not an error via csv_source.make_empty_source_when_no_files_found settings)
	if !lookupTable.IsEmptyTable() {
		// build the lookup value evaluators
		for i := range spec.LookupValues {
			columnSpec := &spec.LookupValues[i]
			switch columnSpec.Type {
			case "select":
				inputPos, ok := lookupTable.ColumnMap()[*columnSpec.Expr]
				if !ok {
					return nil, fmt.Errorf("error: lookup table '%s' does not have column '%s'",
						*spec.LookupName, *columnSpec.Expr)
				}
				outputPos, ok := (*outCh.columns)[columnSpec.Name]
				if !ok {
					return nil, fmt.Errorf("error: output column '%s' is not valid for lookup table '%s' (buildLookupEvaluator)",
						columnSpec.Name, *spec.LookupName)
				}
				valueEvaluator[i] = &lceSelect{
					lookupName: spec.LookupName,
					inputPos:   inputPos,
					outputPos:  outputPos,
				}
			case "value":
				value, err := ctx.parseValue(columnSpec.Expr)
				if err != nil {
					return nil, fmt.Errorf("while building lookup value evaluator of type 'value' for lookup %s: %v",
						*spec.LookupName, err)
				}
				outputPos, ok := (*outCh.columns)[columnSpec.Name]
				if !ok {
					return nil, fmt.Errorf("error: output column '%s' is not valid for lookup table '%s' (buildLookupEvaluator)",
						columnSpec.Name, *spec.LookupName)
				}
				valueEvaluator[i] = &lceValue{
					lookupName: spec.LookupName,
					value:      value,
					outputPos:  outputPos,
				}
			}
		}
	}
	return &lookupColumnTransformationEval{
		lookupTable:    lookupTable,
		keyEvaluator:   keyEvaluator,
		valueEvaluator: valueEvaluator,
	}, nil
}
