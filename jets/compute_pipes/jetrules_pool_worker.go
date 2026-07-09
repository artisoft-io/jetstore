package compute_pipes

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/utils"
	togo "github.com/toon-format/toon-go"
)

// Worker to perform jetrules execute rules function

type JrPoolWorker struct {
	config                   *JetrulesSpec
	source                   *InputChannel
	rdfType2Columns          map[string][]string
	multiValueDataProperties map[string]bool
	objectProperties         map[string]bool
	column2RdfType           map[string]string
	ruleEngine               JetRuleEngine
	errorCount               int
	nbrReteSessionsSaved     int
	errorOutputCh            *OutputChannel
	outputChannels           []*JetrulesOutputChan
	done                     chan struct{}
	errCh                    chan error
	builderContext           *BuilderContext
}

func (ctx *BuilderContext) NewJrPoolWorker(config *JetrulesSpec, source *InputChannel, rdfType2Columns map[string][]string,
	re JetRuleEngine, errorOutputCh *OutputChannel, outputChannels []*JetrulesOutputChan,
	done chan struct{}, errCh chan error) *JrPoolWorker {

	// Prepare a map of the multi-value properties for the output channels, to ensure proper cardinality.
	mvProperties := make(map[string]bool)
	objProperties := make(map[string]bool)
	var column2RdfType map[string]string
	for _, outChannel := range outputChannels {
		pm, err := GetMultiValueDataProperties(outChannel.ClassName)
		if err != nil {
			log.Println("Error getting multi-value data properties for class", outChannel.ClassName, ":", err)
			continue
		}
		for _, prop := range pm {
			mvProperties[prop] = true
		}
		op, err := GetObjectProperties(outChannel.ClassName)
		if err != nil {
			log.Println("Error getting object properties for class", outChannel.ClassName, ":", err)
			continue
		}
		for _, prop := range op {
			objProperties[prop] = true
		}
		p2t, err := GetDataPropertyRdfType(outChannel.ClassName)
		if err != nil {
			log.Println("Error getting data property RDF types for class", outChannel.ClassName, ":", err)
			continue
		}
		if column2RdfType == nil {
			column2RdfType = p2t
		} else {
			maps.Copy(column2RdfType, p2t)
		}
	}
	if column2RdfType == nil {
		column2RdfType = make(map[string]string)
	}

	// log.Println("New Pool Worker Created")
	return &JrPoolWorker{
		config:                   config,
		source:                   source,
		ruleEngine:               re,
		errorOutputCh:            errorOutputCh,
		outputChannels:           outputChannels,
		done:                     done,
		errCh:                    errCh,
		rdfType2Columns:          rdfType2Columns,
		multiValueDataProperties: mvProperties,
		objectProperties:         objProperties,
		column2RdfType:           column2RdfType,
		builderContext:           ctx,
	}
}

func (ctx *JrPoolWorker) DoWork(mgr *JrPoolManager, resultCh chan JetrulesWorkerResult) {
	var count int64
	var err error
	for task := range mgr.WorkersTaskCh {
		err = ctx.executeRules(&task, resultCh)
		if err != nil {
			return
		}
		count += 1
	}
	select {
	case resultCh <- JetrulesWorkerResult{
		ReteSessionCount: count,
		ErrorsCount:      int64(ctx.errorCount),
	}:
	case <-ctx.done:
		log.Println("jetrules pool worker interrupted")
	}
}

// Perform jetrules execute rules
// errorOutputCh to collect rule errors / exception to write to process_errors table:
//   - rete session triples saved
//   - BAD ROW via ExecuteRules() returned error
//   - error: max loop reached
//   - Rete Session Has Rule Exception
func (ctx *JrPoolWorker) executeRules(inputRecords *[]any,
	resultCh chan JetrulesWorkerResult) (err error) {
	// Create a rdf session for input and execute rules on that session
	// Steps to do here
	// 	- Create the rdf session
	//	- Assert current source period to meta graph (done in NewJetrulesTransformationPipe)
	//	- Assert the input records to rdf session
	//	- Assert rule config to meta graph from the pipeline configuration (done in NewJetrulesTransformationPipe)
	//	- For each ruleset, create a rete session and execute_rules

	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			var buf strings.Builder
			err = fmt.Errorf("executeRules: recovered error: %v", r)
			buf.WriteString(err.Error())
			buf.WriteString("\n")
			buf.WriteString(string(debug.Stack()))
			cpErr := errors.New(buf.String())
			log.Println(cpErr)
			resultCh <- JetrulesWorkerResult{Err: cpErr}
			ctx.errCh <- cpErr
			// Avoid closing a closed channel
			select {
			case <-ctx.done:
			default:
				close(ctx.done)
			}
		}
	}()

	// log.Println("*** Pool Worker == Entering executeRules")

	var cpErr error
	re := ctx.ruleEngine
	jr := re.JetResources()
	var maxLooping, iloop int
	var wc *rete.WorkspaceControl
	var rm JetResourceManager
	var reteSession JetReteSession
	var inputAsserted bool
	var ruleFileNames []string
	// var ctor TripleIterator
	isDebug := ctx.config.IsDebug
	// Create the rdf session
	rdfSession, err := re.NewRdfSession()
	if err != nil {
		cpErr = fmt.Errorf("error: while creating new rdf session: %v", err)
		goto gotError
	}
	defer rdfSession.Release()

	wc, err = GetWorkspaceControl()
	if err != nil {
		cpErr = fmt.Errorf("while getting workspace control in executeRules: %v", err)
		goto gotError
	}
	// Loop over all rulesets
	if isDebug {
		log.Printf("jetrules: Looping over rulesets %s", re.MainRuleFile())
	}
	ruleFileNames = wc.RuleFileNames(re.MainRuleFile())
	if len(ruleFileNames) == 0 {
		cpErr = fmt.Errorf("error: no rulesets found for main rule file name %s", re.MainRuleFile())
		goto gotError
	}
	for _, ruleset := range ruleFileNames {
		// Create the rete session
		reteSession, err = rdfSession.NewReteSession(ruleset)
		rm = rdfSession.GetResourceManager()
		if err != nil {
			cpErr = fmt.Errorf("error: while creating rete session for ruleset %s: %v", ruleset, err)
			goto gotError
		}
		if !inputAsserted {
			// Assert the input records to rdf session
			err = assertInputRecords(ctx.config, ctx.source, ctx.rdfType2Columns, rdfSession, inputRecords)
			if err != nil {
				cpErr = fmt.Errorf("while asserting input records to rdf session: %v", err)
				goto gotError
			}
			inputAsserted = true
		}

		// Step 0 of loop is pre loop or no loop
		// Step 1+ for looping
		rdfSession.Erase(jr.Jets__istate, jr.Jets__loop, nil)
		rdfSession.Erase(jr.Jets__istate, jr.Jets__completed, nil)
		maxLooping = 0
		if ctx.config.MaxLooping == 0 {
			// get the $max_looping of the workspace
			v, err := GetRuleEngineConfig(ruleset, "$max_looping")
			if err != nil {
				cpErr = fmt.Errorf(
					"error: while getting '$max_looping' property from workspace %s: %v",
					ruleset, err)
				goto gotError
			}
			if len(v) > 0 {
				maxLooping, err = strconv.Atoi(v)
				if err != nil {
					cpErr = fmt.Errorf(
						"error: invalid '$max_looping' property in workspace %s: %v",
						ruleset, err)
					goto gotError
				}
			}
		} else {
			maxLooping = ctx.config.MaxLooping
		}
		if maxLooping > 0 {
			log.Printf("jetrules: looping in use, max number of loops is %d", maxLooping)
		}
		// do for iloop <= maxloop (since looping start at one!)
		for iloop = 0; iloop <= maxLooping; iloop++ {
			if iloop > 0 {
				rdfSession.Insert(jr.Jets__istate, jr.Jets__loop, rm.NewIntLiteral(iloop))
			}
			err2 := reteSession.ExecuteRules()
			if err2 != nil {
				if ctx.errorOutputCh != nil && ctx.errorCount < 50 {
					// report the rule error
					peRow := ctx.builderContext.NewProcessError()
					peRow.ErrorMessage = fmt.Sprintf("ExecuteRules returned error: %v", err2)
					peRow.write2Chan(ctx.errorOutputCh, ctx.done)
					log.Printf("jetrules: ExecuteRules returned error: %v", err2)
				} else {
					if ctx.config.IsDebug {
						log.Printf("jetrules: ExecuteRules returned error: %v", err2)
					}
				}
				ctx.errorCount++
				break
			}
			// Check if looping is completed (Jets__completed)
			if rdfSession.ContainsSP(jr.Jets__istate, jr.Jets__completed) {
				log.Printf("jetrules: Rete Session Looping Completed with iloop %d", iloop)
				break
			}
		}
		if maxLooping > 0 && iloop >= maxLooping {
			// Looped til the end, something might be wrong
			if ctx.errorOutputCh != nil && ctx.errorCount < 40 {
				peRow := ctx.builderContext.NewProcessError()
				peRow.ErrorMessage = fmt.Sprintf("MAX LOOP REACHED, maxLooping is %d", maxLooping)
				peRow.write2Chan(ctx.errorOutputCh, ctx.done)
				log.Printf("jetrules: MAX LOOP REACHED, maxLooping is %d", maxLooping)
			} else {
				if ctx.config.IsDebug {
					log.Printf("jetrules: MAX LOOP REACHED, maxLooping is %d", maxLooping)
				}
			}
			ctx.errorCount++
		}
		// Check for any jets:exceptions in the rdfSession
		ctor := rdfSession.FindSP(jr.Jets__istate, jr.Jets__exception)
		for !ctor.IsEnd() {
			hasException := ctor.GetObject()
			if hasException != nil {
				// report jetrules exception, save rete session
				if ctx.errorOutputCh != nil && ctx.errorCount < 25 {
					peRow := ctx.builderContext.NewProcessError()
					peRow.ErrorMessage = fmt.Sprintf("jets:exception caught: %s", hasException)
					if ctx.config.MaxReteSessionsSaved > 0 && ctx.nbrReteSessionsSaved < ctx.config.MaxReteSessionsSaved {
						ctx.nbrReteSessionsSaved++
						peRow.ReteSessionSaved = "Y"
						peRow.ReteSessionTriples = sql.NullString{String: rdfSession.EncodeRdfSession(), Valid: true}
					}
					peRow.write2Chan(ctx.errorOutputCh, ctx.done)
					log.Printf("jetrule: jets:exception caught: %s", hasException)
				} else {
					if ctx.config.IsDebug {
						log.Printf("jetrule: jets:exception caught: %s", hasException)
					}
				}
				ctx.errorCount++
			}
			ctor.Next()
		}
		ctor.Release()
		reteSession.Release()
	}

	// // Print rdf session if in debug mode
	// // if isDebug {
	// 	log.Println("Execute Rules Completed")
	// 		//************************
	// 		log.Println("************************")
	// 		ctor = rdfSession.Find()
	// 		for !ctor.IsEnd() {
	// 			s := ctor.GetSubject()
	// 			p := ctor.GetPredicate()
	// 			o := ctor.GetObject()
	// 			log.Printf("triple: (%v, %v, %v)", s, p, o)
	// 			ctor.Next()
	// 		}
	// 		log.Println("************************")
	// // }

	// Extract data from the rdf session based on class names
	for _, outChannel := range ctx.outputChannels {

		switch outChannel.OutputCh.Config.EntityEncoding {
		case "toon":
			err = ctx.extractSessionData(rdfSession, outChannel, "toon")
		case "json":
			err = ctx.extractSessionData(rdfSession, outChannel, "json")
		default:
			err = ctx.extractSessionData(rdfSession, outChannel, "row")
		}
		if err != nil {
			cpErr = fmt.Errorf(
				"while extraction entity from jetrules for class %s: %v",
				outChannel.ClassName, err)
			goto gotError
		}
	}

	// log.Println("*** Pool Worker == Done Extracting Session DATA")

	return

gotError:
	log.Println(cpErr)
	resultCh <- JetrulesWorkerResult{Err: cpErr}
	ctx.errCh <- cpErr
	// Avoid closing a closed channel
	select {
	case <-ctx.done:
	default:
		close(ctx.done)
	}
	return cpErr
}

func keepObjectForCurrentSourcePeriod(rdfSession JetRdfSession, subject RdfNode) (bool, error) {
	// Check if subject is an entity for the current source period
	// i.e. is not an historical entity comming from the lookback period
	// We don't extract historical entities but only one from the current source period
	// identified with jets:source_period_sequence == 0 or
	// entities created during the rule session, identified with jets:source_period_sequence is null.
	// Additional Measure: entities with jets:source_period_sequence == 0, must have jets:InputRecord
	// as rdf:type to ensure it's a mapped entity and not an injected entity.
	// Note: Do not save the jets:InputRecord marker type on the extracted obj.
	var sourcePeriod int
	var err error
	keepObj := true
	jr := rdfSession.JetResources()
	obj := rdfSession.GetObject(subject, jr.Jets__source_period_sequence)
	if obj != nil && obj.Value() != nil {
		switch v := GetRdfNodeValue(obj).(type) {
		case int:
			sourcePeriod = v
		case string:
			sourcePeriod, err = strconv.Atoi(v)
			if err != nil {
				// invalid source period sequence value, don't extract the obj
				log.Printf("warning: invalid jets:source_period_sequence value for subject %s, expected int, got string: %s, skipping obj extraction",
					subject, v)
				keepObj = false
				sourcePeriod = -1
			}
		default:
			// invalid source period sequence value, don't extract the obj
			log.Printf("warning: invalid jets:source_period_sequence value for subject %s, expected int, got %v (%T), skipping obj extraction",
				subject, GetRdfNodeValue(obj), GetRdfNodeValue(obj))
			keepObj = false
			sourcePeriod = -1
		}
		if sourcePeriod == 0 {
			// Check if obj has marker type jets:InputRecord, extract obj if it does.
			if !rdfSession.Contains(subject, jr.Rdf__type, jr.Jets__input_record) {
				// jets:InputRecord marker is missing, don't extract the obj
				keepObj = false
			}
		} else {
			keepObj = false
		}
	}
	log.Printf("*** keepObject? subject: %s, sourcePeriod: %d, keepObj: %v", subject, sourcePeriod, keepObj)
	return keepObj, err
}

func (ctx *JrPoolWorker) extractLiteralValue(rdfSession JetRdfSession, subject, predicate RdfNode,
	currentSourcePeriod int, outChannel *JetrulesOutputChan) any {
	var data any
	var dataArr []any
	var isArray bool
	pname := predicate.String()
	switch pname {
	case "jets:source_period_sequence":
		// Set the current source period to the extracted data based on the value in the rdf session
		data = currentSourcePeriod
	case "rdf:type":
		// Special handling for rdf:type, keep only the asserted rdf:type, which is the channel's class name
		data = []any{outChannel.ClassName}
	default:
		data = nil
		isArray = false
		itor := rdfSession.FindSP(subject, predicate)
		for !itor.IsEnd() {
			value := GetRdfNodeValue(itor.GetObject())
			if data == nil {
				data = value
			} else {
				if isArray {
					dataArr = append(dataArr, value)
				} else {
					dataArr = []any{data, value}
					isArray = true
				}
			}
			itor.Next()
		}
		itor.Release()
		if ctx.multiValueDataProperties[pname] {
			if isArray {
				data = dataArr
			} else {
				data = []any{data}
			}
		} else {
			if isArray {
				// If the data property is of type text, then keep as array
				if ctx.column2RdfType[pname] == "text" {
					data = dataArr
				} else {
					// Report the first 20 as error, set to null
					if ctx.errorOutputCh != nil && ctx.errorCount < 20 {
						peRow := ctx.builderContext.NewProcessError()
						peRow.ErrorMessage = fmt.Sprintf("property %s is not multi-value but has multiple values for subject %s, setting value to null", pname, subject)
						peRow.write2Chan(ctx.errorOutputCh, ctx.done)
						ctx.errorCount += 1
						log.Printf("warning: property %s is not multi-value but has multiple values for subject %s, setting value to null", pname, subject)
					} else {
						if ctx.config.IsDebug {
							log.Printf("warning: property %s is not multi-value but has multiple values for subject %s, setting value to null", pname, subject)
						}
					}
					data = nil
				}
			}
		}
	}
	return data
}

// Navigate recursively the object properties and extract their values into a map[string]any
// excluding the properties starting with _0:
func (ctx *JrPoolWorker) extractObjectValue(rdfSession JetRdfSession, subject RdfNode,
	entityObj map[string]any, currentSourcePeriod int, outChannel *JetrulesOutputChan) {
	itor := rdfSession.FindS(subject)
	for !itor.IsEnd() {
		log.Printf("*** Triple (%s, %s, %s)", itor.GetSubject(), itor.GetPredicate(), itor.GetObject())
		prop := itor.GetPredicate()
		if strings.HasPrefix(prop.String(), "_0:") {
			itor.Next()
			continue
		}
		// Check if it's an obj property
		jtor := rdfSession.FindS(itor.GetObject())
		isObjProperty := false
		for !jtor.IsEnd() {
			isObjProperty = true
			subEntityObj := make(map[string]any)
			addToEntityObj(entityObj, prop.String(), subEntityObj)
			ctx.extractObjectValue(rdfSession, jtor.GetSubject(), subEntityObj, currentSourcePeriod, outChannel)
			jtor.Next()
		}
		if !isObjProperty {
			// It's a literal property, extract its value
			addToEntityObj(entityObj, prop.String(), itor.GetObject().Value())
		}
		itor.Next()
	}
}

func addToEntityObj(entityObj map[string]any, prop string, value any) {
	if value == nil {
		return
	}
	if existing, ok := entityObj[prop]; ok {
		// If existing is any, then create a slice to hold current and existing values
		// If existing is []any then add to it
		switch existingVal := existing.(type) {
		case []any:
			existingVal = append(existingVal, value)
			entityObj[prop] = existingVal
		case nil:
			entityObj[prop] = value
		default:
			entityObj[prop] = []any{existingVal, value}
		}
	} else {
		entityObj[prop] = value
	}
}

func (ctx *JrPoolWorker) extractSessionData(rdfSession JetRdfSession,
	outChannel *JetrulesOutputChan, encoding string) error {

	jr := rdfSession.JetResources()
	rm := rdfSession.GetResourceManager()
	entityCount := 0
	columns := outChannel.OutputCh.Config.Columns
	var data any
	var keepObj bool
	var err error
	isDebug := ctx.config.IsDebug
	currentSourcePeriod := ctx.config.CurrentSourcePeriod

	// Extract entity by rdf type
	log.Printf("*** Pool Worker == Extracting entities of class %s, encoding %s", outChannel.ClassName, encoding)
	ctor := rdfSession.FindSPO(nil, jr.Rdf__type, rm.NewResource(outChannel.ClassName))
	for !ctor.IsEnd() {
		subject := ctor.GetSubject()
		keepObj, err = keepObjectForCurrentSourcePeriod(rdfSession, subject)
		if err != nil {
			log.Printf("error: failed to determine if object should be kept for subject %s: %v", subject, err)
			ctor.Next()
			continue
		}
		// extract entity if we keep it (i.e. not an historical entity)
		if keepObj {
			log.Printf("*** Extracting entity for subject %s", subject)
			entityRow := make([]any, len(*outChannel.OutputCh.Columns))
			switch encoding {
			case "toon", "json":
				// For toon and json encoding, we extract the entire object as a map[string]any
				log.Printf("*** Extracting json/toon obj - start")
				entityObj := make(map[string]any)
				ctx.extractObjectValue(rdfSession, subject, entityObj, currentSourcePeriod, outChannel)
				log.Printf("*** Extracting json/toon obj - end")
				if encoding == "toon" {
					// For toon encoding, we need to convert the map to a toon string
					toonBytes, err := togo.Marshal(entityObj)
					if err != nil {
						err = fmt.Errorf("error: failed to marshal entity object to toon for subject %s: %v", subject, err)
						log.Println(err)
						return err
					}
					log.Printf("*** toon encoded obj:\n%s", string(toonBytes))
					entityRow[(*outChannel.OutputCh.Columns)["json:data"]] = string(toonBytes)
				} else {
					// For json encoding, we need to convert the map to a json string
					jsonBytes, err := json.Marshal(entityObj)
					if err != nil {
						err = fmt.Errorf("error: failed to marshal entity object to json for subject %s: %v", subject, err)
						log.Println(err)
						return err
					}
					log.Printf("*** json encoded obj:\n%s", string(jsonBytes))
					entityRow[(*outChannel.OutputCh.Columns)["json:data"]] = string(jsonBytes)
				}

			default:
				log.Printf("*** Extracting DEFAULT encoding for subject %s", subject)
				for i, p := range columns {
					data = ctx.extractLiteralValue(rdfSession, subject, rm.NewResource(p), currentSourcePeriod, outChannel)
					entityRow[i] = data
				}
			}
			// Apply the TransformationColumn, these are const values
			// NOTE there is no initialize and done called on the column evaluators
			//      since they should be only of type 'select' or 'value'
			// Note: using entityRow as both current value and input for the purpose of these operators
			for i := range outChannel.ColumnEvaluators {
				err := outChannel.ColumnEvaluators[i].Update(&entityRow, &entityRow)
				if err != nil {
					err = fmt.Errorf("while calling column transformation from jetrules extract session data: %v", err)
					log.Println(err)
					return err
				}
			}
			// Send the record to output channel
			log.Printf("*** Extracted ENTITY_ROW: %v", entityRow)
			select {
			case outChannel.OutputCh.Channel <- entityRow:
				entityCount += 1
			case <-ctx.done:
				log.Printf("jetrule extractSessionData writing to '%s' interrupted", outChannel.OutputCh.Name)
				return nil
			}
		}
		ctor.Next()
	}
	ctor.Release()
	log.Printf("*** jetrules: Extracted %d entities for class %s", entityCount, outChannel.ClassName)
	if isDebug {
		log.Printf("jetrules: Extracted %d entities for class %s", entityCount, outChannel.ClassName)
	}
	return nil
}

func assertInputRecords(config *JetrulesSpec, source *InputChannel, rdfType2Columns map[string][]string,
	rdfSession JetRdfSession, inputRecords *[]any) (err error) {

	nrows := 0
	if source.HasGroupedRows {
		// log.Printf("*** Pool Worker == Asserting bundle of %d entities\n", len(*inputRecords))
		for i := range *inputRecords {
			row, ok := (*inputRecords)[i].([]any)
			if !ok {
				return fmt.Errorf("error: inputRecords are invalid")
			}
			err = assertInputRow(config, rdfSession, &row, rdfType2Columns)
			if err != nil {
				return fmt.Errorf("while asserting input record %d: %v", i, err)
			}
			nrows += 1
		}
	} else {
		// log.Printf("*** Pool Worker == Asserting single entities\n")
		err = assertInputRow(config, rdfSession, inputRecords, rdfType2Columns)
		if err != nil {
			return fmt.Errorf("while asserting input records: %v", err)
		}
		nrows += 1
	}
	if config.IsDebug {
		log.Printf("jetrules: Asserted %d input records", nrows)
	}
	return
}

func assertInputRow(config *JetrulesSpec, rdfSession JetRdfSession, row *[]any, rdfType2Columns map[string][]string) (err error) {
	var predicate RdfNode
	// assert record i
	var jetsKey string
	var rdfTypes []any
	var sourcePeriodSequence int
	var subject RdfNode
	var node RdfNode
	jr := rdfSession.JetResources()
	rm := rdfSession.GetResourceManager()

	// Input channel have a class name, which will have jets:key, rdf:type, jets:source_period_sequence in pos 0, 1 and 2 resp.
	var ok bool
	jetsKey, ok = (*row)[0].(string)
	if !ok {
		jetsKey = ComputeRowHash((*row)[3:], config.CurrentSourcePeriod)
	}

	rdfTypes, ok = (*row)[1].([]any)
	if !ok {
		if config.InputRdfType != "" {
			// Use class name from config, and generate jets:key and set source period sequence to -1 (i.e. not from the input data but generated during the rule session)
			rdfTypes = []any{config.InputRdfType}
		} else {
			return fmt.Errorf("error: input rdf:Type not provided and invalid rdf:type in the row")
		}
	}
	assertType := rdfTypes[0].(string)
	if config.IsDebug {
		log.Printf("Asserting Input Record with rdf:type %s and jets:key %s", assertType, jetsKey)
	}
	columns := rdfType2Columns[assertType]
	if len(columns) == 0 {
		return fmt.Errorf("error: no columns found for rdf:type %s in input record", assertType)
	}
	nbrCol := len(columns)
	if config.IsDebug {
		data, err := utils.ZipSlicesNoNil(columns, *row)
		if err != nil {
			return fmt.Errorf("while zipping input columns and values for debug logging: %v", err)
		}
		outBytes, _ := json.Marshal(data)
		log.Printf("Asserting Input Record (zipped no null): %s", string(outBytes))
	}

	// sourcePeriodSequence is file value, ie. the source period of the entity when extracted.
	// will correct below with config.CurrentSourcePeriod
	sourcePeriodSequence, ok = (*row)[2].(int)
	if !ok {
		sourcePeriodSequence = -1
	}

	subject = rm.NewResource(jetsKey)
	err = rdfSession.Insert(subject, jr.Jets__key, rm.NewTextLiteral(jetsKey))
	if err != nil {
		return
	}
	for _, t := range rdfTypes {
		err = rdfSession.Insert(subject, jr.Rdf__type, rm.NewResource(t.(string)))
		if err != nil {
			return
		}
	}

	// jets:source_period_sequence
	// Assert the jets:InputRecord rdf:type if sourcePeriodSequence == -1
	if sourcePeriodSequence == -1 {
		err = rdfSession.Insert(subject, jr.Rdf__type, jr.Jets__input_record)
		if err != nil {
			return
		}
		sourcePeriodSequence = config.CurrentSourcePeriod
	}
	// Insert the jets:source_period_sequence property
	err = rdfSession.Insert(subject, jr.Jets__source_period_sequence,
		rm.NewIntLiteral(config.CurrentSourcePeriod-sourcePeriodSequence))
	if err != nil {
		return
	}

	// assert the rest of the properties
nextField:
	for j := range *row {
		if (*row)[j] == nil {
			continue
		}
		if j < nbrCol {
			cname := columns[j]
			if cname == "rdf:type" || cname == "jets:key" || cname == "jets:source_period_sequence" {
				// already asserted these properties, skip to avoid confusion and potential error
				continue nextField
			}
			predicate = rm.NewResource(cname)
		} else {
			predicate = rm.NewResource(fmt.Sprintf("column%d", j))
		}
		switch vv := (*row)[j].(type) {
		case []any:
			for _, value := range vv {
				node, err = NewRdfNode(value, rm)
				if err != nil {
					return fmt.Errorf("while NewRdfNode for value in array: %v", err)
				}
				err = rdfSession.Insert(subject, predicate, node)
				if err != nil {
					return
				}
			}
		default:
			node, err = NewRdfNode(vv, rm)
			if err != nil {
				return fmt.Errorf("while NewRdfNode: %v", err)
			}
			err = rdfSession.Insert(subject, predicate, node)
			if err != nil {
				return
			}
		}
	}
	return
}

func NewRdfNode(inValue any, re JetResourceManager) (RdfNode, error) {
	switch vv := inValue.(type) {
	case string:
		return re.NewTextLiteral(vv), nil
	case int:
		return re.NewIntLiteral(vv), nil
	case uint:
		return re.NewUIntLiteral(vv), nil
	case float64:
		return re.NewDoubleLiteral(vv), nil
	case time.Time:
		// Check if it's a date or a datetime
		if vv.Hour() == 0 && vv.Minute() == 0 && vv.Second() == 0 {
			// Date
			return re.NewDateDetails(vv.Year(), int(vv.Month()), vv.Day()), nil
		} else {
			// Datetime
			return re.NewDatetimeDetails(vv.Year(), int(vv.Month()), vv.Day(),
				vv.Hour(), vv.Minute(), vv.Second()), nil
		}
	case int64:
		return re.NewIntLiteral(int(vv)), nil
	case uint64:
		return re.NewUIntLiteral(uint(vv)), nil
	case int32:
		return re.NewIntLiteral(int(vv)), nil
	case uint32:
		return re.NewUIntLiteral(uint(vv)), nil
	case float32:
		return re.NewDoubleLiteral(float64(vv)), nil
	default:
		return nil, fmt.Errorf("error: unknown type %T for NewRdfNode", vv)
	}
}
