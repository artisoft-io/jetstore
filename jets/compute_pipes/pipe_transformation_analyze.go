package compute_pipes

import (
	"fmt"
	"log"
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
//	"distinct_count",
//	"distinct_count_pct",
//	"null_count",
//	"null_count_pct",
//	"total_count",
//	"avr_length",
//	"length_var",
//
// Other columns are added based on regex_tokens, lookup_tokens, and keyword_tokens
// The value of the domain counts are expressed in percentage of the non null count:
//
//	ratio = <domain count>/(totalCount - nullCount) * 100.0
//
// Note that if totalCount - nullCount == 0, then ratio = -1
type AnalyzeTransformationPipe struct {
	cpConfig         *ComputePipesConfig
	source           *InputChannel
	outputCh         *OutputChannel
	analyzeState     []*AnalyzeState
	columnEvaluators []TransformationColumnEvaluator
	firstInputRow    *[]interface{}
	spec             *TransformationSpec
	env              map[string]interface{}
	doneCh           chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *AnalyzeTransformationPipe) Apply(input *[]interface{}) error {
	var err error
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in AnalyzeTransformationPipe")
	}
	if ctx.firstInputRow == nil {
		ctx.firstInputRow = input
	}
	for i := range *input {
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
	// For each column state in ctx.analyzeState, send out a row to ctx.outputCh
	var ok bool
	for _, state := range ctx.analyzeState {
		outputRow := make([]interface{}, len(ctx.outputCh.columns))

		// The first base columns
		var ipos int
		ipos, ok = ctx.outputCh.columns["column_name"]
		if ok {
			outputRow[ipos] = state.ColumnName
		}
		ipos, ok = ctx.outputCh.columns["column_pos"]
		if ok {
			outputRow[ipos] = state.ColumnPos
		}
		distinctCount := len(state.DistinctValues)
		var ratioFactor float64
		if state.TotalRowCount != state.NullCount {
			ratioFactor = 100.0 / float64(state.TotalRowCount-state.NullCount)
		}
		ipos, ok = ctx.outputCh.columns["distinct_count"]
		if ok {
			outputRow[ipos] = distinctCount
		}
		ipos, ok = ctx.outputCh.columns["distinct_count_pct"]
		if ok {
			if ratioFactor > 0 {
				outputRow[ipos] = float64(distinctCount) * ratioFactor
			} else {
				outputRow[ipos] = -1.0
			}
		}
		ipos, ok = ctx.outputCh.columns["null_count"]
		if ok {
			outputRow[ipos] = state.NullCount
		}
		ipos, ok = ctx.outputCh.columns["null_count_pct"]
		if ok {
			outputRow[ipos] = float64(state.NullCount) / float64(state.TotalRowCount) * 100
		}
		ipos, ok = ctx.outputCh.columns["total_count"]
		if ok {
			outputRow[ipos] = state.TotalRowCount
		}

		avrLen, avrVar := state.Welford.Finalize()
		ipos, ok = ctx.outputCh.columns["avr_length"]
		if ok {
			outputRow[ipos] = avrLen
		}
		ipos, ok = ctx.outputCh.columns["length_var"]
		if ok {
			outputRow[ipos] = avrVar
		}

		// The value of the domain counts are expressed in percentage of the non null count:
		//		ratio = 100 * <domain count>/(totalCount - nullCount)
		// Note that if totalCount - nullCount == 0, then ratio = -1

		// The regex tokens
		for name, m := range state.RegexMatch {
			ipos, ok = ctx.outputCh.columns[name]
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
				ipos, ok := ctx.outputCh.columns[name]
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
			ipos, ok = ctx.outputCh.columns[name]
			if ok {
				if ratioFactor > 0 {
					outputRow[ipos] = float64(m.Count) * ratioFactor
				} else {
					outputRow[ipos] = -1.0
				}
			}
		}

		// The functions tokens
		for name, m := range state.FunctionMatch {
			ipos, ok = ctx.outputCh.columns[name]
			if ok {
				if ratioFactor > 0 {
					outputRow[ipos] = float64(m.Count) * ratioFactor
				} else {
					outputRow[ipos] = -1.0
				}
			}
		}

		// Add the carry over select and const values
		// NOTE there is no initialize and done called on the column evaluators
		//      since they should be only of type 'select' or 'value'
		for i := range ctx.columnEvaluators {
			err := ctx.columnEvaluators[i].Update(&outputRow, ctx.firstInputRow)
			if err != nil {
				err = fmt.Errorf("while calling column transformation from analyze operator: %v", err)
				log.Println(err)
				return err
			}
		}

		// Send the column result to output
		// log.Println("**!@@ ** Send AGGREGATE Result to", ctx.outputCh.config.Name)
		select {
		case ctx.outputCh.channel <- outputRow:
		case <-ctx.doneCh:
			log.Println("AnalyzeTransform interrupted")
		}
	}

	// log.Println("**!@@ ** Send ANALYZE Result to", ctx.outputCh.config.Name, "DONE")
	return nil
}

func (ctx *AnalyzeTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewAnalyzeTransformationPipe(source *InputChannel, outputCh *OutputChannel,
	spec *TransformationSpec) (*AnalyzeTransformationPipe, error) {

	var err error
	if spec == nil {
		return nil, fmt.Errorf(
			"error: Analyze Pipe Transformation spec (analyze_config) is missing regex, lookup, and/or keywords definition")
	}
	config := spec.AnalyzeConfig
	if config == nil || config.RegexTokens == nil ||
		config.LookupTokens == nil || config.KeywordTokens == nil {
		return nil, fmt.Errorf(
			"error: Analyze Pipe Transformation spec (analyze_config) is missing regex, lookup, and/or keywords definition")
	}
	// Validate the config: must have NewRecord set to true
	spec.NewRecord = true

	// Set up the AnalyzeState for each input column
	analyzeState := make([]*AnalyzeState, len(source.config.Columns))
	for i := range analyzeState {
		analyzeState[i], err =
			ctx.NewAnalyzeState(source.config.Columns[i], i, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling NewAnalyzeState for column %s: %v",
				source.config.Columns[i], err)
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
		cpConfig:         ctx.cpConfig,
		source:           source,
		outputCh:         outputCh,
		analyzeState:     analyzeState,
		columnEvaluators: columnEvaluators,
		spec:             spec,
		env:              ctx.env,
		doneCh:           ctx.done,
	}, nil
}

// Welford's online algorithm
// https://en.wikipedia.org/wiki/Algorithms_for_calculating_variance#Online
// An example Python implementation for Welford's algorithm is given below.
//
// # For a new value new_value, compute the new count, new mean, the new M2.
// # mean accumulates the mean of the entire dataset
// # M2 aggregates the squared distance from the mean
// # count aggregates the number of samples seen so far
// def update(existing_aggregate, new_value):
//     (count, mean, M2) = existing_aggregate
//     count += 1
//     delta = new_value - mean
//     mean += delta / count
//     delta2 = new_value - mean
//     M2 += delta * delta2
//     return (count, mean, M2)

// # Retrieve the mean, variance and sample variance from an aggregate
// def finalize(existing_aggregate):
//     (count, mean, M2) = existing_aggregate
//     if count < 2:
//         return float("nan")
//     else:
//         (mean, variance, sample_variance) = (mean, M2 / count, M2 / (count - 1))
//         return (mean, variance, sample_variance)

type WelfordAlgo struct {
	Mean  float64
	M2    float64
	Count int
}

func NewWelfordAlgo() *WelfordAlgo {
	return &WelfordAlgo{}
}

func (w *WelfordAlgo) Update(value float64) {
	w.Count += 1
	delta := value - w.Mean
	w.Mean += delta / float64(w.Count)
	delta2 := value - w.Mean
	w.M2 += delta * delta2
}

func (w *WelfordAlgo) Finalize() (mean, variance float64) {
	return w.Mean, w.M2 / float64(w.Count)
}
