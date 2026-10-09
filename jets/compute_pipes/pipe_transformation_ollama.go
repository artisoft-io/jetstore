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
// Since 15a this file holds only what is Ollama-specific: the request shapes for
// /api/generate and /api/chat, the response envelope, the endpoint and status handling
// of one call attempt, the server url resolution and the model-lifecycle options. The
// operator shell, worker pool, prompt template, response mapping, retry policy and
// on_error handling are the shared inference plumbing of pipe_transformation_infer.go,
// reached through the inferBackend seam.
//
// See pipe_transformation_ollama_design.md for the design notes.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/utils"
)

// Default settings, see OllamaSpec for the associated configuration properties.
const (
	ollamaDefaultKeepAlive = "30m"
)

// ollamaBackend implements the inferBackend seam: build the Ollama request for a
// rendered prompt, and perform one call attempt against the Ollama api.
type ollamaBackend struct {
	config *OllamaSpec
	client *ollamaClient
}

type ollamaClient struct {
	url        string
	headers    map[string]string
	httpClient *http.Client
	timeout    time.Duration
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
// It implements inferResponse for the shared plumbing.
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

// Text returns the model's answer and, when in use, its reasoning text.
func (r *ollamaApiResponse) Text() (string, string) {
	if r.Message != nil {
		return r.Message.Content, r.MessageThinking
	}
	return r.Response, r.Thinking
}

func (r *ollamaApiResponse) Tokens() (int, int) {
	return r.PromptEvalCount, r.EvalCount
}

func (r *ollamaApiResponse) ModelName() string {
	return r.Model
}

func (b *ollamaBackend) BuildRequest(prompt string) ([]byte, error) {
	config := b.config
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

// CallOnce performs one call attempt; the retry policy is the shared worker's.
func (b *ollamaBackend) CallOnce(ctx context.Context, payload []byte) ([]byte, inferResponse, bool, error) {
	body, resp, retryable, err := b.client.callOnce(ctx, payload)
	if resp == nil {
		return body, nil, retryable, err
	}
	return body, resp, retryable, err
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
			response.Status, inferTruncate(string(body), 500))
		// A 5xx is the server; a 429 is the server asking to be asked more
		// slowly, which the retry already answers. See inferServerUnavailable.
		if response.StatusCode >= 500 {
			return nil, nil, retryable, unavailable(statusErr)
		}
		return nil, nil, retryable, statusErr
	}
	var resp ollamaApiResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, nil, false, fmt.Errorf("while parsing the infer server response: %v, response was: %s",
			err, inferTruncate(string(body), 500))
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
	spec *TransformationSpec) (*inferTransformationPipe, error) {

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
	if err := validateInferOnError(&config.InferCommonSpec, "ollama_config"); err != nil {
		return nil, err
	}

	client, err := newOllamaClient(config, ctx.env)
	if err != nil {
		return nil, err
	}
	log.Printf("Starting an Ollama Worker Pool of size %d, model %s, url %s",
		config.PoolSize, config.Model, client.url)

	return ctx.newInferTransformationPipe(source, outputCh, spec, &config.InferCommonSpec,
		&ollamaBackend{config: config, client: client},
		inferLabels{
			Pipe:       "OllamaTransformationPipe",
			Operator:   "ollama operator",
			Type:       "ollama",
			ConfigName: "ollama_config",
			ErrPrefix:  fmt.Sprintf("ollama operator (model %s)", config.Model),
			Summary:    fmt.Sprintf("OllamaTransformationPipe completed: model %s", config.Model),
		})
}

func applyOllamaDefaults(config *OllamaSpec) {
	if len(config.Api) == 0 {
		config.Api = "generate"
	}
	if len(config.KeepAlive) == 0 {
		config.KeepAlive = ollamaDefaultKeepAlive
	}
	applyInferCommonDefaults(&config.InferCommonSpec)
}

func newOllamaClient(config *OllamaSpec, env map[string]any) (*ollamaClient, error) {
	url, err := resolveInferServerUrl(config.Server, env, "ollama_config")
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
		timeout: timeout,
	}, nil
}

// resolveInferServerUrl takes the infer server url from the operator config, then from
// the cpipes env, then from the JETS_INFER_URL environment variable which the deployed
// containers get when the stack is built with BUILD_INFER_SERVICE.
// It is shared by the inference operators; configName names the caller's config element
// in the failure message, so each operator points at its own property.
func resolveInferServerUrl(server *OllamaServerSpec, env map[string]any, configName string) (string, error) {
	if server != nil && len(server.Url) > 0 {
		url := strings.TrimSpace(utils.ReplaceEnvVars(server.Url, env))
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
		"error: cannot determine the infer server url, set %s.server.url in the configuration or "+
			"deploy the stack with BUILD_INFER_SERVICE so JETS_INFER_URL is set on the containers", configName)
}
