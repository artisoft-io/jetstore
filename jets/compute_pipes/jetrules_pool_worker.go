package compute_pipes

import (
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/google/uuid"
)

// Worker to perform jetrules execute rules function

type JrPoolWorker struct {
	config         *JetrulesSpec
	source         *InputChannel
	reteMetaStore  *rete.ReteMetaStoreFactory
	outputChannels []*JetrulesOutputChan
	done           chan struct{}
	errCh          chan error
}

func NewJrPoolWorker(config *JetrulesSpec, source *InputChannel,
	reteMetaStore *rete.ReteMetaStoreFactory, outputChannels []*JetrulesOutputChan,
	done chan struct{}, errCh chan error) *JrPoolWorker {

	return &JrPoolWorker{
		config:         config,
		source:         source,
		reteMetaStore:  reteMetaStore,
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
	resultCh <- JetrulesWorkerResult{
		ReteSessionCount: count,
		ErrorsCount:      errCount,
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

	var cpErr error
	// var reteSessionSaved bool
	rdfSession := rdf.NewRdfSession(ctx.reteMetaStore.ResourceMgr,
		ctx.reteMetaStore.MetaGraph)
	rm := rdfSession.ResourceMgr
	jr := rm.JetsResources
	var maxLooping, iloop int

	// Assert the input records to rdf session
	err = assertInputRecords(ctx.config, ctx.source, rm, jr,
		rdfSession.AssertedGraph, inputRecords)

	// Loop over all rulesets
	for _, ruleset := range ctx.reteMetaStore.MainRuleFileNames {
		// Create the rete session
		ms := ctx.reteMetaStore.MetaStoreLookup[ruleset]
		if ms == nil {
			cpErr = fmt.Errorf("error: metastore not found for %s", ruleset)
			goto gotError
		}
		reteSession := rete.NewReteSession(rdfSession)
		reteSession.Initialize(ms)

		// Step 0 of loop is pre loop or no loop
		// Step 1+ for looping
		rdfSession.Erase(jr.Jets__istate, jr.Jets__loop, nil)
		rdfSession.Erase(jr.Jets__istate, jr.Jets__completed, nil)
		maxLooping = 0
		if ctx.config.MaxLooping == 0 {
			// get the $max_looping of the metastore
			v := (*ms.JetStoreConfig)["$max_looping"]
			if len(v) > 0 {
				maxLooping, err = strconv.Atoi(v)
				if err != nil {
					cpErr = fmt.Errorf(
						"error: invalid '$max_looping' property in metastore %s, using 1000: %v",
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
		for t3 := range ctor.Itor {
			hasException := t3[2]
			if hasException != nil {
				//*TODO report jetrules exception, save rete session
				log.Printf("jetrule: jets:exception caught: %s", hasException)
				errCount += 1
			}
		}
		ctor.Done()

		// Extract data from the rdf session based on class names
		for _, outChannel := range ctx.outputChannels {
			err = ctx.extractSessionData(rdfSession, outChannel)
			cpErr = fmt.Errorf(
				"while extraction entity from jetrules for class %s: %v",
				outChannel.className, err)
			goto gotError
		}
		reteSession.Done()
	}
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

func (ctx *JrPoolWorker) extractSessionData(rdfSession *rdf.RdfSession,
	outChannel *JetrulesOutputChan) error {

	rm := rdfSession.ResourceMgr
	jr := rm.JetsResources
	entityCount := 0
	columns := outChannel.outputCh.config.Columns
	var data any
	var dataArr *[]any
	var isArray bool
	// Extract entity by rdf type
	ctor := rdfSession.FindSPO(nil, jr.Rdf__type, rm.NewResource(outChannel.className))
	for t3 := range ctor.Itor {
		subject := t3[0]
		// Check if subject is an entity for the current source period
		// i.e. is not an historical entity comming from the lookback period
		// We don't extract historical entities but only one from the current source period
		// identified with jets:source_period_sequence == 0 or
		// entities created during the rule session, identified with jets:source_period_sequence is null
		// Additional Measure: entities with jets:source_period_sequence == 0, must have jets:InputRecord
		// as rdf:type to ensure it's a mapped entity and not an injected entity.
		// Note: Do not save the jets:InputEntity marker type on the extracted obj.
		keepObj := true
		obj := rdfSession.GetObject(subject, jr.Jets__source_period_sequence)
		if obj != nil {
			v := obj.Value.(int)
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
				for t3 := range itor.Itor {
					if data == nil {
						data = getValue(t3[2])
					} else {
						if isArray {
							*dataArr = append(*dataArr, getValue(t3[2]))
						} else {
							dataArr = &[]any{data, getValue(t3[2])}
							data = *dataArr
							isArray = true
						}
					}
				}
				itor.Done()
				entityRow[i] = data
			}
			// Send the record to output channel
			select {
			case outChannel.outputCh.channel <- entityRow:
				entityCount += 1
			case <-ctx.done:
				log.Printf("jetrule extractSessionData writing to '%s' interrupted", outChannel.outputCh.config.Name)
				return nil
			}
		}
	}
	ctor.Done()
	log.Printf("jetrules: Extracted %d entities for class %s", entityCount, outChannel.className)
	return nil
}

func getValue(r *rdf.Node) any {
	switch vv := r.Value.(type) {
	case int, float64, string:
		return r.Value
	case rdf.LDate:
		return *vv.Date
	case rdf.NamedResource:
		return vv.Name
	case rdf.LDatetime:
		return *vv.Datetime
	case rdf.RdfNull:
		return nil
	case rdf.BlankNode:
		return fmt.Sprintf("BN%d", vv.Key)
	case int64:
		return int(vv)
	case int32:
		return int(vv)
	default:
		return nil
	}
}

func assertInputRecords(config *JetrulesSpec, source *InputChannel,
	rm *rdf.ResourceManager, jr *rdf.JetResources, graph *rdf.RdfGraph,
	inputRecords *[]any) (err error) {

	columns := source.config.Columns
	if source.hasGroupedRows {
		for i := range *inputRecords {
			row, ok := (*inputRecords)[i].([]any)
			if !ok {
				return fmt.Errorf("error: inputRecords are invalid")
			}
			err = assertInputRow(config, rm, jr, graph, &row, &columns)
		}
	} else {
		err = assertInputRow(config, rm, jr, graph, inputRecords, &columns)
	}
	return
}

func assertInputRow(config *JetrulesSpec, rm *rdf.ResourceManager, jr *rdf.JetResources,
	graph *rdf.RdfGraph, row *[]any, columns *[]string) (err error) {

	nbrCol := len(*columns)
	var predicate *rdf.Node
	// assert record i
	jetsKey := uuid.New().String()
	subject := rm.NewResource(jetsKey)
	// Assert the rdf type if provided in config, otherwise it must be part of the data
	if config.InputRdfType != "" {
		_, err = graph.Insert(subject, jr.Rdf__type, rm.NewResource(config.InputRdfType))
		if err != nil {
			return
		}
	}
	for j := range *row {
		if j < nbrCol {
			predicate = rm.NewResource((*columns)[j])
		} else {
			predicate = rm.NewResource(fmt.Sprintf("column%d", j))
		}
		switch vv := (*row)[j].(type) {
		case string:
			_, err = graph.Insert(subject, predicate, rm.NewTextLiteral(vv))
		case []string:
			for k := range vv {
				_, err = graph.Insert(subject, predicate, rm.NewTextLiteral(vv[k]))
			}
		case int:
			_, err = graph.Insert(subject, predicate, rm.NewIntLiteral(vv))
		case []int:
			for k := range vv {
				_, err = graph.Insert(subject, predicate, rm.NewIntLiteral(vv[k]))
			}
		case float64:
			_, err = graph.Insert(subject, predicate, rm.NewDoubleLiteral(vv))
		case []float64:
			for k := range vv {
				_, err = graph.Insert(subject, predicate, rm.NewDoubleLiteral(vv[k]))
			}
		case rdf.LDate:
			_, err = graph.Insert(subject, predicate, rm.NewDateLiteral(vv))
		case []rdf.LDate:
			for k := range vv {
				_, err = graph.Insert(subject, predicate, rm.NewDateLiteral(vv[k]))
			}
		case rdf.LDatetime:
			_, err = graph.Insert(subject, predicate, rm.NewDatetimeLiteral(vv))
		case []rdf.LDatetime:
			for k := range vv {
				_, err = graph.Insert(subject, predicate, rm.NewDatetimeLiteral(vv[k]))
			}
		}
		if err != nil {
			return
		}
	}
	return
}