package compute_pipes

import (
	"fmt"
	"log"
	"regexp"
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
func (ctx *HighFreqTransformationPipe) Apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in HighFreqTransformationPipe")
	}
	if ctx.firstInputRow == nil {
		ctx.firstInputRow = input
	}
	var token string
	for _, c := range ctx.spec.HighFreqColumns {
		highFreqMap := ctx.highFreqState[c.Name]
		value := (*input)[(*ctx.source.columns)[c.Name]]
		if value != nil {
			switch vv := value.(type) {
			case string:
				token = vv
			default:
				token = fmt.Sprintf("%v", value)
			}
			token = strings.ToUpper(token)
			// check for key extraction regex
			if c.re != nil {
				key := c.re.FindStringSubmatch(token)
				if len(key) > 1 {
					token = key[1]
				} else {
					token = ""
				}
			}
			if len(token) > 0 {
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
	}

	return nil
}

// Analysis complete, now send out the results to ctx.outputCh.
// A row is produced for each column and each high freq value.
// High freq values are those in top top_pct percentile

func (ctx *HighFreqTransformationPipe) Done() error {
	// For each tracked columns, send out the top percentile values
	for _, column := range ctx.spec.HighFreqColumns {
		highFreqMap := ctx.highFreqState[column.Name]
		totalCount := 0
		nbrDistinctValues := len(highFreqMap)
		dcSlice := make([]*DistinctCount, 0, nbrDistinctValues)
		for _, dc := range highFreqMap {
			dcSlice = append(dcSlice, dc)
			totalCount += dc.Count
		}
		log.Printf("HighFreqTransformationPipe.done: sending results for column: %s, got %d distinct values out of %d values\n",
			column.Name, nbrDistinctValues, totalCount)
		sort.Slice(dcSlice, func(i, j int) bool {
			return dcSlice[i].Count > dcSlice[j].Count
		})
		var topPctFactor float64 = 1
		if column.TopPercentile > 0 {
			topPctFactor = float64(column.TopPercentile) / 100
		}
		maxTotalCount := int(float64(totalCount)*topPctFactor + 0.5)
		maxDistinctValueCount := nbrDistinctValues
		if column.TopRank > 0 {
			topRankFactor := float64(column.TopRank) / 100
			maxDistinctValueCount = nbrDistinctValues * int(float64(nbrDistinctValues)*topRankFactor+0.5)
			if maxDistinctValueCount > nbrDistinctValues {
				maxDistinctValueCount = nbrDistinctValues
			}
		}
		var valueCount int
		for i := 0; i < maxDistinctValueCount; i++ {
			// make the output row
			outputRow := make([]interface{}, len(*ctx.outputCh.columns))
			outputRow[(*ctx.outputCh.columns)["column_name"]] = column.Name
			// The freq count columns
			dc := dcSlice[i]
			outputRow[(*ctx.outputCh.columns)["freq_count"]] = dc.Count
			outputRow[(*ctx.outputCh.columns)["freq_value"]] = dc.Value
			// Add the carry over select and const values
			// NOTE there is no initialize and done called on the column evaluators
			//      since they should be only of type 'select' or 'value'
			for i := range ctx.columnEvaluators {
				err := ctx.columnEvaluators[i].Update(&outputRow, ctx.firstInputRow)
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
			valueCount += dc.Count
			if valueCount > maxTotalCount {
				break
			}
		}
	}
	fmt.Println("**!@@ ** Send Freq Count Result to", ctx.outputCh.name, "DONE")
	return nil
}

func (ctx *HighFreqTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewHighFreqTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*HighFreqTransformationPipe, error) {
	var err error
	if spec == nil || len(spec.HighFreqColumns) == 0 {
		return nil, fmt.Errorf("error: High Freq Pipe Transformation spec is missing columns definition")
	}
	if source == nil || outputCh == nil {
		return nil, fmt.Errorf("error: High Freq Pipe Transformation spec is missing source and/or outputCh channels")
	}
	// Validate the config: must have NewRecord set to true
	spec.NewRecord = true
	// Set up the High Freq State for each input column that are tracked
	analyzeState := make(map[string]map[string]*DistinctCount)
	for _, c := range spec.HighFreqColumns {
		analyzeState[c.Name] = make(map[string]*DistinctCount)
		// Compile the key extraction regex
		if len(c.KeyRe) > 0 {
			c.re, err = regexp.Compile(c.KeyRe)
			if err != nil {
				return nil, fmt.Errorf("while compiling regex %s: %v", c.KeyRe, err)
			}
		}
	}
	// Prepare the column evaluators
	columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.BuildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while BuildTransformationColumnEvaluator (in NewHighFreqTransformationPipe) %v", err)
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
