package compute_pipes

import (
	"fmt"
	"log"
	"math/rand"
)

// Note: No columnEvaluators is used by this operator.
type ShufflingTransformationPipe struct {
	cpConfig      *ComputePipesConfig
	source        *InputChannel
	outputCh      *OutputChannel
	metaLookupTbl LookupTable
	sourceData    [][]any
	maxInputCount int
	padShortRows  bool
	spec          *TransformationSpec
	env           map[string]any
	doneCh        chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *ShufflingTransformationPipe) Apply(input *[]any) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in ShufflingTransformationPipe")
	}
	inputLen := len(*input)
	expectedLen := len(ctx.source.config.Columns)
	if inputLen < expectedLen {
		if ctx.padShortRows {
			for range expectedLen - inputLen {
				*input = append(*input, nil)
			}
		} else {
			// Skip the row
			return nil
		}
	}

	if len(ctx.sourceData) < ctx.maxInputCount {
		ctx.sourceData = append(ctx.sourceData, *input)
	}
	return nil
}

// Analysis complete, now send out the results to ctx.outputCh.
func (ctx *ShufflingTransformationPipe) Done() error {
	nbrRecIn := len(ctx.sourceData)
	for range ctx.spec.ShufflingConfig.OutputSampleSize {
		outputRow := make([]any, len(*ctx.outputCh.columns))
		// For each column take a random value from the sourceData set
		for name, jcol := range *ctx.outputCh.columns {
			outputRow[jcol] = ctx.sourceData[rand.Intn(nbrRecIn)][(*ctx.source.columns)[name]]
		}
		// Send the result to output
		// log.Println("**!@@ ** Send SHUFFLING Result to", ctx.outputCh.name)
		select {
		case ctx.outputCh.channel <- outputRow:
		case <-ctx.doneCh:
			log.Println("ShufflingTransform interrupted")
		}
	}
	return nil
}

func (ctx *ShufflingTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewShufflingTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*ShufflingTransformationPipe, error) {
	if spec == nil || spec.ShufflingConfig == nil {
		return nil, fmt.Errorf("error: Shuffling Pipe Transformation spec is missing regex, lookup, and/or keywords definition")
	}
	config := spec.ShufflingConfig
	// Validate the config: must have NewRecord set to true
	spec.NewRecord = true
	nsize := 1000

	if config.MaxInputSampleSize > 0 {
		nsize = config.MaxInputSampleSize
	}

	// Check if the output rows have columns filtered out
	var metaLookupTbl LookupTable
	if config.FilterColumns != nil {
		// Determine the columns to retain based on the lookup table
		// Get the metadata lookup table
		metaLookupTbl = ctx.lookupTableManager.LookupTableMap[config.FilterColumns.LookupName]
		if metaLookupTbl == nil {
			return nil, fmt.Errorf("error: shuffling operator metadata lookup table %s not found", config.FilterColumns.LookupName)
		}
		metaLookupColumnsMap := metaLookupTbl.ColumnMap()
		lookupColumn, ok := metaLookupColumnsMap[config.FilterColumns.LookupColumn]
		if !ok {
			return nil, fmt.Errorf("error: shuffling operator metadata lookup table does not have column %s", config.FilterColumns.LookupColumn)
		}
		lookupColumnName, ok := metaLookupColumnsMap[config.FilterColumns.ColumnName]
		if !ok {
			return nil, fmt.Errorf("error: shuffling operator metadata lookup table does not have column %s", config.FilterColumns.ColumnName)
		}

		// Make a lookup of the value indicating to retain the column
		retainOnValues := make(map[any]bool)
		for _, v := range config.FilterColumns.RetainOnValues {
			retainOnValues[v] = true
		}

		// Prepare to replace the output column info
		outputCh.config.Columns = make([]string, 0)
		// remove the placeholder columns
		for k := range *outputCh.columns {
			delete(*outputCh.columns, k)
		}

		for _, ipos := range *source.columns {
			columnPosStr := fmt.Sprintf("%d", ipos)
			// Lookup the metadata row by column position
			metaRow, err := metaLookupTbl.Lookup(&columnPosStr)
			if err != nil {
				return nil, fmt.Errorf("while getting the metadata row for column position %d: %v", ipos, err)
			}
			if metaRow == nil {
				return nil, fmt.Errorf("error: metadata row not found for column position %d", ipos)
			}
			value := (*metaRow)[lookupColumn]
			if retainOnValues[value] {
				// Retain this column, get the column name from metaRow
				name := (*metaRow)[lookupColumnName].(string)
				(*outputCh.columns)[name] = len(outputCh.config.Columns)
				outputCh.config.Columns = append(outputCh.config.Columns, name)
			}
		}
		if len(outputCh.config.Columns) == 0 {
			// There is no column retained, put a placeholder so it's not empty
			(*outputCh.columns)["placeholder"] = 0
			outputCh.config.Columns = append(outputCh.config.Columns, "placeholder")
		}
		// log.Println("*** Updated SHUFFLING OUTPUT Columns:", outputCh.config.Columns)
	}

	return &ShufflingTransformationPipe{
		cpConfig:      ctx.cpConfig,
		source:        source,
		outputCh:      outputCh,
		metaLookupTbl: metaLookupTbl,
		sourceData:    make([][]any, 0, nsize),
		maxInputCount: config.MaxInputSampleSize,
		padShortRows:  config.PadShortRowsWithNulls,
		spec:          spec,
		env:           ctx.env,
		doneCh:        ctx.done,
	}, nil
}
