package compute_pipes

import (
	"bytes"
	"fmt"
	"hash"
	"hash/fnv"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/date_utils"
	"github.com/artisoft-io/jetstore/jets/utils"
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
	blankMarkers      *BlankFieldMarkers
	spec              *TransformationSpec
	inputDateLayout   string
	outputDateLayout  string
	keyMapDateLayout  string
	invalidDate       *time.Time
	outputInvalidDate string
	keyInvalidDate    string
	capDobYears       int
	setDodToJan1      bool
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
	tnow := time.Now()
	currentYear := tnow.Year()
	currentMonth := tnow.Month()

	// hashedValue4KeyFile is the value to use in the crosswalk file, it is
	// the same as hashedValue, except for dates it may use a different date formatter.
	var inputStr, hashedValue, hashedValue4KeyFile string
	var ok bool
	inputLen := len(*input)
	expectedLen := len(ctx.source.Config.Columns)
	// log.Println("*** Anonymize Input:",*input)
	// log.Println("*** Len Input:",inputLen, "Expected Len:", expectedLen)
	// NOTE: Must handle rows with less or more columns than expected. Anonymize the extra columns without a prefix
nextAction:
	for _, action := range ctx.anonymActions {
		if action.inputColumn >= inputLen {
			continue nextAction
		}
		value := (*input)[action.inputColumn]
		if value == nil {
			continue nextAction
		}
		outputDateLayout := ctx.outputDateLayout
		switch vv := value.(type) {
		case string:
			upperValue := strings.ToUpper(vv)
			if upperValue == "NULL" {
				continue nextAction
			}
			if ctx.blankMarkers != nil {
				txt := &upperValue
				if ctx.blankMarkers.CaseSensitive {
					txt = &vv
				}
				if slices.Contains(ctx.blankMarkers.Markers, *txt) {
					continue nextAction
				}
			}
			inputStr = vv
		case int:
			inputStr = strconv.Itoa(vv)
		case int64:
			inputStr = strconv.FormatInt(vv, 10)
		case float64:
			inputStr = strconv.FormatFloat(vv, 'f', -1, 64)
		default:
			inputStr = fmt.Sprintf("%v", vv)
		}
		switch action.anonymizeType {
		case "text":
			ctx.hasher.Reset()
			ctx.hasher.Write([]byte(inputStr))
			switch ctx.mode {
			case "de-identification":
				switch {
				case action.deidLookupTbl != nil:
					// use de-identification lookup table
					// Lookup the anonymized value from the de-identification lookup table
					nrows := uint64(action.deidLookupTbl.Size())
					if nrows == 0 {
						return fmt.Errorf("error: de-identification lookup table for key prefix '%s' is empty",
							action.keyPrefix)
					}
					rowKey := fmt.Sprintf("%d", ctx.hasher.Sum64()%nrows+1)
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
				case len(action.deidFunctionName) > 0:
					// use de-identification function
					// Use the de-identification function
					switch action.deidFunctionName {
					case "hashed_value":
						hashedValue = fmt.Sprintf("%016x", ctx.hasher.Sum64())
					default:
						return fmt.Errorf("error: unknown de-identification function '%s' for key prefix '%s'",
							action.deidFunctionName, action.keyPrefix)
					}
				default:
					// blank out the value
					hashedValue = ""
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
				year := date.Year()
				month := date.Month()
				if ctx.capDobYears > 0 && action.keyPrefix == "dob" {
					if currentYear-year > ctx.capDobYears {
						year = currentYear - ctx.capDobYears
					}
					if month > currentMonth {
						year--
					}
				}
				if ctx.setDodToJan1 && action.keyPrefix == "dod" {
					month = time.January
				}
				anonymizeDate := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
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
	case ctx.outputCh.Channel <- *input:
	case <-ctx.doneCh:
		log.Printf("AnonymizeTransformationPipe writing to '%s' interrupted", ctx.outputCh.Name)
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
		outputRow := make([]any, len(*ctx.keysOutputCh.Columns))
		outputRow[(*ctx.keysOutputCh.Columns)["hashed_key"]] = k
		outputRow[(*ctx.keysOutputCh.Columns)["original_value"]] = v[0]
		outputRow[(*ctx.keysOutputCh.Columns)["anonymized_value"]] = v[1]

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
		// log.Println("**!@@ ** Send AGGREGATE Result to", ctx.keysOutputCh.Name)
		select {
		case ctx.keysOutputCh.Channel <- outputRow:
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
		ctx.channelRegistry.CloseChannel(ctx.keysOutputCh.Name)
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
		closeIfNotNil = keysOutCh.Channel
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

	var capDobYears int
	var setDodToJan1 bool
	var blankMarkers *BlankFieldMarkers
	if sp != nil {
		blankMarkers = sp.BlankFieldMarkers()
		if sp.Format() == "fixed_width" {
			if config.AdjustFieldWidthOnFW {
				newWidth = make(map[string]int)
			}
		}
		capDobYears = sp.CapDobYears()
		setDodToJan1 = sp.SetDodToJan1()
	}

	// Prepare the actions to anonymize marked columns
	// Note: since the metaLookupTbl is generated by the analyze operator,
	// the column names in the metaLookupTbl may be the original column names (which may contain duplicates),
	// so we use the column position as lookup key.
	metaLookupTbl = ctx.lookupTableManager.LookupTableMap[config.LookupName]
	if metaLookupTbl == nil {
		return nil, fmt.Errorf("error: anonymize metadata lookup table %s not found", config.LookupName)
	}
	anonymActions = make([]*AnonymizationAction, 0, len(*source.Columns))
	metaLookupColumnsMap := metaLookupTbl.ColumnMap()

	// Also collect the original column name of the columns that are anopnymized.
	anonymizedColumns := make([]string, 0)
	colNamePos, ok := metaLookupColumnsMap["column_name"]
	if !ok {
		return nil, fmt.Errorf("error: metadata lookup table '%s' is missing column 'column_name'", config.LookupName)
	}

	for name, ipos := range *source.Columns {
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
			anonymizedColumns = append(anonymizedColumns, (*metaRow)[colNamePos].(string))
			keyPrefixI := (*metaRow)[metaLookupColumnsMap[config.KeyPrefix]]
			keyPrefix, ok = keyPrefixI.(string)
			if !ok {
				return nil, fmt.Errorf("error: expecting string for key prefix (e.g. ssn, dob, etc), got %v", keyPrefixI)
			}
			if anonymizeType == "text" {
				switch config.Mode {
				case "de-identification":
					// Get the de-identification lookup table name for this key prefix
					lookupTableName, ok := config.DeidLookups[keyPrefix]
					if !ok {
						// See if it's a deid function
						deidFunctionName, ok = config.DeidFunctions[keyPrefix]
						if ok {
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
						if !omitPrefix {
							w = 28
						}
						newWidth[name] = w
					}
					if omitPrefix {
						keyPrefix = ""
					}
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

	// If this is node 0, than save the anonymized column names to s3 location, even if it's an empty file
	if ctx.nodeId == 0 && config.AnonymizedColumnsOutputFile != nil {
		outputFileSpec := config.AnonymizedColumnsOutputFile
		delimit := string(outputFileSpec.Delimiter)
		bucket := utils.ReplaceEnvVars(outputFileSpec.Bucket, ctx.env)
		path := utils.ReplaceEnvVars(outputFileSpec.OutputLocation, ctx.env)

		data := fmt.Sprintf("\"%s\"\n", strings.Join(anonymizedColumns, fmt.Sprintf("\"%s\"", delimit)))
		if ctx.cpConfig.ClusterConfig.IsDebugMode {
			log.Println("***", ctx.sessionId, "Uploading anonymized columns file to s3 bucket:", bucket, "path:", path)
			log.Println(data)
		}
		err = awsi.UploadToS3FromReader(bucket, path, strings.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("while uploading anonymized columns file to s3 output location %s/%s: %v",
				outputFileSpec.Bucket, outputFileSpec.OutputLocation, err)
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
		blankMarkers:      blankMarkers,
		spec:              spec,
		invalidDate:       invalidDate,
		inputDateLayout:   inputDateLayout,
		outputDateLayout:  outputDateLayout,
		keyMapDateLayout:  keyDateLayout,
		outputInvalidDate: outputInvalidDate,
		keyInvalidDate:    keyInvalidDate,
		capDobYears:       capDobYears,
		setDodToJan1:      setDodToJan1,
		env:               ctx.env,
		doneCh:            ctx.done,
	}, nil
}
