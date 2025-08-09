package compute_pipes

import (
	"bytes"
	"fmt"
	"hash"
	"hash/fnv"
	"log"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/dolthub/swiss"
)

type AnonymizeTransformationPipe struct {
	cpConfig          *ComputePipesConfig
	source            *InputChannel
	outputCh          *OutputChannel
	keysOutputCh      *OutputChannel
	hasher            hash.Hash64
	keysMap           *swiss.Map[uint64, [2]string]
	metaLookupTbl     LookupTable
	anonymActions     []*AnonymizationAction
	columnEvaluators  []TransformationColumnEvaluator
	firstInputRow     *[]any
	spec              *TransformationSpec
	inputDateLayout   string
	outputDateLayout  string
	keyMapDateLayout  string
	outputInvalidDate string
	keyInvalidDate    string
	channelRegistry   *ChannelRegistry
	env               map[string]any
	doneCh            chan struct{}
}

type AnonymizationAction struct {
	inputColumn   int
	dateLayouts   []string
	anonymizeType string
	keyPrefix     string
}

// Implementing interface PipeTransformationEvaluator
func (ctx *AnonymizeTransformationPipe) Apply(input *[]any) error {
	var err error
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in AnonymizeTransformationPipe")
	}
	if ctx.firstInputRow == nil {
		ctx.firstInputRow = input
	}
	// hashedValue4KeyFile is the value to use in the crosswalk file, it is
	// the same as hashedValue, except for dates it may use a different date formatter.
	var inputStr, hashedValue, hashedValue4KeyFile string
	inputLen := len(*input)
	expectedLen := len(ctx.source.config.Columns)
	// log.Println("*** Anonymize Input:",*input)
	// log.Println("*** Len Input:",inputLen, "Expected Len:", expectedLen)
	// NOTE: Must handle rows with less or more columns than expected. Anonymize the extra columns without a prefix
	for _, action := range ctx.anonymActions {
		if action.inputColumn >= inputLen {
			continue
		}
		value := (*input)[action.inputColumn]
		if value == nil {
			continue
		}
		switch vv := value.(type) {
		case string:
			if strings.ToUpper(vv) == "NULL" {
				continue
			}
			inputStr = vv
		default:
			inputStr = fmt.Sprintf("%v", vv)
		}
		switch action.anonymizeType {
		case "text":
			ctx.hasher.Reset()
			ctx.hasher.Write([]byte(inputStr))
			if len(action.keyPrefix) > 0 {
				hashedValue = fmt.Sprintf("%s.%016x", action.keyPrefix, ctx.hasher.Sum64())
			} else {
				hashedValue = fmt.Sprintf("%016x", ctx.hasher.Sum64())
			}
			hashedValue4KeyFile = hashedValue
		case "date":
			var date time.Time
			switch {
			case len(ctx.inputDateLayout) > 0:
				date, err = time.Parse(ctx.inputDateLayout, inputStr)
				if err != nil {
					// try jetstore date parser
					var d *time.Time
					d, err = ParseDate(inputStr)
					if d != nil {
						date = *d
					}
				}
			case len(action.dateLayouts) > 0:
				for _, layout := range action.dateLayouts {
					date, err = time.Parse(layout, inputStr)
					if err != nil {
						break
					}
				}

			default:
				var d *time.Time
				d, err = ParseDate(inputStr)
				if d != nil {
					date = *d
				}
			}
			if err == nil {
				// hashedValue = fmt.Sprintf("%d/%02d/01", date.Year(), date.Month())
				anonymizeDate := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
				hashedValue = strings.ToUpper(anonymizeDate.Format(ctx.outputDateLayout))
				if len(ctx.keyMapDateLayout) == 0 {
					hashedValue4KeyFile = hashedValue
				} else {
					hashedValue4KeyFile = strings.ToUpper(anonymizeDate.Format(ctx.keyMapDateLayout))
				}
			} else {
				if len(ctx.outputInvalidDate) > 0 {
					hashedValue = ctx.outputInvalidDate
					hashedValue4KeyFile = ctx.keyInvalidDate
				} else {
					hashedValue = inputStr
					hashedValue4KeyFile = inputStr
				}
			}
		}
		(*input)[action.inputColumn] = hashedValue
		ctx.hasher.Reset()
		ctx.hasher.Write([]byte(inputStr))
		ctx.hasher.Write([]byte(hashedValue4KeyFile))
		ctx.keysMap.Put(ctx.hasher.Sum64(), [2]string{inputStr, hashedValue4KeyFile})
	}
	for icol := expectedLen; icol < inputLen; icol++ {
		value := (*input)[icol]
		if value == nil {
			continue
		}
		switch vv := value.(type) {
		case string:
			if strings.ToUpper(vv) == "NULL" {
				continue
			}
			inputStr = vv
		default:
			inputStr = fmt.Sprintf("%v", vv)
		}
		ctx.hasher.Reset()
		ctx.hasher.Write([]byte(inputStr))
		hashedValue = fmt.Sprintf("%016x", ctx.hasher.Sum64())
		hashedValue4KeyFile = hashedValue
		(*input)[icol] = hashedValue
		ctx.hasher.Reset()
		ctx.hasher.Write([]byte(inputStr))
		ctx.hasher.Write([]byte(hashedValue4KeyFile))
		ctx.keysMap.Put(ctx.hasher.Sum64(), [2]string{inputStr, hashedValue4KeyFile})
	}
	// Send the result to output
	// log.Println("*** Anonymize Output:",*input)
	select {
	case ctx.outputCh.channel <- *input:
	case <-ctx.doneCh:
		log.Printf("AnonymizeTransformationPipe writing to '%s' interrupted", ctx.outputCh.name)
		return nil
	}
	return nil
}

// Anonymization complete, now send out the keys mapping to keys_output_channel
func (ctx *AnonymizeTransformationPipe) Done() error {
	var err error
	ctx.keysMap.Iter(func(k uint64, v [2]string) (stop bool) {
		outputRow := make([]any, len(*ctx.keysOutputCh.columns))
		outputRow[(*ctx.keysOutputCh.columns)["hashed_key"]] = k
		outputRow[(*ctx.keysOutputCh.columns)["original_value"]] = v[0]
		outputRow[(*ctx.keysOutputCh.columns)["anonymized_value"]] = v[1]

		// Add the carry over select and const values
		// NOTE there is no initialize and done called on the column evaluators
		//      since they should be only of type 'select' or 'value'
		for i := range ctx.columnEvaluators {
			err2 := ctx.columnEvaluators[i].Update(&outputRow, ctx.firstInputRow)
			if err2 != nil {
				err2 = fmt.Errorf("while calling column transformation from anonymize operator: %v", err)
				log.Println(err2)
				err = err2
				return true // stop
			}
		}

		// Send the keys mapping to output
		// log.Println("**!@@ ** Send AGGREGATE Result to", ctx.keysOutputCh.name)
		select {
		case ctx.keysOutputCh.channel <- outputRow:
		case <-ctx.doneCh:
			log.Println("AnonymizeTransform interrupted")
			return true // stop
		}
		return false // continue
	})
	return err
}

func (ctx *AnonymizeTransformationPipe) Finally() {
	// Done sending the keys, closing the keys output channel
	ctx.channelRegistry.CloseChannel(ctx.keysOutputCh.name)
}

func (ctx *BuilderContext) NewAnonymizeTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*AnonymizeTransformationPipe, error) {
	if spec == nil || spec.AnonymizeConfig == nil {
		return nil, fmt.Errorf("error: Anonymize Pipe Transformation spec is missing or anonymize_config is missing")
	}
	config := spec.AnonymizeConfig
	if len(config.AnonymizeType) == 0 || len(config.KeyPrefix) == 0 {
		return nil, fmt.Errorf("error: Anonymize Pipe Transformation spec is missing anonymize_type or key_prefix")
	}
	var keysOutCh *OutputChannel
	var metaLookupTbl LookupTable
	var anonymActions []*AnonymizationAction
	var hasher hash.Hash64
	var columnEvaluators []TransformationColumnEvaluator
	var anonymizeType string
	var err error
	var ok bool
	// Get the channel for sending the keys
	keysOutCh, err = ctx.channelRegistry.GetOutputChannel(config.KeysOutputChannel.Name)
	if err != nil {
		return nil, fmt.Errorf("while getting the keys output channel %s: %v", config.KeysOutputChannel.Name, err)
	}
	closeIfNotNil := keysOutCh.channel
	defer func() {
		if closeIfNotNil != nil {
			close(closeIfNotNil)
		}
	}()
	// Prepare the actions to anonymize marked columns
	sp := ctx.schemaManager.GetSchemaProvider(config.SchemaProvider)
	omitPrefix := false
	var newWidth map[string]int
	if sp != nil && sp.Format() == "fixed_width" {
		if config.OmitPrefixOnFW {
			omitPrefix = true
		}
		if config.AdjustFieldWidthOnFW {
			newWidth = make(map[string]int)
		}
	}
	metaLookupTbl = ctx.lookupTableManager.LookupTableMap[config.LookupName]
	if metaLookupTbl == nil {
		return nil, fmt.Errorf("error: anonymize metadata lookup table %s not found", config.LookupName)
	}
	anonymActions = make([]*AnonymizationAction, 0, len(*source.columns))
	metaLookupColumnsMap := metaLookupTbl.ColumnMap()
	for name, ipos := range *source.columns {
		metaRow, err := metaLookupTbl.Lookup(&name)
		if err != nil {
			return nil, fmt.Errorf("while getting the metadata row for column %s: %v", name, err)
		}
		if metaRow == nil {
			return nil, fmt.Errorf("error: metadata row not found for column %s", name)
		}
		anonymizeTypeI := (*metaRow)[metaLookupColumnsMap[config.AnonymizeType]]
		if anonymizeTypeI == nil {
			anonymizeType = ""
		} else {
			anonymizeType, ok = anonymizeTypeI.(string)
			if !ok {
				return nil, fmt.Errorf("error: expecting string for anonymize type (e.g. text, date), got %v", anonymizeTypeI)
			}
		}
		var keyPrefix string
		var dateLayouts []string
		switch anonymizeType {
		case "text", "date":
			keyPrefix = ""
			w := 16
			if anonymizeType == "text" && !omitPrefix {
				keyPrefixI := (*metaRow)[metaLookupColumnsMap[config.KeyPrefix]]
				keyPrefix, ok = keyPrefixI.(string)
				if !ok {
					return nil, fmt.Errorf("error: expecting string for key prefix (e.g. ssn, dob, etc), got %v", keyPrefixI)
				}
				w = 28
			}
			if newWidth != nil {
				newWidth[name] = w
			}
			// Get the date layouts if any
			if len(config.DateFormatsColumn) > 0 {
				dlcI := (*metaRow)[metaLookupColumnsMap[config.DateFormatsColumn]]
				dateLayoutsCsv, ok := dlcI.(string)
				if ok {
					r := csv.NewReader(bytes.NewReader([]byte(dateLayoutsCsv)))
					dateLayouts, err = r.Read()
					if err != nil {
						return nil, fmt.Errorf("while decoding date formats from csv:%v", err)
					}
				}
			}

			anonymActions = append(anonymActions, &AnonymizationAction{
				inputColumn:   ipos,
				dateLayouts:   dateLayouts,
				anonymizeType: anonymizeType,
				keyPrefix:     keyPrefix,
			})
		case "":
			// Not anonymized
		default:
			return nil, fmt.Errorf("error: unknown anonymize type '%s', known values: test, date", anonymizeType)
		}
	}
	if newWidth != nil {
		err = sp.AdjustColumnWidth(newWidth)
		if err != nil {
			return nil, fmt.Errorf("while adjusting column width of fixed-width file: %v", err)
		}
	}
	hasher = fnv.New64a()
	// Determine the date format to use, start with default value
	outputDateLayout := "2006/01/02"
	var inputDateLayout, keyDateLayout string
	// Check if the schema provider is specified
	if sp != nil {
		if sp.ReadDateLayout() != "" {
			inputDateLayout = sp.ReadDateLayout()
		}
		if sp.WriteDateLayout() != "" {
			outputDateLayout = sp.WriteDateLayout()
		}
	}
	if config.InputDateLayout != "" {
		inputDateLayout = config.InputDateLayout
		if config.OutputDateLayout == "" {
			outputDateLayout = inputDateLayout
		}
	}
	if config.OutputDateLayout != "" {
		outputDateLayout = config.OutputDateLayout
	}
	if config.KeyDateLayout != "" {
		keyDateLayout = config.KeyDateLayout
	}
	// Note: keyDateLayout defaults to outputDateLayout, keyDateLayout is left empty to re-use the output date value.
	// Format the default invalid date to the key date format
	var outputInvalidDate, keyInvalidDate string
	if len(config.DefaultInvalidDate) > 0 {
		d, err := ParseDate(config.DefaultInvalidDate)
		if err != nil {
			err = fmt.Errorf(
				"configuration error: anonymize_config.default_invalid_date '%s' is not a valid date (use YYYY/MM/DD format)",
				config.DefaultInvalidDate)
			log.Println(err)
			return nil, err
		}
		outputInvalidDate = d.Format(outputDateLayout)
		if len(keyDateLayout) == 0 {
			keyInvalidDate = outputInvalidDate
		} else {
			keyInvalidDate = d.Format(keyDateLayout)
		}
	}

	// Prepare the column evaluators
	columnEvaluators = make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.BuildTransformationColumnEvaluator(source, keysOutCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while BuildTransformationColumnEvaluator (in NewAnonymizeTransformationPipe) %v", err)
			log.Println(err)
			return nil, err
		}
	}
	// All good, no errors
	closeIfNotNil = nil
	return &AnonymizeTransformationPipe{
		cpConfig:          ctx.cpConfig,
		source:            source,
		outputCh:          outputCh,
		keysOutputCh:      keysOutCh,
		hasher:            hasher,
		keysMap:           swiss.NewMap[uint64, [2]string](2048),
		metaLookupTbl:     metaLookupTbl,
		anonymActions:     anonymActions,
		columnEvaluators:  columnEvaluators,
		channelRegistry:   ctx.channelRegistry,
		spec:              spec,
		inputDateLayout:   inputDateLayout,
		outputDateLayout:  outputDateLayout,
		keyMapDateLayout:  keyDateLayout,
		outputInvalidDate: outputInvalidDate,
		keyInvalidDate:    keyInvalidDate,
		env:               ctx.env,
		doneCh:            ctx.done,
	}, nil
}
