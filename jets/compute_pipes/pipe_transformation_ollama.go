package compute_pipes

// Ollama operator: call the infer server (Ollama) once per input record and augment the
// record in place with values extracted from the model response.
//
// This is an augmentation operator: the input record is mutated and forwarded, no new
// record is built. The input and output channels must therefore share the same
// ChannelSpec (same channel_spec_name in the configuration); this is checked when the
// operator is built rather than trusted, since a mismatch would silently put values in
// the wrong columns.
//
// Records are processed by a worker pool, of size one by default. A single worker
// consumes the task channel in order, so the default preserves the record order while a
// larger pool trades that order for concurrent requests to the infer server.
//
// Row-level failures do not stop the pipeline: they are reported to the error channel
// using the process_errors shape (as the jetrules operator does) and the record is then
// passed through, dropped or escalated according to on_error.
//
// See pipe_transformation_ollama_design.md for the design notes.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/artisoft-io/jetstore/jets/utils"
)

// Default settings, see OllamaSpec for the associated configuration properties.
const (
	ollamaDefaultKeepAlive         = "30m"
	ollamaDefaultRequestTimeoutSec = 120
	ollamaDefaultConnectTimeoutSec = 10
	ollamaDefaultMaxRetry          = 2
	ollamaDefaultRetryWaitSec      = 2
	ollamaDefaultMaxErrorCount     = 50
	// Cap on the number of column names listed when reporting an unknown placeholder.
	ollamaMaxColumnsInErrMsg = 40
)

// on_error values
const (
	ollamaOnErrorPassThrough = "pass_through"
	ollamaOnErrorDrop        = "drop"
	ollamaOnErrorFail        = "fail"
)

// output_mapping source values
const (
	ollamaSourceResponse    = "response"
	ollamaSourceRawResponse = "raw_response"
	ollamaSourceEnvelope    = "envelope"
	ollamaSourceThinking    = "thinking"
)

type OllamaTransformationPipe struct {
	cpConfig        *ComputePipesConfig
	source          *InputChannel
	outputCh        *OutputChannel
	errorOutputCh   *OutputChannel
	channelRegistry *ChannelRegistry
	spec            *TransformationSpec
	config          *OllamaSpec
	poolManager     *ollamaPoolManager
	cancelCalls     context.CancelFunc
	doneCh          chan struct{}
}

// Implementing interface PipeTransformationEvaluator
// Hand the record over to the worker pool; the workers own the call to the infer server
// and the write to the output channel.
func (ctx *OllamaTransformationPipe) Apply(input *[]any) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in OllamaTransformationPipe")
	}
	select {
	case ctx.poolManager.workersTaskCh <- *input:
	case <-ctx.doneCh:
		log.Println("OllamaTransformationPipe interrupted")
	}
	return nil
}

func (ctx *OllamaTransformationPipe) Done() error {
	return nil
}

func (ctx *OllamaTransformationPipe) Finally() {
	if ctx.poolManager == nil {
		return
	}
	close(ctx.poolManager.workersTaskCh)
	// Wait for the workers before letting the caller close the output channel: the pool
	// writes to it asynchronously, and StartFanOutPipe closes the output channels right
	// after Finally returns.
	ctx.poolManager.workersWg.Wait()
	// Release the goroutine watching doneCh (see newOllamaCallContext).
	ctx.cancelCalls()
	// Note - closing the error channel is moved with closing all the output channels in 
	// the pipe_executor_fan_out.go and pipe_executor_fsplitter.go
	log.Println(ctx.poolManager.summary(ctx.config.Model))
}

// ollamaPoolManager owns the worker pool and the run counters.
// workersTaskCh is closed by OllamaTransformationPipe.Finally.
type ollamaPoolManager struct {
	workersTaskCh chan []any
	workersWg     *sync.WaitGroup
	// Counters, shared by all the workers.
	dispatchCount atomic.Int64 // records received
	callCount     atomic.Int64 // records sent to the model
	errorCount    atomic.Int64 // records that failed
	retryCount    atomic.Int64
	latencyMs     atomic.Int64
	promptTokens  atomic.Int64
	evalTokens    atomic.Int64
	// interruptOnce guards the close of the done channel, since every worker may hit an
	// error concurrently.
	interruptOnce sync.Once
}

func (pm *ollamaPoolManager) summary(model string) string {
	calls := pm.callCount.Load()
	var avg int64
	if calls > 0 {
		avg = pm.latencyMs.Load() / calls
	}
	return fmt.Sprintf(
		"OllamaTransformationPipe completed: model %s, %d records, %d calls, %d errors, %d retries, "+
			"avg latency %dms, prompt tokens %d, eval tokens %d",
		model, pm.dispatchCount.Load(), calls, pm.errorCount.Load(), pm.retryCount.Load(),
		avg, pm.promptTokens.Load(), pm.evalTokens.Load())
}

// interrupt stops the whole pipeline, used when on_error is fail.
func (pm *ollamaPoolManager) interrupt(errCh chan error, doneCh chan struct{}, err error) {
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

// ollamaWorker processes records off the pool's task channel.
// Each worker has its own column evaluators: those carry state and are not safe to share
// across goroutines.
type ollamaWorker struct {
	config           *OllamaSpec
	source           *InputChannel
	outputCh         *OutputChannel
	errorOutputCh    *OutputChannel
	client           *ollamaClient
	template         *ollamaPromptTemplate
	mappings         []*ollamaCompiledMapping
	needParsedJson   bool
	needEnvelope     bool
	columnEvaluators []TransformationColumnEvaluator
	rowKeyPos        int // -1 when row_key_column is not configured
	nbrColumns       int
	pm               *ollamaPoolManager
	builderContext   *BuilderContext
	callCtx          context.Context
	doneCh           chan struct{}
	errCh            chan error
}

func (w *ollamaWorker) doWork() {
	for record := range w.pm.workersTaskCh {
		w.processRecord(&record)
	}
}

// processRecord runs one record through the model and sends it to the output channel.
// Errors are handled here (reported and applied to on_error), never returned: one bad
// record must not take the pipeline down unless on_error says so.
func (w *ollamaWorker) processRecord(record *[]any) {
	n := w.pm.dispatchCount.Add(1)

	// Cost guard: past max_input_count the records are passed through un-inferred rather
	// than filtered out, this operator augments rather than selects.
	if w.config.MaxInputCount > 0 && n > int64(w.config.MaxInputCount) {
		w.sendRecord(record)
		return
	}

	prompt, err := w.template.render(record, w.source.Config.Columns)
	if err != nil {
		w.failedRecord(record, "", fmt.Errorf("while rendering the prompt template: %v", err))
		return
	}
	payload, err := w.buildRequest(prompt)
	if err != nil {
		w.failedRecord(record, "", fmt.Errorf("while building the infer server request: %v", err))
		return
	}
	if w.config.IsDebug {
		log.Printf("OllamaTransformationPipe prompt: %s", prompt)
	}

	start := time.Now()
	body, resp, err := w.client.call(w.callCtx, payload, w.pm)
	w.pm.callCount.Add(1)
	w.pm.latencyMs.Add(time.Since(start).Milliseconds())
	if err != nil {
		w.failedRecord(record, "", err)
		return
	}
	w.pm.promptTokens.Add(int64(resp.PromptEvalCount))
	w.pm.evalTokens.Add(int64(resp.EvalCount))
	if w.config.IsDebug {
		log.Printf("OllamaTransformationPipe response (%dms, %d eval tokens): %s",
			time.Since(start).Milliseconds(), resp.EvalCount, string(body))
	}

	// The failing column, when there is one, is reported as input_column on the error row.
	if column, err := w.applyMappings(record, body, resp); err != nil {
		w.failedRecord(record, column, err)
		return
	}

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

func (w *ollamaWorker) sendRecord(record *[]any) {
	select {
	case w.outputCh.Channel <- *record:
	case <-w.doneCh:
		log.Printf("OllamaTransformationPipe writing to '%s' interrupted", w.outputCh.Name)
	}
}

// failedRecord reports a row-level failure and applies the on_error policy.
func (w *ollamaWorker) failedRecord(record *[]any, column string, err error) {
	nbrErrors := w.pm.errorCount.Add(1)
	err = fmt.Errorf("ollama operator (model %s): %v", w.config.Model, err)
	maxErrors := int64(w.config.MaxErrorCount)
	switch {
	case nbrErrors <= maxErrors:
		log.Println(err)
		if w.errorOutputCh != nil {
			peRow := w.builderContext.NewProcessError()
			peRow.ErrorMessage = err.Error()
			if len(column) > 0 {
				peRow.InputColumn = sql.NullString{String: column, Valid: true}
			}
			if w.rowKeyPos >= 0 && w.rowKeyPos < len(*record) {
				peRow.RowJetsKey = sql.NullString{
					String: ollamaToString((*record)[w.rowKeyPos]), Valid: true}
			}
			peRow.write2Chan(w.errorOutputCh, w.doneCh)
		}
	case nbrErrors == maxErrors+1:
		log.Printf("ollama operator: reached max_error_count (%d), stop reporting errors", maxErrors)
	}

	switch w.config.OnError {
	case ollamaOnErrorDrop:
	case ollamaOnErrorFail:
		w.pm.interrupt(w.errCh, w.doneCh, err)
	default: // ollamaOnErrorPassThrough
		w.sendRecord(record)
	}
}

// ---------------------------------------------------------------------------
// Prompt template
// ---------------------------------------------------------------------------

// ollamaPromptTemplate is a prompt template compiled against the input channel columns.
// Rendering a record is then a walk over the segments, with no lookup or parsing.
type ollamaPromptTemplate struct {
	segments []ollamaPromptSegment
}

// A segment is either a literal or a substitution; colPos identifies which:
// ollamaSegLiteral for the literal text, ollamaSegRecord for the whole record as json,
// and any value >= 0 is the position of a column of the input channel.
type ollamaPromptSegment struct {
	literal string
	colPos  int
}

const (
	ollamaSegLiteral = -1
	ollamaSegRecord  = -2
	// Reserved placeholder expanding to the whole record as a json object.
	ollamaRecordPlaceholder = "@record"
)

// compileOllamaPromptTemplate compiles the {{column_name}} placeholders of the template
// against the columns of the input channel.
// Note the env var placeholders ($VAR, ${VAR}) are expected to have been substituted by
// the caller already, they do not vary from one record to the next.
func compileOllamaPromptTemplate(template string, columns map[string]int) (*ollamaPromptTemplate, error) {
	if len(strings.TrimSpace(template)) == 0 {
		return nil, fmt.Errorf("error: the prompt template is empty")
	}
	result := &ollamaPromptTemplate{segments: make([]ollamaPromptSegment, 0)}
	addLiteral := func(s string) {
		if len(s) > 0 {
			result.segments = append(result.segments,
				ollamaPromptSegment{literal: s, colPos: ollamaSegLiteral})
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
		case name == ollamaRecordPlaceholder:
			result.segments = append(result.segments, ollamaPromptSegment{colPos: ollamaSegRecord})
		default:
			pos, ok := columns[name]
			if !ok {
				return nil, fmt.Errorf(
					"error: the prompt template refers to '{{%s}}' which is not a column of the input channel, "+
						"available columns are: %s", name, ollamaColumnNames(columns))
			}
			result.segments = append(result.segments, ollamaPromptSegment{colPos: pos})
		}
	}
	return result, nil
}

func (t *ollamaPromptTemplate) render(record *[]any, columns []string) (string, error) {
	var buf strings.Builder
	for i := range t.segments {
		segment := &t.segments[i]
		switch segment.colPos {
		case ollamaSegLiteral:
			buf.WriteString(segment.literal)
		case ollamaSegRecord:
			data, err := json.Marshal(ollamaZipRecord(columns, record))
			if err != nil {
				return "", fmt.Errorf("while encoding the record as json for {{@record}}: %v", err)
			}
			buf.Write(data)
		default:
			// Short records are legitimate here (see pad_short_rows_with_nulls), a missing
			// value renders as empty rather than failing the record.
			if segment.colPos < len(*record) {
				buf.WriteString(ollamaToString((*record)[segment.colPos]))
			}
		}
	}
	return buf.String(), nil
}

// ---------------------------------------------------------------------------
// Response mapping
// ---------------------------------------------------------------------------

// ollamaCompiledMapping is an OllamaMappingSpec resolved against the output channel.
type ollamaCompiledMapping struct {
	spec   *OllamaMappingSpec
	colPos int
	path   []string
}

func compileOllamaMappings(config *OllamaSpec, outputCh *OutputChannel) (
	mappings []*ollamaCompiledMapping, needParsedJson, needEnvelope bool, err error) {

	if len(config.OutputMapping) == 0 {
		return nil, false, false, fmt.Errorf(
			"error: ollama_config must have at least one output_mapping, the operator would have no effect")
	}
	mappings = make([]*ollamaCompiledMapping, 0, len(config.OutputMapping))
	for i := range config.OutputMapping {
		spec := &config.OutputMapping[i]
		if len(spec.Column) == 0 {
			return nil, false, false, fmt.Errorf("error: output_mapping[%d] is missing the column name", i)
		}
		colPos, ok := (*outputCh.Columns)[spec.Column]
		if !ok {
			return nil, false, false, fmt.Errorf(
				"error: output_mapping column '%s' is not a column of the channel '%s', available columns are: %s",
				spec.Column, outputCh.Name, ollamaColumnNames(*outputCh.Columns))
		}
		if len(spec.Source) == 0 {
			spec.Source = ollamaSourceResponse
		}
		switch spec.Source {
		case ollamaSourceResponse:
			if len(spec.Path) > 0 {
				needParsedJson = true
			}
		case ollamaSourceEnvelope:
			needEnvelope = true
		case ollamaSourceRawResponse, ollamaSourceThinking:
		default:
			return nil, false, false, fmt.Errorf(
				"error: unknown output_mapping source '%s' for column '%s', expecting one of %s, %s, %s, %s",
				spec.Source, spec.Column, ollamaSourceResponse, ollamaSourceRawResponse,
				ollamaSourceEnvelope, ollamaSourceThinking)
		}
		var path []string
		if len(spec.Path) > 0 {
			path = strings.Split(spec.Path, ".")
		}
		mappings = append(mappings, &ollamaCompiledMapping{spec: spec, colPos: colPos, path: path})
	}
	return mappings, needParsedJson, needEnvelope, nil
}

// applyMappings writes the mapped values onto the record, in place.
// On failure it also returns the column being mapped, when the failure is specific to one,
// so the error report can name it.
func (w *ollamaWorker) applyMappings(record *[]any, body []byte, resp *ollamaApiResponse) (string, error) {
	// The record must be able to hold every column of the channel: short records are
	// legitimate and an in place assignment past the end would panic.
	if len(*record) < w.nbrColumns {
		grown := make([]any, w.nbrColumns)
		copy(grown, *record)
		*record = grown
	}

	text, thinking := resp.text()
	if !w.config.DisableStripCodeFences {
		text = ollamaStripCodeFences(text)
	}

	var parsed any
	if w.needParsedJson {
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			return "", fmt.Errorf(
				"while parsing the model response as json (consider setting response_format): %v, response was: %s",
				err, ollamaTruncate(text, 500))
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
		case ollamaSourceRawResponse:
			value, found = text, true
		case ollamaSourceThinking:
			value, found = thinking, len(thinking) > 0
		case ollamaSourceEnvelope:
			value, found = ollamaWalkPath(envelope, mapping.path)
		default: // ollamaSourceResponse
			if len(mapping.path) == 0 {
				value, found = text, true
			} else {
				value, found = ollamaWalkPath(parsed, mapping.path)
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

// ollamaWalkPath follows a dot notation path into a parsed json value.
// A path element that is an integer indexes into an array.
func ollamaWalkPath(value any, path []string) (any, bool) {
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
// Infer server client
// ---------------------------------------------------------------------------

type ollamaClient struct {
	url        string
	headers    map[string]string
	httpClient *http.Client
	timeout    time.Duration
	maxRetry   int
	retryWait  time.Duration
}

// ollamaGenerateRequest is the /api/generate request, ollamaChatRequest the /api/chat one.
// Streaming is always off: the operator wants the complete response before it can map it.
type ollamaGenerateRequest struct {
	Model     string          `json:"model"`
	Prompt    string          `json:"prompt"`
	System    string          `json:"system,omitempty"`
	Stream    bool            `json:"stream"`
	Format    json.RawMessage `json:"format,omitempty"`
	Options   map[string]any  `json:"options,omitempty"`
	KeepAlive string          `json:"keep_alive,omitempty"`
	Think     *bool           `json:"think,omitempty"`
}

type ollamaChatRequest struct {
	Model     string              `json:"model"`
	Messages  []ollamaChatMessage `json:"messages"`
	Stream    bool                `json:"stream"`
	Format    json.RawMessage     `json:"format,omitempty"`
	Options   map[string]any      `json:"options,omitempty"`
	KeepAlive string              `json:"keep_alive,omitempty"`
	Think     *bool               `json:"think,omitempty"`
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaApiResponse covers both /api/generate (Response) and /api/chat (Message).
type ollamaApiResponse struct {
	Model           string             `json:"model"`
	Response        string             `json:"response"`
	Thinking        string             `json:"thinking"`
	Message         *ollamaChatMessage `json:"message"`
	MessageThinking string             `json:"-"`
	Done            bool               `json:"done"`
	DoneReason      string             `json:"done_reason"`
	TotalDuration   int64              `json:"total_duration"`
	PromptEvalCount int                `json:"prompt_eval_count"`
	EvalCount       int                `json:"eval_count"`
	Error           string             `json:"error"`
}

// text returns the model's answer and, when in use, its reasoning text.
func (r *ollamaApiResponse) text() (string, string) {
	if r.Message != nil {
		return r.Message.Content, r.MessageThinking
	}
	return r.Response, r.Thinking
}

func (w *ollamaWorker) buildRequest(prompt string) ([]byte, error) {
	config := w.config
	if config.Api == "chat" {
		request := &ollamaChatRequest{
			Model:     config.Model,
			Messages:  make([]ollamaChatMessage, 0, 2),
			Format:    config.ResponseFormat,
			Options:   config.Options,
			KeepAlive: config.KeepAlive,
			Think:     config.Think,
		}
		if len(config.SystemPrompt) > 0 {
			request.Messages = append(request.Messages,
				ollamaChatMessage{Role: "system", Content: config.SystemPrompt})
		}
		request.Messages = append(request.Messages, ollamaChatMessage{Role: "user", Content: prompt})
		return json.Marshal(request)
	}
	return json.Marshal(&ollamaGenerateRequest{
		Model:     config.Model,
		Prompt:    prompt,
		System:    config.SystemPrompt,
		Format:    config.ResponseFormat,
		Options:   config.Options,
		KeepAlive: config.KeepAlive,
		Think:     config.Think,
	})
}

// call posts the payload to the infer server, retrying the failures that are worth
// retrying: timeouts, connection errors, 429 and 5xx. A 4xx is a request the server will
// keep refusing, so it fails the record immediately.
func (c *ollamaClient) call(ctx context.Context, payload []byte, pm *ollamaPoolManager) (
	[]byte, *ollamaApiResponse, error) {

	var lastErr error
	wait := c.retryWait
	for attempt := 0; attempt <= c.maxRetry; attempt++ {
		if attempt > 0 {
			pm.retryCount.Add(1)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, nil, fmt.Errorf("interrupted while waiting to retry the infer server: %v", lastErr)
			}
			wait *= 2
		}
		body, resp, retryable, err := c.callOnce(ctx, payload)
		if err == nil {
			return body, resp, nil
		}
		lastErr = err
		if !retryable || ctx.Err() != nil {
			return nil, nil, err
		}
	}
	return nil, nil, fmt.Errorf("after %d attempts: %v", c.maxRetry+1, lastErr)
}

func (c *ollamaClient) callOnce(ctx context.Context, payload []byte) (
	[]byte, *ollamaApiResponse, bool, error) {

	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, false, fmt.Errorf("while building the infer server request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		request.Header.Set(k, v)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		// The common case is the infer server being stopped; say so in terms the operator
		// can act on rather than surfacing a bare dial error.
		return nil, nil, true, fmt.Errorf("while calling the infer server at %s, is it running? %v", c.url, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, true, fmt.Errorf("while reading the infer server response: %v", err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		retryable := response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
		return nil, nil, retryable, fmt.Errorf("error: the infer server returned %s: %s",
			response.Status, ollamaTruncate(string(body), 500))
	}
	var resp ollamaApiResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, nil, false, fmt.Errorf("while parsing the infer server response: %v, response was: %s",
			err, ollamaTruncate(string(body), 500))
	}
	if len(resp.Error) > 0 {
		// Ollama reports some failures (an unknown model, most notably) with a 200.
		return nil, nil, false, fmt.Errorf("error: the infer server returned: %s", resp.Error)
	}
	// The chat response carries the reasoning text inside the message.
	if resp.Message != nil {
		var envelope struct {
			Message struct {
				Thinking string `json:"thinking"`
			} `json:"message"`
		}
		if json.Unmarshal(body, &envelope) == nil {
			resp.MessageThinking = envelope.Message.Thinking
		}
	}
	return body, &resp, false, nil
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

func (ctx *BuilderContext) NewOllamaTransformationPipe(source *InputChannel, outputCh *OutputChannel,
	spec *TransformationSpec) (*OllamaTransformationPipe, error) {

	if spec == nil || spec.OllamaConfig == nil {
		return nil, fmt.Errorf("error: Ollama Pipe Transformation spec is missing ollama_config element")
	}
	if outputCh == nil {
		return nil, fmt.Errorf("error: ollama operator requires an output_channel")
	}
	config := spec.OllamaConfig
	// This operator augments the input record, it never builds a new one.
	spec.NewRecord = false
	applyOllamaDefaults(config)

	if len(config.Model) == 0 {
		return nil, fmt.Errorf("error: ollama_config must specify the model")
	}
	switch config.Api {
	case "generate", "chat":
	default:
		return nil, fmt.Errorf("error: unknown ollama_config api '%s', expecting 'generate' or 'chat'", config.Api)
	}
	switch config.OnError {
	case ollamaOnErrorPassThrough, ollamaOnErrorDrop, ollamaOnErrorFail:
	default:
		return nil, fmt.Errorf("error: unknown ollama_config on_error '%s', expecting one of %s, %s, %s",
			config.OnError, ollamaOnErrorPassThrough, ollamaOnErrorDrop, ollamaOnErrorFail)
	}

	// The record is augmented in place, so the input and output channels must have the
	// same columns - in practice the same channel_spec_name.
	if err := validateOllamaChannels(source, outputCh); err != nil {
		return nil, err
	}

	// Resolve the prompt template and substitute the env vars: they are fixed for the
	// life of the node, only the column placeholders vary from one record to the next.
	template, err := resolveOllamaTemplate(ctx.cpConfig, config)
	if err != nil {
		return nil, err
	}
	promptTemplate, err := compileOllamaPromptTemplate(utils.ReplaceEnvVars(template, ctx.env), *source.Columns)
	if err != nil {
		return nil, err
	}
	config.SystemPrompt = utils.ReplaceEnvVars(config.SystemPrompt, ctx.env)

	mappings, needParsedJson, needEnvelope, err := compileOllamaMappings(config, outputCh)
	if err != nil {
		return nil, err
	}

	rowKeyPos := -1
	if len(config.RowKeyColumn) > 0 {
		pos, ok := (*source.Columns)[config.RowKeyColumn]
		if !ok {
			return nil, fmt.Errorf(
				"error: ollama_config row_key_column '%s' is not a column of the input channel '%s'",
				config.RowKeyColumn, source.Name)
		}
		rowKeyPos = pos
	}

	client, err := newOllamaClient(config, ctx.env)
	if err != nil {
		return nil, err
	}

	// Build one set of column evaluators per worker: they carry state and are not safe to
	// share across goroutines. Building them here rather than in the worker keeps a bad
	// column spec a build time error.
	columnEvaluators := make([][]TransformationColumnEvaluator, config.PoolSize)
	for w := range columnEvaluators {
		columnEvaluators[w] = make([]TransformationColumnEvaluator, len(spec.Columns))
		for i := range spec.Columns {
			columnEvaluators[w][i], err = ctx.BuildTransformationColumnEvaluator(source, outputCh, &spec.Columns[i])
			if err != nil {
				return nil, fmt.Errorf("while BuildTransformationColumnEvaluator (in NewOllamaTransformationPipe): %v", err)
			}
		}
	}

	// Get the error channel if configured
	var errorOutputCh *OutputChannel
	if config.ErrorChannel != nil {
		if len(config.ErrorChannel.Name) == 0 {
			return nil, fmt.Errorf("error: error_channel name cannot be empty")
		}
		if len(config.ErrorChannel.SpecName) == 0 {
			return nil, fmt.Errorf("error: error_channel spec name cannot be empty")
		}
		errorOutputCh, err = ctx.channelRegistry.GetOutputChannel(config.ErrorChannel.Name)
		if err != nil {
			return nil, err
		}
	}

	// A context cancelled when the pipeline is interrupted, so an in flight call does not
	// hold a record for the whole request timeout when everything else is shutting down.
	callCtx, cancelCalls := newOllamaCallContext(ctx.done)

	pm := &ollamaPoolManager{
		workersTaskCh: make(chan []any, 1),
		workersWg:     new(sync.WaitGroup),
	}
	log.Printf("Starting an Ollama Worker Pool of size %d, model %s, url %s",
		config.PoolSize, config.Model, client.url)
	for w := range config.PoolSize {
		worker := &ollamaWorker{
			config:           config,
			source:           source,
			outputCh:         outputCh,
			errorOutputCh:    errorOutputCh,
			client:           client,
			template:         promptTemplate,
			mappings:         mappings,
			needParsedJson:   needParsedJson,
			needEnvelope:     needEnvelope,
			columnEvaluators: columnEvaluators[w],
			rowKeyPos:        rowKeyPos,
			nbrColumns:       len(outputCh.Config.Columns),
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

	return &OllamaTransformationPipe{
		cpConfig:        ctx.cpConfig,
		source:          source,
		outputCh:        outputCh,
		errorOutputCh:   errorOutputCh,
		channelRegistry: ctx.channelRegistry,
		spec:            spec,
		config:          config,
		poolManager:     pm,
		cancelCalls:     cancelCalls,
		doneCh:          ctx.done,
	}, nil
}

func applyOllamaDefaults(config *OllamaSpec) {
	if len(config.Api) == 0 {
		config.Api = "generate"
	}
	if len(config.OnError) == 0 {
		config.OnError = ollamaOnErrorPassThrough
	}
	if len(config.KeepAlive) == 0 {
		config.KeepAlive = ollamaDefaultKeepAlive
	}
	if config.PoolSize < 1 {
		config.PoolSize = 1
	}
	if config.RequestTimeoutSec < 1 {
		config.RequestTimeoutSec = ollamaDefaultRequestTimeoutSec
	}
	if config.ConnectTimeoutSec < 1 {
		config.ConnectTimeoutSec = ollamaDefaultConnectTimeoutSec
	}
	if config.MaxRetry == nil || *config.MaxRetry < 0 {
		// Unset means the default; an explicit 0 disables the retries.
		maxRetry := ollamaDefaultMaxRetry
		config.MaxRetry = &maxRetry
	}
	if config.RetryWaitSec < 1 {
		config.RetryWaitSec = ollamaDefaultRetryWaitSec
	}
	if config.MaxErrorCount < 1 {
		config.MaxErrorCount = ollamaDefaultMaxErrorCount
	}
}

// validateOllamaChannels checks that the input and output channels share the same
// ChannelSpec, which is what makes the in place augmentation legal.
// The channel registry maps every channel name to the same *ChannelSpec instance, so the
// pointers are equal when both channels are configured with the same channel_spec_name;
// otherwise the columns must still match one for one.
func validateOllamaChannels(source *InputChannel, outputCh *OutputChannel) error {
	if source.Config == nil || outputCh.Config == nil {
		return fmt.Errorf("error: ollama operator: input or output channel has no channel spec")
	}
	if source.Config == outputCh.Config {
		return nil
	}
	inColumns := source.Config.Columns
	outColumns := outputCh.Config.Columns
	sharedSpecMsg := fmt.Sprintf(
		"the ollama operator augments the input record in place, so the input channel '%s' and the output "+
			"channel '%s' must share the same channel spec (same channel_spec_name)", source.Name, outputCh.Name)
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

// resolveOllamaTemplate returns the prompt template, taken from the operator config or
// from the named template of the cpipes config, and applies the named template's defaults.
func resolveOllamaTemplate(cpConfig *ComputePipesConfig, config *OllamaSpec) (string, error) {
	hasInline := len(config.PromptTemplate) > 0
	hasNamed := len(config.PromptTemplateName) > 0
	switch {
	case hasInline && hasNamed:
		return "", fmt.Errorf(
			"error: ollama_config has both prompt_template and prompt_template_name, specify only one")
	case hasInline:
		return config.PromptTemplate, nil
	case hasNamed:
		if cpConfig != nil {
			for i := range cpConfig.PromptTemplates {
				promptTemplate := &cpConfig.PromptTemplates[i]
				if promptTemplate.Key == config.PromptTemplateName {
					// The operator's own settings win over the template's defaults.
					if len(config.SystemPrompt) == 0 {
						config.SystemPrompt = promptTemplate.SystemPrompt
					}
					if len(config.ResponseFormat) == 0 {
						config.ResponseFormat = promptTemplate.ResponseFormat
					}
					return promptTemplate.Template, nil
				}
			}
		}
		return "", fmt.Errorf(
			"error: ollama_config refers to prompt_template_name '%s' which is not defined in prompt_templates",
			config.PromptTemplateName)
	default:
		return "", fmt.Errorf(
			"error: ollama_config must specify either prompt_template or prompt_template_name")
	}
}

func newOllamaClient(config *OllamaSpec, env map[string]any) (*ollamaClient, error) {
	url, err := resolveOllamaUrl(config, env)
	if err != nil {
		return nil, err
	}
	path := "/api/generate"
	if config.Api == "chat" {
		path = "/api/chat"
	}
	var headers map[string]string
	if config.Server != nil {
		headers = config.Server.Headers
	}
	timeout := time.Duration(config.RequestTimeoutSec) * time.Second
	return &ollamaClient{
		url:     strings.TrimSuffix(url, "/") + path,
		headers: headers,
		httpClient: &http.Client{
			// No client level timeout: it is applied per attempt via the request context,
			// so a retry gets a full budget rather than the remainder of a shared one.
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: time.Duration(config.ConnectTimeoutSec) * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: time.Duration(config.ConnectTimeoutSec) * time.Second,
				MaxIdleConns:        config.PoolSize + 1,
				MaxIdleConnsPerHost: config.PoolSize + 1,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		timeout:   timeout,
		maxRetry:  *config.MaxRetry,
		retryWait: time.Duration(config.RetryWaitSec) * time.Second,
	}, nil
}

// resolveOllamaUrl takes the infer server url from the operator config, then from the
// cpipes env, then from the JETS_INFER_URL environment variable which the deployed
// containers get when the stack is built with BUILD_INFER_SERVICE.
func resolveOllamaUrl(config *OllamaSpec, env map[string]any) (string, error) {
	if config.Server != nil && len(config.Server.Url) > 0 {
		url := strings.TrimSpace(utils.ReplaceEnvVars(config.Server.Url, env))
		if len(url) > 0 {
			return url, nil
		}
	}
	if env != nil {
		if url, ok := env["$JETS_INFER_URL"].(string); ok && len(url) > 0 {
			return url, nil
		}
	}
	if url := os.Getenv("JETS_INFER_URL"); len(url) > 0 {
		return url, nil
	}
	return "", fmt.Errorf(
		"error: cannot determine the infer server url, set ollama_config.server.url in the configuration or " +
			"deploy the stack with BUILD_INFER_SERVICE so JETS_INFER_URL is set on the containers")
}

// newOllamaCallContext returns a context cancelled when the pipeline is interrupted.
// The watching goroutine also exits when the returned cancel function is called, which
// OllamaTransformationPipe.Finally does.
func newOllamaCallContext(doneCh chan struct{}) (context.Context, context.CancelFunc) {
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

// ollamaStripCodeFences removes the markdown code fences that models put around json
// even when asked not to.
func ollamaStripCodeFences(text string) string {
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

func ollamaToString(value any) string {
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

// ollamaZipRecord makes a column name to value map for the {{@record}} placeholder.
// Unlike utils.ZipSlices it tolerates a short record, which is legitimate here.
func ollamaZipRecord(columns []string, record *[]any) map[string]any {
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

func ollamaColumnNames(columns map[string]int) string {
	names := make([]string, len(columns))
	for name, pos := range columns {
		if pos >= 0 && pos < len(names) {
			names[pos] = name
		}
	}
	if len(names) > ollamaMaxColumnsInErrMsg {
		return strings.Join(names[:ollamaMaxColumnsInErrMsg], ", ") +
			fmt.Sprintf(", ... (%d more)", len(names)-ollamaMaxColumnsInErrMsg)
	}
	return strings.Join(names, ", ")
}

func ollamaTruncate(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
