package compute_pipes

import (
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/google/uuid"
)

// Worker to perform jetrules execute rules function

type JrPoolWorker struct {
	config         *JetrulesSpec
	source         *InputChannel
	ruleEngine     JetRuleEngine
	outputChannels []*JetrulesOutputChan
	done           chan struct{}
	errCh          chan error
}

func NewJrPoolWorker(config *JetrulesSpec, source *InputChannel,
	re JetRuleEngine, outputChannels []*JetrulesOutputChan,
	done chan struct{}, errCh chan error) *JrPoolWorker {
	// log.Println("New Pool Worker Created")
	return &JrPoolWorker{
		config:         config,
		source:         source,
		ruleEngine:     re,
		outputChannels: outputChannels,
		done:           done,
		errCh:          errCh,
	}
}

func (ctx *JrPoolWorker) DoWork(mgr *JrPoolManager, resultCh chan JetrulesWorkerResult) {
	var count int64
	var errCount int64
	var err error
	for task := range mgr.WorkersTaskCh {
		errCount, err = ctx.executeRules(&task, resultCh)
		if err != nil {
			return
		}
		count += 1
	}
	select {
	case resultCh <- JetrulesWorkerResult{
		ReteSessionCount: count,
		ErrorsCount:      errCount,
	}:
	case <-ctx.done:
		log.Println("jetrules pool worker interrupted")
	}
}

// Perform jetrules execute rules
// TODO Add reteSessionSaved to save rete session to process_errors table
// TODO Add rule errors / exception to process_errors table
//   - BAD ROW via ExecuteRules() returned error
//   - error: max loop reached
//   - Rete Session Has Rule Exception
func (ctx *JrPoolWorker) executeRules(inputRecords *[]any,
	resultCh chan JetrulesWorkerResult) (errCount int64, err error) {
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
	// Create the rdf session
	rdfSession, err := re.NewRdfSession()
	if err != nil {
		cpErr = fmt.Errorf("error: while creating new rdf session: %v", err)
		goto gotError
	}
	defer rdfSession.Release()
	rm = rdfSession.GetResourceManager()

	// Assert the input records to rdf session
	err = assertInputRecords(ctx.config, ctx.source, rdfSession, inputRecords)
	if err != nil {
		cpErr = fmt.Errorf("while asserting input records to rdf session: %v", err)
		goto gotError
	}
	wc, err = GetWorkspaceControl()
	if err != nil {
		cpErr = fmt.Errorf("while getting workspace control in executeRules: %v", err)
		goto gotError
	}
	// Loop over all rulesets
	for _, ruleset := range wc.RuleFileNames(re.MainRuleFile()) {
		// Create the rete session
		log.Printf("*** executeRules: Creating Rete Session for %s\n", ruleset)
		reteSession, err = rdfSession.NewReteSession(ruleset)
		if err != nil {
			cpErr = fmt.Errorf("error: while creating rete session for ruleset %s: %v", ruleset, err)
			goto gotError
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
						"error: invalid '$max_looping' property in workspace %s, using 1000: %v",
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
				//*TODO report the rule error
				log.Printf("jetrules: ExecuteRules returned error: %v", err2)
				errCount += 1
				break
			}
			// Check if looping is completed (Jets__completed)
			if rdfSession.ContainsSP(jr.Jets__istate, jr.Jets__completed) {
				log.Print("jetrules: Rete Session Looping Completed")
				break
			}
		}
		if maxLooping > 0 && iloop >= maxLooping {
			// Looped til the end, something might be wrong
			//*TODO report the rule error
			log.Printf("jetrules: MAX LOOP REACHED, maxLooping is %d", maxLooping)
			errCount += 1
		}
		// Check for any jets:exceptions in the rdfSession
		ctor := rdfSession.FindSP(jr.Jets__istate, jr.Jets__exception)
		for !ctor.IsEnd() {
			hasException := ctor.GetObject()
			if hasException != nil {
				//*TODO report jetrules exception, save rete session
				log.Printf("jetrule: jets:exception caught: %s", hasException)
				errCount += 1
			}
			ctor.Next()
		}
		ctor.Release()
		reteSession.Release()
	}

	log.Println("*** Pool Worker == Done executing the rulesets")

	// Print rdf session if in debug mode
	if ctx.config.IsDebug {
		log.Println("ASSERTED GRAPH")
		// log.Printf("\n%s\n", strings.Join(rdfSession.AssertedGraph.ToTriples(), "\n"))
		log.Println("INFERRED GRAPH")
		// log.Printf("\n%s\n", strings.Join(rdfSession.InferredGraph.ToTriples(), "\n"))
	}

	// Extract data from the rdf session based on class names
	for _, outChannel := range ctx.outputChannels {
		err = ctx.extractSessionData(rdfSession, outChannel)
		if err != nil {
			cpErr = fmt.Errorf(
				"while extraction entity from jetrules for class %s: %v",
				outChannel.ClassName, err)
			goto gotError
		}
	}

	log.Println("*** Pool Worker == Done Extracting Session DATA")

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
	return 0, cpErr
}

func (ctx *JrPoolWorker) extractSessionData(rdfSession JetRdfSession,
	outChannel *JetrulesOutputChan) error {

	jr := rdfSession.JetResources()
	rm := rdfSession.GetResourceManager()
	entityCount := 0
	columns := outChannel.OutputCh.Config.Columns
	var data any
	var dataArr *[]any
	var isArray bool
	// Extract entity by rdf type
	log.Println("*** Pool Worker == Extracting entities of class", outChannel.ClassName)
	ctor := rdfSession.FindSPO(nil, jr.Rdf__type, rm.NewResource(outChannel.ClassName))
	for !ctor.IsEnd() {
		subject := ctor.GetSubject()
		// Check if subject is an entity for the current source period
		// i.e. is not an historical entity comming from the lookback period
		// We don't extract historical entities but only one from the current source period
		// identified with jets:source_period_sequence == 0 or
		// entities created during the rule session, identified with jets:source_period_sequence is null.
		// Additional Measure: entities with jets:source_period_sequence == 0, must have jets:InputRecord
		// as rdf:type to ensure it's a mapped entity and not an injected entity.
		// Note: Do not save the jets:InputEntity marker type on the extracted obj.
		keepObj := true
		obj := rdfSession.GetObject(subject, jr.Jets__source_period_sequence)
		if obj != nil && obj.Value() != nil {
			v := GetRdfNodeValue(obj).(int)
			if v == 0 {
				// Check if obj has marker type jets:InputRecord, extract obj if it does.
				if !rdfSession.Contains(subject, jr.Rdf__type, jr.Jets__input_record) {
					// jets:InputEntity marker is missing, don't extract the obj
					keepObj = false
				}
			} else {
				keepObj = false
			}
		}
		// extract entity if we keep it (i.e. not an historical entity)
		if keepObj {
			entityRow := make([]any, len(columns))
			for i, p := range columns {
				data = nil
				isArray = false
				itor := rdfSession.FindSP(subject, rm.NewResource(p))
				for !itor.IsEnd() {
					value := GetRdfNodeValue(itor.GetObject())
					if p == "rdf:type" {
						c, ok := value.(string)
						if ok && c == "jets:InputRecord" {
							goto go_next
						}
					}
					if data == nil {
						data = value
					} else {
						if isArray {
							*dataArr = append(*dataArr, value)
						} else {
							dataArr = &[]any{data, value}
							isArray = true
						}
					}
				go_next:
					itor.Next()
				}
				itor.Release()
				if isArray {
					data = *dataArr
				}
				entityRow[i] = data
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
			// log.Println("*** Extracted ENTITY_ROW:", entityRow)
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
	log.Printf("jetrules: Extracted %d entities for class %s", entityCount, outChannel.ClassName)
	return nil
}

func assertInputRecords(config *JetrulesSpec, source *InputChannel,
	rdfSession JetRdfSession, inputRecords *[]any) (err error) {

	columns := source.Config.Columns
	if source.HasGroupedRows {
		// log.Printf("*** Pool Worker == Asserting bundle of %d entities\n", len(*inputRecords))
		for i := range *inputRecords {
			row, ok := (*inputRecords)[i].([]any)
			if !ok {
				return fmt.Errorf("error: inputRecords are invalid")
			}
			err = assertInputRow(config, rdfSession, &row, &columns)
		}
	} else {
		// log.Printf("*** Pool Worker == Asserting single entities\n")
		err = assertInputRow(config, rdfSession, inputRecords, &columns)
	}
	return
}

func assertInputRow(config *JetrulesSpec, rdfSession JetRdfSession, row *[]any, columns *[]string) (err error) {

	nbrCol := len(*columns)
	var predicate RdfNode
	// assert record i
	var jetsKey, rdfType string
	var subject RdfNode
	var node RdfNode
	jr := rdfSession.JetResources()
	rm := rdfSession.GetResourceManager()
	// Assert the rdf type if provided in config, otherwise it must be part of the data
	if config.InputRdfType != "" {
		jetsKey = uuid.New().String()
		rdfType = config.InputRdfType
	} else {
		// Input channel must have a class name, which will have jets:key and rdf:type in pos 0 and 1 resp.
		var ok1, ok2 bool
		jetsKey, ok1 = (*row)[0].(string)
		rdfType, ok2 = (*row)[1].(string)
		if !ok1 || !ok2 {
			return fmt.Errorf("error: invalid type for jets:key or rdf:type as first 2 elements of row")
		}
	}
	subject = rm.NewResource(jetsKey)
	err = rdfSession.Insert(subject, jr.Jets__key, rm.NewTextLiteral(jetsKey))
	if err != nil {
		return
	}
	err = rdfSession.Insert(subject, jr.Rdf__type, rm.NewResource(rdfType))
	if err != nil {
		return
	}

	// Assert the jets:InputRecord rdf:type
	err = rdfSession.Insert(subject, jr.Rdf__type, jr.Jets__input_record)
	if err != nil {
		return
	}

	for j := range *row {
		if (*row)[j] == nil {
			continue
		}
		if j < nbrCol {
			predicate = rm.NewResource((*columns)[j])
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
			}
		default:
			node, err = NewRdfNode(vv, rm)
			if err != nil {
				return fmt.Errorf("while NewRdfNode: %v", err)
			}
			err = rdfSession.Insert(subject, predicate, node)
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
