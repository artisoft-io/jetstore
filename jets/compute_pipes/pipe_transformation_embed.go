package compute_pipes

// Embed operator: call the infer server's embeddings endpoint once per input record and
// put the resulting vector on the record.
//
// It is the ollama operator's sibling rather than a fork of it: everything that is not
// specific to the embeddings api - the operator shell, the worker pool, the prompt
// template, the response mapping, the retry policy, the cost guard and the on_error
// handling - is the shared inference plumbing of pipe_transformation_infer.go, reached
// through the same inferBackend seam (15a). What this file holds is the /api/embed
// request and response shapes, and the small amount of validation that an embeddings
// call needs and a generative one does not.
//
// Three things are worth knowing before reading the code.
//
// The endpoint is not configurable, and that is forced rather than preferred. inferResponse
// requires Text, Tokens and ModelName; the legacy /api/embeddings returns the vector and
// nothing else, so a backend on it could satisfy neither Tokens nor ModelName except by
// inventing values. /api/embed supplies `model` and `prompt_eval_count` and is the only
// endpoint that fits the seam. The two also normalise differently - /api/embed returns an
// L2 normalised vector, the legacy endpoint the raw one, same direction - so mixing them
// across one corpus compares correctly under cosine and wrongly under dot product. Using
// exactly one endpoint is what avoids that, and the operator does not offer the choice.
//
// Nothing arrives through Text(). A vector is not text, so embedApiResponse implements
// Text as ("", ""), and the vector reaches the record through the `envelope` mapping
// source, which walks the raw response body rather than the model's answer. The plumbing
// needed no change for this: CallOnce already returns the body alongside the typed
// response, and applyMappings already turns a []any into json text, or into a typed array
// when as_rdf_type is given.
//
// The configuration does not name the response path. The vector lives at `embeddings.0`
// and nowhere else, so vector_column is the property and the mapping is synthesized from
// it. output_mapping stays available and is applied on top, for the envelope values worth
// keeping (the model name, the prompt token count).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// The path of the vector inside the /api/embed response envelope. One record is one
	// input, so the response carries a list of exactly one vector.
	embedVectorPath = "embeddings.0"
	// The endpoint, appended to the resolved infer server url. Not configurable, see above.
	embedApiPath = "/api/embed"
)

// embedBackend implements the inferBackend seam: build the /api/embed request for a
// rendered text, and perform one call attempt against it.
type embedBackend struct {
	config *EmbedSpec
	client *embedClient
}

type embedClient struct {
	url        string
	headers    map[string]string
	httpClient *http.Client
	timeout    time.Duration
}

// embedRequest is the /api/embed request. Input is sent as a single string rather than a
// list of one: the operator embeds one record per call, and ollama answers either form
// with a list of one vector.
type embedRequest struct {
	Model     string         `json:"model"`
	Input     string         `json:"input"`
	Truncate  *bool          `json:"truncate,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
	KeepAlive string         `json:"keep_alive,omitempty"`
}

// embedApiResponse is the /api/embed response envelope. It implements inferResponse for
// the shared plumbing; the vector itself is read from the raw body by the `envelope`
// mapping, not from here.
type embedApiResponse struct {
	Model           string      `json:"model"`
	Embeddings      [][]float64 `json:"embeddings"`
	TotalDuration   int64       `json:"total_duration"`
	LoadDuration    int64       `json:"load_duration"`
	PromptEvalCount int         `json:"prompt_eval_count"`
	Error           string      `json:"error"`
}

// Text returns nothing: an embeddings response carries no text, and the operator's
// mappings read the vector from the response envelope. The shared plumbing calls this
// unconditionally and only parses the result when a `response` mapping asks it to, which
// the embed builder refuses - so the empty string is never parsed.
func (r *embedApiResponse) Text() (string, string) {
	return "", ""
}

// Tokens returns the prompt token count and zero evaluated tokens: an embeddings call
// evaluates the input and generates nothing, and the response has no eval_count.
func (r *embedApiResponse) Tokens() (int, int) {
	return r.PromptEvalCount, 0
}

func (r *embedApiResponse) ModelName() string {
	return r.Model
}

func (b *embedBackend) BuildRequest(prompt string) ([]byte, error) {
	return json.Marshal(&embedRequest{
		Model:     b.config.Model,
		Input:     prompt,
		Truncate:  b.config.Truncate,
		Options:   b.config.Options,
		KeepAlive: b.config.KeepAlive,
	})
}

// CallOnce performs one call attempt; the retry policy is the shared worker's.
func (b *embedBackend) CallOnce(ctx context.Context, payload []byte) ([]byte, inferResponse, bool, error) {
	body, resp, retryable, err := b.client.callOnce(ctx, payload)
	if resp == nil {
		return body, nil, retryable, err
	}
	return body, resp, retryable, err
}

func (c *embedClient) callOnce(ctx context.Context, payload []byte) (
	[]byte, *embedApiResponse, bool, error) {

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
			response.Status, embedExplainError(string(body)))
		// A 5xx is the server; a 429 is the server asking to be asked more
		// slowly, which the retry already answers. See inferServerUnavailable.
		if response.StatusCode >= 500 {
			return nil, nil, retryable, unavailable(statusErr)
		}
		return nil, nil, retryable, statusErr
	}
	var resp embedApiResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, nil, false, fmt.Errorf("while parsing the infer server response: %v, response was: %s",
			err, inferTruncate(string(body), 500))
	}
	if len(resp.Error) > 0 {
		// Ollama reports some failures (an unknown model, most notably) with a 200.
		return nil, nil, false, fmt.Errorf("error: the infer server returned: %s", embedExplainError(resp.Error))
	}
	// An empty vector list is a successful response the mappings cannot use; catching it
	// here reports the model rather than letting a required mapping report a missing path.
	if len(resp.Embeddings) == 0 || len(resp.Embeddings[0]) == 0 {
		return nil, nil, false, fmt.Errorf(
			"error: the infer server returned no embedding for model %s, response was: %s",
			resp.Model, inferTruncate(string(body), 500))
	}
	return body, &resp, false, nil
}

// embedExplainError appends the actual cause to the infer server's embeddings refusal.
// Ollama passes through a llama.cpp message naming a server flag, and the flag is not the
// problem: the limit is per model, and it is the model that has to support embeddings.
// Anyone meeting the message unannotated goes looking for a deployment setting that does
// not exist.
func embedExplainError(body string) string {
	const refusal = "does not support embeddings"
	msg := inferTruncate(body, 500)
	if strings.Contains(body, refusal) {
		return msg + " (note: this names a server flag, but the limit is per model - check that " +
			"the model's capabilities include 'embedding', with GET /api/show)"
	}
	return msg
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

func (ctx *BuilderContext) NewEmbedTransformationPipe(source *InputChannel, outputCh *OutputChannel,
	spec *TransformationSpec) (*inferTransformationPipe, error) {

	if spec == nil || spec.EmbedConfig == nil {
		return nil, fmt.Errorf("error: Embed Pipe Transformation spec is missing embed_config element")
	}
	if outputCh == nil {
		return nil, fmt.Errorf("error: embed operator requires an output_channel")
	}
	config := spec.EmbedConfig
	// This operator augments the input record, it never builds a new one.
	spec.NewRecord = false
	applyEmbedDefaults(config)

	if len(config.Model) == 0 {
		return nil, fmt.Errorf("error: embed_config must specify the model")
	}
	if len(config.VectorColumn) == 0 {
		return nil, fmt.Errorf("error: embed_config must specify vector_column, the column receiving the embedding")
	}
	// Reject the promoted fields that an embeddings call has no use for, rather than
	// accepting them and doing nothing: a configuration that sets response_format is
	// asking for something the endpoint does not do, and silence would leave the author
	// believing it took effect.
	for _, unusable := range []struct {
		set  bool
		name string
		why  string
	}{
		{len(config.SystemPrompt) > 0, "system_prompt", "an embeddings call has no system message"},
		{len(config.ResponseFormat) > 0, "response_format", "an embeddings response is a vector, not model text"},
		{config.DisableStripCodeFences, "disable_strip_code_fences", "an embeddings response carries no model text to strip"},
	} {
		if unusable.set {
			return nil, fmt.Errorf("error: embed_config does not support %s: %s", unusable.name, unusable.why)
		}
	}
	if err := validateInferOnError(&config.InferCommonSpec, "embed_config"); err != nil {
		return nil, err
	}
	if err := prependEmbedVectorMapping(config, outputCh); err != nil {
		return nil, err
	}

	client, err := newEmbedClient(config, ctx.env)
	if err != nil {
		return nil, err
	}
	log.Printf("Starting an Embed Worker Pool of size %d, model %s, url %s",
		config.PoolSize, config.Model, client.url)

	return ctx.newInferTransformationPipe(source, outputCh, spec, &config.InferCommonSpec,
		&embedBackend{config: config, client: client},
		inferLabels{
			Pipe:       "EmbedTransformationPipe",
			Operator:   "embed operator",
			Type:       "embed",
			ConfigName: "embed_config",
			ErrPrefix:  fmt.Sprintf("embed operator (model %s)", config.Model),
			Summary:    fmt.Sprintf("EmbedTransformationPipe completed: model %s", config.Model),
		})
}

// prependEmbedVectorMapping puts the vector mapping in front of the configured ones, so
// the configuration never has to name the response path. It also rejects the mapping
// sources that only make sense against model text: response and thinking would both read
// the empty string this backend's Text returns, and `response` with a path is the one
// combination the shared plumbing tries to parse as json.
func prependEmbedVectorMapping(config *EmbedSpec, outputCh *OutputChannel) error {
	for i := range config.OutputMapping {
		mapping := &config.OutputMapping[i]
		switch mapping.Source {
		case inferSourceResponse, inferSourceRawResponse, inferSourceThinking:
			return fmt.Errorf(
				"error: embed_config output_mapping '%s' cannot use source '%s': an embeddings response has no "+
					"model text, map from '%s' (eg path %s or model) or use '%s'",
				mapping.Column, mapping.Source, inferSourceEnvelope, "prompt_eval_count", inferSourceModelName)
		case "":
			// The shared default is `response`, which is unusable here; there is no
			// sensible silent default either, so ask rather than guess.
			return fmt.Errorf(
				"error: embed_config output_mapping '%s' must specify a source: '%s' for a value of the response "+
					"envelope, or '%s'. The embedding vector itself is mapped by vector_column",
				mapping.Column, inferSourceEnvelope, inferSourceModelName)
		}
	}
	if outputCh.Columns != nil {
		if _, ok := (*outputCh.Columns)[config.VectorColumn]; !ok {
			return fmt.Errorf(
				"error: embed_config vector_column '%s' is not a column of the channel '%s', available columns are: %s",
				config.VectorColumn, outputCh.Name, inferColumnNames(*outputCh.Columns))
		}
	}
	vector := InferMappingSpec{
		Comment:   "the embedding vector; synthesized from embed_config.vector_column",
		Column:    config.VectorColumn,
		Source:    inferSourceEnvelope,
		Path:      embedVectorPath,
		AsRdfType: config.VectorAsRdfType,
		Required:  true,
	}
	config.OutputMapping = append([]InferMappingSpec{vector}, config.OutputMapping...)
	return nil
}

func applyEmbedDefaults(config *EmbedSpec) {
	if len(config.KeepAlive) == 0 {
		config.KeepAlive = ollamaDefaultKeepAlive
	}
	applyInferCommonDefaults(&config.InferCommonSpec)
}

func newEmbedClient(config *EmbedSpec, env map[string]any) (*embedClient, error) {
	url, err := resolveInferServerUrl(config.Server, env, "embed_config")
	if err != nil {
		return nil, err
	}
	var headers map[string]string
	if config.Server != nil {
		headers = config.Server.Headers
	}
	return &embedClient{
		url:     strings.TrimSuffix(url, "/") + embedApiPath,
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
