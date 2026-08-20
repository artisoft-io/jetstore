// Package infer is the agent loop's inference client: one call, one answer,
// constrained by a JSON Schema and validated against the same schema.
//
// **Why this is not the cpipes inference plumbing** (plan §3.2, decision 11).
// jets/compute_pipes has a well-built inference layer, and reusing it was the
// obvious move. Two things argue against it, and only the first is an accident:
//
//   - It exports nothing. inferBackend, newInferTransformationPipe and the
//     rest are package-private, and NewOllamaTransformationPipe is a method on
//     BuilderContext returning an unexported type (F12). That is a one-line
//     change and not the real reason.
//   - The shapes differ. That layer is a *record-stream* operator: a worker
//     pool fed by Apply, an on_error policy routing failed rows to an error
//     channel, a completion summary at Finally, and mappings that write model
//     output back into output-channel columns. An authoring loop makes one
//     call at a time, holds a conversation, and has no rows, no channels and
//     no pool. Reusing it would mean instantiating a one-record stream to ask
//     one question.
//
// **What changed since, and it matters before you extend this file.** cpipes
// gap 15a split that operator behind an inferBackend seam
// (pipe_transformation_infer.go:54), and the seam is prompt-in / bytes-out:
// BuildRequest(prompt) and CallOnce(ctx, payload), with no records, no channels
// and no pool in it. The second bullet above is therefore still true of the
// *operator* and no longer true of the *backend* — which is the part this
// package would actually have shared. What still does not fit is narrower:
// BuildRequest takes one prompt string where this package varies System and
// User per call, the seam is unexported, and its retry lives in the worker
// rather than in the seam.
//
// The honest cost is two request/response envelope implementations against one
// server, recorded as Q-13. The recommendation there is to revisit when gap
// 15b's vLLM operator lands, now on the narrower question of whether the schema
// belongs in the seam — guided_json is not format.
//
// **The size to watch is the envelope, not the file.** Of this file's 247 code
// lines about 86 are envelope; the rest is schema compilation, validation and
// the SchemaError the repair loop acts on, which the cpipes side has never had
// and 15b will not give it. An earlier version of this comment said "a few
// hundred lines", which measured the file and concluded about the duplication.
//
// **What this is a copy of.** tools/sample_projects/tasks_go demonstrates the
// whole of Rung 1 (analysis §7) in Go: send a JSON Schema as Ollama's `format`,
// then validate the response against the same schema with
// santhosh-tekuri/jsonschema/v6, which is already in go.mod — including the
// failure mode where a model ignores `format` and client-side validation
// catches it. This is that sample with retries, a timeout, and token counts.
package infer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	defaultHost           = "http://localhost:11434"
	defaultRequestTimeout = 120 * time.Second
	defaultMaxRetry       = 2
	defaultRetryWait      = 2 * time.Second
	// Cap on how much of a bad response is quoted back in an error. The whole
	// of a runaway generation in a log line helps nobody.
	maxQuotedResponse = 2000
)

// Client calls one inference server. It is safe for concurrent use — the
// generate-and-filter mode of §4.3 fans out K candidates — because it holds no
// per-call state; everything mutable lives in the Request and the response.
type Client struct {
	Host  string
	Model string
	// HTTPClient is exposed so a test can substitute a transport and so a
	// caller can set its own timeouts; nil means a client built from
	// RequestTimeout.
	HTTPClient *http.Client
	// RequestTimeout bounds one attempt. The run-level wall-clock cap
	// (AgentRun.wall_clock_cap_seconds) is enforced by the caller's context and
	// is a different thing: this bounds a call, that bounds a run.
	RequestTimeout time.Duration
	// MaxRetry is the number of *additional* attempts after the first. Zero
	// means the default of 2 rather than "no retries", so that the zero value
	// of Client is the sensible one; pass a negative number to mean exactly
	// one attempt. That asymmetry is ugly and is the lesser evil — the
	// alternative is a client whose zero value silently gives up on the first
	// dropped connection.
	MaxRetry  int
	RetryWait time.Duration
	// Options passes model options through to Ollama untouched (temperature,
	// num_ctx, num_thread). Deliberately opaque: it is the server's contract,
	// not this package's.
	Options map[string]any
}

// Request is one turn. Schema is required and is the point of the package: it
// constrains generation server-side and validates the answer client-side, and
// those must be the same document or the guarantee is not what it appears.
type Request struct {
	System string
	User   string
	// Schema is a JSON Schema document. For the narrow tasks of Phase 1 this
	// is one $defs entry of the cpipes contract — a lone TransformationSpec
	// rather than the whole config — which is what makes the task narrow at
	// the wire rather than only in the instruction text (plan §3.3).
	Schema json.RawMessage
}

// Response is one answer, already parsed and already validated against the
// request's schema.
type Response struct {
	// Content is the raw text the model returned.
	Content string
	// Value is Content parsed as JSON. Validation has passed by the time a
	// caller sees this.
	Value any
	// PromptTokens and EvalTokens feed AgentRun.token_spend (D.5). Recorded
	// per call so a run's spend is a sum rather than an estimate.
	PromptTokens int
	EvalTokens   int
	// ModelName is what the server says answered, which need not be what was
	// asked for — a tag can resolve to a different build.
	ModelName string
	// Attempts is how many calls were made, including the successful one.
	Attempts int
}

// Tokens is the pair D.5 accumulates.
func (r *Response) Tokens() int { return r.PromptTokens + r.EvalTokens }

// SchemaError is returned when the model answered but the answer does not
// satisfy the schema. It is separated from every other failure on purpose:
// this is the one the repair loop can act on, and the raw content is what a
// repair prompt quotes back.
type SchemaError struct {
	Content string
	Err     error
	// PromptTokens and EvalTokens are what the rejected answer cost. They are
	// on the error because the call still spent them: a budget that counted
	// only accepted answers would undercount every run that had to repair,
	// which is precisely the runs a repair loop produces. Found by D.3, which
	// is the first caller in a position to notice.
	PromptTokens int
	EvalTokens   int
}

// Tokens is what this rejected attempt cost.
func (e *SchemaError) Tokens() int { return e.PromptTokens + e.EvalTokens }

func (e *SchemaError) Error() string {
	return fmt.Sprintf("the model's answer does not satisfy the schema: %v\n\nanswer:\n%s",
		e.Err, truncate(e.Content, maxQuotedResponse))
}

func (e *SchemaError) Unwrap() error { return e.Err }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Format   json.RawMessage `json:"format,omitempty"`
	Options  map[string]any  `json:"options,omitempty"`
	Messages []chatMessage   `json:"messages"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
	Error           string `json:"error"`
}

// Chat makes one request, retrying transport and server failures, and returns
// an answer that has been parsed and validated.
//
// **Retries cover the call, not the answer.** A connection refused or a 503 is
// retried; a well-formed answer that fails the schema is *not*, because sending
// the identical prompt again is unlikely to produce a different result and the
// repair loop's structured feedback is the mechanism that should handle it
// (analysis §7, Rung 3). Retrying a schema failure here would spend the token
// budget on the strategy the plan explicitly rejects.
func (c *Client) Chat(ctx context.Context, req *Request) (*Response, error) {
	if c.Model == "" {
		return nil, errors.New("infer: no model configured on the client")
	}
	if len(req.Schema) == 0 {
		return nil, errors.New("infer: a schema is required - it is what constrains generation and what validates the answer")
	}
	schema, err := compileSchema(req.Schema)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(chatRequest{
		Model:   c.Model,
		Stream:  false,
		Format:  req.Schema,
		Options: c.Options,
		Messages: []chatMessage{
			{Role: "system", Content: req.System},
			{Role: "user", Content: req.User},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("infer: while encoding the request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts(); attempt++ {
		raw, retryable, err := c.callOnce(ctx, payload)
		if err != nil {
			lastErr = err
			if !retryable {
				return nil, err
			}
			// Give up early rather than sleeping into a deadline that has
			// already passed.
			if err := sleepOrDone(ctx, c.retryWait()); err != nil {
				return nil, fmt.Errorf("infer: %w (last failure: %v)", err, lastErr)
			}
			continue
		}
		return c.validate(raw, schema, attempt)
	}
	return nil, fmt.Errorf("infer: giving up after %d attempts: %w", c.maxAttempts(), lastErr)
}

func (c *Client) validate(raw *chatResponse, schema *jsonschema.Schema, attempts int) (*Response, error) {
	resp := &Response{
		Content:      raw.Message.Content,
		PromptTokens: raw.PromptEvalCount,
		EvalTokens:   raw.EvalCount,
		ModelName:    raw.Model,
		Attempts:     attempts,
	}
	// Ollama's `format` is best-effort rather than a hard constraint — this is
	// the failure mode tasks_go documents and the reason §7.1 prefers vLLM's
	// guided decoding. Client-side validation is what makes it safe to rely on
	// in the meantime, so it is not optional here.
	if err := json.Unmarshal([]byte(resp.Content), &resp.Value); err != nil {
		return nil, &SchemaError{
			Content: resp.Content, Err: fmt.Errorf("not valid JSON: %w", err),
			PromptTokens: resp.PromptTokens, EvalTokens: resp.EvalTokens,
		}
	}
	if err := schema.Validate(resp.Value); err != nil {
		return nil, &SchemaError{
			Content: resp.Content, Err: err,
			PromptTokens: resp.PromptTokens, EvalTokens: resp.EvalTokens,
		}
	}
	return resp, nil
}

// callOnce makes a single attempt and says whether its failure is worth
// retrying.
func (c *Client) callOnce(ctx context.Context, payload []byte) (*chatResponse, bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.requestTimeout())
	defer cancel()

	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost,
		c.host()+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, false, fmt.Errorf("infer: while building the request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient().Do(httpReq)
	if err != nil {
		// A cancelled parent context is the caller's decision, not a transient
		// failure, so it must not be retried into the budget.
		if ctx.Err() != nil {
			return nil, false, fmt.Errorf("infer: %w", ctx.Err())
		}
		return nil, true, fmt.Errorf("infer: cannot reach the inference server at %s: %w", c.host(), err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 500 || httpResp.StatusCode == http.StatusTooManyRequests {
		return nil, true, fmt.Errorf("infer: the inference server returned %s", httpResp.Status)
	}
	if httpResp.StatusCode != http.StatusOK {
		// A 4xx is a bad request; sending it again produces the same 4xx.
		return nil, false, fmt.Errorf("infer: the inference server rejected the request with %s", httpResp.Status)
	}

	var data chatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&data); err != nil {
		return nil, true, fmt.Errorf("infer: while decoding the response: %w", err)
	}
	if data.Error != "" {
		return nil, false, fmt.Errorf("infer: the inference server reported an error: %s", data.Error)
	}
	return &data, false, nil
}

// compileSchema compiles the request's schema once per call. The $id stripping
// is inherited from tasks_go and its reason still holds: a schema mirrored from
// another type system can repeat an "$id" label, which Ollama tolerates and a
// strict draft 2020-12 validator rejects as a duplicate anchor. The labels are
// not $ref targets, so dropping them changes nothing that is checked.
func compileSchema(raw json.RawMessage) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("infer: the request's schema is not valid JSON: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("request_schema.json", stripIDs(doc)); err != nil {
		return nil, fmt.Errorf("infer: while adding the request's schema: %w", err)
	}
	schema, err := compiler.Compile("request_schema.json")
	if err != nil {
		return nil, fmt.Errorf("infer: the request's schema does not compile: %w", err)
	}
	return schema, nil
}

func stripIDs(v any) any {
	switch t := v.(type) {
	case map[string]any:
		delete(t, "$id")
		for _, val := range t {
			stripIDs(val)
		}
	case []any:
		for _, val := range t {
			stripIDs(val)
		}
	}
	return v
}

func sleepOrDone(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) host() string {
	if c.Host != "" {
		return strings.TrimSuffix(c.Host, "/")
	}
	return defaultHost
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: c.requestTimeout()}
}

func (c *Client) requestTimeout() time.Duration {
	if c.RequestTimeout > 0 {
		return c.RequestTimeout
	}
	return defaultRequestTimeout
}

func (c *Client) maxAttempts() int {
	if c.MaxRetry > 0 {
		return c.MaxRetry + 1
	}
	if c.MaxRetry < 0 {
		return 1
	}
	return defaultMaxRetry + 1
}

func (c *Client) retryWait() time.Duration {
	if c.RetryWait > 0 {
		return c.RetryWait
	}
	return defaultRetryWait
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("\n... (%d bytes truncated)", len(s)-max)
}
