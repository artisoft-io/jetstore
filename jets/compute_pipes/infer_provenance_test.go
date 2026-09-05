package compute_pipes

// Tests for the per-field provenance check on an inference operator, see
// infer_provenance.go. The build-time cases exercise the resolution; the end to
// end cases run the operator against the same stand in infer server the ollama
// tests use, and assert on what came out of the output and error channels
// rather than on the checker, which has its own tests in jets/agentic/briefing.

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// A workspace on disk, since a provenance schema is a workspace file
// ---------------------------------------------------------------------------

// briefingResponseFormat is the shape the model is constrained to and the shape
// the rules below are covered against. It is one string in this file for the
// same reason it is one document in the workspace.
const briefingResponseFormat = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["conditions", "event_count"],
  "properties": {
    "conditions": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["code"],
        "properties": {"code": {"type": "string"}}
      }
    },
    "event_count": {"type": "integer"}
  }
}`

// testProvenanceSchema is a complete .pv.json: a response_format, a rule for
// every leaf it can produce, and the intended-use notice Confine requires of a
// schema that declares a shape.
func testProvenanceSchema(key string) string {
	return `{
  "key": ` + strconv.Quote(key) + `,
  "version": "1.0.0",
  "disclaimer": {"field": "cintel:Briefing_Disclaimer"},
  "response_format": ` + briefingResponseFormat + `,
  "rules": [
    {"field": "conditions[].code", "kind": "grounded", "match": "exact",
     "sources": ["events[].Diagnosis_Code[]"]},
    {"field": "event_count", "kind": "count_of", "sources": ["events[]"]}
  ]
}`
}

// writeTestWorkspace lays out WORKSPACES_HOME/WORKSPACE/provenance/<name>.pv.json
// and points the package's workspace variables at it for the test's lifetime.
// It is the jetrules cache tests' move (jetrules_cache_test.go), for the same
// reason: the path is composed from package level variables and a test that
// cannot set them cannot reach the code that reads them.
func writeTestWorkspace(t *testing.T, files map[string]string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "testws", provenanceSchemaDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name+provenanceSchemaSuffix), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oldHome, oldPrefix := workspaceHome, wsPrefix
	workspaceHome, wsPrefix = home, "testws"
	t.Cleanup(func() { workspaceHome, wsPrefix = oldHome, oldPrefix })
}

// briefingTestColumns is a channel carrying the serialised entity, the model's
// answer and a row key - the three things a checked briefing record needs.
var briefingTestColumns = []string{"member_key", "cintel:Briefing_Input", "cintel:Briefing_Document", "event_count"}

func briefingTestColumnsMap() map[string]int {
	m := make(map[string]int, len(briefingTestColumns))
	for i, c := range briefingTestColumns {
		m[c] = i
	}
	return m
}

// briefingTestChannelSpec is the shared spec, with the one column_encodings
// entry the check resolves against.
func briefingTestChannelSpec(encodings ...*ColumnEncodingSpec) *ChannelSpec {
	if encodings == nil {
		encodings = []*ColumnEncodingSpec{
			{Column: "cintel:Briefing_Input", EntityEncoding: "json"},
		}
	}
	columnsMap := briefingTestColumnsMap()
	return &ChannelSpec{
		Name:            "briefing",
		Columns:         briefingTestColumns,
		columnsMap:      &columnsMap,
		ColumnEncodings: encodings,
	}
}

func briefingTestSource(spec *ChannelSpec) *InputChannel {
	columnsMap := briefingTestColumnsMap()
	return &InputChannel{Name: "briefing.in", Columns: &columnsMap, Config: spec}
}

// ---------------------------------------------------------------------------
// Resolution
// ---------------------------------------------------------------------------

func TestNoProvenanceSchemaNameIsNoCheck(t *testing.T) {
	common := &InferCommonSpec{}
	check, err := resolveInferProvenanceSchema(common, briefingTestSource(briefingTestChannelSpec()), "ollama_config")
	if err != nil {
		t.Fatal(err)
	}
	if check != nil {
		t.Fatalf("expecting no check when no schema is named, got %+v", check)
	}
}

func TestTheSchemaSuppliesTheResponseFormatWhenTheOperatorHasNone(t *testing.T) {
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("briefing")})
	common := &InferCommonSpec{ProvenanceSchemaName: "briefing"}
	check, err := resolveInferProvenanceSchema(common, briefingTestSource(briefingTestChannelSpec()), "ollama_config")
	if err != nil {
		t.Fatal(err)
	}
	if check == nil {
		t.Fatal("expecting a check")
	}
	// The point of I-438: one document, and the operator reads it.
	same, err := sameJSONDocument(common.ResponseFormat, json.RawMessage(briefingResponseFormat))
	if err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Fatalf("expecting the schema's response_format to be adopted, got %s", string(common.ResponseFormat))
	}
	if check.entityColumn != "cintel:Briefing_Input" || check.entityColPos != 1 {
		t.Fatalf("expecting the entity column to be resolved, got %q at %d", check.entityColumn, check.entityColPos)
	}
}

func TestTwoResponseFormatsThatAgreeAreAccepted(t *testing.T) {
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("briefing")})
	// Same document, different whitespace and key order: the comparison is by
	// value, because two hand-written copies of one contract will never be
	// byte-identical and refusing them for an indentation would teach an author
	// to delete the check rather than the copy.
	reordered := `{"additionalProperties":false,` +
		`"properties":{"event_count":{"type":"integer"},` +
		`"conditions":{"items":{"additionalProperties":false,"properties":{"code":{"type":"string"}},` +
		`"required":["code"],"type":"object"},"type":"array"}},` +
		`"required":["conditions","event_count"],"type":"object"}`
	common := &InferCommonSpec{ProvenanceSchemaName: "briefing", ResponseFormat: json.RawMessage(reordered)}
	if _, err := resolveInferProvenanceSchema(common, briefingTestSource(briefingTestChannelSpec()), "ollama_config"); err != nil {
		t.Fatalf("expecting two equivalent response_formats to be accepted, got %v", err)
	}
}

func TestTwoResponseFormatsThatDifferAreRefused(t *testing.T) {
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("briefing")})
	// One extra field on the operator's copy: exactly the drift I-438 records,
	// a field the model may produce and no rule covers.
	drifted := `{"type":"object","additionalProperties":false,` +
		`"required":["conditions","event_count"],` +
		`"properties":{"conditions":{"type":"array","items":{"type":"object","additionalProperties":false,` +
		`"required":["code"],"properties":{"code":{"type":"string"}}}},` +
		`"event_count":{"type":"integer"},"summary":{"type":"string"}}}`
	common := &InferCommonSpec{ProvenanceSchemaName: "briefing", ResponseFormat: json.RawMessage(drifted)}
	_, err := resolveInferProvenanceSchema(common, briefingTestSource(briefingTestChannelSpec()), "ollama_config")
	if err == nil {
		t.Fatal("expecting a refusal when the two response_formats differ")
	}
	if !strings.Contains(err.Error(), "must be the same document") {
		t.Errorf("expecting the message to say why, got %v", err)
	}
}

func TestTheSchemaKeyMustMatchTheName(t *testing.T) {
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("something_else")})
	common := &InferCommonSpec{ProvenanceSchemaName: "briefing"}
	_, err := resolveInferProvenanceSchema(common, briefingTestSource(briefingTestChannelSpec()), "ollama_config")
	if err == nil {
		t.Fatal("expecting a refusal when the file name and the schema key disagree")
	}
}

func TestAnAbsentSchemaIsAConfigurationError(t *testing.T) {
	writeTestWorkspace(t, map[string]string{})
	common := &InferCommonSpec{ProvenanceSchemaName: "briefing"}
	_, err := resolveInferProvenanceSchema(common, briefingTestSource(briefingTestChannelSpec()), "ollama_config")
	if err == nil {
		t.Fatal("expecting a refusal when the named schema does not exist")
	}
}

func TestASchemaNameIsNotAPath(t *testing.T) {
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("briefing")})
	for _, name := range []string{"provenance/briefing", "../briefing", "a/b"} {
		common := &InferCommonSpec{ProvenanceSchemaName: name}
		if _, err := resolveInferProvenanceSchema(common, briefingTestSource(briefingTestChannelSpec()), "ollama_config"); err == nil {
			t.Errorf("expecting %q to be refused as a path", name)
		}
	}
}

func TestASchemaThatDoesNotCoverItsBriefingIsRefusedAtBuild(t *testing.T) {
	// A response_format leaf with no rule. Cover already refuses this when the
	// file is saved; asserting it here is what makes the operator refuse to
	// start rather than discover it on the first record.
	uncovered := `{
  "key": "briefing",
  "disclaimer": {"field": "cintel:Briefing_Disclaimer"},
  "response_format": ` + briefingResponseFormat + `,
  "rules": [
    {"field": "conditions[].code", "kind": "grounded", "match": "exact",
     "sources": ["events[].Diagnosis_Code[]"]}
  ]
}`
	writeTestWorkspace(t, map[string]string{"briefing": uncovered})
	common := &InferCommonSpec{ProvenanceSchemaName: "briefing"}
	_, err := resolveInferProvenanceSchema(common, briefingTestSource(briefingTestChannelSpec()), "ollama_config")
	if err == nil {
		t.Fatal("expecting a refusal when a briefing field has no rule")
	}
	if !strings.Contains(err.Error(), "event_count") {
		t.Errorf("expecting the message to name the uncovered field, got %v", err)
	}
}

func TestAChannelThatSerialisesNoEntityIsRefused(t *testing.T) {
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("briefing")})
	common := &InferCommonSpec{ProvenanceSchemaName: "briefing"}
	spec := briefingTestChannelSpec()
	spec.ColumnEncodings = nil
	_, err := resolveInferProvenanceSchema(common, briefingTestSource(spec), "ollama_config")
	if err == nil {
		t.Fatal("expecting a refusal when the channel spec has no column_encodings")
	}
	if !strings.Contains(err.Error(), "column_encodings") {
		t.Errorf("expecting the message to name the missing section, got %v", err)
	}
}

func TestAChannelThatSerialisesTwoEntitiesIsRefused(t *testing.T) {
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("briefing")})
	common := &InferCommonSpec{ProvenanceSchemaName: "briefing"}
	spec := briefingTestChannelSpec(
		&ColumnEncodingSpec{Column: "cintel:Briefing_Input", EntityEncoding: "json"},
		&ColumnEncodingSpec{Column: "cintel:Briefing_Document", EntityEncoding: "json"},
	)
	_, err := resolveInferProvenanceSchema(common, briefingTestSource(spec), "ollama_config")
	if err == nil {
		t.Fatal("expecting a refusal when two columns are encoded")
	}
}

func TestTheEmbedOperatorRefusesAProvenanceSchema(t *testing.T) {
	// The embed operator refuses the promoted fields an embeddings call has no
	// use for; a provenance schema grounds a model's answer and this backend
	// returns a vector.
	ctx := &BuilderContext{cpConfig: &ComputePipesConfig{}, env: make(map[string]any)}
	columnsMap := briefingTestColumnsMap()
	spec := briefingTestChannelSpec()
	source := briefingTestSource(spec)
	outputCh := &OutputChannel{Name: "briefing.out", Columns: &columnsMap, Config: spec}
	_, err := ctx.NewEmbedTransformationPipe(source, outputCh, &TransformationSpec{
		Type: "embed",
		EmbedConfig: &EmbedSpec{
			Model:        "nomic-embed-text",
			VectorColumn: "event_count",
			InferCommonSpec: InferCommonSpec{
				PromptTemplate:       "{{member_key}}",
				ProvenanceSchemaName: "briefing",
			},
		},
	})
	if err == nil {
		t.Fatal("expecting the embed operator to refuse provenance_schema_name")
	}
	if !strings.Contains(err.Error(), "provenance_schema_name") {
		t.Errorf("expecting the message to name the field, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// End to end, against the stand in infer server
// ---------------------------------------------------------------------------

// runBriefingTestPipe is runOllamaTestPipe over the briefing channel: a spec
// carrying a column_encodings entry, and a record whose entity column is
// already populated the way the jetrules operator would have left it.
func runBriefingTestPipe(t *testing.T, serverUrl string, config *OllamaSpec,
	records [][]any) *ollamaTestResult {
	t.Helper()

	channelSpec := briefingTestChannelSpec()
	columnsMap := briefingTestColumnsMap()
	peColumnsMap := make(map[string]int, len(ollamaProcessErrorColumns))
	for i, c := range ollamaProcessErrorColumns {
		peColumnsMap[c] = i
	}
	peSpec := &ChannelSpec{Name: "process_errors", Columns: ollamaProcessErrorColumns, columnsMap: &peColumnsMap}

	registry := &ChannelRegistry{
		ComputeChannels: map[string]*Channel{
			"briefing.out": {Name: "briefing.out", Channel: make(chan []any),
				Columns: &columnsMap, Config: channelSpec},
			"process_errors.out": {Name: "process_errors.out", Channel: make(chan []any),
				Columns: &peColumnsMap, Config: peSpec},
		},
		ClosedChannels: make(map[string]bool),
	}
	source := &InputChannel{Name: "briefing.in", Columns: &columnsMap, Config: channelSpec}
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
	outputCh, err := registry.GetOutputChannel("briefing.out")
	if err != nil {
		t.Fatal(err)
	}
	pipe, err := builderContext.NewOllamaTransformationPipe(source, outputCh,
		&TransformationSpec{Type: "ollama", OllamaConfig: config})
	if err != nil {
		t.Fatal(err)
	}

	result := &ollamaTestResult{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for record := range registry.ComputeChannels["briefing.out"].Channel {
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
	registry.CloseChannel("briefing.out")
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

// briefingTestEntity is the serialised entity the prompt was built from: one
// member with two diagnosis codes.
const briefingTestEntity = `{"events":[{"Diagnosis_Code":"E119"},{"Diagnosis_Code":"I10"}]}`

func briefingTestConfig() *OllamaSpec {
	return &OllamaSpec{
		Model: "granite4.1:3b",
		InferCommonSpec: InferCommonSpec{
			PromptTemplate:       "Summarise {{cintel:Briefing_Input}}",
			ProvenanceSchemaName: "briefing",
			RowKeyColumn:         "member_key",
			OutputMapping: []InferMappingSpec{
				{Column: "cintel:Briefing_Document", Source: inferSourceRawResponse, Required: true},
				{Column: "event_count", Path: "event_count"},
			},
			ErrorChannel: &OutputChannelConfig{Name: "process_errors.out", SpecName: "process_errors"},
		},
	}
}

func TestAGroundedBriefingReportsNothing(t *testing.T) {
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("briefing")})
	server, _ := ollamaTestServer(t, []ollamaTestResponse{
		{body: `{"conditions":[{"code":"E119"},{"code":"I10"}],"event_count":2}`},
	})
	result := runBriefingTestPipe(t, server.URL, briefingTestConfig(),
		[][]any{{"m-1", briefingTestEntity, nil, nil}})
	if len(result.outputRecords) != 1 {
		t.Fatalf("expecting the record to be delivered, got %d", len(result.outputRecords))
	}
	if len(result.errorRecords) != 0 {
		t.Fatalf("expecting no findings, got %v", result.errorRecords)
	}
}

func TestAnUngroundedBriefingIsReportedAndTheRecordIsDelivered(t *testing.T) {
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("briefing")})
	// A code the claim data does not carry, and a count that is not the number
	// of events. Both are what A section 8.3 means by a statement with no
	// corresponding event in the input entity.
	server, _ := ollamaTestServer(t, []ollamaTestResponse{
		{body: `{"conditions":[{"code":"E119"},{"code":"C349"}],"event_count":7}`},
	})
	result := runBriefingTestPipe(t, server.URL, briefingTestConfig(),
		[][]any{{"m-1", briefingTestEntity, nil, nil}})

	// pass_through: the briefing is delivered with its findings attached rather
	// than withheld. That is Q-59's answer and the reason it is a sequencing
	// decision - it is the only disposition that produces a rate.
	if len(result.outputRecords) != 1 {
		t.Fatalf("expecting the record to be delivered, got %d", len(result.outputRecords))
	}
	if len(result.errorRecords) != 1 {
		t.Fatalf("expecting one report, got %d: %v", len(result.errorRecords), result.errorRecords)
	}
	row := result.errorRecords[0]
	message, _ := row[5].(string)
	for _, want := range []string{"C349", "event_count", "provenance schema"} {
		if !strings.Contains(message, want) {
			t.Errorf("expecting the report to mention %q, got %q", want, message)
		}
	}
	if got, _ := row[4].(sql.NullString); got.String != "cintel:Briefing_Input" {
		t.Errorf("expecting input_column to name the entity column, got %q", got.String)
	}
	if got, _ := row[3].(sql.NullString); got.String != "m-1" {
		t.Errorf("expecting row_jets_key to carry the row key, got %q", got.String)
	}
	if len(result.pipelineErrs) != 0 {
		t.Errorf("expecting no pipeline error, got %v", result.pipelineErrs)
	}
}

func TestAProvenanceFindingDoesNotInheritOnErrorFail(t *testing.T) {
	// The distinction failedRecord's own comment draws, one field over: on_error
	// is about a record whose *call* failed. A briefing that was answered and is
	// ungrounded is a different population, and Q-59 answered pass_through for
	// it - so an operator set to fail must not stop the pipeline on a finding.
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("briefing")})
	server, _ := ollamaTestServer(t, []ollamaTestResponse{
		{body: `{"conditions":[{"code":"C349"}],"event_count":2}`},
	})
	config := briefingTestConfig()
	config.OnError = OnErrorFail
	result := runBriefingTestPipe(t, server.URL, config, [][]any{{"m-1", briefingTestEntity, nil, nil}})
	if len(result.pipelineErrs) != 0 {
		t.Fatalf("expecting a finding not to interrupt the pipeline, got %v", result.pipelineErrs)
	}
	if len(result.outputRecords) != 1 {
		t.Fatalf("expecting the record to be delivered, got %d", len(result.outputRecords))
	}
	if len(result.errorRecords) != 1 {
		t.Fatalf("expecting one report, got %d", len(result.errorRecords))
	}
}

func TestAnAnswerThatCannotBeCheckedIsReported(t *testing.T) {
	// A check that could not run is not a clean check. The answer here is
	// well-formed enough for the mapping that reads it verbatim and is not a
	// json object, so the closure never sees it - which is precisely the case a
	// guardrail must not answer with silence.
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("briefing")})
	server, _ := ollamaTestServer(t, []ollamaTestResponse{{body: `I am sorry, I cannot help with that.`}})
	config := briefingTestConfig()
	config.OutputMapping = []InferMappingSpec{
		{Column: "cintel:Briefing_Document", Source: inferSourceRawResponse, Required: true},
	}
	result := runBriefingTestPipe(t, server.URL, config, [][]any{{"m-1", briefingTestEntity, nil, nil}})
	if len(result.errorRecords) != 1 {
		t.Fatalf("expecting the unreadable answer to be reported, got %d", len(result.errorRecords))
	}
	if message, _ := result.errorRecords[0][5].(string); !strings.Contains(message, "could not be checked") {
		t.Errorf("expecting the report to say the check did not run, got %q", message)
	}
}

func TestProvenanceReportsHaveTheirOwnBudget(t *testing.T) {
	// max_error_count bounds each kind of report rather than their sum: a run
	// whose every briefing is ungrounded would otherwise spend the operator's
	// whole budget before the first failed call was reported.
	writeTestWorkspace(t, map[string]string{"briefing": testProvenanceSchema("briefing")})
	server, _ := ollamaTestServer(t, []ollamaTestResponse{
		{body: `{"conditions":[{"code":"C349"}],"event_count":2}`},
	})
	config := briefingTestConfig()
	config.MaxErrorCount = 2
	records := [][]any{
		{"m-1", briefingTestEntity, nil, nil},
		{"m-2", briefingTestEntity, nil, nil},
		{"m-3", briefingTestEntity, nil, nil},
		{"m-4", briefingTestEntity, nil, nil},
	}
	result := runBriefingTestPipe(t, server.URL, config, records)
	if len(result.outputRecords) != 4 {
		t.Fatalf("expecting every record to be delivered, got %d", len(result.outputRecords))
	}
	if len(result.errorRecords) != 2 {
		t.Fatalf("expecting max_error_count to bound the reports at 2, got %d", len(result.errorRecords))
	}
}

func TestNoProvenanceSchemaLeavesTheOperatorUnchanged(t *testing.T) {
	// The regression the whole change has to not cause: an operator that names
	// no schema behaves exactly as it did, including on a channel that carries
	// an encoded column.
	server, _ := ollamaTestServer(t, []ollamaTestResponse{{body: `{"event_count":2}`}})
	config := briefingTestConfig()
	config.ProvenanceSchemaName = ""
	result := runBriefingTestPipe(t, server.URL, config, [][]any{{"m-1", briefingTestEntity, nil, nil}})
	if len(result.outputRecords) != 1 || len(result.errorRecords) != 0 {
		t.Fatalf("expecting one record and no report, got %d and %d",
			len(result.outputRecords), len(result.errorRecords))
	}
}

// ---------------------------------------------------------------------------
// The live workspace, consumed rather than restated
// ---------------------------------------------------------------------------

// briefingWorkspaceRoot locates workspaces/jets_ws the way the briefing
// package's own workspace test does, and for the same reason: the workspaces are
// separate repositories and a plain checkout of this one does not carry them.
func briefingWorkspaceRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("JETS_BRIEFING_WORKSPACE"); root != "" {
		return root
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// jets/compute_pipes -> jetstore_ai -> the parent checkout.
	return filepath.Clean(filepath.Join(wd, "..", "..", "..", "workspaces", "jets_ws"))
}

// TestTheLiveBriefingOperatorResolvesItsSchema opens the workspace documents
// themselves rather than copies beside them, which is P3 I-147's lesson: a stub
// shaped like the caller agrees with the caller by construction. What it asserts
// is the whole of I-438 - that the briefing's shape is now in one document, and
// that the operator reads it.
func TestTheLiveBriefingOperatorResolvesItsSchema(t *testing.T) {
	root := briefingWorkspaceRoot(t)
	configPath := filepath.Join(root, "pipes_config", "patient_profile.pc.json")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Skipf("the jets_ws pipeline is not in this checkout (%s)", configPath)
	}
	var cpConfig ComputePipesConfig
	if err := json.Unmarshal(content, &cpConfig); err != nil {
		t.Fatal(err)
	}
	if len(cpConfig.PromptTemplates) != 1 {
		t.Fatalf("expecting one prompt template, got %d", len(cpConfig.PromptTemplates))
	}
	// **Written to pass against either pin, deliberately, and it is not a
	// weakened assertion.** This repository's submodules move separately, so a
	// build of this commit may carry the workspace either before or after the
	// change that removes the prompt template's copy of the response_format. A
	// test that demanded the copy be gone would be red on a jetstore head built
	// against the older jets_ws, which is a merge-order failure rather than a
	// defect - the shape AK.3 met. So what is asserted is the invariant the
	// operator enforces rather than the state of one file: a second copy is
	// allowed to exist and is not allowed to disagree. The one-copy state is
	// reported rather than required.
	secondCopy := cpConfig.PromptTemplates[0].ResponseFormat
	if len(secondCopy) == 0 {
		t.Log("the prompt template carries no response_format: the provenance schema is the one copy (I-438 closed)")
	} else {
		t.Log("the prompt template still carries a response_format: this checkout's jets_ws predates its removal")
	}

	// Point the package's workspace variables at the real workspace and resolve
	// the operator's schema through the code the builder runs.
	oldHome, oldPrefix := workspaceHome, wsPrefix
	workspaceHome, wsPrefix = filepath.Dir(root), filepath.Base(root)
	t.Cleanup(func() { workspaceHome, wsPrefix = oldHome, oldPrefix })

	common := &InferCommonSpec{ProvenanceSchemaName: "patient_briefing"}
	spec := briefingTestChannelSpec(&ColumnEncodingSpec{
		Column: "cintel:Briefing_Input", EntityEncoding: "toon", RemoveModelPrefixes: true})
	check, err := resolveInferProvenanceSchema(common, briefingTestSource(spec), "ollama_config")
	if err != nil {
		t.Fatalf("the delivered configuration does not resolve: %v", err)
	}
	if check == nil || check.schema == nil {
		t.Fatal("expecting the delivered schema to resolve")
	}
	if len(common.ResponseFormat) == 0 {
		t.Fatal("expecting the operator to take its response_format from the provenance schema")
	}
	// And the invariant: whatever copies exist, they say the same thing. This is
	// what the resolution refuses at build and what nothing compared before.
	if len(secondCopy) > 0 {
		same, err := sameJSONDocument(secondCopy, check.schema.Response)
		if err != nil || !same {
			t.Errorf("the prompt template's response_format and the provenance schema's differ (err %v)", err)
		}
	}
	t.Logf("patient_briefing: %d rules, response_format adopted from provenance/%s%s, entity column %s as %s",
		len(check.schema.Rules), check.name, provenanceSchemaSuffix, check.entityColumn, check.encoding)
}
