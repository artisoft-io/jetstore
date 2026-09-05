package compute_pipes

// The shared inference plumbing (item 15a, plan §8): everything of an inference
// operator that is not backend-specific. The operator shell (Apply/Done/
// Finally), the worker pool and its counters, the record processing with its
// on_error policy, the prompt template compile/render (including the
// build-time {{col}} check), the response mapping compile/apply, the retry
// with backoff, the cost guard, the channel validation and the call context
// all live here and never name a backend. What a backend supplies is the
// inferBackend seam below plus the labels used in log and error messages, so
// the messages stay byte-identical to what each operator emitted before the
// extraction.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/artisoft-io/jetstore/jets/utils"
)

// Default settings of InferCommonSpec; the associated configuration
// properties are described on the backend spec that embeds it (pipes_model.go).
const (
	inferDefaultRequestTimeoutSec = 120
	inferDefaultConnectTimeoutSec = 10
	inferDefaultMaxRetry          = 2
	inferDefaultRetryWaitSec      = 2
	inferDefaultMaxErrorCount     = 50
	// Cap on the number of column names listed when reporting an unknown placeholder.
	inferMaxColumnsInErrMsg = 40
)

// output_mapping source values
const (
	inferSourceResponse    = "response"
	inferSourceRawResponse = "raw_response"
	inferSourceEnvelope    = "envelope"
	inferSourceThinking    = "thinking"
	inferSourceModelName   = "model_name"
)

// inferBackend is the seam between the shared plumbing and one inference
// backend: build the request payload for a rendered prompt, and perform one
// call attempt. The retry policy lives in the shared worker, so CallOnce
// reports whether its failure is worth retrying.
type inferBackend interface {
	BuildRequest(prompt string) ([]byte, error)
	CallOnce(ctx context.Context, payload []byte) (body []byte, resp inferResponse, retryable bool, err error)
}

// inferResponse is what the shared plumbing needs from a backend's response
// envelope; the envelope's own field names stay with the backend (and with the
// configuration's `envelope` mapping paths, which walk the raw body).
type inferResponse interface {
	// Text returns the model's answer and, when in use, its reasoning text.
	Text() (text, thinking string)
	// Tokens returns the prompt and eval token counts for the run counters.
	Tokens() (prompt, eval int)
	// ModelName returns the model that answered, for the model_name mapping source.
	ModelName() string
}

// inferLabels carries the operator-specific wording of the shared plumbing's
// log and error messages, so extraction changes no emitted text.
type inferLabels struct {
	Pipe       string // the operator pipe name, used in log lines
	Operator   string // the operator name, used in error messages
	Type       string // the operator type as the configuration spells it, recorded on process_errors
	ConfigName string // the config element name, used in configuration errors
	ErrPrefix  string // operator + model, prefixes row-level failure reports
	Summary    string // the completion-summary prefix logged by Finally
}

type inferTransformationPipe struct {
	cpConfig        *ComputePipesConfig
	source          *InputChannel
	outputCh        *OutputChannel
	errorOutputCh   *OutputChannel
	channelRegistry *ChannelRegistry
	spec            *TransformationSpec
	common          *InferCommonSpec
	poolManager     *inferPoolManager
	cancelCalls     context.CancelFunc
	labels          inferLabels
	doneCh          chan struct{}
}

// Implementing interface PipeTransformationEvaluator
// Hand the record over to the worker pool; the workers own the call to the infer server
// and the write to the output channel.
func (ctx *inferTransformationPipe) Apply(input *[]any) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in %s", ctx.labels.Pipe)
	}
	select {
	case ctx.poolManager.workersTaskCh <- *input:
	case <-ctx.doneCh:
		log.Printf("%s interrupted", ctx.labels.Pipe)
	}
	return nil
}

func (ctx *inferTransformationPipe) Done() error {
	return nil
}

func (ctx *inferTransformationPipe) Finally() {
	if ctx.poolManager == nil {
		return
	}
	close(ctx.poolManager.workersTaskCh)
	// Wait for the workers before letting the caller close the output channel: the pool
	// writes to it asynchronously, and StartFanOutPipe closes the output channels right
	// after Finally returns.
	ctx.poolManager.workersWg.Wait()
	// Release the goroutine watching doneCh (see newInferCallContext).
	ctx.cancelCalls()
	// Note - closing the error channel is moved with closing all the output channels in
	// the pipe_executor_fan_out.go and pipe_executor_fsplitter.go
	log.Println(ctx.poolManager.summary(ctx.labels.Summary))
}

// inferPoolManager owns the worker pool and the run counters.
// workersTaskCh is closed by inferTransformationPipe.Finally.
type inferPoolManager struct {
	workersTaskCh chan []any
	workersWg     *sync.WaitGroup
	// Counters, shared by all the workers.
	dispatchCount atomic.Int64 // records received
	callCount     atomic.Int64 // records sent to the model
	errorCount    atomic.Int64 // records that failed
	retryCount    atomic.Int64
	latencyMs     atomic.Int64
	// The provenance counters, kept apart from errorCount because they count a
	// different population: a briefing that was answered and is ungrounded is
	// not a record that failed. provenanceErrorCount is records reported and
	// bounds itself against max_error_count; provenanceFindingCount is the
	// findings inside them and is unbounded, since it is the run's own answer to
	// "what is the refusal rate" and capping it would make the number a
	// property of the cap.
	provenanceErrorCount   atomic.Int64
	provenanceFindingCount atomic.Int64
	// hasProvenance is set at build when a provenance schema resolved, so the
	// summary can report a clean run as 0 rather than as silence.
	hasProvenance bool
	// unavailableUntil is the unix nano before which the infer server is treated
	// as down without asking it again. See `serverDown` and `call`.
	unavailableUntil atomic.Int64
	promptTokens     atomic.Int64
	evalTokens       atomic.Int64
	// interruptOnce guards the close of the done channel, since every worker may hit an
	// error concurrently.
	interruptOnce sync.Once
}

// The circuit breaker, and why it is not a latch.
//
// **Remembering costs nothing and forgetting is what makes it safe.** With
// `on_error` explicitly `pass_through` or `drop`, a stopped server does not fail
// the run — every record then pays its full retry sequence and backoff before
// being passed through, which is the same fifteen-minute timeout as before,
// merely opted into. One record's exhausted retries are already proof enough
// about the server, so the rest skip the call.
//
// **It reopens on a timer rather than latching for the run.** A latch is simpler
// and wrong for the case that actually happens here: the infer server is an ECS
// service on a single GPU instance, so a deploy stops and restarts it, and a
// pipeline running across that window would otherwise fail every record after
// the outage — including the ones the server came back in time to answer. The
// cooldown bounds the hammering without deciding the server is gone forever.
//
// No explicit close: a probe that succeeds simply does not extend the window.
const inferUnavailableCooldown = 30 * time.Second

// serverDown reports whether the infer server was found unavailable recently
// enough that there is no point asking again.
func (pm *inferPoolManager) serverDown() bool {
	until := pm.unavailableUntil.Load()
	return until > 0 && time.Now().UnixNano() < until
}

// noteServerDown opens the window. Monotonic under concurrency: several workers
// may exhaust their retries at once and the latest wins, which is the one that
// learnt most recently.
func (pm *inferPoolManager) noteServerDown() {
	deadline := time.Now().Add(inferUnavailableCooldown).UnixNano()
	for {
		current := pm.unavailableUntil.Load()
		if current >= deadline || pm.unavailableUntil.CompareAndSwap(current, deadline) {
			return
		}
	}
}

func (pm *inferPoolManager) summary(prefix string) string {
	calls := pm.callCount.Load()
	var avg int64
	if calls > 0 {
		avg = pm.latencyMs.Load() / calls
	}
	line := fmt.Sprintf(
		"%s, %d records, %d calls, %d errors, %d retries, "+
			"avg latency %dms, prompt tokens %d, eval tokens %d",
		prefix, pm.dispatchCount.Load(), calls, pm.errorCount.Load(), pm.retryCount.Load(),
		avg, pm.promptTokens.Load(), pm.evalTokens.Load())
	// Appended only when a provenance schema is configured, so every summary
	// that existed before this stays byte-identical. The two numbers are the
	// only instrument anybody has for the rate R-60 says nobody has: how often a
	// model's answer asserts something its input entity does not support.
	// A run with no findings reports the zero rather than saying nothing: a
	// clean guardrail and an absent one are otherwise the same silence.
	if pm.hasProvenance {
		line += fmt.Sprintf(", %d records with provenance findings, %d findings",
			pm.provenanceErrorCount.Load(), pm.provenanceFindingCount.Load())
	}
	return line
}

// interrupt stops the whole pipeline, used when on_error is fail.
func (pm *inferPoolManager) interrupt(errCh chan error, doneCh chan struct{}, err error) {
	log.Println(err)
	pm.interruptOnce.Do(func() {
		select {
		case errCh <- err:
		default:
			// ErrCh is buffered; dropping the message here still leaves the pipeline
			// interrupted by the close below.
		}
		select {
		case <-doneCh:
		default:
			close(doneCh)
		}
	})
}

// inferWorker processes records off the pool's task channel.
// Each worker has its own column evaluators: those carry state and are not safe to share
// across goroutines.
type inferWorker struct {
	common           *InferCommonSpec
	backend          inferBackend
	source           *InputChannel
	outputCh         *OutputChannel
	errorOutputCh    *OutputChannel
	template         *inferPromptTemplate
	mappings         []*inferCompiledMapping
	needParsedJson   bool
	needEnvelope     bool
	columnEvaluators []TransformationColumnEvaluator
	provenance       *inferProvenanceCheck // nil when no provenance_schema_name is set
	rowKeyPos        int                   // -1 when row_key_column is not configured
	nbrColumns       int
	maxRetry         int
	retryWait        time.Duration
	labels           inferLabels
	pm               *inferPoolManager
	builderContext   *BuilderContext
	callCtx          context.Context
	doneCh           chan struct{}
	errCh            chan error
}

func (w *inferWorker) doWork() {
	for record := range w.pm.workersTaskCh {
		w.processRecord(&record)
	}
}

// processRecord runs one record through the model and sends it to the output channel.
// Errors are handled here (reported and applied to on_error), never returned: one bad
// record must not take the pipeline down unless on_error says so.
func (w *inferWorker) processRecord(record *[]any) {
	n := w.pm.dispatchCount.Add(1)

	// Cost guard: past max_input_count the records are passed through un-inferred rather
	// than filtered out, this operator augments rather than selects.
	if w.common.MaxInputCount > 0 && n > int64(w.common.MaxInputCount) {
		w.sendRecord(record)
		return
	}

	prompt, err := w.template.render(record, w.source.Config.Columns)
	if err != nil {
		w.failedRecord(record, "", fmt.Errorf("while rendering the prompt template: %v", err))
		return
	}
	payload, err := w.backend.BuildRequest(prompt)
	if err != nil {
		w.failedRecord(record, "", fmt.Errorf("while building the infer server request: %v", err))
		return
	}
	if w.common.IsDebug {
		log.Printf("%s prompt: %s", w.labels.Pipe, prompt)
	}

	start := time.Now()
	body, resp, err := w.call(w.callCtx, payload)
	w.pm.callCount.Add(1)
	w.pm.latencyMs.Add(time.Since(start).Milliseconds())
	if err != nil {
		w.failedRecord(record, "", err)
		return
	}
	promptTokens, evalTokens := resp.Tokens()
	w.pm.promptTokens.Add(int64(promptTokens))
	w.pm.evalTokens.Add(int64(evalTokens))
	if w.common.IsDebug {
		log.Printf("%s response (%dms, %d eval tokens): %s",
			w.labels.Pipe, time.Since(start).Milliseconds(), evalTokens, string(body))
	}

	// The failing column, when there is one, is reported as input_column on the error row.
	if column, err := w.applyMappings(record, body, resp); err != nil {
		w.failedRecord(record, column, err)
		return
	}

	// Per-field provenance, when a provenance_schema_name is configured. It runs
	// **after** the mappings and not before, because the check is about the
	// briefing that is delivered: a record whose required mapping was missing has
	// already been reported and is drop/fail/pass_through by on_error, and a
	// provenance report about it would be a finding on something no reader sees.
	// It never changes the record and never changes its fate - see checkProvenance.
	w.checkProvenance(record, resp)

	// Apply the column transformations, if any, so the model output can be post-processed
	// with the standard column evaluators.
	for i := range w.columnEvaluators {
		if err = w.columnEvaluators[i].Update(record, record); err != nil {
			w.failedRecord(record, "", fmt.Errorf("while applying the column transformation: %v", err))
			return
		}
	}
	w.sendRecord(record)
}

func (w *inferWorker) sendRecord(record *[]any) {
	select {
	case w.outputCh.Channel <- *record:
	case <-w.doneCh:
		log.Printf("%s writing to '%s' interrupted", w.labels.Pipe, w.outputCh.Name)
	}
}

// failedRecord reports a row-level failure and applies the on_error policy.
func (w *inferWorker) failedRecord(record *[]any, column string, err error) {
	// **The server being down overrules the *default* `on_error`, and never an
	// explicit one.** Every policy this switch offers is about one record — drop
	// it, fail on it, pass it through — and none is an answer to "there is no
	// server". Pass-through is the worst of the three here and it is also the
	// default, so with the server stopped a pipeline that never mentioned
	// `on_error` completed having silently skipped the inference it exists to
	// perform, after paying every record's retries.
	//
	// **But a default that is not written down should not get to decide
	// either way**, which is why this checks `onErrorDefaulted` rather than the
	// value: an author who wrote `on_error: pass_through` has said what they want
	// and gets it. The message names the setting, so the operator who did not
	// write one learns what to write from the failure rather than from the
	// documentation for a field they did not know existed.
	if isServerUnavailable(err) && w.common.onErrorDefaulted {
		w.pm.interrupt(w.errCh, w.doneCh, fmt.Errorf(
			"%s: Infer server is not available (is it running?). To let the pipeline "+
				"continue without inference instead, set \"on_error\": \"%s\" on this operator. Cause: %v",
			w.labels.ErrPrefix, OnErrorPassThrough, err))
		return
	}
	nbrErrors := w.pm.errorCount.Add(1)
	err = fmt.Errorf("%s: %v", w.labels.ErrPrefix, err)
	maxErrors := int64(w.common.MaxErrorCount)
	switch {
	case nbrErrors <= maxErrors:
		log.Println(err)
		if w.errorOutputCh != nil {
			peRow := w.builderContext.NewProcessError(w.labels.Type)
			peRow.ErrorMessage = err.Error()
			if len(column) > 0 {
				peRow.InputColumn = sql.NullString{String: column, Valid: true}
			}
			if w.rowKeyPos >= 0 && w.rowKeyPos < len(*record) {
				peRow.RowJetsKey = sql.NullString{
					String: inferToString((*record)[w.rowKeyPos]), Valid: true}
			}
			peRow.write2Chan(w.errorOutputCh, w.doneCh)
		}
	case nbrErrors == maxErrors+1:
		log.Printf("%s: reached max_error_count (%d), stop reporting errors", w.labels.Operator, maxErrors)
	}

	switch w.common.OnError {
	case OnErrorDrop:
	case OnErrorFail:
		w.pm.interrupt(w.errCh, w.doneCh, err)
	default: // OnErrorPassThrough
		w.sendRecord(record)
	}
}

// inferServerUnavailable marks a failure that is about the **server** rather
// than about the record being processed.
//
// **The distinction is what stops a stopped infer server from costing fifteen
// minutes.** `on_error` defaults to pass-through, so a record whose call fails
// is logged and forwarded — correct for a record the model cannot handle, and
// exactly wrong when the server is down: every record then pays its attempts and
// their backoff before being passed through unchanged, and the lambda runs to
// its timeout while the pipeline reports success, having skipped the inference
// it exists to perform. Reported 2026-08-31 against a stopped server, where the
// operator saw `after 3 attempts: error: the infer server returned 503 Service
// Temporarily Unavailable` repeated until the 15-minute wall.
//
// Two failures carry it: a connection error, and a 5xx. **A 429 does not** — that
// is the server asking to be asked more slowly, which is what the retry is for,
// and a throttled pipeline should not die.
type inferServerUnavailable struct{ err error }

func (e *inferServerUnavailable) Error() string { return e.err.Error() }
func (e *inferServerUnavailable) Unwrap() error { return e.err }

// unavailable wraps err when the status says the server, not the request, is the
// problem. The backends call it so the classification lives in one place.
func unavailable(err error) error { return &inferServerUnavailable{err: err} }

// isServerUnavailable reports whether err is a server-level failure.
func isServerUnavailable(err error) bool {
	var u *inferServerUnavailable
	return errors.As(err, &u)
}

// call posts the payload to the infer server, retrying the failures that are worth
// retrying: timeouts, connection errors, 429 and 5xx. A 4xx is a request the server will
// keep refusing, so it fails the record immediately.
func (w *inferWorker) call(ctx context.Context, payload []byte) ([]byte, inferResponse, error) {
	// **Ask the breaker before the server.** A record arriving inside the
	// cooldown fails immediately, with no attempt and no backoff — which is the
	// whole saving, since the alternative is every record repeating the
	// discovery that the server is down.
	if w.pm.serverDown() {
		return nil, nil, unavailable(fmt.Errorf(
			"the infer server was unavailable within the last %s, not retried for this record",
			inferUnavailableCooldown))
	}
	var lastErr error
	wait := w.retryWait
	for attempt := 0; attempt <= w.maxRetry; attempt++ {
		if attempt > 0 {
			w.pm.retryCount.Add(1)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, nil, fmt.Errorf("interrupted while waiting to retry the infer server: %v", lastErr)
			}
			wait *= 2
		}
		body, resp, retryable, err := w.backend.CallOnce(ctx, payload)
		if err == nil {
			return body, resp, nil
		}
		lastErr = err
		if !retryable || ctx.Err() != nil {
			return nil, nil, err
		}
	}
	// **Retry exhaustion against an unavailable server is not a record-level
	// failure**, and saying so here is what lets `failedRecord` stop the pipeline
	// rather than pass the record through. The retry policy has already judged
	// these errors transient; still failing on the last attempt means the server
	// is not there.
	err := fmt.Errorf("after %d attempts: %v", w.maxRetry+1, lastErr)
	if isServerUnavailable(lastErr) {
		// One record's exhausted retries are proof enough for the rest.
		w.pm.noteServerDown()
		return nil, nil, unavailable(err)
	}
	return nil, nil, err
}

// ---------------------------------------------------------------------------
// Prompt template
// ---------------------------------------------------------------------------

// inferPromptTemplate is a prompt template compiled against the input channel columns.
// Rendering a record is then a walk over the segments, with no lookup or parsing.
type inferPromptTemplate struct {
	segments []inferPromptSegment
}

// A segment is either a literal or a substitution; colPos identifies which:
// inferSegLiteral for the literal text, inferSegRecord for the whole record as json,
// and any value >= 0 is the position of a column of the input channel.
type inferPromptSegment struct {
	literal string
	colPos  int
}

const (
	inferSegLiteral = -1
	inferSegRecord  = -2
	// Reserved placeholder expanding to the whole record as a json object.
	inferRecordPlaceholder = "@record"
)

// compileInferPromptTemplate compiles the {{column_name}} placeholders of the template
// against the columns of the input channel.
// Note the env var placeholders ($VAR, ${VAR}) are expected to have been substituted by
// the caller already, they do not vary from one record to the next.
func compileInferPromptTemplate(template string, columns map[string]int) (*inferPromptTemplate, error) {
	if len(strings.TrimSpace(template)) == 0 {
		return nil, fmt.Errorf("error: the prompt template is empty")
	}
	result := &inferPromptTemplate{segments: make([]inferPromptSegment, 0)}
	addLiteral := func(s string) {
		if len(s) > 0 {
			result.segments = append(result.segments,
				inferPromptSegment{literal: s, colPos: inferSegLiteral})
		}
	}
	rest := template
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			addLiteral(rest)
			break
		}
		addLiteral(rest[:start])
		end := strings.Index(rest[start:], "}}")
		if end < 0 {
			return nil, fmt.Errorf(
				"error: unterminated placeholder in the prompt template, missing '}}' after position %d", start)
		}
		name := strings.TrimSpace(rest[start+2 : start+end])
		rest = rest[start+end+2:]
		switch {
		case name == inferRecordPlaceholder:
			result.segments = append(result.segments, inferPromptSegment{colPos: inferSegRecord})
		default:
			pos, ok := columns[name]
			if !ok {
				return nil, fmt.Errorf(
					"error: the prompt template refers to '{{%s}}' which is not a column of the input channel, "+
						"available columns are: %s", name, inferColumnNames(columns))
			}
			result.segments = append(result.segments, inferPromptSegment{colPos: pos})
		}
	}
	return result, nil
}

func (t *inferPromptTemplate) render(record *[]any, columns []string) (string, error) {
	var buf strings.Builder
	for i := range t.segments {
		segment := &t.segments[i]
		switch segment.colPos {
		case inferSegLiteral:
			buf.WriteString(segment.literal)
		case inferSegRecord:
			data, err := json.Marshal(inferZipRecord(columns, record))
			if err != nil {
				return "", fmt.Errorf("while encoding the record as json for {{@record}}: %v", err)
			}
			buf.Write(data)
		default:
			// Short records are legitimate here (see pad_short_rows_with_nulls), a missing
			// value renders as empty rather than failing the record.
			if segment.colPos < len(*record) {
				buf.WriteString(inferToString((*record)[segment.colPos]))
			}
		}
	}
	return buf.String(), nil
}

// resolveInferTemplate returns the prompt template, taken from the operator config or
// from the named template of the cpipes config, and applies the named template's defaults.
func resolveInferTemplate(cpConfig *ComputePipesConfig, common *InferCommonSpec, configName string) (string, error) {
	hasInline := len(common.PromptTemplate) > 0
	hasNamed := len(common.PromptTemplateName) > 0
	switch {
	case hasInline && hasNamed:
		return "", fmt.Errorf(
			"error: %s has both prompt_template and prompt_template_name, specify only one", configName)
	case hasInline:
		return common.PromptTemplate, nil
	case hasNamed:
		if cpConfig != nil {
			for i := range cpConfig.PromptTemplates {
				promptTemplate := &cpConfig.PromptTemplates[i]
				if promptTemplate.Key == common.PromptTemplateName {
					// The operator's own settings win over the template's defaults.
					if len(common.SystemPrompt) == 0 {
						common.SystemPrompt = promptTemplate.SystemPrompt
					}
					if len(common.ResponseFormat) == 0 {
						common.ResponseFormat = promptTemplate.ResponseFormat
					}
					return promptTemplate.Template, nil
				}
			}
		}
		return "", fmt.Errorf(
			"error: %s refers to prompt_template_name '%s' which is not defined in prompt_templates",
			configName, common.PromptTemplateName)
	default:
		return "", fmt.Errorf(
			"error: %s must specify either prompt_template or prompt_template_name", configName)
	}
}

// ---------------------------------------------------------------------------
// Response mapping
// ---------------------------------------------------------------------------

// inferCompiledMapping is an InferMappingSpec resolved against the output channel.
type inferCompiledMapping struct {
	spec   *InferMappingSpec
	colPos int
	path   []string
}

func compileInferMappings(common *InferCommonSpec, outputCh *OutputChannel, configName string) (
	mappings []*inferCompiledMapping, needParsedJson, needEnvelope bool, err error) {

	if len(common.OutputMapping) == 0 {
		return nil, false, false, fmt.Errorf(
			"error: %s must have at least one output_mapping, the operator would have no effect", configName)
	}
	mappings = make([]*inferCompiledMapping, 0, len(common.OutputMapping))
	for i := range common.OutputMapping {
		spec := &common.OutputMapping[i]
		if len(spec.Column) == 0 {
			return nil, false, false, fmt.Errorf("error: output_mapping[%d] is missing the column name", i)
		}
		colPos, ok := (*outputCh.Columns)[spec.Column]
		if !ok {
			return nil, false, false, fmt.Errorf(
				"error: output_mapping column '%s' is not a column of the channel '%s', available columns are: %s",
				spec.Column, outputCh.Name, inferColumnNames(*outputCh.Columns))
		}
		if len(spec.Source) == 0 {
			spec.Source = inferSourceResponse
		}
		switch spec.Source {
		case inferSourceResponse:
			if len(spec.Path) > 0 {
				needParsedJson = true
			}
		case inferSourceEnvelope:
			needEnvelope = true
		case inferSourceRawResponse, inferSourceThinking, inferSourceModelName:
		default:
			return nil, false, false, fmt.Errorf(
				"error: unknown output_mapping source '%s' for column '%s', expecting one of %s, %s, %s, %s, %s",
				spec.Source, spec.Column, inferSourceResponse, inferSourceRawResponse,
				inferSourceEnvelope, inferSourceThinking, inferSourceModelName)
		}
		var path []string
		if len(spec.Path) > 0 {
			path = strings.Split(spec.Path, ".")
		}
		mappings = append(mappings, &inferCompiledMapping{spec: spec, colPos: colPos, path: path})
	}
	return mappings, needParsedJson, needEnvelope, nil
}

// applyMappings writes the mapped values onto the record, in place.
// On failure it also returns the column being mapped, when the failure is specific to one,
// so the error report can name it.
func (w *inferWorker) applyMappings(record *[]any, body []byte, resp inferResponse) (string, error) {
	// The record must be able to hold every column of the channel: short records are
	// legitimate and an in place assignment past the end would panic.
	if len(*record) < w.nbrColumns {
		grown := make([]any, w.nbrColumns)
		copy(grown, *record)
		*record = grown
	}

	text, thinking := resp.Text()
	text = w.stripFences(text)

	var parsed any
	if w.needParsedJson {
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			return "", fmt.Errorf(
				"while parsing the model response as json (consider setting response_format): %v, response was: %s",
				err, inferTruncate(text, 500))
		}
	}
	var envelope map[string]any
	if w.needEnvelope {
		if err := json.Unmarshal(body, &envelope); err != nil {
			return "", fmt.Errorf("while parsing the infer server response envelope: %v", err)
		}
	}

	for _, mapping := range w.mappings {
		var value any
		var found bool
		switch mapping.spec.Source {
		case inferSourceRawResponse:
			value, found = text, true
		case inferSourceThinking:
			value, found = thinking, len(thinking) > 0
		case inferSourceModelName:
			value, found = resp.ModelName(), true
		case inferSourceEnvelope:
			value, found = inferWalkPath(envelope, mapping.path)
		default: // inferSourceResponse
			if len(mapping.path) == 0 {
				value, found = text, true
			} else {
				value, found = inferWalkPath(parsed, mapping.path)
			}
		}
		if !found || value == nil {
			switch {
			case len(mapping.spec.Default) > 0:
				value = mapping.spec.Default
			case mapping.spec.Required:
				return mapping.spec.Column, fmt.Errorf("error: required output_mapping '%s' (path '%s') is missing from the model response",
					mapping.spec.Column, mapping.spec.Path)
			default:
				// Leave the column as it was: clearing it would destroy the input value
				// when the mapping targets an existing column.
				continue
			}
		}
		// A json object or array cannot be a column value, encode it back to json text.
		switch value.(type) {
		case map[string]any:
			data, err := json.Marshal(value)
			if err != nil {
				return mapping.spec.Column,
					fmt.Errorf("while encoding the value of column '%s' back to json: %v", mapping.spec.Column, err)
			}
			value = string(data)
		case []any:
			if len(mapping.spec.AsRdfType) == 0 {
				data, err := json.Marshal(value)
				if err != nil {
					return mapping.spec.Column, fmt.Errorf("while encoding the value of column '%s' back to json: %v", mapping.spec.Column, err)
				}
				value = string(data)
			}
		}
		if len(mapping.spec.AsRdfType) > 0 {
			casted, err := CastToRdfType(value, mapping.spec.AsRdfType, nil)
			if err != nil {
				return mapping.spec.Column, fmt.Errorf("while casting the value of column '%s' to %s: %v",
					mapping.spec.Column, mapping.spec.AsRdfType, err)
			}
			value = casted
		}
		(*record)[mapping.colPos] = value
	}
	return "", nil
}

// stripFences removes the markdown code fences around the model's answer unless
// the configuration turned that off.
func (w *inferWorker) stripFences(text string) string {
	if w.common.DisableStripCodeFences {
		return text
	}
	return inferStripCodeFences(text)
}

// answerText is the model's answer as the mappings see it.
//
// It exists so the provenance check and applyMappings read the **same** string:
// two readers of one value with nothing comparing them is how a date came to be
// a nested object in the prompt and a scalar in a column (AK.2's F554), and the
// cheapest way not to repeat that is to have one reader.
func (w *inferWorker) answerText(resp inferResponse) string {
	text, _ := resp.Text()
	return w.stripFences(text)
}

// inferWalkPath follows a dot notation path into a parsed json value.
// A path element that is an integer indexes into an array.
func inferWalkPath(value any, path []string) (any, bool) {
	if value == nil {
		return nil, false
	}
	current := value
	for _, element := range path {
		switch vv := current.(type) {
		case map[string]any:
			v, ok := vv[element]
			if !ok {
				return nil, false
			}
			current = v
		case []any:
			pos, err := strconv.Atoi(element)
			if err != nil || pos < 0 || pos >= len(vv) {
				return nil, false
			}
			current = vv[pos]
		default:
			return nil, false
		}
	}
	return current, current != nil
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

// newInferTransformationPipe builds the shared pipe: channel validation, the
// prompt template, the response mappings, the row key, the per-worker column
// evaluators, the error channel, the call context and the worker pool. The
// backend and the labels are the only operator-specific inputs.
func (ctx *BuilderContext) newInferTransformationPipe(source *InputChannel, outputCh *OutputChannel,
	spec *TransformationSpec, common *InferCommonSpec, backend inferBackend,
	labels inferLabels) (*inferTransformationPipe, error) {

	// The record is augmented in place, so the input and output channels must have the
	// same columns - in practice the same channel_spec_name.
	if err := validateInferChannels(source, outputCh, labels.Operator); err != nil {
		return nil, err
	}

	// Resolve the prompt template and substitute the env vars: they are fixed for the
	// life of the node, only the column placeholders vary from one record to the next.
	template, err := resolveInferTemplate(ctx.cpConfig, common, labels.ConfigName)
	if err != nil {
		return nil, err
	}
	promptTemplate, err := compileInferPromptTemplate(utils.ReplaceEnvVars(template, ctx.env), *source.Columns)
	if err != nil {
		return nil, err
	}
	common.SystemPrompt = utils.ReplaceEnvVars(common.SystemPrompt, ctx.env)

	// The provenance check, when one is named. It resolves after the template so
	// that the template's response_format is already on `common` when the schema's
	// copy is reconciled against it, and before the mappings so that a
	// configuration this operator will refuse is refused before anything else is
	// compiled.
	provenance, err := resolveInferProvenanceSchema(common, source, labels.ConfigName)
	if err != nil {
		return nil, err
	}

	mappings, needParsedJson, needEnvelope, err := compileInferMappings(common, outputCh, labels.ConfigName)
	if err != nil {
		return nil, err
	}

	rowKeyPos := -1
	if len(common.RowKeyColumn) > 0 {
		pos, ok := (*source.Columns)[common.RowKeyColumn]
		if !ok {
			return nil, fmt.Errorf(
				"error: %s row_key_column '%s' is not a column of the input channel '%s'",
				labels.ConfigName, common.RowKeyColumn, source.Name)
		}
		rowKeyPos = pos
	}

	// Build one set of column evaluators per worker: they carry state and are not safe to
	// share across goroutines. Building them here rather than in the worker keeps a bad
	// column spec a build time error.
	columnEvaluators := make([][]TransformationColumnEvaluator, common.PoolSize)
	for w := range columnEvaluators {
		columnEvaluators[w] = make([]TransformationColumnEvaluator, len(spec.Columns))
		for i := range spec.Columns {
			columnEvaluators[w][i], err = ctx.BuildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
			if err != nil {
				return nil, fmt.Errorf("while BuildTransformationColumnEvaluator (in %s): %v", labels.Pipe, err)
			}
		}
	}

	// Get the error channel if configured
	var errorOutputCh *OutputChannel
	if common.ErrorChannel != nil {
		if len(common.ErrorChannel.Name) == 0 {
			return nil, fmt.Errorf("error: error_channel name cannot be empty")
		}
		if len(common.ErrorChannel.SpecName) == 0 {
			return nil, fmt.Errorf("error: error_channel spec name cannot be empty")
		}
		errorOutputCh, err = ctx.channelRegistry.GetOutputChannel(common.ErrorChannel.Name)
		if err != nil {
			return nil, err
		}
	}

	// A context cancelled when the pipeline is interrupted, so an in flight call does not
	// hold a record for the whole request timeout when everything else is shutting down.
	callCtx, cancelCalls := newInferCallContext(ctx.done)

	pm := &inferPoolManager{
		workersTaskCh: make(chan []any, 1),
		workersWg:     new(sync.WaitGroup),
		hasProvenance: provenance != nil,
	}
	for w := range common.PoolSize {
		worker := &inferWorker{
			common:           common,
			backend:          backend,
			source:           source,
			outputCh:         outputCh,
			errorOutputCh:    errorOutputCh,
			template:         promptTemplate,
			mappings:         mappings,
			needParsedJson:   needParsedJson,
			needEnvelope:     needEnvelope,
			columnEvaluators: columnEvaluators[w],
			provenance:       provenance,
			rowKeyPos:        rowKeyPos,
			nbrColumns:       len(outputCh.Config.Columns),
			maxRetry:         *common.MaxRetry,
			retryWait:        time.Duration(common.RetryWaitSec) * time.Second,
			labels:           labels,
			pm:               pm,
			builderContext:   ctx,
			callCtx:          callCtx,
			doneCh:           ctx.done,
			errCh:            ctx.errCh,
		}
		pm.workersWg.Add(1)
		go func() {
			defer pm.workersWg.Done()
			worker.doWork()
		}()
	}

	return &inferTransformationPipe{
		cpConfig:        ctx.cpConfig,
		source:          source,
		outputCh:        outputCh,
		errorOutputCh:   errorOutputCh,
		channelRegistry: ctx.channelRegistry,
		spec:            spec,
		common:          common,
		poolManager:     pm,
		cancelCalls:     cancelCalls,
		labels:          labels,
		doneCh:          ctx.done,
	}, nil
}

// applyInferCommonDefaults applies the defaults of the backend-agnostic
// configuration; the backend applies its own on top.
func applyInferCommonDefaults(common *InferCommonSpec) {
	if len(common.OnError) == 0 {
		// **Recorded before it is filled in**, because from here on an unset
		// `on_error` and an explicit `on_error: pass_through` are the same
		// string, and the difference decides whether a stopped infer server may
		// be overruled. See `failedRecord`.
		common.onErrorDefaulted = true
		common.OnError = OnErrorPassThrough
	}
	if common.PoolSize < 1 {
		common.PoolSize = 1
	}
	if common.RequestTimeoutSec < 1 {
		common.RequestTimeoutSec = inferDefaultRequestTimeoutSec
	}
	if common.ConnectTimeoutSec < 1 {
		common.ConnectTimeoutSec = inferDefaultConnectTimeoutSec
	}
	if common.MaxRetry == nil || *common.MaxRetry < 0 {
		// Unset means the default; an explicit 0 disables the retries.
		maxRetry := inferDefaultMaxRetry
		common.MaxRetry = &maxRetry
	}
	if common.RetryWaitSec < 1 {
		common.RetryWaitSec = inferDefaultRetryWaitSec
	}
	if common.MaxErrorCount < 1 {
		common.MaxErrorCount = inferDefaultMaxErrorCount
	}
}

// validateInferOnError rejects an on_error value outside the policy range.
func validateInferOnError(common *InferCommonSpec, configName string) error {
	switch common.OnError {
	case OnErrorPassThrough, OnErrorDrop, OnErrorFail:
		return nil
	default:
		return fmt.Errorf("error: unknown %s on_error '%s', expecting one of %s, %s, %s",
			configName, common.OnError, OnErrorPassThrough, OnErrorDrop, OnErrorFail)
	}
}

// validateInferChannels checks that the input and output channels share the same
// ChannelSpec, which is what makes the in place augmentation legal.
// The channel registry maps every channel name to the same *ChannelSpec instance, so the
// pointers are equal when both channels are configured with the same channel_spec_name;
// otherwise the columns must still match one for one.
func validateInferChannels(source *InputChannel, outputCh *OutputChannel, operatorLabel string) error {
	if source.Config == nil || outputCh.Config == nil {
		return fmt.Errorf("error: %s: input or output channel has no channel spec", operatorLabel)
	}
	if source.Config == outputCh.Config {
		return nil
	}
	inColumns := source.Config.Columns
	outColumns := outputCh.Config.Columns
	sharedSpecMsg := fmt.Sprintf(
		"the %s augments the input record in place, so the input channel '%s' and the output "+
			"channel '%s' must share the same channel spec (same channel_spec_name)", operatorLabel, source.Name, outputCh.Name)
	if len(inColumns) != len(outColumns) {
		return fmt.Errorf("error: %s, they have %d and %d columns respectively",
			sharedSpecMsg, len(inColumns), len(outColumns))
	}
	for i := range inColumns {
		if inColumns[i] != outColumns[i] {
			return fmt.Errorf("error: %s, they differ at column %d: '%s' vs '%s'",
				sharedSpecMsg, i, inColumns[i], outColumns[i])
		}
	}
	return nil
}

// newInferCallContext returns a context cancelled when the pipeline is interrupted.
// The watching goroutine also exits when the returned cancel function is called, which
// inferTransformationPipe.Finally does.
func newInferCallContext(doneCh chan struct{}) (context.Context, context.CancelFunc) {
	callCtx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-doneCh:
			cancel()
		case <-callCtx.Done():
		}
	}()
	return callCtx, cancel
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

// inferStripCodeFences removes the markdown code fences that models put around json
// even when asked not to.
func inferStripCodeFences(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "```") {
		return text
	}
	// Drop the opening fence and its optional language tag.
	trimmed = strings.TrimPrefix(trimmed, "```")
	if pos := strings.IndexByte(trimmed, '\n'); pos >= 0 {
		if len(strings.TrimSpace(trimmed[:pos])) < 20 {
			// It is a language tag (json, JSON, ...) and not the content itself.
			trimmed = trimmed[pos+1:]
		}
	}
	if pos := strings.LastIndex(trimmed, "```"); pos >= 0 {
		trimmed = trimmed[:pos]
	}
	return strings.TrimSpace(trimmed)
}

func inferToString(value any) string {
	switch vv := value.(type) {
	case nil:
		return ""
	case string:
		return vv
	case []byte:
		return string(vv)
	case sql.NullString:
		if vv.Valid {
			return vv.String
		}
		return ""
	default:
		return fmt.Sprintf("%v", vv)
	}
}

// inferZipRecord makes a column name to value map for the {{@record}} placeholder.
// Unlike utils.ZipSlices it tolerates a short record, which is legitimate here.
func inferZipRecord(columns []string, record *[]any) map[string]any {
	result := make(map[string]any, len(columns))
	for i, column := range columns {
		if i < len(*record) {
			result[column] = (*record)[i]
		} else {
			result[column] = nil
		}
	}
	return result
}

func inferColumnNames(columns map[string]int) string {
	names := make([]string, len(columns))
	for name, pos := range columns {
		if pos >= 0 && pos < len(names) {
			names[pos] = name
		}
	}
	if len(names) > inferMaxColumnsInErrMsg {
		return strings.Join(names[:inferMaxColumnsInErrMsg], ", ") +
			fmt.Sprintf(", ... (%d more)", len(names)-inferMaxColumnsInErrMsg)
	}
	return strings.Join(names, ", ")
}

func inferTruncate(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
