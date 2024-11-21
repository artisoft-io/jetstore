package compute_pipes

import (
	"fmt"
	"hash"
	"hash/fnv"
	"log"
	"time"

	"github.com/dolthub/swiss"
)

type AnonymizeTransformationPipe struct {
	cpConfig         *ComputePipesConfig
	source           *InputChannel
	outputCh         *OutputChannel
	keysOutputCh     *OutputChannel
	hasher           hash.Hash64
	keysMap          *swiss.Map[uint64, [2]string]
	metaLookupTbl    LookupTable
	anonymActions    []*AnonymizationAction
	columnEvaluators []TransformationColumnEvaluator
	firstInputRow    *[]interface{}
	spec             *TransformationSpec
	dateFormat       string
	keyMapDateFormat string
	channelRegistry  *ChannelRegistry
	env              map[string]interface{}
	doneCh           chan struct{}
}

type AnonymizationAction struct {
	inputColumn   int
	anonymizeType string
	keyPrefix     string
}

// Implementing interface PipeTransformationEvaluator
func (ctx *AnonymizeTransformationPipe) apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in AnonymizeTransformationPipe")
	}
	if ctx.firstInputRow == nil {
		ctx.firstInputRow = input
	}
	// hashedValue4KeyFile is the value to use in the crosswalk file, it is
	// the same as hashedValue, except for dates it may use a different date formatter.
	var inputStr, hashedValue, hashedValue4KeyFile string
	for _, action := range ctx.anonymActions {
		value := (*input)[action.inputColumn]
		if value == nil {
			continue
		}
		switch vv := value.(type) {
		case string:
			inputStr = vv
		default:
			inputStr = fmt.Sprintf("%v", vv)
		}
		switch action.anonymizeType {
		case "text":
			ctx.hasher.Reset()
			ctx.hasher.Write([]byte(inputStr))
			if len(action.keyPrefix) > 0 {
				hashedValue = fmt.Sprintf("%s.%x", action.keyPrefix, ctx.hasher.Sum64())
			} else {
				hashedValue = fmt.Sprintf("%x", ctx.hasher.Sum64())
			}
			hashedValue4KeyFile = hashedValue
		case "date":
			date, err := ParseDate(inputStr)
			if err == nil {
				// hashedValue = fmt.Sprintf("%d/%02d/01", date.Year(), date.Month())
				anonymizeDate := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
				hashedValue = anonymizeDate.Format(ctx.dateFormat)
				if ctx.keyMapDateFormat == "" {
					hashedValue4KeyFile = hashedValue
				} else {
					hashedValue4KeyFile = anonymizeDate.Format(ctx.keyMapDateFormat)
				}
			} else {
				hashedValue = inputStr
				hashedValue4KeyFile = hashedValue
			}
		}
		(*input)[action.inputColumn] = hashedValue
		ctx.hasher.Reset()
		ctx.hasher.Write([]byte(inputStr))
		ctx.hasher.Write([]byte(hashedValue4KeyFile))
		ctx.keysMap.Put(ctx.hasher.Sum64(), [2]string{inputStr, hashedValue4KeyFile})
	}
	// Send the result to output
	select {
	case ctx.outputCh.channel <- *input:
	case <-ctx.doneCh:
		log.Printf("AnonymizeTransformationPipe writing to '%s' interrupted", ctx.outputCh.config.Name)
		return nil
	}
	return nil
}

// Anonymization complete, now send out the keys mapping to keys_output_channel
func (ctx *AnonymizeTransformationPipe) done() error {
	var err error
	ctx.keysMap.Iter(func(k uint64, v [2]string) (stop bool) {
		outputRow := make([]interface{}, len(ctx.keysOutputCh.columns))
		outputRow[ctx.keysOutputCh.columns["hashed_key"]] = k
		outputRow[ctx.keysOutputCh.columns["original_value"]] = v[0]
		outputRow[ctx.keysOutputCh.columns["anonymized_value"]] = v[1]

		// Add the carry over select and const values
		// NOTE there is no initialize and done called on the column evaluators
		//      since they should be only of type 'select' or 'value'
		for i := range ctx.columnEvaluators {
			err2 := ctx.columnEvaluators[i].update(&outputRow, ctx.firstInputRow)
			if err2 != nil {
				err2 = fmt.Errorf("while calling column transformation from anonymize operator: %v", err)
				log.Println(err2)
				err = err2
				return true // stop
			}
		}

		// Send the keys mapping to output
		// log.Println("**!@@ ** Send AGGREGATE Result to", ctx.keysOutputCh.config.Name)
		select {
		case ctx.keysOutputCh.channel <- outputRow:
		case <-ctx.doneCh:
			log.Println("AnonymizeTransform interrupted")
			return true // stop
		}
		return false // continue
	})
	// Done sending the keys, closing the keys output channel
	ctx.channelRegistry.CloseChannel(ctx.keysOutputCh.config.Name)
	return err
}

func (ctx *AnonymizeTransformationPipe) finally() {}

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
	metaLookupTbl = ctx.lookupTableManager.LookupTableMap[config.LookupName]
	if metaLookupTbl == nil {
		return nil, fmt.Errorf("error: anonymize metadata lookup table %s not found", config.LookupName)
	}
	anonymActions = make([]*AnonymizationAction, 0)
	metaLookupColumnsMap := metaLookupTbl.ColumnMap()
	for name, ipos := range source.columns {
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
		switch anonymizeType {
		case "text", "date":
			keyPrefixI := (*metaRow)[metaLookupColumnsMap[config.KeyPrefix]]
			keyPrefix, ok := keyPrefixI.(string)
			if !ok {
				return nil, fmt.Errorf("error: expecting string for key prefix (e.g. ssn, dob, etc), got %v", keyPrefixI)
			}
			anonymActions = append(anonymActions, &AnonymizationAction{
				inputColumn:   ipos,
				anonymizeType: anonymizeType,
				keyPrefix:     keyPrefix,
			})
		case "":
			// Not anonymized
		default:
			return nil, fmt.Errorf("error: unknown anonymize type '%s', known values: test, date", anonymizeType)
		}
	}
	hasher = fnv.New64a()
	// Determine the date format to use, start with default value
	dateFormat := "2006/01/02"
	var keyDateFormat string
	// Check if the schema provider is specified
	var sp SchemaProvider
	if len(config.SchemaProvider) > 0 {
		sp = ctx.schemaManager.GetSchemaProvider(config.SchemaProvider)
		if sp == nil {
			return nil, fmt.Errorf("error: anonymize_config has schema_provider '%s', but it is not found", config.SchemaProvider)
		}
	}
	if sp != nil && sp.DateFormat() != "" {
		dateFormat = sp.DateFormat()
	}
	if config.DateFormat != "" {
		dateFormat = config.DateFormat
	}
	if config.KeyDateFormat != "" {
		keyDateFormat = config.KeyDateFormat
	}

	// Prepare the column evaluators
	columnEvaluators = make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.buildTransformationColumnEvaluator(source, keysOutCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while buildTransformationColumnEvaluator (in NewAnonymizeTransformationPipe) %v", err)
			log.Println(err)
			return nil, err
		}
	}
	// All good, no errors
	closeIfNotNil = nil
	return &AnonymizeTransformationPipe{
		cpConfig:         ctx.cpConfig,
		source:           source,
		outputCh:         outputCh,
		keysOutputCh:     keysOutCh,
		hasher:           hasher,
		keysMap:          swiss.NewMap[uint64, [2]string](2048),
		metaLookupTbl:    metaLookupTbl,
		anonymActions:    anonymActions,
		columnEvaluators: columnEvaluators,
		channelRegistry:  ctx.channelRegistry,
		spec:             spec,
		dateFormat:       dateFormat,
		keyMapDateFormat: keyDateFormat,
		env:              ctx.env,
		doneCh:           ctx.done,
	}, nil
}
