package compute_pipes

// vLLM operator: call a vLLM server's OpenAI-compatible api once per input record and
// augment the record in place with values extracted from the model response.
//
// It is the ollama operator's second sibling rather than a fork of it (item 15b): the
// operator shell, the worker pool, the prompt template, the response mapping, the retry
// policy, the circuit breaker, the cost guard and the on_error handling are the shared
// inference plumbing of pipe_transformation_infer.go, reached through the same
// inferBackend seam 15a extracted. What this file holds is the OpenAI-compatible request
// and response shapes, the translation of `response_format` into what vLLM constrains
// generation with, and the two failure modes that are this backend's own.
//
// Four things are worth knowing before reading the code.
//
// **guided_json is not format, and that is the whole of the request difference.** Ollama
// takes a schema in `format` and treats it as best effort; vLLM constrains the decoder
// itself, and the field it takes the schema in depends on which arm the server supports:
// `guided_json` (vLLM's own parameter, the default here) or the OpenAI-compatible
// `response_format: {"type": "json_schema", ...}`. The operator's configuration is the
// same `response_format` property the ollama operator has, and the translation happens
// once at build time — so a .pc.json moves between the two operators by changing the
// type token and the config element, not by rewriting the schema. The `"json"` string
// form, which ollama accepts, becomes `response_format: {"type": "json_object"}`.
//
// **A truncated constrained generation is a row-level error rather than a mapping
// failure.** With the decoder constrained to a schema, a response that stopped at the
// token ceiling is invalid json by construction, and the shared plumbing would report it
// as "consider setting response_format" — advice for the opposite problem. The backend
// checks finish_reason and says what actually happened, but only when generation was
// constrained: an unconstrained call producing free text may legitimately be cut short.
//
// **The sampling parameters are top-level, not nested under `options`.** That is the
// OpenAI api's shape and not a choice: temperature, max_tokens, top_p and vLLM's own
// extensions are peers of `model`. The `options` map is therefore merged into the request
// body rather than nested in it, and the keys the operator sets itself are refused at
// build time rather than silently overwritten by (or silently overwriting) a config.
//
// **JETS_INFER_URL points at Ollama in the deployed stack.** The url resolution is shared
// with the ollama and embed operators (resolveInferServerUrl), and the deployment sets
// that variable to the infer service, which serves Ollama on 11434
// (cdk/jetstore_one/stack/build_infer_service.go). A vllm operator relying on the
// fallback therefore reaches an api that does not exist and gets a 404, so `server.url`
// is in practice required today; vllmExplainError says so on a 404 rather than leaving
// the bare status to be interpreted.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Default settings and the api paths, see VllmSpec for the associated configuration
// properties.
const (
	vllmApiChat        = "chat"
	vllmApiCompletions = "completions"
	// The OpenAI-compatible routes vLLM serves. Neither is configurable: the api
	// property chooses between them, as the ollama operator's does.
	vllmChatApiPath        = "/v1/chat/completions"
	vllmCompletionsApiPath = "/v1/completions"
	// The two arms of guided decoding, see vllmStructuredOutput.
	vllmStructuredGuidedJson = "guided_json"
	vllmStructuredJsonSchema = "json_schema"
	// The name the json_schema arm gives the schema. The OpenAI shape requires one and
	// nothing reads it back, so it is a constant rather than a configuration property.
	vllmSchemaName = "response"
	// finish_reason when the generation stopped at the token ceiling.
	vllmFinishLength = "length"
)

// vllmReservedRequestKeys are the request body keys the operator sets itself; `options`
// may not carry them. They are refused at build time rather than merged, because either
// precedence is wrong: letting the config win silently redirects the operator to another
// model, and letting the operator win silently discards what the author wrote.
var vllmReservedRequestKeys = []string{
	"model", "stream", "messages", "prompt", "guided_json", "response_format",
}

// vllmBackend implements the inferBackend seam: build the OpenAI-compatible request for a
// rendered prompt, and perform one call attempt against the vLLM server.
type vllmBackend struct {
	config *VllmSpec
	client *vllmClient
	// base is the fixed part of the request body, built once at build time: the model,
	// the guided-decoding field and the merged `options`. Only the prompt varies from one
	// record to the next.
	base map[string]any
	// constrained records that generation is guided by a schema or by json mode, which is
	// what makes a truncated response a certain failure rather than a possible one.
	constrained bool
}

type vllmClient struct {
	url        string
	headers    map[string]string
	httpClient *http.Client
	timeout    time.Duration
}

// vllmRequest is the fixed part of the request body. The per-record part (`messages` for
// the chat api, `prompt` for the completions one) and the merged `options` are added to
// the map this is converted into; see vllmRequestBase.
// Streaming is always off: the operator wants the complete response before it can map it.
type vllmRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
	// GuidedJson is vLLM's own guided-decoding parameter and takes the schema itself.
	GuidedJson json.RawMessage `json:"guided_json,omitempty"`
	// ResponseFormat is the OpenAI-compatible arm: json mode, or a named schema.
	ResponseFormat json.RawMessage `json:"response_format,omitempty"`
}

type vllmChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// vllmApiResponse covers both /v1/chat/completions (choices[].message) and
// /v1/completions (choices[].text). It implements inferResponse for the shared plumbing.
//
// The error fields cover two shapes: OpenAI nests the failure under `error`, vLLM's own
// handlers flatten it with `object: "error"` and a top-level `message`. Both are read, so
// a server behind a proxy that rewrites one into the other still reports its cause.
type vllmApiResponse struct {
	Id      string       `json:"id"`
	Object  string       `json:"object"`
	Model   string       `json:"model"`
	Created int64        `json:"created"`
	Choices []vllmChoice `json:"choices"`
	Usage   vllmUsage    `json:"usage"`
	Error   *vllmError   `json:"error"`
	Message string       `json:"message"`
}

type vllmChoice struct {
	Index int `json:"index"`
	// Text is the /v1/completions answer, Message the /v1/chat/completions one.
	Text         string       `json:"text"`
	Message      *vllmMessage `json:"message"`
	FinishReason string       `json:"finish_reason"`
	StopReason   any          `json:"stop_reason"`
}

// vllmMessage carries the answer and, when the server runs a reasoning parser, the
// reasoning text. Unlike ollama's `think`, this is a server-side setting rather than a
// request property: there is nothing for the operator to send, and the `thinking` mapping
// source simply finds a value or does not.
type vllmMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
}

type vllmUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type vllmError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   any    `json:"param"`
	Code    any    `json:"code"`
}

// Text returns the model's answer and, when the server supplies it, its reasoning text.
func (r *vllmApiResponse) Text() (string, string) {
	if len(r.Choices) == 0 {
		return "", ""
	}
	choice := &r.Choices[0]
	if choice.Message != nil {
		return choice.Message.Content, choice.Message.ReasoningContent
	}
	return choice.Text, ""
}

func (r *vllmApiResponse) Tokens() (int, int) {
	return r.Usage.PromptTokens, r.Usage.CompletionTokens
}

func (r *vllmApiResponse) ModelName() string {
	return r.Model
}

// finishReason returns why the first choice stopped generating, empty when the response
// carries no choice.
func (r *vllmApiResponse) finishReason() string {
	if len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].FinishReason
}

// errorMessage returns the failure the server reported in a body, empty when it reported
// none. It reads both the nested and the flattened shape.
func (r *vllmApiResponse) errorMessage() string {
	if r.Error != nil && len(r.Error.Message) > 0 {
		return r.Error.Message
	}
	if r.Object == "error" && len(r.Message) > 0 {
		return r.Message
	}
	return ""
}

func (b *vllmBackend) BuildRequest(prompt string) ([]byte, error) {
	payload := make(map[string]any, len(b.base)+1)
	maps.Copy(payload, b.base)
	if b.config.Api == vllmApiCompletions {
		payload["prompt"] = prompt
		return json.Marshal(payload)
	}
	messages := make([]vllmChatMessage, 0, 2)
	if len(b.config.SystemPrompt) > 0 {
		messages = append(messages, vllmChatMessage{Role: "system", Content: b.config.SystemPrompt})
	}
	messages = append(messages, vllmChatMessage{Role: "user", Content: prompt})
	payload["messages"] = messages
	return json.Marshal(payload)
}

// CallOnce performs one call attempt; the retry policy is the shared worker's.
func (b *vllmBackend) CallOnce(ctx context.Context, payload []byte) ([]byte, inferResponse, bool, error) {
	body, resp, retryable, err := b.client.callOnce(ctx, payload)
	if resp == nil {
		return body, nil, retryable, err
	}
	// A constrained generation that hit the token ceiling is truncated json, and every
	// mapping below would report it as a parse failure suggesting response_format - which
	// is already set, and is not the problem. Retrying the same call reproduces it, so
	// this fails the record rather than the attempt.
	if b.constrained && resp.finishReason() == vllmFinishLength {
		return body, nil, false, fmt.Errorf(
			"error: the model stopped at the token ceiling with generation constrained by a schema, "+
				"so the response is truncated and cannot be valid json; raise max_tokens in "+
				"vllm_config.options. Response was: %s", inferTruncate(string(body), 500))
	}
	return body, resp, retryable, err
}

func (c *vllmClient) callOnce(ctx context.Context, payload []byte) (
	[]byte, *vllmApiResponse, bool, error) {

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
		return nil, nil, true, unavailable(
			fmt.Errorf("while calling the infer server at %s, is it running? %v", c.url, err))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, true, fmt.Errorf("while reading the infer server response: %v", err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		retryable := response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
		statusErr := fmt.Errorf("error: the infer server returned %s: %s",
			response.Status, vllmExplainError(response.StatusCode, string(body), c.url))
		// A 5xx is the server; a 429 is the server asking to be asked more
		// slowly, which the retry already answers. See inferServerUnavailable.
		if response.StatusCode >= 500 {
			return nil, nil, retryable, unavailable(statusErr)
		}
		return nil, nil, retryable, statusErr
	}
	var resp vllmApiResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, nil, false, fmt.Errorf("while parsing the infer server response: %v, response was: %s",
			err, inferTruncate(string(body), 500))
	}
	if message := resp.errorMessage(); len(message) > 0 {
		// A failure reported with a 2xx, the way ollama reports an unknown model.
		return nil, nil, false, fmt.Errorf("error: the infer server returned: %s", inferTruncate(message, 500))
	}
	// An empty choices list is a successful response the mappings cannot use; catching it
	// here names the model rather than letting a required mapping report a missing path.
	if len(resp.Choices) == 0 {
		return nil, nil, false, fmt.Errorf(
			"error: the infer server returned no choice for model %s, response was: %s",
			resp.Model, inferTruncate(string(body), 500))
	}
	return body, &resp, false, nil
}

// vllmExplainError annotates the status the server returned when the status alone points
// at the wrong cause.
//
// A 404 on the OpenAI routes is almost always the url: the deployed stack sets
// JETS_INFER_URL to its Ollama infer service, and this operator falls back to it exactly
// as the ollama operator does. Ollama answers /api/*, not /v1/*, so a vllm operator with
// no server.url reaches a live server that has never heard of the route - and a bare
// "404 Not Found" reads like a missing model rather than a misdirected operator.
func vllmExplainError(status int, body, url string) string {
	msg := inferTruncate(body, 500)
	if status == http.StatusNotFound {
		return msg + fmt.Sprintf(
			" (note: %s is an OpenAI-compatible route that only a vLLM server serves - check that "+
				"vllm_config.server.url points at vLLM and not at the Ollama infer server, which "+
				"JETS_INFER_URL names and which answers /api/* instead)", url)
	}
	return msg
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

func (ctx *BuilderContext) NewVllmTransformationPipe(source *InputChannel, outputCh *OutputChannel,
	spec *TransformationSpec) (*inferTransformationPipe, error) {

	if spec == nil || spec.VllmConfig == nil {
		return nil, fmt.Errorf("error: Vllm Pipe Transformation spec is missing vllm_config element")
	}
	if outputCh == nil {
		return nil, fmt.Errorf("error: vllm operator requires an output_channel")
	}
	config := spec.VllmConfig
	// This operator augments the input record, it never builds a new one.
	spec.NewRecord = false
	applyVllmDefaults(config)

	if len(config.Model) == 0 {
		return nil, fmt.Errorf("error: vllm_config must specify the model")
	}
	switch config.Api {
	case vllmApiChat, vllmApiCompletions:
	default:
		return nil, fmt.Errorf("error: unknown vllm_config api '%s', expecting '%s' or '%s'",
			config.Api, vllmApiChat, vllmApiCompletions)
	}
	// /v1/completions has no message roles, so a system prompt has nowhere to go. Folding
	// it into the prompt would be a silent reinterpretation of what the author wrote, and
	// the instruct models this operator is for expect the chat template anyway.
	if len(config.SystemPrompt) > 0 && config.Api == vllmApiCompletions {
		return nil, fmt.Errorf(
			"error: vllm_config system_prompt requires api '%s': the '%s' api has no system message",
			vllmApiChat, vllmApiCompletions)
	}
	if err := validateInferOnError(&config.InferCommonSpec, "vllm_config"); err != nil {
		return nil, err
	}
	base, constrained, err := vllmRequestBase(config)
	if err != nil {
		return nil, err
	}

	client, err := newVllmClient(config, ctx.env)
	if err != nil {
		return nil, err
	}
	log.Printf("Starting a Vllm Worker Pool of size %d, model %s, url %s",
		config.PoolSize, config.Model, client.url)

	return ctx.newInferTransformationPipe(source, outputCh, spec, &config.InferCommonSpec,
		&vllmBackend{config: config, client: client, base: base, constrained: constrained},
		inferLabels{
			Pipe:       "VllmTransformationPipe",
			Operator:   "vllm operator",
			Type:       "vllm",
			ConfigName: "vllm_config",
			ErrPrefix:  fmt.Sprintf("vllm operator (model %s)", config.Model),
			Summary:    fmt.Sprintf("VllmTransformationPipe completed: model %s", config.Model),
		})
}

// vllmRequestBase builds the fixed part of the request body once, and reports whether
// generation is constrained. Doing it here rather than per record is not only the cost:
// it makes an unusable response_format or a colliding option a build time error, which is
// where every other configuration failure of this operator is reported.
func vllmRequestBase(config *VllmSpec) (map[string]any, bool, error) {
	request := &vllmRequest{Model: config.Model, Stream: false}
	guidedJson, responseFormat, constrained, err := vllmStructuredOutput(config)
	if err != nil {
		return nil, false, err
	}
	request.GuidedJson = guidedJson
	request.ResponseFormat = responseFormat

	data, err := json.Marshal(request)
	if err != nil {
		return nil, false, fmt.Errorf("error: while encoding the vllm_config request: %v", err)
	}
	base := make(map[string]any)
	if err = json.Unmarshal(data, &base); err != nil {
		return nil, false, fmt.Errorf("error: while encoding the vllm_config request: %v", err)
	}
	for _, key := range slices.Sorted(maps.Keys(config.Options)) {
		if slices.Contains(vllmReservedRequestKeys, key) {
			return nil, false, fmt.Errorf(
				"error: vllm_config options cannot set '%s', the operator sets it itself "+
					"(reserved: %s). Sampling parameters such as temperature, max_tokens and top_p "+
					"are what options is for", key, strings.Join(vllmReservedRequestKeys, ", "))
		}
		base[key] = config.Options[key]
	}
	return base, constrained, nil
}

// vllmStructuredOutput translates the operator's response_format into the request field
// vLLM constrains generation with, and this is the whole of the "guided_json is not
// format" difference between the two backends.
//
// The configured value is ollama's: the string "json", or a json schema document. The
// string becomes OpenAI json mode; a schema goes into `guided_json` or into a named
// `response_format`, depending on structured_output. Which arm a server supports is a
// property of its vLLM version rather than of the configuration, which is why this is a
// property at all and not a fixed choice.
func vllmStructuredOutput(config *VllmSpec) (guidedJson, responseFormat json.RawMessage, constrained bool, err error) {
	if len(config.ResponseFormat) == 0 {
		return nil, nil, false, nil
	}
	var document any
	if err := json.Unmarshal(config.ResponseFormat, &document); err != nil {
		return nil, nil, false, fmt.Errorf(
			"error: vllm_config response_format is not valid json: %v", err)
	}
	switch value := document.(type) {
	case string:
		if value != "json" {
			return nil, nil, false, fmt.Errorf(
				"error: vllm_config response_format '%s' is not understood, expecting the string "+
					"\"json\" or a json schema document", value)
		}
		return nil, json.RawMessage(`{"type":"json_object"}`), true, nil
	case map[string]any:
		switch config.StructuredOutput {
		case vllmStructuredGuidedJson:
			return config.ResponseFormat, nil, true, nil
		case vllmStructuredJsonSchema:
			// strict is what makes this the equivalent of guided_json rather than a hint;
			// an unconstrained schema is what the operator already has in ollama.
			named, err := json.Marshal(map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":   vllmSchemaName,
					"schema": config.ResponseFormat,
					"strict": true,
				},
			})
			if err != nil {
				return nil, nil, false, fmt.Errorf(
					"error: while encoding the vllm_config response_format schema: %v", err)
			}
			return nil, named, true, nil
		default:
			return nil, nil, false, fmt.Errorf(
				"error: unknown vllm_config structured_output '%s', expecting '%s' or '%s'",
				config.StructuredOutput, vllmStructuredGuidedJson, vllmStructuredJsonSchema)
		}
	default:
		return nil, nil, false, fmt.Errorf(
			"error: vllm_config response_format must be the string \"json\" or a json schema document, "+
				"got %s", inferTruncate(string(config.ResponseFormat), 200))
	}
}

func applyVllmDefaults(config *VllmSpec) {
	if len(config.Api) == 0 {
		// Chat rather than completions: an instruct model needs its chat template, and
		// the prompt templates this operator is configured with are instructions.
		config.Api = vllmApiChat
	}
	if len(config.StructuredOutput) == 0 {
		config.StructuredOutput = vllmStructuredGuidedJson
	}
	applyInferCommonDefaults(&config.InferCommonSpec)
}

func newVllmClient(config *VllmSpec, env map[string]any) (*vllmClient, error) {
	url, err := resolveInferServerUrl(config.Server, env, "vllm_config")
	if err != nil {
		return nil, err
	}
	path := vllmChatApiPath
	if config.Api == vllmApiCompletions {
		path = vllmCompletionsApiPath
	}
	var headers map[string]string
	if config.Server != nil {
		headers = config.Server.Headers
	}
	return &vllmClient{
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
		timeout: time.Duration(config.RequestTimeoutSec) * time.Second,
	}, nil
}
