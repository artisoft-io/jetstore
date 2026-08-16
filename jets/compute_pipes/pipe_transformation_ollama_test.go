package compute_pipes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Test cases for the ollama transformation operator, see pipe_transformation_ollama.go
// The end to end cases run against an httptest server standing in for the infer server.

// ---------------------------------------------------------------------------
// Prompt template
// ---------------------------------------------------------------------------

var ollamaTestColumns = []string{"claim_id", "diagnosis", "claim_category", "claim_confidence", "infer_tokens"}

func ollamaTestColumnsMap() map[string]int {
	m := make(map[string]int, len(ollamaTestColumns))
	for i, c := range ollamaTestColumns {
		m[c] = i
	}
	return m
}

func TestOllamaPromptTemplateRender(t *testing.T) {
	template, err := compileOllamaPromptTemplate(
		"Classify claim {{claim_id}}.\nDiagnosis: {{ diagnosis }}\n", ollamaTestColumnsMap())
	if err != nil {
		t.Fatal(err)
	}
	record := []any{"c-1", "tooth ache", nil, nil, nil}
	got, err := template.render(&record, ollamaTestColumns)
	if err != nil {
		t.Fatal(err)
	}
	expecting := "Classify claim c-1.\nDiagnosis: tooth ache\n"
	if got != expecting {
		t.Errorf("expecting %q, got %q", expecting, got)
	}
}

// A short record is legitimate (see pad_short_rows_with_nulls), the missing value must
// render as empty rather than panic.
func TestOllamaPromptTemplateShortRecord(t *testing.T) {
	template, err := compileOllamaPromptTemplate("[{{claim_id}}][{{diagnosis}}]", ollamaTestColumnsMap())
	if err != nil {
		t.Fatal(err)
	}
	record := []any{"c-1"}
	got, err := template.render(&record, ollamaTestColumns)
	if err != nil {
		t.Fatal(err)
	}
	if got != "[c-1][]" {
		t.Errorf("expecting %q, got %q", "[c-1][]", got)
	}
}

func TestOllamaPromptTemplateRecordPlaceholder(t *testing.T) {
	template, err := compileOllamaPromptTemplate("Record: {{@record}}", ollamaTestColumnsMap())
	if err != nil {
		t.Fatal(err)
	}
	record := []any{"c-1", "tooth ache", nil, nil, nil}
	got, err := template.render(&record, ollamaTestColumns)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"claim_id":"c-1"`) || !strings.Contains(got, `"diagnosis":"tooth ache"`) {
		t.Errorf("expecting the record as json, got %q", got)
	}
}

func TestOllamaPromptTemplateUnknownColumn(t *testing.T) {
	_, err := compileOllamaPromptTemplate("Hello {{not_a_column}}", ollamaTestColumnsMap())
	if err == nil {
		t.Fatal("expecting an error for an unknown column")
	}
	if !strings.Contains(err.Error(), "not_a_column") || !strings.Contains(err.Error(), "claim_id") {
		t.Errorf("the error must name the placeholder and list the available columns, got: %v", err)
	}
}

func TestOllamaPromptTemplateUnterminated(t *testing.T) {
	_, err := compileOllamaPromptTemplate("Hello {{claim_id", ollamaTestColumnsMap())
	if err == nil {
		t.Fatal("expecting an error for an unterminated placeholder")
	}
}

func TestOllamaPromptTemplateEmpty(t *testing.T) {
	if _, err := compileOllamaPromptTemplate("   ", ollamaTestColumnsMap()); err == nil {
		t.Fatal("expecting an error for an empty template")
	}
}

// ---------------------------------------------------------------------------
// Dot notation path
// ---------------------------------------------------------------------------

func TestOllamaWalkPath(t *testing.T) {
	var parsed any
	err := json.Unmarshal([]byte(`{
		"category": "dental",
		"detail": {"score": 0.87, "codes": [{"icd10": "K08.9"}, {"icd10": "Z01.20"}]},
		"empty": null
	}`), &parsed)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		path      string
		expecting any
		found     bool
	}{
		{"category", "dental", true},
		{"detail.score", 0.87, true},
		{"detail.codes.1.icd10", "Z01.20", true},
		{"detail.codes.9.icd10", nil, false},
		{"detail.codes.x.icd10", nil, false},
		{"not_there", nil, false},
		{"category.nested", nil, false},
		{"empty", nil, false},
	}
	for _, test := range tests {
		value, found := ollamaWalkPath(parsed, strings.Split(test.path, "."))
		if found != test.found {
			t.Errorf("path %s: expecting found=%v, got %v", test.path, test.found, found)
			continue
		}
		if found && value != test.expecting {
			t.Errorf("path %s: expecting %v, got %v", test.path, test.expecting, value)
		}
	}
}

// ---------------------------------------------------------------------------
// Code fences
// ---------------------------------------------------------------------------

func TestOllamaStripCodeFences(t *testing.T) {
	tests := []struct{ in, expecting string }{
		{"{\"a\":1}", "{\"a\":1}"},
		{"```json\n{\"a\":1}\n```", "{\"a\":1}"},
		{"```\n{\"a\":1}\n```", "{\"a\":1}"},
		{"  ```JSON\n{\"a\":1}\n```  ", "{\"a\":1}"},
	}
	for _, test := range tests {
		if got := ollamaStripCodeFences(test.in); got != test.expecting {
			t.Errorf("input %q: expecting %q, got %q", test.in, test.expecting, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Configuration validation
// ---------------------------------------------------------------------------

func TestOllamaResolveTemplate(t *testing.T) {
	cpConfig := &ComputePipesConfig{
		PromptTemplates: []PromptTemplateSpec{
			{Key: "classify", Template: "classify {{claim_id}}", SystemPrompt: "be terse"},
		},
	}
	// By name, and the named template's defaults are applied
	config := &OllamaSpec{PromptTemplateName: "classify"}
	template, err := resolveOllamaTemplate(cpConfig, config)
	if err != nil {
		t.Fatal(err)
	}
	if template != "classify {{claim_id}}" || config.SystemPrompt != "be terse" {
		t.Errorf("unexpected template %q / system prompt %q", template, config.SystemPrompt)
	}
	// The operator's own setting wins over the template's default
	config = &OllamaSpec{PromptTemplateName: "classify", SystemPrompt: "mine"}
	if _, err = resolveOllamaTemplate(cpConfig, config); err != nil {
		t.Fatal(err)
	}
	if config.SystemPrompt != "mine" {
		t.Errorf("expecting the operator's system prompt to win, got %q", config.SystemPrompt)
	}
	// Inline
	config = &OllamaSpec{PromptTemplate: "inline"}
	if template, err = resolveOllamaTemplate(cpConfig, config); err != nil || template != "inline" {
		t.Errorf("expecting the inline template, got %q, err %v", template, err)
	}
	// Both, neither, and an unknown name are configuration errors
	if _, err = resolveOllamaTemplate(cpConfig, &OllamaSpec{PromptTemplate: "a", PromptTemplateName: "classify"}); err == nil {
		t.Error("expecting an error when both prompt_template and prompt_template_name are set")
	}
	if _, err = resolveOllamaTemplate(cpConfig, &OllamaSpec{}); err == nil {
		t.Error("expecting an error when no template is provided")
	}
	if _, err = resolveOllamaTemplate(cpConfig, &OllamaSpec{PromptTemplateName: "nope"}); err == nil {
		t.Error("expecting an error for an unknown prompt_template_name")
	}
}

func TestOllamaValidateChannels(t *testing.T) {
	columnsMap := ollamaTestColumnsMap()
	spec := &ChannelSpec{Name: "claims", Columns: ollamaTestColumns, columnsMap: &columnsMap}
	source := &InputChannel{Name: "claims.in", Columns: &columnsMap, Config: spec}
	// Same spec instance: the normal case, both channels use the same channel_spec_name
	if err := validateOllamaChannels(source, &OutputChannel{Name: "claims.out", Config: spec}); err != nil {
		t.Errorf("expecting no error when the channels share the spec, got %v", err)
	}
	// Distinct spec instances with the same columns are accepted
	sameColumns := &ChannelSpec{Name: "claims2", Columns: ollamaTestColumns}
	if err := validateOllamaChannels(source, &OutputChannel{Name: "claims.out", Config: sameColumns}); err != nil {
		t.Errorf("expecting no error when the columns match, got %v", err)
	}
	// Different columns are not
	other := &ChannelSpec{Name: "other", Columns: []string{"claim_id", "something_else", "claim_category",
		"claim_confidence", "infer_tokens"}}
	err := validateOllamaChannels(source, &OutputChannel{Name: "other.out", Config: other})
	if err == nil {
		t.Fatal("expecting an error when the columns differ")
	}
	if !strings.Contains(err.Error(), "something_else") {
		t.Errorf("the error must point at the divergence, got: %v", err)
	}
	// So is a different column count
	shorter := &ChannelSpec{Name: "shorter", Columns: []string{"claim_id"}}
	if err = validateOllamaChannels(source, &OutputChannel{Name: "shorter.out", Config: shorter}); err == nil {
		t.Fatal("expecting an error when the column counts differ")
	}
}

// ---------------------------------------------------------------------------
// Error channel ownership, see CpipesStartup.ValidatePipeSpecConfig
// ---------------------------------------------------------------------------

// errorChannelConfig is what both the channel registration (compute_pipes.go) and the
// error channel validation rely on to know which operators report row level errors.
func TestErrorChannelConfig(t *testing.T) {
	errorChannel := &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"}
	tests := []struct {
		name      string
		spec      *TransformationSpec
		expecting *OutputChannelConfig
	}{
		{"map_record", &TransformationSpec{Type: "map_record",
			MapRecordConfig: &MapRecordSpec{ErrorChannel: errorChannel}}, errorChannel},
		{"jetrules", &TransformationSpec{Type: "jetrules",
			JetrulesConfig: &JetrulesSpec{ErrorChannel: errorChannel}}, errorChannel},
		{"ollama", &TransformationSpec{Type: "ollama",
			OllamaConfig: &OllamaSpec{ErrorChannel: errorChannel}}, errorChannel},
		{"map_record without error channel", &TransformationSpec{Type: "map_record",
			MapRecordConfig: &MapRecordSpec{}}, nil},
		{"map_record without config", &TransformationSpec{Type: "map_record"}, nil},
		{"ollama without config", &TransformationSpec{Type: "ollama"}, nil},
		{"operator with no error channel", &TransformationSpec{Type: "filter"}, nil},
	}
	for _, test := range tests {
		if got := errorChannelConfig(test.spec); got != test.expecting {
			t.Errorf("%s: expecting %v, got %v", test.name, test.expecting, got)
		}
	}
}

func TestValidateErrorChannels(t *testing.T) {
	errorChannel := func(name string) *OutputChannelConfig {
		return &OutputChannelConfig{Name: name, SpecName: "process_errors"}
	}
	// Two operators, each with its own error channel: fine
	pipeConfig := []PipeSpec{
		{Apply: []TransformationSpec{
			{Type: "ollama", OllamaConfig: &OllamaSpec{ErrorChannel: errorChannel("errors.ollama")},
				OutputChannel: OutputChannelConfig{Name: "claims.out"}},
			{Type: "jetrules", JetrulesConfig: &JetrulesSpec{ErrorChannel: errorChannel("errors.jetrules")}},
		}},
	}
	if err := validateErrorChannels(pipeConfig); err != nil {
		t.Errorf("expecting no error for distinct error channels, got %v", err)
	}

	// The same error channel used twice: rejected
	pipeConfig = []PipeSpec{
		{Apply: []TransformationSpec{
			{Type: "ollama", OllamaConfig: &OllamaSpec{ErrorChannel: errorChannel("process_errors.out")}},
		}},
		{Apply: []TransformationSpec{
			{Type: "jetrules", JetrulesConfig: &JetrulesSpec{ErrorChannel: errorChannel("process_errors.out")}},
		}},
	}
	err := validateErrorChannels(pipeConfig)
	if err == nil {
		t.Fatal("expecting an error when two operators share an error channel")
	}
	if !strings.Contains(err.Error(), "process_errors.out") {
		t.Errorf("the error must name the channel, got: %v", err)
	}

	// An error channel that is also an output channel: rejected
	pipeConfig = []PipeSpec{
		{Apply: []TransformationSpec{
			{Type: "map_record", OutputChannel: OutputChannelConfig{Name: "shared.ch"}},
			{Type: "ollama", OllamaConfig: &OllamaSpec{ErrorChannel: errorChannel("shared.ch")}},
		}},
	}
	if err = validateErrorChannels(pipeConfig); err == nil {
		t.Fatal("expecting an error when an error channel is also an output channel")
	}

	// Sharing a regular output channel stays legal, the qc_* pipelines rely on it
	pipeConfig = []PipeSpec{
		{Apply: []TransformationSpec{
			{Type: "map_record", OutputChannel: OutputChannelConfig{Name: "qc_metrics.writer"}},
			{Type: "map_record", OutputChannel: OutputChannelConfig{Name: "qc_metrics.writer"}},
		}},
	}
	if err = validateErrorChannels(pipeConfig); err != nil {
		t.Errorf("expecting no error when operators share a regular output channel, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// End to end, against a stand in infer server
// ---------------------------------------------------------------------------

var ollamaProcessErrorColumns = []string{"pipeline_execution_status_key", "session_id", "grouping_key",
	"row_jets_key", "input_column", "error_message", "rete_session_saved", "rete_session_triples", "shard_id"}

type ollamaTestResult struct {
	outputRecords [][]any
	errorRecords  [][]any
	prompts       []string
	pipelineErrs  []error
}

// runOllamaTestPipe builds the operator against the given stand in server, applies the
// records and returns what came out of the output and error channels.
func runOllamaTestPipe(t *testing.T, serverUrl string, config *OllamaSpec, spec *TransformationSpec,
	records [][]any) *ollamaTestResult {
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
		spec = &TransformationSpec{Type: "ollama"}
	}
	spec.OllamaConfig = config

	outputCh, err := registry.GetOutputChannel("claims.out")
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := builderContext.NewOllamaTransformationPipe(source, outputCh, spec)
	if err != nil {
		t.Fatal(err)
	}

	// Drain the channels while the pool is running: they are unbuffered.
	// The error channel is drained only when the operator owns one; it is closed
	// below together with the output channel, the way the pipe executor does —
	// the operator's Finally no longer closes it (see StartFanOutPipe).
	result := &ollamaTestResult{}
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
	// The pipe executor closes the output channels — the operator's error channel
	// included — once every operator is done (StartFanOutPipe); mimic it here.
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

// ollamaTestServer answers /api/generate with the responses provided, in order, and
// records the prompts it was sent. A status of 0 means 200 with the body as the model
// response.
func ollamaTestServer(t *testing.T, responses []ollamaTestResponse) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	prompts := make([]string, 0)
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ollamaGenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("while decoding the request: %v", err)
		}
		mu.Lock()
		prompts = append(prompts, request.Prompt)
		response := responses[min(callCount, len(responses)-1)]
		callCount++
		mu.Unlock()

		if response.status != 0 && response.status != http.StatusOK {
			w.WriteHeader(response.status)
			_, _ = w.Write([]byte(`{"error":"stand in failure"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":             request.Model,
			"response":          response.body,
			"done":              true,
			"eval_count":        42,
			"prompt_eval_count": 7,
		})
	}))
	t.Cleanup(server.Close)
	return server, &prompts
}

type ollamaTestResponse struct {
	status int
	body   string
}

func TestOllamaAugmentsRecordInPlace(t *testing.T) {
	server, prompts := ollamaTestServer(t, []ollamaTestResponse{
		{body: `{"category":"dental","confidence":0.87}`},
	})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}: {{diagnosis}}",
		OutputMapping: []OllamaMappingSpec{
			{Column: "claim_category", Path: "category"},
			{Column: "claim_confidence", Path: "confidence", AsRdfType: "double"},
			{Column: "infer_tokens", Source: ollamaSourceEnvelope, Path: "eval_count", AsRdfType: "int"},
		},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.outputRecords) != 1 {
		t.Fatalf("expecting 1 output record, got %d", len(result.outputRecords))
	}
	record := result.outputRecords[0]
	// The input columns are preserved, the mapped ones are filled
	if record[0] != "c-1" || record[1] != "tooth ache" {
		t.Errorf("the input columns must be preserved, got %v", record)
	}
	if record[2] != "dental" {
		t.Errorf("expecting claim_category 'dental', got %v", record[2])
	}
	if record[3] != 0.87 {
		t.Errorf("expecting claim_confidence 0.87, got %v (%T)", record[3], record[3])
	}
	if record[4] != 42 {
		t.Errorf("expecting infer_tokens 42, got %v (%T)", record[4], record[4])
	}
	if len(result.errorRecords) != 0 {
		t.Errorf("expecting no error record, got %v", result.errorRecords)
	}
	if len(*prompts) != 1 || (*prompts)[0] != "Classify c-1: tooth ache" {
		t.Errorf("unexpected prompt: %v", *prompts)
	}
}

// A short record must be grown before the mapped columns are assigned.
func TestOllamaAugmentsShortRecord(t *testing.T) {
	server, _ := ollamaTestServer(t, []ollamaTestResponse{{body: `{"category":"dental"}`}})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
	}, nil, [][]any{{"c-1", "tooth ache"}})

	if len(result.outputRecords) != 1 {
		t.Fatalf("expecting 1 output record, got %d", len(result.outputRecords))
	}
	record := result.outputRecords[0]
	if len(record) != len(ollamaTestColumns) {
		t.Fatalf("expecting the record to be grown to %d columns, got %d", len(ollamaTestColumns), len(record))
	}
	if record[2] != "dental" {
		t.Errorf("expecting claim_category 'dental', got %v", record[2])
	}
}

func TestOllamaServerFailurePassesRecordThrough(t *testing.T) {
	server, _ := ollamaTestServer(t, []ollamaTestResponse{{status: http.StatusBadRequest}})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		RowKeyColumn:   "claim_id",
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
		ErrorChannel:   &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	// pass_through is the default: the record comes out unchanged
	if len(result.outputRecords) != 1 {
		t.Fatalf("expecting the record to be passed through, got %d records", len(result.outputRecords))
	}
	if result.outputRecords[0][2] != nil {
		t.Errorf("expecting claim_category to be left null, got %v", result.outputRecords[0][2])
	}
	if len(result.errorRecords) != 1 {
		t.Fatalf("expecting 1 error record, got %d", len(result.errorRecords))
	}
	errorRecord := result.errorRecords[0]
	if rowKey := ollamaToString(errorRecord[3]); rowKey != "c-1" {
		t.Errorf("expecting row_jets_key 'c-1', got %v", errorRecord[3])
	}
	if msg := ollamaToString(errorRecord[5]); !strings.Contains(msg, "400") {
		t.Errorf("expecting the error message to report the status, got %q", msg)
	}
	if len(result.pipelineErrs) != 0 {
		t.Errorf("a row level failure must not interrupt the pipeline, got %v", result.pipelineErrs)
	}
}

func TestOllamaOnErrorDrop(t *testing.T) {
	server, _ := ollamaTestServer(t, []ollamaTestResponse{{status: http.StatusBadRequest}})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		OnError:        ollamaOnErrorDrop,
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
		ErrorChannel:   &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.outputRecords) != 0 {
		t.Errorf("expecting the record to be dropped, got %v", result.outputRecords)
	}
	if len(result.errorRecords) != 1 {
		t.Errorf("expecting 1 error record, got %d", len(result.errorRecords))
	}
}

func TestOllamaOnErrorFail(t *testing.T) {
	server, _ := ollamaTestServer(t, []ollamaTestResponse{{status: http.StatusBadRequest}})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		OnError:        ollamaOnErrorFail,
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.pipelineErrs) != 1 {
		t.Fatalf("expecting the pipeline to be interrupted with 1 error, got %v", result.pipelineErrs)
	}
	if !strings.Contains(result.pipelineErrs[0].Error(), "400") {
		t.Errorf("unexpected pipeline error: %v", result.pipelineErrs[0])
	}
}

// A 5xx is retried, a success on the retry salvages the record.
func TestOllamaRetriesServerError(t *testing.T) {
	server, prompts := ollamaTestServer(t, []ollamaTestResponse{
		{status: http.StatusServiceUnavailable},
		{body: `{"category":"dental"}`},
	})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		RetryWaitSec:   1,
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(*prompts) != 2 {
		t.Fatalf("expecting the call to be retried once, got %d calls", len(*prompts))
	}
	if len(result.outputRecords) != 1 || result.outputRecords[0][2] != "dental" {
		t.Errorf("expecting the retry to succeed, got %v", result.outputRecords)
	}
}

// An explicit max_retry of 0 disables the retries, unset means the default.
func TestOllamaMaxRetryZeroDisablesRetries(t *testing.T) {
	server, prompts := ollamaTestServer(t, []ollamaTestResponse{
		{status: http.StatusServiceUnavailable},
		{body: `{"category":"dental"}`},
	})
	noRetry := 0
	runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		MaxRetry:       &noRetry,
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(*prompts) != 1 {
		t.Errorf("expecting a single call when max_retry is 0, got %d", len(*prompts))
	}
}

// A 4xx is the server refusing the request, retrying it would only waste time.
func TestOllamaDoesNotRetryClientError(t *testing.T) {
	server, prompts := ollamaTestServer(t, []ollamaTestResponse{{status: http.StatusBadRequest}})
	runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(*prompts) != 1 {
		t.Errorf("expecting a single call for a 4xx, got %d", len(*prompts))
	}
}

// Past max_input_count the records are passed through un-inferred rather than filtered.
func TestOllamaMaxInputCount(t *testing.T) {
	server, prompts := ollamaTestServer(t, []ollamaTestResponse{{body: `{"category":"dental"}`}})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		MaxInputCount:  2,
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
	}, nil, [][]any{
		{"c-1", "a", nil, nil, nil},
		{"c-2", "b", nil, nil, nil},
		{"c-3", "c", nil, nil, nil},
		{"c-4", "d", nil, nil, nil},
	})

	if len(*prompts) != 2 {
		t.Errorf("expecting 2 calls, got %d", len(*prompts))
	}
	if len(result.outputRecords) != 4 {
		t.Errorf("expecting all 4 records on the output, got %d", len(result.outputRecords))
	}
}

// A required mapping that the model did not answer is a row level error.
func TestOllamaRequiredMappingMissing(t *testing.T) {
	server, _ := ollamaTestServer(t, []ollamaTestResponse{{body: `{"something_else":"x"}`}})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category", Required: true}},
		ErrorChannel:   &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.errorRecords) != 1 {
		t.Fatalf("expecting 1 error record, got %d", len(result.errorRecords))
	}
	if msg := ollamaToString(result.errorRecords[0][5]); !strings.Contains(msg, "claim_category") {
		t.Errorf("expecting the error to name the column, got %q", msg)
	}
	// A mapping failure reports which column it was on
	if column := ollamaToString(result.errorRecords[0][4]); column != "claim_category" {
		t.Errorf("expecting input_column 'claim_category', got %q", column)
	}
}

// A mapping with no value and no default leaves the column as it was, so mapping onto an
// existing column does not destroy the input value.
func TestOllamaMissingMappingKeepsInputValue(t *testing.T) {
	server, _ := ollamaTestServer(t, []ollamaTestResponse{{body: `{"something_else":"x"}`}})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		OutputMapping: []OllamaMappingSpec{
			{Column: "diagnosis", Path: "category"},
			{Column: "claim_category", Path: "category", Default: "unknown"},
		},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.outputRecords) != 1 {
		t.Fatalf("expecting 1 output record, got %d", len(result.outputRecords))
	}
	if result.outputRecords[0][1] != "tooth ache" {
		t.Errorf("expecting the input value to be kept, got %v", result.outputRecords[0][1])
	}
	if result.outputRecords[0][2] != "unknown" {
		t.Errorf("expecting the default value, got %v", result.outputRecords[0][2])
	}
}

// A json object or array cannot be a column value, it is encoded back to json text.
func TestOllamaObjectValueIsEncoded(t *testing.T) {
	server, _ := ollamaTestServer(t, []ollamaTestResponse{
		{body: `{"detail":{"score":0.9},"codes":["a","b"]}`},
	})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		OutputMapping: []OllamaMappingSpec{
			{Column: "claim_category", Path: "detail"},
			{Column: "claim_confidence", Path: "codes"},
		},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	record := result.outputRecords[0]
	if record[2] != `{"score":0.9}` {
		t.Errorf("expecting the object as json text, got %v (%T)", record[2], record[2])
	}
	if record[3] != `["a","b"]` {
		t.Errorf("expecting the array as json text, got %v (%T)", record[3], record[3])
	}
}

// The raw_response source takes the model text as is, without parsing it.
func TestOllamaRawResponseMapping(t *testing.T) {
	server, _ := ollamaTestServer(t, []ollamaTestResponse{{body: "a plain sentence"}})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Summarize {{claim_id}}",
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Source: ollamaSourceRawResponse}},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if result.outputRecords[0][2] != "a plain sentence" {
		t.Errorf("expecting the raw response, got %v", result.outputRecords[0][2])
	}
}

// Ollama reports some failures with a 200 and an error property.
func TestOllamaErrorInSuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":"model 'nope' not found"}`))
	}))
	t.Cleanup(server.Close)
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "nope",
		PromptTemplate: "Classify {{claim_id}}",
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
		ErrorChannel:   &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.errorRecords) != 1 {
		t.Fatalf("expecting 1 error record, got %d", len(result.errorRecords))
	}
	if msg := ollamaToString(result.errorRecords[0][5]); !strings.Contains(msg, "not found") {
		t.Errorf("expecting the model error to be reported, got %q", msg)
	}
}

// A response that is not json fails the record with a message that shows what came back.
func TestOllamaUnparseableResponse(t *testing.T) {
	server, _ := ollamaTestServer(t, []ollamaTestResponse{{body: "I am afraid I cannot do that"}})
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
		ErrorChannel:   &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(result.errorRecords) != 1 {
		t.Fatalf("expecting 1 error record, got %d", len(result.errorRecords))
	}
	if msg := ollamaToString(result.errorRecords[0][5]); !strings.Contains(msg, "I am afraid") {
		t.Errorf("expecting the response to be quoted in the error, got %q", msg)
	}
}

// With the default pool size of 1 the records keep their order.
func TestOllamaPreservesOrderWithSingleWorker(t *testing.T) {
	server, _ := ollamaTestServer(t, []ollamaTestResponse{{body: `{"category":"dental"}`}})
	records := make([][]any, 0, 20)
	for i := range 20 {
		records = append(records, []any{"c-" + string(rune('a'+i)), "d", nil, nil, nil})
	}
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
	}, nil, records)

	if len(result.outputRecords) != len(records) {
		t.Fatalf("expecting %d records, got %d", len(records), len(result.outputRecords))
	}
	for i := range records {
		if result.outputRecords[i][0] != records[i][0] {
			t.Fatalf("record %d out of order: expecting %v, got %v", i, records[i][0], result.outputRecords[i][0])
		}
	}
}

// Every record is processed when the pool has several workers, order aside.
func TestOllamaWorkerPool(t *testing.T) {
	server, prompts := ollamaTestServer(t, []ollamaTestResponse{{body: `{"category":"dental"}`}})
	records := make([][]any, 0, 25)
	for i := range 25 {
		records = append(records, []any{i, "d", nil, nil, nil})
	}
	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		PromptTemplate: "Classify {{claim_id}}",
		PoolSize:       5,
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
	}, nil, records)

	if len(*prompts) != len(records) {
		t.Errorf("expecting %d calls, got %d", len(records), len(*prompts))
	}
	if len(result.outputRecords) != len(records) {
		t.Fatalf("expecting %d records, got %d", len(records), len(result.outputRecords))
	}
	seen := make(map[any]bool, len(records))
	for _, record := range result.outputRecords {
		if record[2] != "dental" {
			t.Fatalf("record %v was not augmented", record)
		}
		seen[record[0]] = true
	}
	if len(seen) != len(records) {
		t.Errorf("expecting %d distinct records, got %d", len(records), len(seen))
	}
}

// The chat api puts the answer in message.content rather than response.
func TestOllamaChatApi(t *testing.T) {
	var gotMessages []ollamaChatMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ollamaChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("while decoding the request: %v", err)
		}
		gotMessages = request.Messages
		if !strings.HasSuffix(r.URL.Path, "/api/chat") {
			t.Errorf("expecting the chat route, got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   request.Model,
			"message": map[string]any{"role": "assistant", "content": `{"category":"dental"}`},
			"done":    true,
		})
	}))
	t.Cleanup(server.Close)

	result := runOllamaTestPipe(t, server.URL, &OllamaSpec{
		Model:          "test-model",
		Api:            "chat",
		SystemPrompt:   "be terse",
		PromptTemplate: "Classify {{claim_id}}",
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
	}, nil, [][]any{{"c-1", "tooth ache", nil, nil, nil}})

	if len(gotMessages) != 2 || gotMessages[0].Role != "system" || gotMessages[1].Content != "Classify c-1" {
		t.Errorf("unexpected chat messages: %v", gotMessages)
	}
	if result.outputRecords[0][2] != "dental" {
		t.Errorf("expecting claim_category 'dental', got %v", result.outputRecords[0][2])
	}
}

// The env vars of the cpipes config are substituted in the template when it is compiled.
func TestOllamaTemplateEnvVarSubstitution(t *testing.T) {
	server, prompts := ollamaTestServer(t, []ollamaTestResponse{{body: `{"category":"dental"}`}})

	columnsMap := ollamaTestColumnsMap()
	channelSpec := &ChannelSpec{Name: "claims", Columns: ollamaTestColumns, columnsMap: &columnsMap}
	registry := &ChannelRegistry{
		ComputeChannels: map[string]*Channel{
			"claims.out": {Name: "claims.out", Channel: make(chan []any), Columns: &columnsMap, Config: channelSpec},
		},
		ClosedChannels: make(map[string]bool),
	}
	builderContext := &BuilderContext{
		cpConfig:        &ComputePipesConfig{},
		channelRegistry: registry,
		done:            make(chan struct{}),
		errCh:           make(chan error, 10),
		env:             map[string]any{"$CLIENT": "acme"},
	}
	source := &InputChannel{Name: "claims.in", Columns: &columnsMap, Config: channelSpec}
	outputCh, err := registry.GetOutputChannel("claims.out")
	if err != nil {
		t.Fatal(err)
	}
	spec := &TransformationSpec{Type: "ollama", OllamaConfig: &OllamaSpec{
		Model:          "test-model",
		Server:         &OllamaServerSpec{Url: server.URL},
		PromptTemplate: "Client $CLIENT claim {{claim_id}}",
		OutputMapping:  []OllamaMappingSpec{{Column: "claim_category", Path: "category"}},
	}}
	pipe, err := builderContext.NewOllamaTransformationPipe(source, outputCh, spec)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range registry.ComputeChannels["claims.out"].Channel {
		}
	}()
	record := []any{"c-1", "tooth ache", nil, nil, nil}
	if err = pipe.Apply(&record); err != nil {
		t.Fatal(err)
	}
	pipe.Finally()
	registry.CloseChannel("claims.out")

	if len(*prompts) != 1 || (*prompts)[0] != "Client acme claim c-1" {
		t.Errorf("expecting the env var to be substituted, got %v", *prompts)
	}
}

// The operator refuses to be built when it cannot tell where the infer server is.
func TestOllamaMissingServerUrl(t *testing.T) {
	if _, err := resolveOllamaUrl(&OllamaSpec{}, map[string]any{}); err == nil {
		t.Error("expecting an error when no url can be resolved")
	}
	env := map[string]any{"$JETS_INFER_URL": "http://from-env:11434"}
	url, err := resolveOllamaUrl(&OllamaSpec{}, env)
	if err != nil || url != "http://from-env:11434" {
		t.Errorf("expecting the url from the cpipes env, got %q, err %v", url, err)
	}
	// The configuration wins over the env, and gets env var substitution
	url, err = resolveOllamaUrl(&OllamaSpec{Server: &OllamaServerSpec{Url: "http://configured:$PORT"}},
		map[string]any{"$PORT": 11434, "$JETS_INFER_URL": "http://from-env:11434"})
	if err != nil || url != "http://configured:11434" {
		t.Errorf("expecting the configured url, got %q, err %v", url, err)
	}
}
