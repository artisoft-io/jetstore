package compute_pipes

import (
	"bytes"
	"fmt"
	"hash"
	"hash/fnv"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/date_utils"
	"github.com/dolthub/swiss"
)

type AnonymizeTransformationPipe struct {
	mode              string
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
	invalidDate       *time.Time
	outputInvalidDate string
	keyInvalidDate    string
	channelRegistry   *ChannelRegistry
	env               map[string]any
	doneCh            chan struct{}
}

type AnonymizationAction struct {
	inputColumn      int
	dateLayouts      []string
	anonymizeType    string
	keyPrefix        string
	deidFunctionName string
	deidLookupTbl    LookupTable
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
	var ok bool
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
		outputDateLayout := ctx.outputDateLayout
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
			switch ctx.mode {
			case "de-identification":
				// See if there is a de-identification lookup table
				if action.deidLookupTbl != nil {
					// Lookup the anonymized value from the de-identification lookup table
					nrows := uint64(action.deidLookupTbl.Size())
					if nrows == 0 {
						return fmt.Errorf("error: de-identification lookup table for key prefix '%s' is empty",
							action.keyPrefix)
					}
					rowKey := fmt.Sprintf("%d", ctx.hasher.Sum64()%nrows)
					lookupRow, err := action.deidLookupTbl.Lookup(&rowKey)
					if err != nil {
						return fmt.Errorf("while looking up de-identification value for key '%s': %v", rowKey, err)
					}
					if lookupRow == nil {
						return fmt.Errorf("error: de-identification lookup value not found for key '%s'", rowKey)
					}
					// Get the anonymized value from the lookup row
					// Assuming the anonymized value is in the first column
					hashedValue, ok = (*lookupRow)[0].(string)
					if !ok {
						return fmt.Errorf("error: expecting string for de-identification anonymized value, got %v", (*lookupRow)[0])
					}
				} else {
					// Use the de-identification function
					switch action.deidFunctionName {
					case "hashed_value":
						hashedValue = fmt.Sprintf("%016x", ctx.hasher.Sum64())
					default:
						return fmt.Errorf("error: unknown de-identification function '%s' for key prefix '%s'",
							action.deidFunctionName, action.keyPrefix)
					}
				}
			case "anonymization":
				// Generate the anonymized value with prefix
				if len(action.keyPrefix) > 0 {
					hashedValue = fmt.Sprintf("%s.%016x", action.keyPrefix, ctx.hasher.Sum64())
				} else {
					hashedValue = fmt.Sprintf("%016x", ctx.hasher.Sum64())
				}
				hashedValue4KeyFile = hashedValue
			}
		case "date":
			var date time.Time
			switch {
			case len(action.dateLayouts) > 0:
				// Use the identified date format - also use the same date format for anonymized output date
				for _, layout := range action.dateLayouts {
					date, err = date_utils.ParseDateTime(layout, inputStr)
					if err == nil {
						// Use the same output layout as the input date
						outputDateLayout = layout
						break
					}
				}
			case len(ctx.inputDateLayout) > 0:
				date, err = date_utils.ParseDateTime(ctx.inputDateLayout, inputStr)
				if err != nil {
					// try jetstore date parser
					var d *time.Time
					d, err = ParseDate(inputStr)
					if d != nil {
						date = *d
					}
				}
			default:
				// Use JetStore date parser
				var d *time.Time
				d, err = ParseDate(inputStr)
				if d != nil {
					date = *d
				}
			}
			if err == nil {
				// hashedValue = fmt.Sprintf("%d/%02d/01", date.Year(), date.Month())
				anonymizeDate := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
				hashedValue = strings.ToUpper(anonymizeDate.Format(outputDateLayout))
				if len(ctx.keyMapDateLayout) == 0 {
					hashedValue4KeyFile = hashedValue
				} else {
					hashedValue4KeyFile = strings.ToUpper(anonymizeDate.Format(ctx.keyMapDateLayout))
				}
			} else {
				switch {
				case len(action.dateLayouts) > 0 && ctx.invalidDate != nil:
					hashedValue = strings.ToUpper((*ctx.invalidDate).Format(action.dateLayouts[0]))
					hashedValue4KeyFile = ctx.keyInvalidDate
				case len(ctx.outputInvalidDate) > 0:
					hashedValue = ctx.outputInvalidDate
					hashedValue4KeyFile = ctx.keyInvalidDate
				default:
					hashedValue = inputStr
					hashedValue4KeyFile = inputStr
				}
				// fmt.Println("*** Error while parsing:", err, "will use blinded date:", hashedValue)
			}
		}
		(*input)[action.inputColumn] = hashedValue
		if ctx.mode == "anonymization" {
			ctx.hasher.Reset()
			ctx.hasher.Write([]byte(inputStr))
			ctx.hasher.Write([]byte(hashedValue4KeyFile))
			ctx.keysMap.Put(ctx.hasher.Sum64(), [2]string{inputStr, hashedValue4KeyFile})
		}
	}
	// Anonymize all the extra columns beyond expectedLen
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
		if ctx.mode == "anonymization" {
			ctx.hasher.Reset()
			ctx.hasher.Write([]byte(inputStr))
			ctx.hasher.Write([]byte(hashedValue4KeyFile))
			ctx.keysMap.Put(ctx.hasher.Sum64(), [2]string{inputStr, hashedValue4KeyFile})
		}
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
// if in mode "anonymization"
func (ctx *AnonymizeTransformationPipe) Done() error {
	if ctx.mode != "anonymization" {
		return nil
	}
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
	if ctx.mode == "anonymization" {
		// Done sending the keys, closing the keys output channel
		ctx.channelRegistry.CloseChannel(ctx.keysOutputCh.name)
	}
}

func (ctx *BuilderContext) NewAnonymizeTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*AnonymizeTransformationPipe, error) {
	if spec == nil || spec.AnonymizeConfig == nil {
		return nil, fmt.Errorf("error: Anonymize Pipe Transformation spec is missing or anonymize_config is missing")
	}
	config := spec.AnonymizeConfig
	if len(config.AnonymizeType) == 0 || len(config.KeyPrefix) == 0 {
		return nil, fmt.Errorf("error: Anonymize Pipe Transformation spec is missing anonymize_type or key_prefix")
	}
	if len(config.Mode) == 0 {
		config.Mode = "anonymization"
	}
	var keysOutCh *OutputChannel
	var metaLookupTbl LookupTable
	var anonymActions []*AnonymizationAction
	var hasher hash.Hash64
	var columnEvaluators []TransformationColumnEvaluator
	var anonymizeType string
	var err error
	var ok bool
	var closeIfNotNil chan<- []any
	sp := ctx.schemaManager.GetSchemaProvider(config.SchemaProvider)
	omitPrefix := false
	var newWidth map[string]int

	switch config.Mode {
	case "anonymization":
		// Get the channel for sending the keys
		keysOutCh, err = ctx.channelRegistry.GetOutputChannel(config.KeysOutputChannel.Name)
		if err != nil {
			return nil, fmt.Errorf("while getting the keys output channel %s: %v", config.KeysOutputChannel.Name, err)
		}
		closeIfNotNil = keysOutCh.channel
		defer func() {
			if closeIfNotNil != nil {
				close(closeIfNotNil)
			}
		}()
		if sp != nil && sp.Format() == "fixed_width" {
			if config.OmitPrefixOnFW {
				omitPrefix = true
			}
		}
	case "de-identification":
		omitPrefix = true
		if len(config.DeidLookups) == 0 {
			return nil, fmt.Errorf("error: de-identification mode requires deid_lookups to be specified")
		}
		if len(config.DeidFunctions) == 0 {
			return nil, fmt.Errorf("error: de-identification mode requires deid_functions to be specified")
		}
	default:
		return nil, fmt.Errorf("error: unknown anonymize mode '%s', known values: anonymization, de-identification", config.Mode)
	}
	if sp != nil && sp.Format() == "fixed_width" {
		if config.AdjustFieldWidthOnFW {
			newWidth = make(map[string]int)
		}
	}
	// Prepare the actions to anonymize marked columns
	// Note: since the metaLookupTbl is generated by the analyze operator,
	// the column names in the metaLookupTbl may be the original column names
	// so we use the column position as lookup key.
	metaLookupTbl = ctx.lookupTableManager.LookupTableMap[config.LookupName]
	if metaLookupTbl == nil {
		return nil, fmt.Errorf("error: anonymize metadata lookup table %s not found", config.LookupName)
	}
	anonymActions = make([]*AnonymizationAction, 0, len(*source.columns))
	metaLookupColumnsMap := metaLookupTbl.ColumnMap()
	for name, ipos := range *source.columns {
		columnPosStr := strconv.Itoa(ipos)
		// Get the metadata row for this column
		// Note: the lookup table may have the original column names, so we use the column position as key
		metaRow, err := metaLookupTbl.Lookup(&columnPosStr)
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
		var deidLookupTbl LookupTable
		var deidFunctionName string

		switch anonymizeType {
		case "text", "date":
			keyPrefixI := (*metaRow)[metaLookupColumnsMap[config.KeyPrefix]]
			keyPrefix, ok = keyPrefixI.(string)
			if !ok {
				return nil, fmt.Errorf("error: expecting string for key prefix (e.g. ssn, dob, etc), got %v", keyPrefixI)
			}
			switch config.Mode {
			case "de-identification":
				// Get the de-identification lookup table name for this key prefix
				lookupTableName, ok := config.DeidLookups[keyPrefix]
				if !ok {
					return nil, fmt.Errorf("error: de-identification lookup table not found for key prefix '%s'",
						keyPrefix)
				}
				if lookupTableName == "" {
					// See if it's a deid function
					deidFunctionName = config.DeidFunctions[keyPrefix]
					if deidFunctionName == "" {
						// Skipping this column
						continue
					}
					// It's a deid function, vaidate the function and adjust column width if needed
					switch deidFunctionName {
					case "hashed_value":
						// Determine the width to adjust for fixed-width files
						if newWidth != nil {
							newWidth[name] = 16
						}
					default:
						return nil, fmt.Errorf("error: unknown de-identification function '%s' for key prefix '%s'",
							deidFunctionName, keyPrefix)
					}
				} else {
					deidLookupTbl = ctx.lookupTableManager.LookupTableMap[lookupTableName]
					if deidLookupTbl == nil {
						return nil, fmt.Errorf("error: de-identification lookup table %s not found for key prefix '%s'",
							lookupTableName, keyPrefix)
					}
				}

			case "anonymization":
				// Determine the width to adjust for fixed-width files
				if newWidth != nil {
					w := 16
					if anonymizeType == "text" && !omitPrefix {
						w = 28
					}
					newWidth[name] = w
				}
				if omitPrefix {
					keyPrefix = ""
				}
			}

			// Get the date layouts if any
			if len(config.DateFormatsColumn) > 0 {
				dlcI := (*metaRow)[metaLookupColumnsMap[config.DateFormatsColumn]]
				dateLayoutsCsv, ok := dlcI.(string)
				if ok {
					r := csv.NewReader(bytes.NewReader([]byte(dateLayoutsCsv)))
					dateLayouts, err = r.Read()
					// fmt.Println("*** Got date layouts:", dateLayouts)
					if err != nil {
						return nil, fmt.Errorf("while decoding date formats from csv:%v", err)
					}
				}
			}

			anonymActions = append(anonymActions, &AnonymizationAction{
				inputColumn:      ipos,
				dateLayouts:      dateLayouts,
				anonymizeType:    anonymizeType,
				keyPrefix:        keyPrefix,
				deidFunctionName: deidFunctionName,
				deidLookupTbl:    deidLookupTbl,
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
	var invalidDate *time.Time
	if len(config.DefaultInvalidDate) > 0 {
		invalidDate, err = ParseDate(config.DefaultInvalidDate)
		if err != nil {
			err = fmt.Errorf(
				"configuration error: anonymize_config.default_invalid_date '%s' is not a valid date (use YYYY/MM/DD format)",
				config.DefaultInvalidDate)
			log.Println(err)
			return nil, err
		}
		outputInvalidDate = invalidDate.Format(outputDateLayout)
		if len(keyDateLayout) == 0 {
			keyInvalidDate = outputInvalidDate
		} else {
			keyInvalidDate = invalidDate.Format(keyDateLayout)
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
		mode:              config.Mode,
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
		invalidDate:       invalidDate,
		inputDateLayout:   inputDateLayout,
		outputDateLayout:  outputDateLayout,
		keyMapDateLayout:  keyDateLayout,
		outputInvalidDate: outputInvalidDate,
		keyInvalidDate:    keyInvalidDate,
		env:               ctx.env,
		doneCh:            ctx.done,
	}, nil
}
