package compute_pipes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Test cases for the vllm transformation operator, see pipe_transformation_vllm.go
// The end to end cases run against an httptest server standing in for a vLLM server.
//
// The channel fixtures are the ollama tests' (ollamaTestColumns, ollamaProcessErrorColumns):
// the two operators augment the same shape of record, and a second set would only be a
// second thing to keep in step. What is not shared is anything about the request or the
// response - that is the part this file is about.
//
// Nothing here tests the shared plumbing. The retry policy, the circuit breaker, the
// on_error handling, the cost guard and the mapping sources are covered against the
// ollama operator and reach this one unchanged through the inferBackend seam; repeating
// them would assert that Go dispatches a method call.

// ---------------------------------------------------------------------------
// Test server and pipe runner
// ---------------------------------------------------------------------------

type vllmTestResponse struct {
	status int
	// content is the model's answer, put in choices[0].message.content (chat) or
	// choices[0].text (completions).
	content string
	// finishReason defaults to "stop" when empty.
	finishReason string
	// reasoning is choices[0].message.reasoning_content, when the server runs a
	// reasoning parser.
	reasoning string
	// errorInBody makes the server answer 200 with a failure in the body, the way an
	// OpenAI-compatible server behind a lenient proxy can.
	errorInBody string
	// noChoices makes the server answer 200 with an empty choices list.
	noChoices bool
}

type vllmTestResult struct {
	outputRecords [][]any
	errorRecords  [][]any
	pipelineErrs  []error
}

// vllmTestServer answers the OpenAI-compatible routes with the responses provided, in
// order, and records the request bodies it was sent as decoded maps - a map rather than a
// struct because the operator's `options` are top level and a struct would drop them,
// which is the property several of these tests are about.
func vllmTestServer(t *testing.T, responses []vllmTestResponse) (*httptest.Server, *[]map[string]any, *[]string) {
	t.Helper()
	var mu sync.Mutex
	requests := make([]map[string]any, 0)
	paths := make([]string, 0)
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("while decoding the request: %v", err)
		}
		mu.Lock()
		requests = append(requests, request)
		paths = append(paths, r.URL.Path)
		response := responses[min(callCount, len(responses)-1)]
		callCount++
		mu.Unlock()

		if response.status != 0 && response.status != http.StatusOK {
			w.WriteHeader(response.status)
			_, _ = w.Write([]byte(`{"object":"error","message":"stand in failure","type":"BadRequestError"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		body := map[string]any{
			"id":     "cmpl-1",
			"object": "chat.completion",
			"model":  request["model"],
			"usage": map[string]any{
				"prompt_tokens": 7, "completion_tokens": 42, "total_tokens": 49,
			},
		}
		if len(response.errorInBody) > 0 {
			body["object"] = "error"
			body["message"] = response.errorInBody
		}
		if !response.noChoices {
			finish := response.finishReason
			if len(finish) == 0 {
				finish = "stop"
			}
			choice := map[string]any{"index": 0, "finish_reason": finish}
			if _, isCompletions := request["prompt"]; isCompletions {
				choice["text"] = response.content
			} else {
				message := map[string]any{"role": "assistant", "content": response.content}
				if len(response.reasoning) > 0 {
					message["reasoning_content"] = response.reasoning
				}
				choice["message"] = message
			}
			body["choices"] = []any{choice}
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(server.Close)
	return server, &requests, &paths
}

// runVllmTestPipe builds the operator against the given stand in server, applies the
// records and returns what came out of the output and error channels. It mirrors
// runOllamaTestPipe; see the comments there for why the channels are drained the way
// they are.
func runVllmTestPipe(t *testing.T, serverUrl string, config *VllmSpec, records [][]any) *vllmTestResult {
	t.Helper()

	columnsMap := ollamaTestColumnsMap()
	channelSpec := &ChannelSpec{Name: "claims", Columns: ollamaTestColumns, columnsMap: &columnsMap}
	peColumnsMap := make(map[string]int, len(ollamaProcessErrorColumns))
	for i, c := range ollamaProcessErrorColumns {
		peColumnsMap[c] = i
	}
	peSpec := &ChannelSpec{Name: "process_errors", Columns: ollamaProcessErrorColumns, columnsMap: &peColumnsMap}

	registry := &ChannelRegistry{
		ComputeChannels: map[string]*Channel{
			"claims.out": {Name: "claims.out", Channel: make(chan []any),
				Columns: &columnsMap, Config: channelSpec},
			"process_errors.out": {Name: "process_errors.out", Channel: make(chan []any),
				Columns: &peColumnsMap, Config: peSpec},
		},
		ClosedChannels: make(map[string]bool),
	}
	source := &InputChannel{Name: "claims.in", Columns: &columnsMap, Config: channelSpec}
	builderContext := &BuilderContext{
		cpConfig:        &ComputePipesConfig{},
		channelRegistry: registry,
		done:            make(chan struct{}),
		errCh:           make(chan error, 10),
		env:             make(map[string]any),
	}
	if config.Server == nil {
		config.Server = &OllamaServerSpec{Url: serverUrl}
	}
	spec := &TransformationSpec{Type: "vllm", VllmConfig: config}

	outputCh, err := registry.GetOutputChannel("claims.out")
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := builderContext.NewVllmTransformationPipe(source, outputCh, spec)
	if err != nil {
		t.Fatal(err)
	}

	result := &vllmTestResult{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for record := range registry.ComputeChannels["claims.out"].Channel {
			result.outputRecords = append(result.outputRecords, record)
		}
	}()
	if config.ErrorChannel != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for record := range registry.ComputeChannels["process_errors.out"].Channel {
				result.errorRecords = append(result.errorRecords, record)
			}
		}()
	}

	for i := range records {
		if err = pipe.Apply(&records[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err = pipe.Done(); err != nil {
		t.Fatal(err)
	}
	pipe.Finally()
	registry.CloseChannel("claims.out")
	if config.ErrorChannel != nil {
		registry.CloseChannel("process_errors.out")
	}
	wg.Wait()

	close(builderContext.errCh)
	for err := range builderContext.errCh {
		result.pipelineErrs = append(result.pipelineErrs, err)
	}
	return result
}

// vllmTestConfig is the minimal working configuration.
func vllmTestConfig() *VllmSpec {
	return &VllmSpec{
		Model: "Qwen/Qwen2.5-7B-Instruct",
		InferCommonSpec: InferCommonSpec{
			PromptTemplate: "Classify {{claim_id}}: {{diagnosis}}",
			OutputMapping:  []InferMappingSpec{{Column: "claim_category", Path: "category"}},
		},
	}
}

func vllmTestErrorChannel() *OutputChannelConfig {
	return &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"}
}

// ---------------------------------------------------------------------------
// The response envelope
// ---------------------------------------------------------------------------

// The chat api answers in choices[0].message, the completions api in choices[0].text, and
// inferResponse hides the difference from the shared plumbing.
func TestVllmResponseText(t *testing.T) {
	chat := &vllmApiResponse{Choices: []vllmChoice{{
		Message: &vllmMessage{Role: "assistant", Content: `{"category":"dental"}`, ReasoningContent: "thinking out loud"},
	}}}
	if text, thinking := chat.Text(); text != `{"category":"dental"}` || thinking != "thinking out loud" {
		t.Errorf("the chat response gave (%q, %q)", text, thinking)
	}
	completions := &vllmApiResponse{Choices: []vllmChoice{{Text: "a plain sentence"}}}
	if text, thinking := completions.Text(); text != "a plain sentence" || thinking != "" {
		t.Errorf("the completions response gave (%q, %q)", text, thinking)
	}
	// No choice at all must not panic: the plumbing calls Text before anything has
	// checked the envelope.
	empty := &vllmApiResponse{}
	if text, thinking := empty.Text(); text != "" || thinking != "" {
		t.Errorf("an empty response gave (%q, %q)", text, thinking)
	}
}

// The token counts feed the run counters and come from `usage`, not from the ollama
// property names.
func TestVllmResponseTokens(t *testing.T) {
	resp := &vllmApiResponse{Model: "m", Usage: vllmUsage{PromptTokens: 7, CompletionTokens: 42, TotalTokens: 49}}
	if prompt, eval := resp.Tokens(); prompt != 7 || eval != 42 {
		t.Errorf("expecting (7, 42), got (%d, %d)", prompt, eval)
	}
	if resp.ModelName() != "m" {
		t.Errorf("expecting the model name, got %q", resp.ModelName())
	}
}

// Two error shapes are read: OpenAI nests under `error`, vLLM flattens with
// object: "error" and a top level message.
func TestVllmResponseErrorMessage(t *testing.T) {
	nested := &vllmApiResponse{Error: &vllmError{Message: "model not found", Type: "NotFoundError"}}
	if nested.errorMessage() != "model not found" {
		t.Errorf("the nested shape gave %q", nested.errorMessage())
	}
	flat := &vllmApiResponse{Object: "error", Message: "bad request"}
	if flat.errorMessage() != "bad request" {
		t.Errorf("the flat shape gave %q", flat.errorMessage())
	}
	// A successful response has a top level `object` that is not "error", and its
	// `message` field is absent - nothing here must read a choice's message as a failure.
	ok := &vllmApiResponse{Object: "chat.completion", Choices: []vllmChoice{{
		Message: &vllmMessage{Content: "fine"}}}}
	if ok.errorMessage() != "" {
		t.Errorf("a successful response was read as a failure: %q", ok.errorMessage())
	}
}

// ---------------------------------------------------------------------------
// guided_json is not format
// ---------------------------------------------------------------------------

// The configured response_format is ollama's; what reaches the server is vLLM's. This is
// the whole of the request difference between the two backends, so it is asserted on the
// wire rather than on the config.
func TestVllmSchemaGoesToGuidedJson(t *testing.T) {
	server, requests, _ := vllmTestServer(t, []vllmTestResponse{{content: `{"category":"dental"}`}})
	config := vllmTestConfig()
	config.ResponseFormat = json.RawMessage(`{"type":"object","properties":{"category":{"type":"string"}}}`)
	runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	request := (*requests)[0]
	guided, ok := request["guided_json"].(map[string]any)
	if !ok {
		t.Fatalf("expecting the schema in guided_json, the request was %v", request)
	}
	if guided["type"] != "object" {
		t.Errorf("guided_json is %v", guided)
	}
	if _, present := request["format"]; present {
		t.Error("the request carries `format`, which is ollama's field and not vLLM's")
	}
	if _, present := request["response_format"]; present {
		t.Error("the request carries both guided_json and response_format")
	}
}

// The json_schema arm is the OpenAI-compatible one, for a server that does not take
// guided_json. It has to be named and strict, or it is a hint rather than a constraint.
func TestVllmSchemaGoesToNamedResponseFormat(t *testing.T) {
	server, requests, _ := vllmTestServer(t, []vllmTestResponse{{content: `{"category":"dental"}`}})
	config := vllmTestConfig()
	config.StructuredOutput = vllmStructuredJsonSchema
	config.ResponseFormat = json.RawMessage(`{"type":"object","properties":{"category":{"type":"string"}}}`)
	runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	request := (*requests)[0]
	if _, present := request["guided_json"]; present {
		t.Error("the json_schema arm still sent guided_json")
	}
	format, ok := request["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("expecting a response_format, the request was %v", request)
	}
	if format["type"] != "json_schema" {
		t.Errorf("response_format type is %v", format["type"])
	}
	named, ok := format["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("response_format has no json_schema: %v", format)
	}
	if named["name"] != vllmSchemaName || named["strict"] != true {
		t.Errorf("the schema is not named and strict: %v", named)
	}
	if schema, ok := named["schema"].(map[string]any); !ok || schema["type"] != "object" {
		t.Errorf("the schema did not survive the wrapping: %v", named["schema"])
	}
}

// ollama accepts the string "json" as well as a schema, and a config moved across should
// not have to be rewritten for it. vLLM has no such string, so it becomes json mode.
func TestVllmJsonStringBecomesJsonMode(t *testing.T) {
	server, requests, _ := vllmTestServer(t, []vllmTestResponse{{content: `{"category":"dental"}`}})
	config := vllmTestConfig()
	config.ResponseFormat = json.RawMessage(`"json"`)
	runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	format, ok := (*requests)[0]["response_format"].(map[string]any)
	if !ok || format["type"] != "json_object" {
		t.Errorf("expecting json mode, the request was %v", (*requests)[0])
	}
}

// An unconstrained operator sends neither field: the ollama operator's response_format is
// optional and so is this one.
func TestVllmNoResponseFormatSendsNeitherField(t *testing.T) {
	server, requests, _ := vllmTestServer(t, []vllmTestResponse{{content: `{"category":"dental"}`}})
	runVllmTestPipe(t, server.URL, vllmTestConfig(), [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	request := (*requests)[0]
	if _, present := request["guided_json"]; present {
		t.Error("an unconstrained operator sent guided_json")
	}
	if _, present := request["response_format"]; present {
		t.Error("an unconstrained operator sent response_format")
	}
}

// The translation is done when the operator is built, so an unusable response_format is a
// configuration error rather than a failure on every record.
func TestVllmRejectsUnusableResponseFormat(t *testing.T) {
	for _, tc := range []struct {
		name   string
		format string
		arm    string
		want   string
	}{
		{"a string that is not json", `"yaml"`, "", "response_format"},
		{"a number", `3`, "", "json schema document"},
		{"an array", `[1,2]`, "", "json schema document"},
		{"not json at all", `{oops`, "", "not valid json"},
		{"an unknown arm", `{"type":"object"}`, "guided-json", "structured_output"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := vllmTestConfig()
			config.ResponseFormat = json.RawMessage(tc.format)
			config.StructuredOutput = tc.arm
			err := buildVllmPipe(t, config)
			if err == nil {
				t.Fatalf("expecting %s to be refused", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the error does not name %s: %v", tc.want, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// options are top level
// ---------------------------------------------------------------------------

// The OpenAI api puts the sampling parameters beside `model` rather than under an
// `options` object, so the map is merged into the request body.
func TestVllmOptionsAreMergedAtTopLevel(t *testing.T) {
	server, requests, _ := vllmTestServer(t, []vllmTestResponse{{content: `{"category":"dental"}`}})
	config := vllmTestConfig()
	config.Options = map[string]any{"temperature": 0.0, "max_tokens": 256, "top_k": 20}
	runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	request := (*requests)[0]
	if request["max_tokens"] != float64(256) || request["top_k"] != float64(20) {
		t.Errorf("the options did not reach the top level of the request: %v", request)
	}
	if _, nested := request["options"]; nested {
		t.Error("the request nests the options, which is ollama's shape and not vLLM's")
	}
}

// Either precedence would be wrong, so a collision is refused when the operator is built.
func TestVllmRejectsReservedOptionKeys(t *testing.T) {
	for _, key := range vllmReservedRequestKeys {
		t.Run(key, func(t *testing.T) {
			config := vllmTestConfig()
			config.Options = map[string]any{key: "whatever"}
			err := buildVllmPipe(t, config)
			if err == nil {
				t.Fatalf("expecting the reserved key %s to be refused", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("the error does not name the key: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// End to end
// ---------------------------------------------------------------------------

func TestVllmAugmentsRecordInPlace(t *testing.T) {
	server, requests, paths := vllmTestServer(t, []vllmTestResponse{
		{content: `{"category":"dental","confidence":0.87}`},
	})
	config := vllmTestConfig()
	config.SystemPrompt = "be terse"
	config.OutputMapping = []InferMappingSpec{
		{Column: "claim_category", Path: "category"},
		{Column: "claim_confidence", Path: "confidence", AsRdfType: "double"},
		{Column: "infer_tokens", Source: inferSourceEnvelope, Path: "usage.completion_tokens"},
	}
	result := runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if (*paths)[0] != vllmChatApiPath {
		t.Errorf("the operator called %s, expecting %s", (*paths)[0], vllmChatApiPath)
	}
	messages, ok := (*requests)[0]["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("expecting a system and a user message, got %v", (*requests)[0]["messages"])
	}
	if system := messages[0].(map[string]any); system["role"] != "system" || system["content"] != "be terse" {
		t.Errorf("unexpected system message: %v", system)
	}
	if user := messages[1].(map[string]any); user["content"] != "Classify c-1: tooth ache" {
		t.Errorf("unexpected user message: %v", user)
	}
	if len(result.outputRecords) != 1 {
		t.Fatalf("expecting 1 output record, got %d", len(result.outputRecords))
	}
	record := result.outputRecords[0]
	if record[0] != "c-1" || record[1] != "tooth ache" {
		t.Errorf("the input columns were not preserved: %v", record)
	}
	if record[2] != "dental" {
		t.Errorf("expecting claim_category 'dental', got %v", record[2])
	}
	if record[3] != 0.87 {
		t.Errorf("expecting claim_confidence 0.87 as a double, got %v (%T)", record[3], record[3])
	}
	// The token count is read off the envelope, which is where the OpenAI api puts it.
	if record[4] != float64(42) {
		t.Errorf("expecting the eval token count from usage, got %v", record[4])
	}
}

// The completions api takes a bare prompt on a different route.
func TestVllmCompletionsApi(t *testing.T) {
	server, requests, paths := vllmTestServer(t, []vllmTestResponse{{content: `{"category":"dental"}`}})
	config := vllmTestConfig()
	config.Api = vllmApiCompletions
	result := runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if (*paths)[0] != vllmCompletionsApiPath {
		t.Errorf("the operator called %s, expecting %s", (*paths)[0], vllmCompletionsApiPath)
	}
	if (*requests)[0]["prompt"] != "Classify c-1: tooth ache" {
		t.Errorf("unexpected prompt: %v", (*requests)[0]["prompt"])
	}
	if _, present := (*requests)[0]["messages"]; present {
		t.Error("the completions request carries messages")
	}
	if result.outputRecords[0][2] != "dental" {
		t.Errorf("expecting claim_category 'dental', got %v", result.outputRecords[0][2])
	}
}

// The reasoning text arrives as message.reasoning_content rather than as ollama's
// top level `thinking`, and reaches the record through the same mapping source.
func TestVllmReasoningContentMapping(t *testing.T) {
	server, _, _ := vllmTestServer(t, []vllmTestResponse{
		{content: `{"category":"dental"}`, reasoning: "the diagnosis is a tooth"},
	})
	config := vllmTestConfig()
	config.OutputMapping = []InferMappingSpec{{Column: "claim_category", Source: inferSourceThinking}}
	result := runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if result.outputRecords[0][2] != "the diagnosis is a tooth" {
		t.Errorf("expecting the reasoning text, got %v", result.outputRecords[0][2])
	}
}

// With generation constrained, a response cut off at the token ceiling cannot be valid
// json - and the shared plumbing would report it as "consider setting response_format",
// which is already set and is not the problem.
func TestVllmTruncatedConstrainedGenerationIsReported(t *testing.T) {
	server, _, _ := vllmTestServer(t, []vllmTestResponse{
		{content: `{"category":"den`, finishReason: vllmFinishLength},
	})
	config := vllmTestConfig()
	config.ResponseFormat = json.RawMessage(`{"type":"object"}`)
	config.ErrorChannel = vllmTestErrorChannel()
	result := runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.errorRecords) != 1 {
		t.Fatalf("expecting 1 error record, got %d", len(result.errorRecords))
	}
	msg := inferToString(result.errorRecords[0][5])
	if !strings.Contains(msg, "token ceiling") || !strings.Contains(msg, "max_tokens") {
		t.Errorf("the error does not say what happened or what to change: %q", msg)
	}
}

// Unconstrained, a generation that stopped at the ceiling is a truncated answer rather
// than a broken one, and the operator has no basis for calling it a failure.
func TestVllmTruncatedFreeTextIsNotAnError(t *testing.T) {
	server, _, _ := vllmTestServer(t, []vllmTestResponse{
		{content: "a summary that ran out of", finishReason: vllmFinishLength},
	})
	config := vllmTestConfig()
	config.OutputMapping = []InferMappingSpec{{Column: "claim_category", Source: inferSourceRawResponse}}
	config.ErrorChannel = vllmTestErrorChannel()
	result := runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.errorRecords) != 0 {
		t.Fatalf("an unconstrained truncation was reported as an error: %v", result.errorRecords)
	}
	if result.outputRecords[0][2] != "a summary that ran out of" {
		t.Errorf("expecting the truncated text, got %v", result.outputRecords[0][2])
	}
}

// A 200 with a failure in the body is a failure, the way ollama reports an unknown model.
func TestVllmErrorInSuccessfulResponse(t *testing.T) {
	server, _, _ := vllmTestServer(t, []vllmTestResponse{{errorInBody: "the model does not exist"}})
	config := vllmTestConfig()
	config.ErrorChannel = vllmTestErrorChannel()
	result := runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.errorRecords) != 1 {
		t.Fatalf("expecting 1 error record, got %d", len(result.errorRecords))
	}
	if msg := inferToString(result.errorRecords[0][5]); !strings.Contains(msg, "does not exist") {
		t.Errorf("expecting the server's message to be reported, got %q", msg)
	}
}

// An empty choices list is a successful response the mappings cannot use; naming the
// model here beats letting a required mapping report a missing path.
func TestVllmNoChoiceIsReported(t *testing.T) {
	server, _, _ := vllmTestServer(t, []vllmTestResponse{{noChoices: true}})
	config := vllmTestConfig()
	config.ErrorChannel = vllmTestErrorChannel()
	result := runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.errorRecords) != 1 {
		t.Fatalf("expecting 1 error record, got %d", len(result.errorRecords))
	}
	msg := inferToString(result.errorRecords[0][5])
	if !strings.Contains(msg, "no choice") || !strings.Contains(msg, "Qwen") {
		t.Errorf("the error does not name the model: %q", msg)
	}
}

// A 404 on /v1/* is almost always the url: JETS_INFER_URL names the deployed Ollama
// service, which answers a live 404 to every OpenAI route. A bare status reads like a
// missing model rather than a misdirected operator.
func TestVllm404ExplainsTheUrl(t *testing.T) {
	server, _, _ := vllmTestServer(t, []vllmTestResponse{{status: http.StatusNotFound}})
	config := vllmTestConfig()
	config.ErrorChannel = vllmTestErrorChannel()
	result := runVllmTestPipe(t, server.URL, config, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.errorRecords) != 1 {
		t.Fatalf("expecting 1 error record, got %d", len(result.errorRecords))
	}
	msg := inferToString(result.errorRecords[0][5])
	if !strings.Contains(msg, "JETS_INFER_URL") || !strings.Contains(msg, "server.url") {
		t.Errorf("the 404 was not explained: %q", msg)
	}
}

// ---------------------------------------------------------------------------
// Build time validation
// ---------------------------------------------------------------------------

func buildVllmPipe(t *testing.T, config *VllmSpec) error {
	t.Helper()
	columnsMap := ollamaTestColumnsMap()
	channelSpec := &ChannelSpec{Name: "claims", Columns: ollamaTestColumns, columnsMap: &columnsMap}
	registry := &ChannelRegistry{
		ComputeChannels: map[string]*Channel{
			"claims.out": {Name: "claims.out", Channel: make(chan []any),
				Columns: &columnsMap, Config: channelSpec},
		},
		ClosedChannels: make(map[string]bool),
	}
	source := &InputChannel{Name: "claims.in", Columns: &columnsMap, Config: channelSpec}
	builderContext := &BuilderContext{
		cpConfig:        &ComputePipesConfig{},
		channelRegistry: registry,
		done:            make(chan struct{}),
		errCh:           make(chan error, 10),
		env:             make(map[string]any),
	}
	if config.Server == nil {
		config.Server = &OllamaServerSpec{Url: "http://stand-in:8000"}
	}
	outputCh, err := registry.GetOutputChannel("claims.out")
	if err != nil {
		t.Fatal(err)
	}
	spec := &TransformationSpec{Type: "vllm", VllmConfig: config}
	pipe, err := builderContext.NewVllmTransformationPipe(source, outputCh, spec)
	if pipe != nil {
		pipe.Finally()
	}
	return err
}

func TestVllmRequiresModel(t *testing.T) {
	config := vllmTestConfig()
	config.Model = ""
	if err := buildVllmPipe(t, config); err == nil || !strings.Contains(err.Error(), "model") {
		t.Errorf("expecting a missing model error, got %v", err)
	}
}

func TestVllmUnknownApi(t *testing.T) {
	config := vllmTestConfig()
	config.Api = "generate"
	err := buildVllmPipe(t, config)
	if err == nil {
		t.Fatal("expecting an error for an unknown api")
	}
	// 'generate' is ollama's token and the likeliest thing to be carried across, so the
	// message has to list what this operator does take.
	if !strings.Contains(err.Error(), vllmApiChat) || !strings.Contains(err.Error(), vllmApiCompletions) {
		t.Errorf("the error does not list the apis: %v", err)
	}
}

// The completions api has no message roles, so a system prompt has nowhere to go. Folding
// it into the prompt would be a silent reinterpretation.
func TestVllmSystemPromptRequiresChat(t *testing.T) {
	config := vllmTestConfig()
	config.Api = vllmApiCompletions
	config.SystemPrompt = "be terse"
	err := buildVllmPipe(t, config)
	if err == nil {
		t.Fatal("expecting system_prompt with the completions api to be refused")
	}
	if !strings.Contains(err.Error(), "system_prompt") {
		t.Errorf("the error does not name system_prompt: %v", err)
	}
}

func TestVllmMissingConfig(t *testing.T) {
	builderContext := &BuilderContext{cpConfig: &ComputePipesConfig{}}
	_, err := builderContext.NewVllmTransformationPipe(nil, nil, &TransformationSpec{Type: "vllm"})
	if err == nil || !strings.Contains(err.Error(), "vllm_config") {
		t.Errorf("expecting a missing vllm_config error, got %v", err)
	}
}

func TestVllmDefaults(t *testing.T) {
	config := &VllmSpec{Model: "m"}
	applyVllmDefaults(config)
	if config.Api != vllmApiChat {
		t.Errorf("api defaulted to %q", config.Api)
	}
	if config.StructuredOutput != vllmStructuredGuidedJson {
		t.Errorf("structured_output defaulted to %q", config.StructuredOutput)
	}
	if config.PoolSize != 1 {
		t.Errorf("pool_size defaulted to %d", config.PoolSize)
	}
	if config.OnError != OnErrorPassThrough {
		t.Errorf("on_error defaulted to %q", config.OnError)
	}
	if config.MaxRetry == nil || *config.MaxRetry != inferDefaultMaxRetry {
		t.Errorf("max_retry defaulted to %v", config.MaxRetry)
	}
}

// The error channel has to be reachable through errorChannelConfig, or the pipe executor
// never closes it and the pipeline hangs at the end of the stage.
func TestVllmErrorChannelConfig(t *testing.T) {
	spec := &TransformationSpec{Type: "vllm", VllmConfig: &VllmSpec{
		InferCommonSpec: InferCommonSpec{ErrorChannel: vllmTestErrorChannel()},
	}}
	channel := errorChannelConfig(spec)
	if channel == nil || channel.Name != "process_errors.out" {
		t.Errorf("errorChannelConfig returned %v for a vllm operator", channel)
	}
	// And nil when the operator has none, rather than a panic.
	if errorChannelConfig(&TransformationSpec{Type: "vllm"}) != nil {
		t.Error("expecting nil for a vllm operator with no config")
	}
}

// The url resolution is the ollama operator's, so vllm_config.server.url wins over the
// env and the failure message names this operator's property rather than ollama's.
func TestVllmServerUrlResolution(t *testing.T) {
	env := map[string]any{"$JETS_INFER_URL": "http://from-env:8000"}
	url, err := resolveInferServerUrl(&OllamaServerSpec{Url: "http://explicit:8000"}, env, "vllm_config")
	if err != nil || url != "http://explicit:8000" {
		t.Errorf("expecting the explicit url, got %q (%v)", url, err)
	}
	url, err = resolveInferServerUrl(nil, env, "vllm_config")
	if err != nil || url != "http://from-env:8000" {
		t.Errorf("expecting the env url, got %q (%v)", url, err)
	}
	_, err = resolveInferServerUrl(nil, map[string]any{}, "vllm_config")
	if err == nil || !strings.Contains(err.Error(), "vllm_config") {
		t.Errorf("the failure must name this operator's property, got %v", err)
	}
}
