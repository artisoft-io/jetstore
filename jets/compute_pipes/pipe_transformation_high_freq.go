package compute_pipes

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

type HighFreqTransformationPipe struct {
	cpConfig      *ComputePipesConfig
	source        *InputChannel
	outputCh      *OutputChannel
	highFreqState map[string]map[string]*DistinctCount
	layoutName    string
	spec          *TransformationSpec
	env           map[string]interface{}
	sessionId     string
	doneCh        chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *HighFreqTransformationPipe) apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in HighFreqTransformationPipe")
	}
	if len(ctx.layoutName) == 0 {
		pos, ok := ctx.source.columns["layout_name"]
		if ok {
			name, ok := (*input)[pos].(string)
			if ok {
				ctx.layoutName = name
			}
		}
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
			// The context driven columns
			outputRow[ctx.outputCh.columns["processing_ticket"]] = ctx.env["$PROCESSING_TICKET"]
			outputRow[ctx.outputCh.columns["layout_name"]] = ctx.layoutName
			outputRow[ctx.outputCh.columns["session_id"]] = ctx.sessionId
			// The freq count columns
			dc := dcSlice[i]
			outputRow[ctx.outputCh.columns["freq_count"]] = dc.Count
			outputRow[ctx.outputCh.columns["freq_value"]] = dc.Value
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
	return &HighFreqTransformationPipe{
		cpConfig:      ctx.cpConfig,
		source:        source,
		outputCh:      outputCh,
		highFreqState: analyzeState,
		spec:          spec,
		env:           ctx.env,
		sessionId:     ctx.sessionId,
		doneCh:        ctx.done,
	}, nil
}
