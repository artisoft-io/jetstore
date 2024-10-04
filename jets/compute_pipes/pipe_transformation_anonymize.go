package compute_pipes

import (
	"fmt"
	"log"

	"github.com/dolthub/maphash"
	"github.com/dolthub/swiss"
)

type AnonymizeTransformationPipe struct {
	cpConfig         *ComputePipesConfig
	source           *InputChannel
	outputCh         *OutputChannel
	keysOutputCh     *OutputChannel
	hasher           *maphash.Hasher[string]
	keysMap          *swiss.Map[uint64, [2]string]
	metaLookupTbl    LookupTable
	anonymActions    []*AnonymizationAction
	columnEvaluators []TransformationColumnEvaluator
	firstInputRow    *[]interface{}
	spec             *TransformationSpec
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
	var inputStr, hashedValue string
	for _, action := range ctx.anonymActions {
		value := (*input)[action.inputColumn]
		switch vv := value.(type) {
		case string:
			inputStr = vv
		default:
			inputStr = fmt.Sprintf("%v", vv)
		}
		switch action.anonymizeType {
		case "text":
			if len(action.keyPrefix) > 0 {
				hashedValue = fmt.Sprintf("%s.%x", action.keyPrefix, ctx.hasher.Hash(inputStr))
			} else {
				hashedValue = fmt.Sprintf("%x", ctx.hasher.Hash(inputStr))
			}
		case "date":
			date, err := ParseDate(inputStr)
			if err == nil {
				hashedValue = fmt.Sprintf("%d/%02d/01", date.Year(), date.Month())
			} else {
				hashedValue = inputStr
			}
		}
		(*input)[action.inputColumn] = hashedValue
		ctx.keysMap.Put(ctx.hasher.Hash(inputStr + hashedValue) , [2]string{inputStr, hashedValue})
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
		outputRow := make([]interface{}, len(ctx.outputCh.columns))
		outputRow[ctx.outputCh.columns["hashed_key"]] = k
		outputRow[ctx.outputCh.columns["original_value"]] = v[0]
		outputRow[ctx.outputCh.columns["anonymized_value"]] = v[1]

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
		// log.Println("**!@@ ** Send AGGREGATE Result to", ctx.outputCh.config.Name)
		select {
		case ctx.outputCh.channel <- outputRow:
		case <-ctx.doneCh:
			log.Println("AnonymizeTransform interrupted")
			return true // stop
		}
		return false // continue
	})
	return err
}

func (ctx *AnonymizeTransformationPipe) finally() {}

func (ctx *BuilderContext) NewAnonymizeTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*AnonymizeTransformationPipe, error) {
	var err error
	if spec == nil || spec.AnonymizeConfig == nil {
		return nil, fmt.Errorf("error: Anonymize Pipe Transformation spec is missing or anonymize_config is missing")
	}
	config := spec.AnonymizeConfig
	if len(config.AnonymizeType) == 0 || len(config.KeyPrefix) == 0 {
		return nil, fmt.Errorf("error: Anonymize Pipe Transformation spec is missing anonymize_type or key_prefix")
	}
	// Get the channel for sending the keys
	keysOutCh, err := ctx.channelRegistry.GetOutputChannel(config.KeysOutputChannel.Name)
	if err != nil {
		return nil, fmt.Errorf("while getting the keys output channel %s: %v", config.KeysOutputChannel.Name, err)
	}
	// Prepare the actions to anonymize marked columns
	metaLookupTbl := ctx.lookupTableManager.LookupTableMap[config.LookupName]
	if metaLookupTbl == nil {
		return nil, fmt.Errorf("error: anonymize metadata lookup table %s not found", config.LookupName)
	}
	anonymActions := make([]*AnonymizationAction, 0)
	metaLookupColumnsMap := metaLookupTbl.ColumnMap()
	for name, ipos := range source.columns {
		metaRow, err := metaLookupTbl.Lookup(&name)
		if err != nil {
			return nil, fmt.Errorf("while getting the metadata row for column %s: %v", name, err)
		}
		if metaRow == nil {
			return nil, fmt.Errorf("error: metadata row not found for column %s", name)
		}
		anonymizeType := (*metaRow)[metaLookupColumnsMap[config.AnonymizeType]].(string)
		switch anonymizeType {
		case "text", "date":
		default:
			return nil, fmt.Errorf("error: unknown anonymize type '%s', known values: test, date", anonymizeType)
		}
		anonymActions = append(anonymActions, &AnonymizationAction{
			inputColumn:   ipos,
			anonymizeType: anonymizeType,
			keyPrefix:     (*metaRow)[metaLookupColumnsMap[config.KeyPrefix]].(string),
		})
	}
	hasher := maphash.NewHasher[string]()

	// Prepare the column evaluators
	columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
	for i := range spec.Columns {
		// log.Printf("**& build TransformationColumn[%d] of type %s for output %s", i, spec.Type, spec.Output)
		columnEvaluators[i], err = ctx.buildTransformationColumnEvaluator(source, keysOutCh, &spec.Columns[i])
		if err != nil {
			err = fmt.Errorf("while buildTransformationColumnEvaluator (in NewAnonymizeTransformationPipe) %v", err)
			log.Println(err)
			return nil, err
		}
	}

	return &AnonymizeTransformationPipe{
		cpConfig:         ctx.cpConfig,
		source:           source,
		outputCh:         outputCh,
		keysOutputCh:     keysOutCh,
		hasher:           &hasher,
		keysMap:          swiss.NewMap[uint64, [2]string](2048),
		metaLookupTbl:    metaLookupTbl,
		anonymActions:    anonymActions,
		columnEvaluators: columnEvaluators,
		spec:             spec,
		env:              ctx.env,
		doneCh:           ctx.done,
	}, nil
}
