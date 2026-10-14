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
)

// Worker to perform jetrules execute rules function

type JrPoolWorker struct {
	config                   *JetrulesSpec
	source                   *InputChannel
	rdfType2Columns          map[string][]string
	multiValueDataProperties map[string]bool
	column2RdfType           map[string]string
	ruleEngine               JetRuleEngine
	errorCount               int
	nbrReteSessionsSaved     int
	errorOutputCh            *OutputChannel
	outputChannels           []*JetrulesOutputChan
	done                     chan struct{}
	errCh                    chan error
	builderContext           *BuilderContext
	// groupingKey computes the domain key of a session's input, nil when the
	// input has none (see jetrulesGroupingDomainKey).
	groupingKey *HashEvaluator
	// The keys of the rete session running, which its error rows carry (setSessionKeys).
	sessionGroupingKey sql.NullString
	sessionRowJetsKey  sql.NullString
}

func (ctx *BuilderContext) NewJrPoolWorker(config *JetrulesSpec, source *InputChannel, rdfType2Columns map[string][]string,
	re JetRuleEngine, errorOutputCh *OutputChannel, outputChannels []*JetrulesOutputChan,
	done chan struct{}, errCh chan error) *JrPoolWorker {

	// Prepare a map of the multi-value properties for the output channels, to ensure proper cardinality.
	mvProperties := make(map[string]bool)
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
		column2RdfType:           column2RdfType,
		builderContext:           ctx,
		groupingKey:              ctx.newJetrulesGroupingKey(source),
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
//
// The rule loop itself is RunRuleSets, which the rules unit-test harness calls
// too. What stays here is what belongs to a pipeline: asserting a bundle from
// the input channel, reporting each failure to the error channel as the rule
// set that raised it ends, the on_error policy, extracting the output, and
// closing done on error.
func (ctx *JrPoolWorker) executeRules(inputRecords *[]any,
	resultCh chan JetrulesWorkerResult) (err error) {
	// Create a rdf session for input and execute rules on that session
	// Steps to do here
	// 	- Create the rdf session
	//	- Assert current source period to meta graph (done in NewJetrulesTransformationPipe)
	//	- Assert the input records to rdf session
	//	- Assert rule config to meta graph from the pipeline configuration (done in NewJetrulesTransformationPipe)
	//	- For each ruleset, create a rete session and execute_rules (RunRuleSets)

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
	var outcome RuleSetsOutcome
	re := ctx.ruleEngine
	// The engine's JetResources are created on first use, through the meta graph,
	// so they must exist before any rdf session opens (see RunRuleSets).
	re.JetResources()
	// The keys every error row of this session carries
	ctx.setSessionKeys(inputRecords)
	// Create the rdf session
	rdfSession, err := re.NewRdfSession()
	if err != nil {
		cpErr = fmt.Errorf("error: while creating new rdf session: %v", err)
		goto gotError
	}
	defer rdfSession.Release()

	outcome = RunRuleSets(rdfSession, re, &RuleSetsConfig{
		MaxLooping:   ctx.config.MaxLooping,
		IsDebug:      ctx.config.IsDebug,
		AfterRuleSet: ctx.reportRuleSet,
	}, func(rdfSession JetRdfSession) error {
		// Assert the input records to rdf session
		return assertInputRecords(ctx.config, ctx.source, ctx.rdfType2Columns, rdfSession, inputRecords)
	})
	if outcome.Err != nil {
		cpErr = outcome.Err
		goto gotError
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

	// Apply the on_error policy when the record bundle failed rule execution
	if outcome.Failed() {
		switch ctx.config.OnError {
		case OnErrorDrop:
			// discard the bundle's output
			return
		case OnErrorFail:
			cpErr = fmt.Errorf("jetrules: record bundle failed rule execution (on_error: fail)")
			goto gotError
		}
	}

	// Extract data from the rdf session based on class names
	for _, outChannel := range ctx.outputChannels {
		err = ctx.extractSessionData(rdfSession, outChannel, outChannel.OutputCh.Config)
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

// reportRuleSet reports one rule set's failures to the log and the error
// channel, in the order the loop meets them: the ExecuteRules error, the
// maximum loop count reached, then each jets:exception. It is RunRuleSets'
// AfterRuleSet, so it runs while the rule set's rete session is still open and
// a saved session holds the triples as they were when the rule set ended.
// Every failure counts toward max_error_count whether or not it is reported.
func (ctx *JrPoolWorker) reportRuleSet(rdfSession JetRdfSession, rs *RuleSetOutcome) {
	if rs.ExecuteErr != nil {
		ctx.errorCount++
		switch {
		case ctx.errorCount <= ctx.config.MaxErrorCount:
			log.Printf("jetrules: ExecuteRules returned error: %v", rs.ExecuteErr)
			if ctx.errorOutputCh != nil {
				// report the rule error
				peRow := ctx.newProcessError()
				peRow.ErrorMessage = fmt.Sprintf("ExecuteRules returned error: %v", rs.ExecuteErr)
				peRow.write2Chan(ctx.errorOutputCh, ctx.done)
			}
		case ctx.errorCount == ctx.config.MaxErrorCount+1:
			log.Printf("jetrules: reached max_error_count (%d), stop reporting errors", ctx.config.MaxErrorCount)
		default:
			if ctx.config.IsDebug {
				log.Printf("jetrules: ExecuteRules returned error: %v", rs.ExecuteErr)
			}
		}
	}
	if rs.MaxLoopReached {
		// Looped til the end, something might be wrong
		ctx.errorCount++
		switch {
		case ctx.errorCount <= ctx.config.MaxErrorCount:
			log.Printf("jetrules: MAX LOOP REACHED, maxLooping is %d", rs.MaxLooping)
			if ctx.errorOutputCh != nil {
				peRow := ctx.newProcessError()
				peRow.ErrorMessage = fmt.Sprintf("MAX LOOP REACHED, maxLooping is %d", rs.MaxLooping)
				peRow.write2Chan(ctx.errorOutputCh, ctx.done)
			}
		case ctx.errorCount == ctx.config.MaxErrorCount+1:
			log.Printf("jetrules: reached max_error_count (%d), stop reporting errors", ctx.config.MaxErrorCount)
		default:
			if ctx.config.IsDebug {
				log.Printf("jetrules: MAX LOOP REACHED, maxLooping is %d", rs.MaxLooping)
			}
		}
	}
	for _, hasException := range rs.Exceptions {
		// report jetrules exception, save rete session
		ctx.errorCount++
		switch {
		case ctx.errorCount <= ctx.config.MaxErrorCount:
			log.Printf("jetrule: jets:exception caught: %s", hasException)
			if ctx.errorOutputCh != nil {
				peRow := ctx.newProcessError()
				peRow.ErrorMessage = fmt.Sprintf("jets:exception caught: %s", hasException)
				if ctx.config.MaxReteSessionsSaved > 0 && ctx.nbrReteSessionsSaved < ctx.config.MaxReteSessionsSaved {
					ctx.nbrReteSessionsSaved++
					peRow.ReteSessionSaved = "Y"
					peRow.ReteSessionTriples = sql.NullString{String: rdfSession.EncodeRdfSession(), Valid: true}
				}
				peRow.write2Chan(ctx.errorOutputCh, ctx.done)
			}
		case ctx.errorCount == ctx.config.MaxErrorCount+1:
			log.Printf("jetrules: reached max_error_count (%d), stop reporting errors", ctx.config.MaxErrorCount)
		default:
			if ctx.config.IsDebug {
				log.Printf("jetrule: jets:exception caught: %s", hasException)
			}
		}
	}
}

// RuleSetsConfig is what RunRuleSets reads besides the sessions. A pipeline
// builds it from its JetrulesSpec; the rules unit-test harness from the
// jetrules_config of a test.
type RuleSetsConfig struct {
	// MaxLooping, when not 0, replaces every rule set's $max_looping
	// (JetrulesSpec.MaxLooping).
	MaxLooping int
	IsDebug    bool
	// AfterRuleSet, when not nil, is called once for each rule set that ran,
	// after its loop and its exception scan and before its rete session is
	// released, with that rule set's outcome. executeRules reports failures
	// through it; it must not retain rdfSession.
	AfterRuleSet func(rdfSession JetRdfSession, outcome *RuleSetOutcome)
}

// RuleSetOutcome is what happened in one rule set.
type RuleSetOutcome struct {
	RuleSet string
	// MaxLooping is the maximum in force, from the rule file's $max_looping or
	// RuleSetsConfig.MaxLooping; 0 means the rule set does not loop and runs
	// loop 0 only.
	MaxLooping int
	// Loops is the last loop value run: 0 is the pre-loop pass, and looping
	// starts at 1.
	Loops int
	// Completed is true when the rule set asserted (jets:iState jets:completed ?).
	Completed bool
	// MaxLoopReached is true when the loop ran out of iterations: every loop
	// MaxLooping allows ran, and none set jets:completed or returned an
	// ExecuteRules error. Completion or an error on the last allowed loop is not
	// the maximum reached (rules_unit_test_harness I-7, fixed by AA.3).
	MaxLoopReached bool
	// ExecuteErr is the error ExecuteRules returned, which ends the rule set's
	// loop; the max vertex visit limit is one.
	ExecuteErr error
	// Exceptions are the objects of (jets:iState jets:exception ?) that this
	// rule set raised: in the rdf session once it has run, and not there when it
	// started. An exception stays in the session for the later rule sets, whose
	// rules may read it, but only the rule set that raised it reports it
	// (rules_unit_test_harness I-8, Michel 2026-10-11: report each exception
	// once). See exceptionKey for what makes two exceptions the same.
	Exceptions []string
}

// RuleSetsOutcome is what happened when RunRuleSets ran a process's rule sets
// over one rdf session.
type RuleSetsOutcome struct {
	// RuleSets holds one outcome per rule set that ran, in order.
	RuleSets []RuleSetOutcome
	// Completed is true when every rule set that loops asserted jets:completed,
	// none returned an ExecuteRules error, and Err is nil.
	Completed bool
	// Loops is the last loop value run by the last rule set that ran.
	Loops int
	// MaxLoopReached is true when any rule set reached its maximum.
	MaxLoopReached bool
	// ExecuteErr is the first ExecuteRules error of any rule set.
	ExecuteErr error
	// Exceptions are every rule set's Exceptions, in order, as pipelines report
	// them: each exception once, from the rule set that raised it.
	Exceptions []string
	// Err is set when the rule sets could not be run to the end: no rule set for
	// the process, a rete session that could not be created, the input that could
	// not be asserted, or an unreadable $max_looping. RuleSets then holds the rule
	// sets that ran before it.
	Err error
}

// Failed reports whether the bundle failed rule execution in on_error's sense:
// an ExecuteRules error, the maximum loop count reached, or a jets:exception.
func (o *RuleSetsOutcome) Failed() bool {
	return o.ExecuteErr != nil || o.MaxLoopReached || len(o.Exceptions) > 0
}

// RunRuleSets runs the rule sets of engine's process over rdfSession: the rule
// loop of a jetrules pipeline, shared with the rules unit-test harness so that a
// test and a pipeline run one loop. It touches no channel.
//
// The rule sets are the workspace control's for engine.MainRuleFile(): a rule
// sequence's, or the one rule set of that name. For each one it creates a rete
// session and runs the looping protocol: erase (jets:iState jets:loop ?) and
// (jets:iState jets:completed ?), then for each loop from 0 to the maximum
// insert (jets:iState jets:loop n) from 1, call ExecuteRules, and stop on an
// ExecuteRules error or on jets:completed; then scan for jets:exception and keep
// the exceptions that were not in the session when the rule set started.
//
// assertInput asserts the input into rdfSession. It is called once, after the
// first rule set's rete session is created and before any rule set runs, which
// is the order executeRules has always used.
//
// **Precondition: call engine.JetResources() before opening rdfSession.** Both
// adaptors create the JetResources on first use, through the meta graph's
// resource manager, and the meta graph locks when the first rdf session opens:
// after that a new meta resource is refused, a nil node on Go and an error on
// C++. (On C++ it aborted the process until 2026-10-11, rules_unit_test_harness
// I-15.) A pipeline meets it because
// AssertSourcePeriodInfo, at pipe construction, and executeRules both call it
// first; the harness must call it before NewRdfSession. RunRuleSets cannot do it
// for the caller, since the session is already open when it is called. The
// same holds for AssertTextRow, which reads the session's JetResources.
func RunRuleSets(rdfSession JetRdfSession, engine JetRuleEngine, config *RuleSetsConfig,
	assertInput func(rdfSession JetRdfSession) error) (outcome RuleSetsOutcome) {

	jr := engine.JetResources()
	var maxLooping, iloop int
	var wc *rete.WorkspaceControl
	var rm JetResourceManager
	var reteSession JetReteSession
	var inputAsserted bool
	var ruleFileNames []string
	var err error
	// The exceptions in the rdf session when the current rule set started, which
	// are the previous rule set's scan; nothing between two scans asserts one.
	var exceptionsBefore map[string]bool
	isDebug := config.IsDebug

	wc, err = GetWorkspaceControl()
	if err != nil {
		outcome.Err = fmt.Errorf("while getting workspace control in executeRules: %v", err)
		return
	}
	// Loop over all rulesets
	if isDebug {
		log.Printf("jetrules: Looping over rulesets %s", engine.MainRuleFile())
	}
	ruleFileNames = wc.RuleFileNames(engine.MainRuleFile())
	if len(ruleFileNames) == 0 {
		outcome.Err = fmt.Errorf("error: no rulesets found for main rule file name %s", engine.MainRuleFile())
		return
	}
	outcome.Completed = true
	defer func() {
		if outcome.Err != nil || outcome.ExecuteErr != nil {
			outcome.Completed = false
		}
	}()
	for _, ruleset := range ruleFileNames {
		// Create the rete session
		reteSession, err = rdfSession.NewReteSession(ruleset)
		rm = rdfSession.GetResourceManager()
		if err != nil {
			outcome.Err = fmt.Errorf("error: while creating rete session for ruleset %s: %v", ruleset, err)
			return
		}
		// From here every return releases the rete session, as the end of the
		// rule set does (rules_unit_test_harness I-10).
		if !inputAsserted {
			// Assert the input records to rdf session
			err = assertInput(rdfSession)
			if err != nil {
				outcome.Err = fmt.Errorf("while asserting input records to rdf session: %v", err)
				reteSession.Release()
				return
			}
			inputAsserted = true
		}

		// Step 0 of loop is pre loop or no loop
		// Step 1+ for looping
		rdfSession.Erase(jr.Jets__istate, jr.Jets__loop, nil)
		rdfSession.Erase(jr.Jets__istate, jr.Jets__completed, nil)
		maxLooping = 0
		if config.MaxLooping == 0 {
			// get the $max_looping of the workspace
			v, err := GetRuleEngineConfig(ruleset, "$max_looping")
			if err != nil {
				outcome.Err = fmt.Errorf(
					"error: while getting '$max_looping' property from workspace %s: %v",
					ruleset, err)
				reteSession.Release()
				return
			}
			if len(v) > 0 {
				maxLooping, err = strconv.Atoi(v)
				if err != nil {
					outcome.Err = fmt.Errorf(
						"error: invalid '$max_looping' property in workspace %s: %v",
						ruleset, err)
					reteSession.Release()
					return
				}
			}
		} else {
			maxLooping = config.MaxLooping
		}
		if maxLooping > 0 {
			log.Printf("jetrules: looping in use, max number of loops is %d", maxLooping)
		}
		rs := RuleSetOutcome{RuleSet: ruleset, MaxLooping: maxLooping}
		// do for iloop <= maxloop (since looping start at one!)
		for iloop = 0; iloop <= maxLooping; iloop++ {
			rs.Loops = iloop
			if iloop > 0 {
				rdfSession.Insert(jr.Jets__istate, jr.Jets__loop, rm.NewIntLiteral(iloop))
			}
			err2 := reteSession.ExecuteRules()
			if err2 != nil {
				rs.ExecuteErr = err2
				break
			}
			// Check if looping is completed (Jets__completed)
			if rdfSession.ContainsSP(jr.Jets__istate, jr.Jets__completed) {
				log.Printf("jetrules: Rete Session Looping Completed with iloop %d", iloop)
				rs.Completed = true
				break
			}
		}
		// The loop ran out only when it went past the last allowed iteration;
		// a break on completion or on an error leaves iloop at that iteration.
		if maxLooping > 0 && iloop > maxLooping {
			// Looped til the end, something might be wrong
			rs.MaxLoopReached = true
		}
		// Check for any jets:exceptions in the rdfSession, and keep the ones this
		// rule set raised. The exceptions stay in the session: a later rule set's
		// rules may read them, and its scan finds them again without reporting them.
		exceptionsNow := make(map[string]bool)
		ctor := rdfSession.FindSP(jr.Jets__istate, jr.Jets__exception)
		for !ctor.IsEnd() {
			hasException := ctor.GetObject()
			if hasException != nil {
				key := exceptionKey(hasException)
				exceptionsNow[key] = true
				if !exceptionsBefore[key] {
					rs.Exceptions = append(rs.Exceptions, hasException.String())
				}
			}
			ctor.Next()
		}
		ctor.Release()
		exceptionsBefore = exceptionsNow

		outcome.RuleSets = append(outcome.RuleSets, rs)
		outcome.Loops = rs.Loops
		if maxLooping > 0 && !rs.Completed {
			outcome.Completed = false
		}
		outcome.MaxLoopReached = outcome.MaxLoopReached || rs.MaxLoopReached
		if outcome.ExecuteErr == nil {
			outcome.ExecuteErr = rs.ExecuteErr
		}
		outcome.Exceptions = append(outcome.Exceptions, rs.Exceptions...)
		if config.AfterRuleSet != nil {
			config.AfterRuleSet(rdfSession, &outcome.RuleSets[len(outcome.RuleSets)-1])
		}
		reteSession.Release()
	}
	return
}

// exceptionKey identifies the object of a (jets:iState jets:exception ?) triple
// across the scans of a rule sequence: its rdf type and its text.
//
// The triple is the identity that matters, and with its subject and predicate
// fixed it is the object node; the rdf session is a set of triples, so one
// exception value is one triple however many rules or rule sets assert it. The
// node itself cannot be the key, because neither adaptor exposes it: Hdle()
// returns the RdfNode wrapper, which GetObject allocates afresh on every call
// (rules_unit_test_harness F93). The type and the text name the node instead,
// the type keeping the text "1" apart from the int 1 and a resource from a
// literal of the same name.
//
// Two raises of one value by two rule sets are therefore one exception, reported
// by the first; a value that a later rule set retracted and another raised again
// is absent from the scan between them and is reported again.
func exceptionKey(n RdfNode) string {
	return n.Type() + "\x00" + n.String()
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
	// log.Printf("*** keepObject? subject: %s, sourcePeriod: %d, keepObj: %v", subject, sourcePeriod, keepObj)
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
				// A text property keeps its values as an array, which is written as an
				// array literal; any other type is set to null. Both are reported, and
				// count toward max_error_count (a text property was reported by nothing
				// until 2026-10-13).
				if ctx.column2RdfType[pname] == "text" {
					data = dataArr
					ctx.reportMultipleValues(rdfSession, subject, pname,
						fmt.Sprintf("property %s is not multi-value but has %d text values for subject %s, keeping all of them as an array",
							pname, len(dataArr), subject))
				} else {
					data = nil
					ctx.reportMultipleValues(rdfSession, subject, pname,
						fmt.Sprintf("property %s is not multi-value but has multiple values for subject %s, setting value to null",
							pname, subject))
				}
			}
		}
	}
	return data
}

// reportMultipleValues reports a property that is not multi-value and has
// several values on subject: to the log, and to the error channel up to
// max_error_count, with the subject's key in row_jets_key and the property in
// input_column.
func (ctx *JrPoolWorker) reportMultipleValues(rdfSession JetRdfSession, subject RdfNode, pname, msg string) {
	ctx.errorCount++
	switch {
	case ctx.errorCount <= ctx.config.MaxErrorCount:
		log.Printf("warning: %s", msg)
		if ctx.errorOutputCh != nil {
			peRow := ctx.newProcessError()
			peRow.RowJetsKey = sql.NullString{String: subjectJetsKey(rdfSession, subject), Valid: true}
			peRow.InputColumn = sql.NullString{String: pname, Valid: true}
			peRow.ErrorMessage = msg
			peRow.write2Chan(ctx.errorOutputCh, ctx.done)
		}
	case ctx.errorCount == ctx.config.MaxErrorCount+1:
		log.Printf("jetrules: reached max_error_count (%d), stop reporting errors", ctx.config.MaxErrorCount)
	default:
		if ctx.config.IsDebug {
			log.Printf("warning: %s", msg)
		}
	}
}

func (ctx *JrPoolWorker) extractSessionData(rdfSession JetRdfSession,
	outChannel *JetrulesOutputChan, config *ChannelSpec) error {

	jr := rdfSession.JetResources()
	rm := rdfSession.GetResourceManager()
	entityCount := 0
	columns := config.Columns
	var data any
	var keepObj bool
	var err error
	isDebug := ctx.config.IsDebug
	currentSourcePeriod := ctx.config.CurrentSourcePeriod

	// Extract entity by rdf type
	if isDebug {
		log.Printf("*** Pool Worker == Extracting entities of class %s", outChannel.ClassName)
	}
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
			// log.Printf("*** Extracting entity for subject %s", subject)
			entityRow := make([]any, len(columns))
			// log.Printf("*** Extracting DEFAULT encoding for subject %s", subject)
			for i, p := range columns {
				specialEncoding := outChannel.JrColumnEncodings[p]
				if specialEncoding != nil {
					// Special encoding for this column, extract the entire object as a json/toon/map[string]any
					entityRow[i] = specialEncoding.EncodeColumnData(rdfSession, subject)
				} else {
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
			// log.Printf("*** Extracted ENTITY_ROW: %v", entityRow)
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
	// log.Printf("*** jetrules: Extracted %d entities for class %s", entityCount, outChannel.ClassName)
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
	jetsKey = inputRowJetsKey(config, *row)

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

// AssertTextRow asserts one entity into rdfSession from a row of text cells,
// as a CSV export of an entity table holds it: columns names each cell's
// property, in any order, and each cell is cast by that property's type and
// array flag in the workspace's build/properties.json (GetWorkspaceDataProperties),
// the cast a pipeline applies to text input (castToRdfTypeFromTxt). The entity is
// then asserted exactly as a pipeline asserts an input row (assertInputRow).
//
// The three columns assertInputRow reads by position are found by name:
//   - jets:key is optional; absent or blank, the key is a hash of the row;
//   - rdf:type is cast as an array, so `hc:Eligibility` and `{"a","b"}` both
//     work; absent or blank, config.InputRdfType is the type;
//   - jets:source_period_sequence is optional and is usually absent from an
//     export; absent or blank, the entity is of config.CurrentSourcePeriod and
//     is marked jets:InputRecord, as a pipeline's input is.
//
// A blank cell asserts nothing. A column that is not a data property of the
// workspace is an error naming it, and so is a cell that does not cast. The
// rules unit-test harness calls this once per CSV row, through the assertInput
// argument of RunRuleSets; RunRuleSets' precondition on JetResources applies.
//
// The data properties are read through GetWorkspaceDataProperties, which loads
// them once per process and returns the cached map after that, so calling it
// per row costs a lock and a lookup.
func AssertTextRow(config *JetrulesSpec, rdfSession JetRdfSession, columns []string, cells []string) error {
	if len(cells) != len(columns) {
		return fmt.Errorf("error: row has %d cells for %d columns", len(cells), len(columns))
	}
	dpMap, err := GetWorkspaceDataProperties()
	if err != nil {
		return err
	}
	// assertInputRow's layout: jets:key, rdf:type, jets:source_period_sequence,
	// then the other columns in the order given.
	layout := make([]string, 3, len(columns)+3)
	layout[0], layout[1], layout[2] = "jets:key", "rdf:type", "jets:source_period_sequence"
	row := make([]any, 3, len(columns)+3)
	for i, column := range columns {
		dp := dpMap[column]
		if dp == nil {
			return fmt.Errorf("error: column '%s' is not a data property of the workspace", column)
		}
		isArray := dp.AsArray
		if column == "rdf:type" {
			isArray = true
		}
		value, err := castToRdfTypeFromTxt(cells[i], dp.Type, &isArray)
		if err != nil {
			return fmt.Errorf("error: column '%s' value '%s' is not a %s: %v", column, cells[i], dp.Type, err)
		}
		switch column {
		case "jets:key":
			row[0] = value
		case "rdf:type":
			row[1] = value
		case "jets:source_period_sequence":
			row[2] = value
		default:
			layout = append(layout, column)
			row = append(row, value)
		}
	}
	// assertInputRow looks the columns up by the entity's first rdf:type.
	assertType := config.InputRdfType
	if types, ok := row[1].([]any); ok && len(types) > 0 {
		if assertType, ok = types[0].(string); !ok {
			return fmt.Errorf("error: rdf:type value %v (%T) is not a resource name", types[0], types[0])
		}
	} else {
		row[1] = nil
	}
	return assertInputRow(config, rdfSession, &row, map[string][]string{assertType: layout})
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
