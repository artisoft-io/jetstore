package compute_pipes

import (
	"bytes"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"

	"github.com/artisoft-io/jetstore/jets/csv"
)

// firstInputRow is the first row from the input channel.
// A reference to it is kept for use in the Done function
// so to carry over the select fields in the columnEvaluators.
// Note: columnEvaluators is applied only on the firstInputRow
// and it is used only to select column having same value for every input row
// or to put constant values comming from the env
//
// Base columns available on the output (only columns specified in outputCh
// are actually send out):
//
//	"column_name",
//	"column_pos",
//	"input_data_type",
//	"entity_hint",
//	"distinct_count",
//	"distinct_count_pct",
//	"distinct_values",
//	"null_count",
//	"null_count_pct",
//	"total_count",
//	"avr_length",
//	"length_var"
//
// Other base columns available when using parse function (parse_date, parse_double, parse_text)
//
//	"min_date",
//	"max_date",
//	"min_double",
//	"max_double",
//	"large_double_pct",
//	"min_length",
//	"max_length",
//	"min_value",
//	"max_value",
//	"large_value_pct",
//	"minmax_type"
//
// Note: for min_value/max_value are determined based on this priority rule:
//  1. min_date/max_date if more than 50% of values are valid dates;
//  2. min_double/max_double if more than 75% of values are valid double (note this includes int as well);
//  3. otherwise it's the text min/max length.
//
// Note: distinct_values will contains a comma-separated list of distinct value
// if distinct_count < spec.distinct_values_when_less_than_count. There is a hardcoded
// check that cap distinct_values_when_less_than_count to 20.
//
// column_name: the name of the column using the original column name if available, otherwise
// it is the column name from the input channel.
// entity_hint: is determined based on the hints provided in spec.analyze_config.entity_hints
//
// Other columns are added based on regex_tokens, lookup_tokens, keyword_tokens, and parse functions
// The value of the domain counts are expressed in percentage of the non null count:
//
//	ratio = <domain count>/(totalCount - nullCount) * 100.0
//
// Note that if totalCount - nullCount == 0, then ratio = -1.
//
// inputDataType contains the data type for each column according to the parquet schema.
// inputDataType is a map of column name -> input data type
// Range of value for input data type: string (default if not parquet), bool, int32, int64,
// float32, float64, date, uint32, uint64
type AnalyzeTransformationPipe struct {
	cpConfig          *ComputePipesConfig
	source            *InputChannel
	outputCh          *OutputChannel
	inputDataType     map[string]string
	colFragment2Hint  map[string]string
	colName2Token     map[string]string
	colFragment2Token map[string]string
	analyzeState      []*AnalyzeState
	columnEvaluators  []TransformationColumnEvaluator
	nbrRowsAnalyzed   int
	firstInputRow     *[]any
	spec              *TransformationSpec
	padShortRows      bool
	env               map[string]any
	doneCh            chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *AnalyzeTransformationPipe) Apply(input *[]any) error {
	var err error
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in AnalyzeTransformationPipe")
	}
	inputLen := len(*input)
	expectedLen := len(ctx.source.Config.Columns)
	if inputLen < expectedLen {
		if ctx.padShortRows {
			for range expectedLen - inputLen {
				*input = append(*input, nil)
			}
		} else {
			// Skip the row
			log.Println("*** AnalyzeTransformationPipe.Aplyt: INVALID ROW LEN", inputLen, "expecting", expectedLen, "columns")
			return nil
		}
	}

	if ctx.firstInputRow == nil {
		ctx.firstInputRow = input
	}
	ctx.nbrRowsAnalyzed++
	for i := range expectedLen {
		analyzeState := ctx.analyzeState[i]
		err = analyzeState.NewValue((*input)[i])
		if err != nil {
			return fmt.Errorf("while calling NewValue on AnalyzeState: %v", err)
		}
	}
	return nil
}

// Analysis complete, now send out the results to ctx.outputCh.
// A row is produced for each column state in ctx.analyzeState.

func (ctx *AnalyzeTransformationPipe) Done() error {
	config := ctx.spec.AnalyzeConfig
	// For each column state in ctx.analyzeState, send out a row to ctx.outputCh
	var ok bool
	if ctx.firstInputRow == nil {
		err := fmt.Errorf("error: the input file contains no data rows, cannot perform file analysis")
		log.Printf("AnalyzeTransformationPipe.Done: Number of rows analyzed is %d", ctx.nbrRowsAnalyzed)
		log.Println(err)
		return err
	}
	if ctx.cpConfig.ClusterConfig.IsDebugMode {
		log.Printf("AnalyzeTransformationPipe.Done: Number of rows analyzed is %d", ctx.nbrRowsAnalyzed)
	}
	for _, state := range ctx.analyzeState {
		outputRow := make([]any, len(*ctx.outputCh.Columns))

		// The first base columns
		var ipos int
		ipos, ok = (*ctx.outputCh.Columns)["column_name"]
		if ok {
			outputRow[ipos] = state.ColumnName
		}

		ipos, ok = (*ctx.outputCh.Columns)["column_pos"]
		if ok {
			outputRow[ipos] = state.ColumnPos
		}

		ipos, ok = (*ctx.outputCh.Columns)["input_data_type"]
		if ok {
			outputRow[ipos] = ctx.inputDataType[state.ColumnName]
		}

		// Determine the classification token based on column name or fragment if available
		columnNameUpper := strings.ToUpper(state.ColumnName)
		if config.ColumnNameToken != nil {
			ipos, ok = (*ctx.outputCh.Columns)[config.ColumnNameToken.Name]
			if ok {
				token, found := ctx.colName2Token[columnNameUpper]
				if !found {
					for frag, tok := range ctx.colFragment2Token {
						if strings.Contains(columnNameUpper, frag) {
							token = tok
							found = true
							break
						}
					}
				}
				if found {
					outputRow[ipos] = token
				}
			}
		}

		// Determine the entity hint based on the hints provided in spec.analyze_config.entity_hints
		if len(config.EntityHints) > 0 {
			ipos, ok = (*ctx.outputCh.Columns)["entity_hint"]
			if ok {
				for frag, hint := range ctx.colFragment2Hint {
					if strings.Contains(columnNameUpper, frag) {
						outputRow[ipos] = hint
						break
					}
				}
			}
		}

		var ratioFactor float64
		if state.TotalRowCount != state.NullCount {
			ratioFactor = 100.0 / float64(state.TotalRowCount-state.NullCount)
		}

		distinctCount := len(state.DistinctValues)
		ipos, ok = (*ctx.outputCh.Columns)["distinct_count"]
		if ok {
			outputRow[ipos] = distinctCount
		}

		ipos, ok = (*ctx.outputCh.Columns)["distinct_count_pct"]
		if ok {
			if ratioFactor > 0 {
				outputRow[ipos] = float64(distinctCount) * ratioFactor
			} else {
				outputRow[ipos] = -1.0
			}
		}

		ipos, ok = (*ctx.outputCh.Columns)["distinct_values"]
		if ok && distinctCount < config.DistinctValuesWhenLessThanCount {
			distinctValues := slices.Sorted(maps.Keys(state.DistinctValues))
			buf := new(bytes.Buffer)
			w := csv.NewWriter(buf)
			err := w.Write(distinctValues)
			if err != nil {
				outputRow[ipos] = err.Error()
			} else {
				w.Flush()
				dv := strings.TrimSuffix(buf.String(), "\n")
				// fmt.Printf("*** DISTINCT VALUES for %s: %v\n",state.ColumnName, dv)
				outputRow[ipos] = dv
			}
		}

		ipos, ok = (*ctx.outputCh.Columns)["null_count"]
		if ok {
			outputRow[ipos] = state.NullCount
		}

		ipos, ok = (*ctx.outputCh.Columns)["null_count_pct"]
		if ok {
			outputRow[ipos] = float64(state.NullCount) / float64(state.TotalRowCount) * 100
		}

		ipos, ok = (*ctx.outputCh.Columns)["total_count"]
		if ok {
			outputRow[ipos] = state.TotalRowCount
		}

		if state.LenWelford != nil {
			avrLen, avrVar := state.LenWelford.Finalize()
			ipos, ok = (*ctx.outputCh.Columns)["avr_length"]
			if ok {
				outputRow[ipos] = avrLen
			}
			ipos, ok = (*ctx.outputCh.Columns)["length_var"]
			if ok {
				outputRow[ipos] = avrVar
			}
		}

		// The value of the domain counts are expressed in percentage of the non null count:
		//		ratio = 100 * <domain count>/(totalCount - nullCount)
		// Note that if totalCount - nullCount == 0, then ratio = -1

		// The regex tokens
		for name, m := range state.RegexMatch {
			ipos, ok = (*ctx.outputCh.Columns)[name]
			if ok {
				if ratioFactor > 0 {
					outputRow[ipos] = float64(m.Count) * ratioFactor
				} else {
					outputRow[ipos] = -1.0
				}
			}
		}

		// log.Printf("Column: %s lookup tokens:", state.ColumnName)
		// for token,count := range state.LookupState[0].LookupMatch {
		// 	log.Printf("     token: %s, count: %d", token, count.Count)
		// }

		// The lookup tokens
		for _, lookupState := range state.LookupState {
			for name, m := range lookupState.LookupMatch {
				ipos, ok := (*ctx.outputCh.Columns)[name]
				if ok {
					if ratioFactor > 0 {
						outputRow[ipos] = float64(m.Count) * ratioFactor
					} else {
						outputRow[ipos] = -1.0
					}
				}
			}
		}

		// The keywords match
		for name, m := range state.KeywordMatch {
			ipos, ok = (*ctx.outputCh.Columns)[name]
			if ok {
				if ratioFactor > 0 {
					outputRow[ipos] = float64(m.Count) * ratioFactor
				} else {
					outputRow[ipos] = -1.0
				}
			}
		}

		// The functions tokens
		var dateMinMax, doubleMinMax, textMinMax, winningValue *MinMaxValue
		if state.ParseDate != nil {
			dateMinMax = state.ParseDate.GetMinMaxValues()
			state.ParseDate.Done(ctx, outputRow)
		}
		if state.ParseDouble != nil {
			doubleMinMax = state.ParseDouble.GetMinMaxValues()
			state.ParseDouble.Done(ctx, outputRow)
		}
		if state.ParseText != nil {
			textMinMax = state.ParseText.GetMinMaxValues()
			state.ParseText.Done(ctx, outputRow)
		}

		// Pick the winning minmax results
		switch {
		case dateMinMax != nil && dateMinMax.NbrSamples > 0 && dateMinMax.HitRatio > 0.98:
			winningValue = dateMinMax
		case doubleMinMax != nil && doubleMinMax.NbrSamples > 0 && doubleMinMax.HitRatio > 0.98:
			winningValue = doubleMinMax
		default:
			winningValue = textMinMax
		}

		// Assign to output columns
		if winningValue != nil {
			ipos, ok = (*ctx.outputCh.Columns)["min_value"]
			if ok {
				outputRow[ipos] = winningValue.MinValue
			}
			ipos, ok = (*ctx.outputCh.Columns)["max_value"]
			if ok {
				outputRow[ipos] = winningValue.MaxValue
			}
			ipos, ok = (*ctx.outputCh.Columns)["minmax_type"]
			if ok {
				outputRow[ipos] = winningValue.MinMaxType
			}
		}

		// Add the carry over select and const values
		// NOTE there is no initialize and done called on the column evaluators
		//      since they should be only of type 'select' or 'value'
		for i := range ctx.columnEvaluators {
			err := ctx.columnEvaluators[i].Update(&outputRow, ctx.firstInputRow)
			if err != nil {
				err = fmt.Errorf(
					"while adding the carry over select and const values from analyze operator for column %s (at pos %d): %v",
					state.ColumnName,
					state.ColumnPos,
					err)
				log.Println(err)
				return err
			}
		}

		// Send the column result to output
		// log.Println("**!@@ ** Send AGGREGATE Result to", ctx.outputCh.name)
		select {
		case ctx.outputCh.Channel <- outputRow:
		case <-ctx.doneCh:
			log.Println("AnalyzeTransform interrupted")
		}
	}

	// log.Println("**!@@ ** Send ANALYZE Result to", ctx.outputCh.name, "DONE")
	return nil
}

func (ctx *AnalyzeTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewAnalyzeTransformationPipe(source *InputChannel, outputCh *OutputChannel,
	spec *TransformationSpec) (*AnalyzeTransformationPipe, error) {

	if spec == nil || spec.AnalyzeConfig == nil || outputCh.Columns == nil {
		return nil, fmt.Errorf("error: analyze Pipe Transformation spec is missing analyze_config section or input columns map is nil")
	}

	var err error
	config := spec.AnalyzeConfig
	// Must have NewRecord set to true
	spec.NewRecord = true

	// Get the input parquet schema, if avail
	inputDataType := make(map[string]string, len(source.Config.Columns))
	parquetSchemaInfo := ctx.inputParquetSchema
	if parquetSchemaInfo != nil {
		for _, field := range parquetSchemaInfo.Fields {
			switch field.Type {
			case "utf8":
				inputDataType[field.Name] = "string"
			case "date32":
				inputDataType[field.Name] = "date"
			default:
				inputDataType[field.Name] = field.Type
			}
		}
	} else {
		for i := range source.Config.Columns {
			inputDataType[source.Config.Columns[i]] = "string"
		}
	}

	// Make sure there is a cap on DistinctValuesWhenLessThanCount
	if config.DistinctValuesWhenLessThanCount == 0 || config.DistinctValuesWhenLessThanCount > 20 {
		config.DistinctValuesWhenLessThanCount = 20
	}

	// Check to see if the original column names are available
	columnNames := source.Config.Columns
	originalColumnNames := ctx.cpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.OriginalInputColumns
	if len(originalColumnNames) > 0 {
		columnNames = originalColumnNames
		if len(columnNames) != len(source.Config.Columns) {
			err = fmt.Errorf("error: number of original column names (%d) is different from number of input channel columns (%d)",
				len(columnNames), len(source.Config.Columns))
			log.Println(err)
			return nil, err
		}
	}

	// Set up the column name fragments to entity hint map if available
	colFragment2Hint := make(map[string]string)
	if config.EntityHints != nil {
		for _, ehint := range config.EntityHints {
			for _, frag := range ehint.NameFragments {
				colFragment2Hint[strings.ToUpper(frag)] = ehint.Entity
			}
		}
	}

	// Set up the column name to token map if available
	colName2Token := make(map[string]string)
	if config.ColumnNameToken != nil {
		for _, tokenEntry := range config.ColumnNameToken.Lookup {
			for _, colName := range tokenEntry.ColumnNames {
				colName2Token[strings.ToUpper(colName)] = tokenEntry.Name
			}
		}
	}

	// Set up the column fragment to token map if available
	colFragment2Token := make(map[string]string)
	if config.ColumnNameToken != nil {
		for _, tokenEntry := range config.ColumnNameToken.Lookup {
			for _, colFragment := range tokenEntry.ColumnNameFragments {
				colFragment2Token[strings.ToUpper(colFragment)] = tokenEntry.Name
			}
		}
	}

	// Set up the blank field markers if available
	sp := ctx.schemaManager.schemaProviders[config.SchemaProvider]
	var blankMarkers *BlankFieldMarkers
	if sp != nil {
		blankMarkers = sp.BlankFieldMarkers()
	}

	// Set up the AnalyzeState for each input column
	analyzeState := make([]*AnalyzeState, len(columnNames))
	for i := range analyzeState {
		analyzeState[i], err =
			ctx.NewAnalyzeState(columnNames[i], i, outputCh.Columns, sp, blankMarkers, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling NewAnalyzeState for column %s: %v",
				source.Config.Columns[i], err)
		}
	}

	// Prepare the column evaluators
	columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.BuildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while BuildTransformationColumnEvaluator (in NewAnalyzeTransformationPipe) %v", err)
			log.Println(err)
			return nil, err
		}
	}

	return &AnalyzeTransformationPipe{
		cpConfig:          ctx.cpConfig,
		source:            source,
		outputCh:          outputCh,
		inputDataType:     inputDataType,
		colFragment2Hint:  colFragment2Hint,
		colName2Token:     colName2Token,
		colFragment2Token: colFragment2Token,
		analyzeState:      analyzeState,
		columnEvaluators:  columnEvaluators,
		padShortRows:      config.PadShortRowsWithNulls,
		spec:              spec,
		env:               ctx.env,
		doneCh:            ctx.done,
	}, nil
}
