package compute_pipes

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

// firstInputRow is the first row from the input channel.
// A reference to it is kept for use in the Done function
// so to carry over the select fields in the columnEvaluators.
// Note: columnEvaluators is applied only on the firstInputRow
// and it is used only to select column having same value for every input row
// or to put constant values comming from the env
type HighFreqTransformationPipe struct {
	cpConfig         *ComputePipesConfig
	source           *InputChannel
	outputCh         *OutputChannel
	highFreqState    map[string]map[string]*DistinctCount
	columnEvaluators []TransformationColumnEvaluator
	firstInputRow    *[]interface{}
	spec             *TransformationSpec
	env              map[string]interface{}
	sessionId        string
	doneCh           chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *HighFreqTransformationPipe) apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in HighFreqTransformationPipe")
	}
	if ctx.firstInputRow == nil {
		ctx.firstInputRow = input
	}
	var token string
	for _, c := range *ctx.spec.HighFreqColumns {
		highFreqMap := ctx.highFreqState[c]
		value := (*input)[ctx.source.columns[c]]
		if value != nil {
			switch vv := value.(type) {
			case string:
				token = vv
			default:
				token = fmt.Sprintf("%v", value)
			}
			token = strings.ToUpper(token)
			dv := highFreqMap[token]
			if dv == nil {
				dv = &DistinctCount{
					Value: token,
				}
				highFreqMap[token] = dv
			}
			dv.Count += 1
		}
	}

	return nil
}

// Analysis complete, now send out the results to ctx.outputCh.
// A row is produced for each column and each high freq value.
// High freq values are those in top 80 percentile, cap at 500 values

func (ctx *HighFreqTransformationPipe) done() error {
	// For each tracked columns, send out the top 80 percentile values
	for _, columnName := range *ctx.spec.HighFreqColumns {
		highFreqMap := ctx.highFreqState[columnName]
		totalCount := 0
		// log.Printf("HighFreqTransformationPipe.done: sending results for column: %s, got %d distinct values", columnName, len(highFreqMap))
		dcSlice := make([]*DistinctCount, 0, len(highFreqMap))
		for _, dc := range highFreqMap {
			dcSlice = append(dcSlice, dc)
			totalCount += dc.Count
		}
		sort.Slice(dcSlice, func(i, j int) bool {
			return dcSlice[i].Count > dcSlice[j].Count
		})
		top80pct := int(float64(totalCount)*0.8 + 0.5)
		var pctCount int
		maxCount := 500
		l := len(dcSlice)
		if l < maxCount {
			maxCount = l
		}
		for i := 0; i < maxCount; i++ {
			// make the output row
			outputRow := make([]interface{}, len(ctx.outputCh.columns))
			outputRow[ctx.outputCh.columns["column_name"]] = columnName
			// The freq count columns
			dc := dcSlice[i]
			outputRow[ctx.outputCh.columns["freq_count"]] = dc.Count
			outputRow[ctx.outputCh.columns["freq_value"]] = dc.Value
			// Add the carry over select and const values
			// NOTE there is no initialize and done called on the column evaluators
			//      since they should be only of type 'select' or 'value'
			for i := range ctx.columnEvaluators {
				err := ctx.columnEvaluators[i].update(&outputRow, ctx.firstInputRow)
				if err != nil {
					err = fmt.Errorf("while calling column transformation from high_freq operator: %v", err)
					log.Println(err)
					return err
				}
			}
			// Send out the row
			select {
			case ctx.outputCh.channel <- outputRow:
			case <-ctx.doneCh:
				log.Println("HighFreqTransform interrupted")
			}
			// see if we have enough value
			pctCount += dc.Count
			if pctCount > top80pct {
				break
			}
		}
	}
	// fmt.Println("**!@@ ** Send Freq Count Result to", ctx.outputCh.config.Name, "DONE")
	return nil
}

func (ctx *HighFreqTransformationPipe) finally() {}

func (ctx *BuilderContext) NewHighFreqTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*HighFreqTransformationPipe, error) {
	if spec == nil || spec.HighFreqColumns == nil {
		return nil, fmt.Errorf("error: High Freq Pipe Transformation spec is missing columns definition")
	}
	if source == nil || outputCh == nil {
		return nil, fmt.Errorf("error: High Freq Pipe Transformation spec is missing source and/or outputCh channels")
	}
	// Validate the config: must have NewRecord set to true
	spec.NewRecord = true
	// Set up the High Freq State for each input column that are tracked
	analyzeState := make(map[string]map[string]*DistinctCount)
	for _, c := range *spec.HighFreqColumns {
		analyzeState[c] = make(map[string]*DistinctCount)
	}
	// Prepare the column evaluators
	var err error
	columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.buildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while buildTransformationColumnEvaluator (in NewHighFreqTransformationPipe) %v", err)
			log.Println(err)
			return nil, err
		}
	}

	return &HighFreqTransformationPipe{
		cpConfig:         ctx.cpConfig,
		source:           source,
		outputCh:         outputCh,
		highFreqState:    analyzeState,
		columnEvaluators: columnEvaluators,
		spec:             spec,
		env:              ctx.env,
		sessionId:        ctx.sessionId,
		doneCh:           ctx.done,
	}, nil
}
