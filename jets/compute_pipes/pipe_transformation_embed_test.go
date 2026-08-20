package compute_pipes

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Test cases for the embed transformation operator, see pipe_transformation_embed.go.
// The end to end cases run against an httptest server standing in for the infer server.
//
// The channel fixtures are the ollama tests' (ollamaTestColumns, ollamaProcessErrorColumns):
// the two operators share the shared plumbing's channel handling, and reusing the columns
// keeps the difference between the two files to what is actually different.

// ---------------------------------------------------------------------------
// The stand in server and the test pipe
// ---------------------------------------------------------------------------

type embedTestResponse struct {
	status int
	// vector is the embedding returned; empty means the response carries an empty
	// embeddings list, which the backend rejects.
	vector []float64
	// errorInBody makes the server answer 200 with an `error` property, the way ollama
	// reports an unknown model.
	errorInBody string
}

type embedTestResult struct {
	outputRecords [][]any
	errorRecords  [][]any
	inputs        []string
	requests      []embedRequest
	pipelineErrs  []error
}

// embedTestServer answers /api/embed with the vectors provided, in order, and records the
// requests it was sent. A status of 0 means 200.
func embedTestServer(t *testing.T, responses []embedTestResponse) (*httptest.Server, *[]embedRequest) {
	t.Helper()
	var mu sync.Mutex
	requests := make([]embedRequest, 0)
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != embedApiPath {
			t.Errorf("the operator called %s, expecting %s", r.URL.Path, embedApiPath)
		}
		var request embedRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("while decoding the request: %v", err)
		}
		mu.Lock()
		requests = append(requests, request)
		response := responses[min(callCount, len(responses)-1)]
		callCount++
		mu.Unlock()

		if response.status != 0 && response.status != http.StatusOK {
			w.WriteHeader(response.status)
			_, _ = w.Write([]byte(`{"error":"stand in failure"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		body := map[string]any{
			"model":             request.Model,
			"total_duration":    1000,
			"load_duration":     100,
			"prompt_eval_count": 7,
		}
		if len(response.errorInBody) > 0 {
			body["error"] = response.errorInBody
		}
		if response.vector == nil {
			body["embeddings"] = []any{}
		} else {
			body["embeddings"] = [][]float64{response.vector}
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

// runEmbedTestPipe builds the operator against the given stand in server, applies the
// records and returns what came out of the output and error channels. It mirrors
// runOllamaTestPipe; see the comments there for why the channels are drained the way
// they are.
func runEmbedTestPipe(t *testing.T, serverUrl string, config *EmbedSpec, spec *TransformationSpec,
	records [][]any) *embedTestResult {
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
	if spec == nil {
		spec = &TransformationSpec{Type: "embed"}
	}
	spec.EmbedConfig = config

	outputCh, err := registry.GetOutputChannel("claims.out")
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := builderContext.NewEmbedTransformationPipe(source, outputCh, spec)
	if err != nil {
		t.Fatal(err)
	}

	result := &embedTestResult{}
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

// embedTestConfig is the minimal working configuration: a template, a model and the
// column the vector lands in.
func embedTestConfig() *EmbedSpec {
	return &EmbedSpec{
		Model:        "nomic-embed-text",
		VectorColumn: "claim_category",
		InferCommonSpec: InferCommonSpec{
			PromptTemplate: "{{diagnosis}}",
		},
	}
}

// ---------------------------------------------------------------------------
// The response envelope
// ---------------------------------------------------------------------------

// The vector reaches the record through the `envelope` mapping source, so nothing arrives
// through Text(); that is the property the whole operator rests on.
func TestEmbedResponseCarriesNoText(t *testing.T) {
	resp := &embedApiResponse{Model: "nomic-embed-text", PromptEvalCount: 7,
		Embeddings: [][]float64{{0.1, 0.2}}}
	text, thinking := resp.Text()
	if text != "" || thinking != "" {
		t.Errorf("Text returned (%q, %q), expecting two empty strings", text, thinking)
	}
}

// An embeddings call evaluates the input and generates nothing, and the response has no
// eval_count; the run counters must see that as zero rather than as a missing value.
func TestEmbedResponseTokens(t *testing.T) {
	resp := &embedApiResponse{PromptEvalCount: 7}
	prompt, eval := resp.Tokens()
	if prompt != 7 || eval != 0 {
		t.Errorf("Tokens returned (%d, %d), expecting (7, 0)", prompt, eval)
	}
}

func TestEmbedResponseModelName(t *testing.T) {
	resp := &embedApiResponse{Model: "nomic-embed-text"}
	if resp.ModelName() != "nomic-embed-text" {
		t.Errorf("ModelName returned %q", resp.ModelName())
	}
}

// The vector path is fixed and the operator's synthesized mapping has to agree with the
// envelope the server actually returns; if either moves, this fails.
func TestEmbedVectorPathMatchesTheEnvelope(t *testing.T) {
	body := []byte(`{"model":"m","embeddings":[[0.5,-0.25]],"prompt_eval_count":7}`)
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	value, found := inferWalkPath(envelope, strings.Split(embedVectorPath, "."))
	if !found {
		t.Fatalf("path %s did not resolve in the response envelope", embedVectorPath)
	}
	vector, ok := value.([]any)
	if !ok {
		t.Fatalf("path %s resolved to %T, expecting []any", embedVectorPath, value)
	}
	if len(vector) != 2 || vector[0].(float64) != 0.5 || vector[1].(float64) != -0.25 {
		t.Errorf("path %s resolved to %v", embedVectorPath, vector)
	}
}

// ---------------------------------------------------------------------------
// The request
// ---------------------------------------------------------------------------

func TestEmbedBuildRequest(t *testing.T) {
	truncate := false
	backend := &embedBackend{config: &EmbedSpec{
		Model:     "nomic-embed-text",
		Truncate:  &truncate,
		KeepAlive: "30m",
		Options:   map[string]any{"num_ctx": 8192},
	}}
	payload, err := backend.BuildRequest("the text to embed")
	if err != nil {
		t.Fatal(err)
	}
	var request map[string]any
	if err = json.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	if request["model"] != "nomic-embed-text" {
		t.Errorf("model is %v", request["model"])
	}
	// Sent as a string rather than a list of one: ollama answers either form with a list
	// of one vector, and the string is what the measurement was taken against.
	if request["input"] != "the text to embed" {
		t.Errorf("input is %v (%T), expecting the rendered text as a string", request["input"], request["input"])
	}
	if request["truncate"] != false {
		t.Errorf("truncate is %v, expecting false to be sent rather than omitted", request["truncate"])
	}
	if request["keep_alive"] != "30m" {
		t.Errorf("keep_alive is %v", request["keep_alive"])
	}
	if _, ok := request["prompt"]; ok {
		t.Error("the request carries a `prompt` property, the embeddings api takes `input`")
	}
}

// truncate is a *bool so that the unset case sends nothing and lets ollama default it,
// while an explicit false is transmitted; omitempty on a bool could not tell them apart.
func TestEmbedBuildRequestOmitsUnsetTruncate(t *testing.T) {
	backend := &embedBackend{config: &EmbedSpec{Model: "m"}}
	payload, err := backend.BuildRequest("text")
	if err != nil {
		t.Fatal(err)
	}
	var request map[string]any
	if err = json.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	if _, ok := request["truncate"]; ok {
		t.Errorf("truncate was sent as %v when the configuration left it unset", request["truncate"])
	}
}

// The endpoint is forced, and the test asserts it against the url the client builds
// rather than against the constant, so a change to newEmbedClient is caught too.
func TestEmbedClientUsesTheEmbedEndpoint(t *testing.T) {
	config := embedTestConfig()
	config.Server = &OllamaServerSpec{Url: "http://infer:11434/"}
	applyEmbedDefaults(config)
	client, err := newEmbedClient(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	if client.url != "http://infer:11434/api/embed" {
		t.Errorf("client url is %s, expecting the trailing slash trimmed and /api/embed appended", client.url)
	}
}

func TestEmbedMissingServerUrl(t *testing.T) {
	config := embedTestConfig()
	applyEmbedDefaults(config)
	_, err := newEmbedClient(config, nil)
	if err == nil {
		t.Fatal("expecting an error when no server url can be resolved")
	}
	// The message must point at this operator's property, not the ollama one.
	if !strings.Contains(err.Error(), "embed_config.server.url") {
		t.Errorf("the error does not name embed_config.server.url: %v", err)
	}
}

// ---------------------------------------------------------------------------
// End to end
// ---------------------------------------------------------------------------

func TestEmbedPutsTheVectorOnTheRecord(t *testing.T) {
	server, requests := embedTestServer(t, []embedTestResponse{{vector: []float64{0.5, -0.25, 0.125}}})
	result := runEmbedTestPipe(t, server.URL, embedTestConfig(), nil,
		[][]any{{"c1", "chest pain", nil, nil, nil}})

	if len(result.outputRecords) != 1 {
		t.Fatalf("got %d output records, expecting 1", len(result.outputRecords))
	}
	// With no as_rdf_type the shared plumbing encodes the array back to json text, which
	// is what a text or csv output channel can carry.
	got := result.outputRecords[0][2]
	if got != "[0.5,-0.25,0.125]" {
		t.Errorf("the vector column holds %#v, expecting the vector as json text", got)
	}
	// The record is augmented in place: the input columns survive.
	if result.outputRecords[0][0] != "c1" || result.outputRecords[0][1] != "chest pain" {
		t.Errorf("the input columns were not preserved: %v", result.outputRecords[0])
	}
	if len(*requests) != 1 || (*requests)[0].Input != "chest pain" {
		t.Errorf("the server was sent %+v, expecting the rendered template as the input", *requests)
	}
}

// vector_as_rdf_type is how a caller asks for the numbers rather than json text; a
// similarity search needs them, and json text would make every consumer parse it back.
func TestEmbedVectorAsTypedArray(t *testing.T) {
	server, _ := embedTestServer(t, []embedTestResponse{{vector: []float64{0.5, -0.25, 0.125}}})
	config := embedTestConfig()
	config.VectorAsRdfType = "double"
	result := runEmbedTestPipe(t, server.URL, config, nil, [][]any{{"c1", "chest pain", nil, nil, nil}})

	if len(result.outputRecords) != 1 {
		t.Fatalf("got %d output records", len(result.outputRecords))
	}
	vector, ok := result.outputRecords[0][2].([]any)
	if !ok {
		t.Fatalf("the vector column holds %T, expecting []any", result.outputRecords[0][2])
	}
	if len(vector) != 3 {
		t.Fatalf("the vector has %d elements, expecting 3", len(vector))
	}
	for i, want := range []float64{0.5, -0.25, 0.125} {
		got, ok := vector[i].(float64)
		if !ok || got != want {
			t.Errorf("element %d is %#v, expecting the float64 %v", i, vector[i], want)
		}
	}
}

// The envelope values worth keeping are reachable, and the synthesized vector mapping
// does not displace them.
func TestEmbedAdditionalMappings(t *testing.T) {
	server, _ := embedTestServer(t, []embedTestResponse{{vector: []float64{1, 0}}})
	config := embedTestConfig()
	config.OutputMapping = []InferMappingSpec{
		{Column: "claim_confidence", Source: inferSourceModelName},
		{Column: "infer_tokens", Source: inferSourceEnvelope, Path: "prompt_eval_count"},
	}
	result := runEmbedTestPipe(t, server.URL, config, nil, [][]any{{"c1", "chest pain", nil, nil, nil}})

	if len(result.outputRecords) != 1 {
		t.Fatalf("got %d output records", len(result.outputRecords))
	}
	record := result.outputRecords[0]
	if record[2] != "[1,0]" {
		t.Errorf("the vector column holds %#v", record[2])
	}
	if record[3] != "nomic-embed-text" {
		t.Errorf("the model_name mapping produced %#v", record[3])
	}
	if record[4] != float64(7) {
		t.Errorf("the envelope mapping produced %#v, expecting prompt_eval_count", record[4])
	}
}

// A configuration must not have to know that the vector lives at embeddings.0, and the
// synthesized mapping is what saves it from that; it is required, so a response without
// the path is a row level error rather than a silently unset column.
func TestEmbedEmptyVectorIsAnError(t *testing.T) {
	server, _ := embedTestServer(t, []embedTestResponse{{vector: []float64{}}})
	config := embedTestConfig()
	config.ErrorChannel = &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"}
	config.RowKeyColumn = "claim_id"
	result := runEmbedTestPipe(t, server.URL, config, nil, [][]any{{"c1", "chest pain", nil, nil, nil}})

	if len(result.errorRecords) != 1 {
		t.Fatalf("got %d error records, expecting 1", len(result.errorRecords))
	}
	message, _ := result.errorRecords[0][5].(string)
	if !strings.Contains(message, "no embedding") {
		t.Errorf("the error message is %q, expecting it to report the missing embedding", message)
	}
	// on_error defaults to pass_through: the record still comes out, unchanged.
	if len(result.outputRecords) != 1 {
		t.Fatalf("got %d output records, expecting the record to pass through", len(result.outputRecords))
	}
}

// The refusal ollama passes through names a server flag and the actual limit is per
// model; the operator annotates it rather than repeating a message that sends the reader
// after a deployment setting that does not exist.
func TestEmbedAnnotatesTheCapabilityRefusal(t *testing.T) {
	server, _ := embedTestServer(t, []embedTestResponse{
		{errorInBody: "This server does not support embeddings. Start it with `--embeddings`"}})
	config := embedTestConfig()
	config.ErrorChannel = &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"}
	config.RowKeyColumn = "claim_id"
	maxRetry := 0
	config.MaxRetry = &maxRetry
	result := runEmbedTestPipe(t, server.URL, config, nil, [][]any{{"c1", "chest pain", nil, nil, nil}})

	if len(result.errorRecords) != 1 {
		t.Fatalf("got %d error records, expecting 1", len(result.errorRecords))
	}
	message, _ := result.errorRecords[0][5].(string)
	if !strings.Contains(message, "/api/show") {
		t.Errorf("the error message is %q, expecting it to point at the model's capabilities", message)
	}
}

func TestEmbedRetriesServerError(t *testing.T) {
	server, requests := embedTestServer(t, []embedTestResponse{
		{status: http.StatusInternalServerError},
		{vector: []float64{1, 0}},
	})
	config := embedTestConfig()
	config.RetryWaitSec = 1
	result := runEmbedTestPipe(t, server.URL, config, nil, [][]any{{"c1", "chest pain", nil, nil, nil}})

	if len(*requests) != 2 {
		t.Errorf("the server was called %d times, expecting a retry after the 500", len(*requests))
	}
	if len(result.outputRecords) != 1 || result.outputRecords[0][2] != "[1,0]" {
		t.Errorf("the retry did not produce the vector: %v", result.outputRecords)
	}
}

func TestEmbedPreservesOrderWithSingleWorker(t *testing.T) {
	server, _ := embedTestServer(t, []embedTestResponse{{vector: []float64{1, 0}}})
	records := [][]any{
		{"c1", "one", nil, nil, nil},
		{"c2", "two", nil, nil, nil},
		{"c3", "three", nil, nil, nil},
	}
	result := runEmbedTestPipe(t, server.URL, embedTestConfig(), nil, records)
	if len(result.outputRecords) != 3 {
		t.Fatalf("got %d output records", len(result.outputRecords))
	}
	for i, want := range []string{"c1", "c2", "c3"} {
		if result.outputRecords[i][0] != want {
			t.Errorf("record %d is %v, expecting %s - the default pool of one preserves order",
				i, result.outputRecords[i][0], want)
		}
	}
}

// ---------------------------------------------------------------------------
// Build time validation
// ---------------------------------------------------------------------------

func buildEmbedPipe(t *testing.T, config *EmbedSpec) error {
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
		config.Server = &OllamaServerSpec{Url: "http://stand-in:11434"}
	}
	outputCh, err := registry.GetOutputChannel("claims.out")
	if err != nil {
		t.Fatal(err)
	}
	spec := &TransformationSpec{Type: "embed", EmbedConfig: config}
	pipe, err := builderContext.NewEmbedTransformationPipe(source, outputCh, spec)
	if pipe != nil {
		pipe.Finally()
	}
	return err
}

func TestEmbedRequiresModelAndVectorColumn(t *testing.T) {
	config := embedTestConfig()
	config.Model = ""
	if err := buildEmbedPipe(t, config); err == nil || !strings.Contains(err.Error(), "model") {
		t.Errorf("expecting a missing model error, got %v", err)
	}
	config = embedTestConfig()
	config.VectorColumn = ""
	if err := buildEmbedPipe(t, config); err == nil || !strings.Contains(err.Error(), "vector_column") {
		t.Errorf("expecting a missing vector_column error, got %v", err)
	}
}

// The vector column is checked against the channel at build time, with the available
// columns listed - the same treatment output_mapping columns get, and the text a repair
// prompt can act on.
func TestEmbedUnknownVectorColumn(t *testing.T) {
	config := embedTestConfig()
	config.VectorColumn = "no_such_column"
	err := buildEmbedPipe(t, config)
	if err == nil {
		t.Fatal("expecting an error for an unknown vector_column")
	}
	if !strings.Contains(err.Error(), "available columns are") || !strings.Contains(err.Error(), "diagnosis") {
		t.Errorf("the error does not list the available columns: %v", err)
	}
}

// The three promoted fields an embeddings call cannot use are refused rather than
// ignored: silence would leave the author believing they took effect.
func TestEmbedRejectsUnusableCommonFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		apply func(*EmbedSpec)
		want  string
	}{
		{"system_prompt", func(c *EmbedSpec) { c.SystemPrompt = "you are a helpful assistant" }, "system_prompt"},
		{"response_format", func(c *EmbedSpec) { c.ResponseFormat = json.RawMessage(`"json"`) }, "response_format"},
		{"disable_strip_code_fences", func(c *EmbedSpec) { c.DisableStripCodeFences = true }, "disable_strip_code_fences"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := embedTestConfig()
			tc.apply(config)
			err := buildEmbedPipe(t, config)
			if err == nil {
				t.Fatalf("expecting %s to be refused", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the error does not name %s: %v", tc.want, err)
			}
		})
	}
}

// The mapping sources that read model text are refused for the same reason: they would
// silently map the empty string this backend's Text returns.
func TestEmbedRejectsTextMappingSources(t *testing.T) {
	for _, source := range []string{inferSourceResponse, inferSourceRawResponse, inferSourceThinking} {
		t.Run(source, func(t *testing.T) {
			config := embedTestConfig()
			config.OutputMapping = []InferMappingSpec{{Column: "claim_confidence", Source: source}}
			err := buildEmbedPipe(t, config)
			if err == nil {
				t.Fatalf("expecting source %s to be refused", source)
			}
			if !strings.Contains(err.Error(), source) {
				t.Errorf("the error does not name the source: %v", err)
			}
		})
	}
}

// The shared default source is `response`, which is unusable here, so an omitted source
// is an error rather than a silent default.
func TestEmbedRejectsOmittedMappingSource(t *testing.T) {
	config := embedTestConfig()
	config.OutputMapping = []InferMappingSpec{{Column: "claim_confidence"}}
	err := buildEmbedPipe(t, config)
	if err == nil {
		t.Fatal("expecting an omitted source to be refused")
	}
	if !strings.Contains(err.Error(), "must specify a source") {
		t.Errorf("unexpected error: %v", err)
	}
}

// The synthesized mapping goes in front of the configured ones and does not replace them.
func TestEmbedSynthesizedMappingIsPrepended(t *testing.T) {
	config := embedTestConfig()
	config.OutputMapping = []InferMappingSpec{{Column: "infer_tokens", Source: inferSourceModelName}}
	if err := buildEmbedPipe(t, config); err != nil {
		t.Fatal(err)
	}
	if len(config.OutputMapping) != 2 {
		t.Fatalf("output_mapping has %d entries, expecting the vector mapping plus the configured one",
			len(config.OutputMapping))
	}
	vector := config.OutputMapping[0]
	if vector.Column != "claim_category" || vector.Source != inferSourceEnvelope ||
		vector.Path != embedVectorPath || !vector.Required {
		t.Errorf("the synthesized vector mapping is %+v", vector)
	}
	if config.OutputMapping[1].Column != "infer_tokens" {
		t.Errorf("the configured mapping was displaced: %+v", config.OutputMapping[1])
	}
}

// An embed operator with no output_mapping at all is the common case, and the shared
// plumbing refuses an empty one - so the synthesis has to happen before it is compiled.
func TestEmbedNeedsNoOutputMapping(t *testing.T) {
	config := embedTestConfig()
	config.OutputMapping = nil
	if err := buildEmbedPipe(t, config); err != nil {
		t.Errorf("a configuration with no output_mapping was refused: %v", err)
	}
}

func TestEmbedDefaults(t *testing.T) {
	config := &EmbedSpec{Model: "m", VectorColumn: "claim_category"}
	applyEmbedDefaults(config)
	if config.KeepAlive != ollamaDefaultKeepAlive {
		t.Errorf("keep_alive defaulted to %q", config.KeepAlive)
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

func TestEmbedMissingConfig(t *testing.T) {
	builderContext := &BuilderContext{cpConfig: &ComputePipesConfig{}}
	_, err := builderContext.NewEmbedTransformationPipe(nil, nil, &TransformationSpec{Type: "embed"})
	if err == nil || !strings.Contains(err.Error(), "embed_config") {
		t.Errorf("expecting a missing embed_config error, got %v", err)
	}
}

// The error channel has to be reachable through errorChannelConfig, or the pipe executor
// never closes it and the pipeline hangs at the end of the stage.
func TestEmbedErrorChannelConfig(t *testing.T) {
	spec := &TransformationSpec{Type: "embed", EmbedConfig: &EmbedSpec{
		InferCommonSpec: InferCommonSpec{
			ErrorChannel: &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"},
		},
	}}
	channel := errorChannelConfig(spec)
	if channel == nil || channel.Name != "process_errors.out" {
		t.Errorf("errorChannelConfig returned %v for an embed operator", channel)
	}
	// And nil when the operator has none, rather than a panic.
	if errorChannelConfig(&TransformationSpec{Type: "embed"}) != nil {
		t.Error("expecting nil for an embed operator with no config")
	}
}

// ---------------------------------------------------------------------------
// The normalisation the endpoint choice carries with it
// ---------------------------------------------------------------------------

// /api/embed returns an L2 normalised vector where the legacy endpoint returns the raw
// one. This does not test ollama; it records what the operator's consumers may assume,
// so that a future change of endpoint fails here rather than silently degrading a
// similarity search. See the endpoint note in pipe_transformation_embed.go.
func TestEmbedEndpointIsTheNormalisingOne(t *testing.T) {
	if embedApiPath != "/api/embed" {
		t.Fatalf("the endpoint is %s: it is /api/embed that returns an L2 normalised vector, and a "+
			"corpus embedded through both endpoints compares wrongly under dot product", embedApiPath)
	}
	// A sanity check on the arithmetic the consumers rely on: with a normalised vector,
	// cosine similarity and dot product coincide.
	vector := []float64{0.6, 0.8}
	var norm float64
	for _, v := range vector {
		norm += v * v
	}
	if math.Abs(math.Sqrt(norm)-1.0) > 1e-9 {
		t.Errorf("the fixture is not normalised, norm is %f", math.Sqrt(norm))
	}
}
