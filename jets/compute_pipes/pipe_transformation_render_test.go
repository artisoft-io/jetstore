package compute_pipes

// Tests for the render transformation operator, see pipe_transformation_render.go
// (agentic_ai Phase 8 `AZ.3`).
//
// **The division of labour with `jets/agentic/template` is deliberate and it is
// what these tests do not cover.** The notation, the ten constructs, the twelve
// compile failures and the totality of the render are the engine's and are
// tested there over seventeen of its own cases. What is the operator's is the
// four questions the engine cannot answer: does a name resolve, does a column
// exist, what does a record that is not a document cost, and what does a
// violated `require` cost. The templates below are therefore as small as the
// question allows -- a briefing-shaped document here would be testing `AY.3`.

import (
	"database/sql"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/template"
	togo "github.com/toon-format/toon-go"
)

var renderTestColumns = []string{"member_key", "briefing_input", "briefing"}

// The process_errors channel as the default synthesis declares it, the three
// discriminators included -- so a test can assert `operator_type` rather than
// take on trust that the row says which operator wrote it.
var renderProcessErrorColumns = []string{"pipeline_execution_status_key", "session_id", "grouping_key",
	"row_jets_key", "input_column", "error_message", "rete_session_saved", "rete_session_triples",
	"shard_id", "cpipes_step_id", "error_channel", "operator_type"}

// renderTestTemplate is one paragraph carrying one `require`, which is the
// smallest document that can exercise both a clean render and a violation.
func renderTestTemplate(key string) TextTemplateSpec {
	return TextTemplateSpec{
		Key:   key,
		Width: 200,
		Elements: []*template.Element{
			{Text: "{{Disclaimer require 'the briefing carries no disclaimer'}}"},
			{Text: "{{Events[]|count}} {{Events[]|count|plural: 'event'}} on record."},
		},
	}
}

type renderTestResult struct {
	outputRecords [][]any
	errorRecords  [][]any
	applyErrs     []error
}

// runRenderTestPipe builds the operator against a two-channel registry, applies
// the records and returns what came out of the output and error channels. It is
// `runOllamaTestPipe`'s shape with the infer server taken out, since a render
// has nothing to call.
func runRenderTestPipe(t *testing.T, cpConfig *ComputePipesConfig, config *RenderSpec,
	records [][]any, encodings []*ColumnEncodingSpec) *renderTestResult {
	t.Helper()

	columnsMap := make(map[string]int, len(renderTestColumns))
	for i, c := range renderTestColumns {
		columnsMap[c] = i
	}
	channelSpec := &ChannelSpec{Name: "briefings", Columns: renderTestColumns,
		columnsMap: &columnsMap, ColumnEncodings: encodings}
	peColumnsMap := make(map[string]int, len(renderProcessErrorColumns))
	for i, c := range renderProcessErrorColumns {
		peColumnsMap[c] = i
	}
	peSpec := &ChannelSpec{Name: "process_errors", Columns: renderProcessErrorColumns, columnsMap: &peColumnsMap}

	registry := &ChannelRegistry{
		ComputeChannels: map[string]*Channel{
			"briefings.out": {Name: "briefings.out", Channel: make(chan []any),
				Columns: &columnsMap, Config: channelSpec},
			"process_errors.out": {Name: "process_errors.out", Channel: make(chan []any),
				Columns: &peColumnsMap, Config: peSpec},
		},
		ClosedChannels: make(map[string]bool),
	}
	source := &InputChannel{Name: "briefings.in", Columns: &columnsMap, Config: channelSpec}
	builderContext := &BuilderContext{
		cpConfig:        cpConfig,
		channelRegistry: registry,
		done:            make(chan struct{}),
		errCh:           make(chan error, 10),
		env:             make(map[string]any),
	}
	outputCh, err := registry.GetOutputChannel("briefings.out")
	if err != nil {
		t.Fatal(err)
	}
	spec := &TransformationSpec{Type: RenderOperatorType, RenderConfig: config}
	pipe, err := builderContext.NewRenderTransformationPipe(source, outputCh, spec)
	if err != nil {
		t.Fatal(err)
	}

	// Drain while the operator runs: the channels are unbuffered.
	result := &renderTestResult{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for record := range registry.ComputeChannels["briefings.out"].Channel {
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
		if err := pipe.Apply(&records[i]); err != nil {
			result.applyErrs = append(result.applyErrs, err)
		}
	}
	if err := pipe.Done(); err != nil {
		t.Fatal(err)
	}
	pipe.Finally()
	// The pipe executor closes the output channels once every operator is done
	// (StartFanOutPipe); mimic it here.
	registry.CloseChannel("briefings.out")
	if config.ErrorChannel != nil {
		registry.CloseChannel("process_errors.out")
	}
	wg.Wait()
	return result
}

func renderTestConfig(templateName string) *RenderSpec {
	return &RenderSpec{
		TemplateName: templateName,
		InputColumn:  "briefing_input",
		OutputColumn: "briefing",
		RowKeyColumn: "member_key",
	}
}

func renderTestCpConfig(templates ...TextTemplateSpec) *ComputePipesConfig {
	return &ComputePipesConfig{TextTemplates: templates}
}

func renderTestErrorChannel() *OutputChannelConfig {
	return &OutputChannelConfig{Name: "process_errors.out", SpecName: DefaultErrorChannelSpecName}
}

func mustJson(t *testing.T, doc map[string]any) string {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// ---------------------------------------------------------------------------
// A record rendering
// ---------------------------------------------------------------------------

// TestRenderWritesTheColumnInPlace is criterion 83's unit: a serialised entity
// column goes in, the rendered text comes out on another column of the same
// record, and every other column is untouched.
func TestRenderWritesTheColumnInPlace(t *testing.T) {
	doc := map[string]any{
		"Disclaimer": "This briefing is generated from claims data.",
		"Events":     []any{map[string]any{"code": "A"}, map[string]any{"code": "B"}},
	}
	result := runRenderTestPipe(t, renderTestCpConfig(renderTestTemplate("briefing")),
		renderTestConfig("briefing"),
		[][]any{{"member-1", mustJson(t, doc), nil}}, nil)

	if len(result.applyErrs) != 0 {
		t.Fatalf("unexpected errors: %v", result.applyErrs)
	}
	if len(result.outputRecords) != 1 {
		t.Fatalf("got %d records out, want 1", len(result.outputRecords))
	}
	out := result.outputRecords[0]
	if out[0] != "member-1" {
		t.Errorf("the operator did not augment in place: member_key is %v", out[0])
	}
	want := "This briefing is generated from claims data.\n\n2 events on record."
	if out[2] != want {
		t.Errorf("briefing is %q, want %q", out[2], want)
	}
}

// TestRenderReadsAToonColumn is the other half of the input contract. The same
// document, the same template and the same expected text -- which is the claim
// `scalar`'s doc block makes, asserted from the operator's side.
func TestRenderReadsAToonColumn(t *testing.T) {
	doc := map[string]any{
		"Disclaimer": "This briefing is generated from claims data.",
		"Events":     []any{map[string]any{"code": "A"}, map[string]any{"code": "B"}},
	}
	encoded, err := togo.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	config := renderTestConfig("briefing")
	config.InputEncoding = "toon"
	result := runRenderTestPipe(t, renderTestCpConfig(renderTestTemplate("briefing")), config,
		[][]any{{"member-1", string(encoded), nil}}, nil)

	if len(result.outputRecords) != 1 {
		t.Fatalf("got %d records out, want 1", len(result.outputRecords))
	}
	want := "This briefing is generated from claims data.\n\n2 events on record."
	if got := result.outputRecords[0][2]; got != want {
		t.Errorf("briefing is %q, want %q", got, want)
	}
}

// TestRenderTakesTheEncodingFromTheChannel is the operator reading what the
// channel spec already says rather than asking an author to repeat it.
func TestRenderTakesTheEncodingFromTheChannel(t *testing.T) {
	doc := map[string]any{"Disclaimer": "notice", "Events": []any{map[string]any{"code": "A"}}}
	encoded, err := togo.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	encodings := []*ColumnEncodingSpec{{Column: "briefing_input", EntityEncoding: "toon"}}
	// No input_encoding on the operator: the channel is the only thing that says
	// so, and a json decode of this column would fail.
	result := runRenderTestPipe(t, renderTestCpConfig(renderTestTemplate("briefing")),
		renderTestConfig("briefing"), [][]any{{"member-1", string(encoded), nil}}, encodings)

	if len(result.outputRecords) != 1 {
		t.Fatalf("got %d records out, want 1", len(result.outputRecords))
	}
	if got := result.outputRecords[0][2]; got != "notice\n\n1 event on record." {
		t.Errorf("briefing is %q", got)
	}
}

// TestRenderMissingPathRendersTheEmptyForm is criterion 84's second sentence:
// **a path resolving to nothing at run time is not a failure.** The document
// carries no `Events`, the count renders as zero, and no error of any kind is
// produced -- which is the property that makes every other failure in this file
// a build failure or a declared assertion rather than a surprise.
func TestRenderMissingPathRendersTheEmptyForm(t *testing.T) {
	doc := map[string]any{"Disclaimer": "notice"}
	config := renderTestConfig("briefing")
	config.ErrorChannel = renderTestErrorChannel()
	result := runRenderTestPipe(t, renderTestCpConfig(renderTestTemplate("briefing")), config,
		[][]any{{"member-1", mustJson(t, doc), nil}}, nil)

	if len(result.applyErrs) != 0 || len(result.errorRecords) != 0 {
		t.Fatalf("a missing path was reported as a failure: %v / %v", result.applyErrs, result.errorRecords)
	}
	if len(result.outputRecords) != 1 {
		t.Fatalf("got %d records out, want 1", len(result.outputRecords))
	}
	if got := result.outputRecords[0][2]; got != "notice\n\n0 events on record." {
		t.Errorf("briefing is %q", got)
	}
}

// ---------------------------------------------------------------------------
// A record whose entity column is not a document
// ---------------------------------------------------------------------------

// TestRenderRejectsARecordThatIsNotADocument walks the shapes a column can
// arrive in that are not a document, and asserts that each is reported rather
// than rendered as an empty briefing.
//
// **The array case is the one worth the row.** `[1,2]` is valid json and decodes
// without complaint; every path then resolves to nothing and the template
// renders its empty form, which is a briefing saying a member has no events.
// That is I-515's shape -- an empty artefact indistinguishable from a broken
// one -- and it is why the type assertion is in `decodeRenderJson` rather than
// left to the engine.
func TestRenderRejectsARecordThatIsNotADocument(t *testing.T) {
	cases := []struct {
		name  string
		value any
		says  string
	}{
		{"not json", "this is not json", "not valid json"},
		{"a json array", "[1, 2]", "rather than to a json object"},
		{"a json scalar", `"just a string"`, "rather than to a json object"},
		{"null", nil, "is null"},
		{"blank", "   ", "is empty"},
		{"not text", 42, "carries a int"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := renderTestConfig("briefing")
			config.ErrorChannel = renderTestErrorChannel()
			config.OnError = OnErrorDrop
			result := runRenderTestPipe(t, renderTestCpConfig(renderTestTemplate("briefing")), config,
				[][]any{{"member-1", tc.value, nil}}, nil)

			if len(result.outputRecords) != 0 {
				t.Errorf("on_error: drop still emitted %d record(s)", len(result.outputRecords))
			}
			if len(result.errorRecords) != 1 {
				t.Fatalf("got %d error rows, want 1", len(result.errorRecords))
			}
			message, _ := result.errorRecords[0][5].(string)
			if !strings.Contains(message, tc.says) {
				t.Errorf("the error row says %q, want it to mention %q", message, tc.says)
			}
		})
	}
}

// TestRenderShortRecordIsReported covers `pad_short_rows_with_nulls`'s legitimate
// short record arriving before the input column exists.
func TestRenderShortRecordIsReported(t *testing.T) {
	config := renderTestConfig("briefing")
	config.ErrorChannel = renderTestErrorChannel()
	config.OnError = OnErrorDrop
	result := runRenderTestPipe(t, renderTestCpConfig(renderTestTemplate("briefing")), config,
		[][]any{{"member-1"}}, nil)

	if len(result.errorRecords) != 1 {
		t.Fatalf("got %d error rows, want 1", len(result.errorRecords))
	}
	if message, _ := result.errorRecords[0][5].(string); !strings.Contains(message, "carries no value") {
		t.Errorf("the error row says %q", message)
	}
}

// ---------------------------------------------------------------------------
// The error channel path, and Q-107
// ---------------------------------------------------------------------------

// TestRenderViolationFailsTheRow is Q-107 asserted rather than described.
//
// The record carries no `Disclaimer`, the template declares one required, and
// the three `on_error` policies give the three answers. **Under every one of
// them the rendered text reaches nobody**, which is the whole of the argument
// for the answer: `prose.Render`'s refusal exists because prose delivered
// without its notice is worse than no prose, and pass-through means *without
// this operator's enrichment* rather than *with a partial artefact*.
func TestRenderViolationFailsTheRow(t *testing.T) {
	doc := map[string]any{"Events": []any{map[string]any{"code": "A"}}}
	cases := []struct {
		onError     string
		wantOut     int
		wantErrRows int
		wantApplyEr int
	}{
		{OnErrorPassThrough, 1, 1, 0},
		{OnErrorDrop, 0, 1, 0},
		{OnErrorFail, 0, 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.onError, func(t *testing.T) {
			config := renderTestConfig("briefing")
			config.ErrorChannel = renderTestErrorChannel()
			config.OnError = tc.onError
			result := runRenderTestPipe(t, renderTestCpConfig(renderTestTemplate("briefing")), config,
				[][]any{{"member-1", mustJson(t, doc), nil}}, nil)

			if len(result.outputRecords) != tc.wantOut {
				t.Errorf("got %d records out, want %d", len(result.outputRecords), tc.wantOut)
			}
			if len(result.applyErrs) != tc.wantApplyEr {
				t.Errorf("got %d Apply errors, want %d: %v",
					len(result.applyErrs), tc.wantApplyEr, result.applyErrs)
			}
			if len(result.errorRecords) != tc.wantErrRows {
				t.Fatalf("got %d error rows, want %d", len(result.errorRecords), tc.wantErrRows)
			}
			// Whatever the policy, the partial briefing was not written.
			for _, record := range result.outputRecords {
				if record[2] != nil {
					t.Errorf("the output column carries %q; a failed row must not carry a partial "+
						"briefing under any policy", record[2])
				}
			}
			row := result.errorRecords[0]
			message, _ := row[5].(string)
			if !strings.Contains(message, "the briefing carries no disclaimer") {
				t.Errorf("the error row does not carry the template's own message: %q", message)
			}
			if !strings.Contains(message, "template 'briefing'") {
				t.Errorf("the error row does not name the template: %q", message)
			}
			if got := renderTestNullString(row[3]); got != "member-1" {
				t.Errorf("row_jets_key is %q, want the row_key_column's value", got)
			}
			if got := renderTestNullString(row[4]); got != "briefing_input" {
				t.Errorf("input_column is %q", got)
			}
			if row[11] != RenderOperatorType {
				t.Errorf("operator_type is %v, want %q", row[11], RenderOperatorType)
			}
			if row[10] != "process_errors.out" {
				t.Errorf("error_channel is %v", row[10])
			}
		})
	}
}

// TestRenderReportsEachViolationSeparately: a violation inside an `each` carries
// the item index, which is the one piece of context the template's message
// cannot carry. Collapsing two violations into one row would throw it away.
func TestRenderReportsEachViolationSeparately(t *testing.T) {
	tmpl := TextTemplateSpec{
		Key:   "items",
		Width: 200,
		Elements: []*template.Element{
			{Text: "{{each Events[] sep: ', ' last: ' and '}}" +
				"{{code require 'an event carries no code'}}{{end}}."},
		},
	}
	doc := map[string]any{"Events": []any{
		map[string]any{"code": "A"},
		map[string]any{"note": "no code here"},
		map[string]any{"note": "nor here"},
	}}
	config := renderTestConfig("items")
	config.ErrorChannel = renderTestErrorChannel()
	config.OnError = OnErrorDrop
	result := runRenderTestPipe(t, renderTestCpConfig(tmpl), config,
		[][]any{{"member-1", mustJson(t, doc), nil}}, nil)

	if len(result.errorRecords) != 2 {
		t.Fatalf("got %d error rows, want one per violation", len(result.errorRecords))
	}
	first, _ := result.errorRecords[0][5].(string)
	second, _ := result.errorRecords[1][5].(string)
	if !strings.Contains(first, "item 1") || !strings.Contains(second, "item 2") {
		t.Errorf("the rows do not say which item: %q / %q", first, second)
	}
}

// TestRenderCapsErrorReporting: max_error_count caps what reaches the channel
// and does not cap what the policy does to the record.
func TestRenderCapsErrorReporting(t *testing.T) {
	doc := mustJson(t, map[string]any{"Events": []any{}})
	config := renderTestConfig("briefing")
	config.ErrorChannel = renderTestErrorChannel()
	config.OnError = OnErrorDrop
	config.MaxErrorCount = 2
	records := [][]any{
		{"m-1", doc, nil}, {"m-2", doc, nil}, {"m-3", doc, nil}, {"m-4", doc, nil},
	}
	result := runRenderTestPipe(t, renderTestCpConfig(renderTestTemplate("briefing")), config, records, nil)

	if len(result.errorRecords) != 2 {
		t.Errorf("got %d error rows, want max_error_count = 2", len(result.errorRecords))
	}
	if len(result.outputRecords) != 0 {
		t.Errorf("got %d records out; every one of the four failed", len(result.outputRecords))
	}
}

// renderTestNullString reads a column write2Chan writes as a sql.NullString.
func renderTestNullString(v any) string {
	if s, ok := v.(sql.NullString); ok {
		return s.String
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ---------------------------------------------------------------------------
// Build failures: C13, C14, C15 and the engine's twelve
// ---------------------------------------------------------------------------

// buildRenderPipe builds the operator and returns the build error, if any.
func buildRenderPipe(t *testing.T, cpConfig *ComputePipesConfig, config *RenderSpec,
	encodings []*ColumnEncodingSpec) error {
	t.Helper()
	columnsMap := make(map[string]int, len(renderTestColumns))
	for i, c := range renderTestColumns {
		columnsMap[c] = i
	}
	channelSpec := &ChannelSpec{Name: "briefings", Columns: renderTestColumns,
		columnsMap: &columnsMap, ColumnEncodings: encodings}
	registry := &ChannelRegistry{
		ComputeChannels: map[string]*Channel{
			"briefings.out": {Name: "briefings.out", Channel: make(chan []any),
				Columns: &columnsMap, Config: channelSpec},
		},
		ClosedChannels: make(map[string]bool),
	}
	source := &InputChannel{Name: "briefings.in", Columns: &columnsMap, Config: channelSpec}
	builderContext := &BuilderContext{
		cpConfig:        cpConfig,
		channelRegistry: registry,
		done:            make(chan struct{}),
		errCh:           make(chan error, 10),
		env:             make(map[string]any),
	}
	outputCh, err := registry.GetOutputChannel("briefings.out")
	if err != nil {
		t.Fatal(err)
	}
	_, err = builderContext.NewRenderTransformationPipe(source, outputCh,
		&TransformationSpec{Type: RenderOperatorType, RenderConfig: config})
	return err
}

// TestRenderBuildFailures is criterion 84 from the operator's side: every
// question about how the step and its template are spelled is answered before a
// record arrives.
func TestRenderBuildFailures(t *testing.T) {
	good := renderTestTemplate("briefing")
	badColumn := renderTestConfig("briefing")
	badColumn.InputColumn = "not_a_column"
	badOutput := renderTestConfig("briefing")
	badOutput.OutputColumn = "not_a_column"
	sameColumn := renderTestConfig("briefing")
	sameColumn.OutputColumn = sameColumn.InputColumn
	badRowKey := renderTestConfig("briefing")
	badRowKey.RowKeyColumn = "not_a_column"
	badEncoding := renderTestConfig("briefing")
	badEncoding.InputEncoding = "yaml"
	noName := renderTestConfig("")

	cases := []struct {
		name      string
		which     string
		cpConfig  *ComputePipesConfig
		config    *RenderSpec
		encodings []*ColumnEncodingSpec
		says      string
	}{
		{"a name text_templates does not carry", "C14",
			renderTestCpConfig(good), renderTestConfig("missing"), nil, "not defined in text_templates"},
		{"no text_templates at all", "C14",
			renderTestCpConfig(), renderTestConfig("briefing"), nil, "declares no text_templates"},
		{"two entries sharing a key", "C13",
			renderTestCpConfig(good, renderTestTemplate("briefing")), renderTestConfig("briefing"), nil,
			"more than one entry with the key"},
		{"an unknown input column", "C15",
			renderTestCpConfig(good), badColumn, nil, "is not a column of the input channel"},
		{"an unknown output column", "C15",
			renderTestCpConfig(good), badOutput, nil, "is not a column of the output channel"},
		{"an unknown row key column", "C15",
			renderTestCpConfig(good), badRowKey, nil, "row_key_column"},
		{"input and output the same column", "operator",
			renderTestCpConfig(good), sameColumn, nil, "overwrite the entity it rendered from"},
		{"no template_name", "operator",
			renderTestCpConfig(good), noName, nil, "must specify a template_name"},
		{"an unknown input_encoding", "operator",
			renderTestCpConfig(good), badEncoding, nil, "unknown input_encoding"},
		{"an encoding the channel contradicts", "operator",
			renderTestCpConfig(good), renderTestConfigWithEncoding("briefing", "json"),
			[]*ColumnEncodingSpec{{Column: "briefing_input", EntityEncoding: "toon"}},
			"one of them is wrong"},
		{"a column that is already prose", "operator",
			renderTestCpConfig(good), renderTestConfig("briefing"),
			[]*ColumnEncodingSpec{{Column: "briefing_input", EntityEncoding: "briefing_prose"}},
			"already rendered prose"},
		// The engine's own twelve reach the build through Compile. One of each
		// kind is enough here: that they are exhaustive is asserted where they
		// are implemented.
		{"a template with no width", "C3",
			renderTestCpConfig(TextTemplateSpec{Key: "briefing",
				Elements: []*template.Element{{Text: "x"}}}), renderTestConfig("briefing"), nil,
			"width is absent"},
		{"a template with an unclosed block", "C5",
			renderTestCpConfig(TextTemplateSpec{Key: "briefing", Width: 78,
				Elements: []*template.Element{{Text: "{{if a == 'b'}}x"}}}), renderTestConfig("briefing"), nil,
			"configuration error"},
		{"a template with no elements", "C2",
			renderTestCpConfig(TextTemplateSpec{Key: "briefing", Width: 78}),
			renderTestConfig("briefing"), nil, "carries no elements"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := buildRenderPipe(t, tc.cpConfig, tc.config, tc.encodings)
			if err == nil {
				t.Fatalf("%s: the build succeeded", tc.which)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("%s: the error is %q, want it to mention %q", tc.which, err, tc.says)
			}
		})
	}
}

func renderTestConfigWithEncoding(name, encoding string) *RenderSpec {
	config := renderTestConfig(name)
	config.InputEncoding = encoding
	return config
}

// TestRenderRefusesMismatchedChannels: the operator augments in place, so the
// two channels must carry the same columns -- the infer operators' rule and
// their message.
func TestRenderRefusesMismatchedChannels(t *testing.T) {
	inColumns := map[string]int{"member_key": 0, "briefing_input": 1, "briefing": 2}
	outColumns := map[string]int{"member_key": 0, "briefing": 1}
	inSpec := &ChannelSpec{Name: "briefings", Columns: renderTestColumns, columnsMap: &inColumns}
	outSpec := &ChannelSpec{Name: "other", Columns: []string{"member_key", "briefing"}, columnsMap: &outColumns}
	registry := &ChannelRegistry{
		ComputeChannels: map[string]*Channel{
			"other.out": {Name: "other.out", Channel: make(chan []any), Columns: &outColumns, Config: outSpec},
		},
		ClosedChannels: make(map[string]bool),
	}
	builderContext := &BuilderContext{
		cpConfig:        renderTestCpConfig(renderTestTemplate("briefing")),
		channelRegistry: registry,
		done:            make(chan struct{}),
		errCh:           make(chan error, 10),
		env:             make(map[string]any),
	}
	outputCh, err := registry.GetOutputChannel("other.out")
	if err != nil {
		t.Fatal(err)
	}
	_, err = builderContext.NewRenderTransformationPipe(
		&InputChannel{Name: "briefings.in", Columns: &inColumns, Config: inSpec}, outputCh,
		&TransformationSpec{Type: RenderOperatorType, RenderConfig: renderTestConfig("briefing")})
	if err == nil {
		t.Fatal("the build succeeded with channels of different shapes")
	}
	if !strings.Contains(err.Error(), "must share the same channel spec") {
		t.Errorf("the error is %q", err)
	}
}

// ---------------------------------------------------------------------------
// Criterion 85 — a text_templates array reaches a worker node
// ---------------------------------------------------------------------------

// TestTextTemplatesReachAWorkerNode is criterion 85, and it asserts the two
// halves separately because they fail separately.
//
// **F1130 is the failure it exists to prevent**, and that failure has already
// happened once: `PromptTemplates` was missing from both node-config literals
// from the day they were written, every named prompt template resolved to
// nothing on the node, an inline one worked, and nothing failed until a live run
// used the named form. A render operator has no inline form at all, so the same
// omission would take every `text_templates` document out of every deployed run.
//
// The first half is structural -- both literals assign the field -- and is the
// assertion `TestNodeConfigCarriesEveryFieldOrSaysWhyNot` makes for the class.
// **The second half is behavioural and is the one the class-level test cannot
// make**: a node receives the literal *as json* and rebuilds it, so what matters
// is that the array survives that round trip and that the operator still
// resolves its name on the far side.
func TestTextTemplatesReachAWorkerNode(t *testing.T) {
	for _, file := range []string{"actions_start_sharding_cp.go", "actions_start_reducing_cp.go"} {
		if !literalFields(t, file)["TextTemplates"] {
			t.Errorf("%s does not give the node TextTemplates; a text_templates array declared at "+
				"the configuration root would reach no render operator, and the failure would be a "+
				"build error on a worker node in a deployed run (F1130)", file)
		}
	}

	// What the node is handed is the marshalled literal and nothing else.
	atStartup := &ComputePipesConfig{TextTemplates: []TextTemplateSpec{renderTestTemplate("briefing")}}
	wire, err := json.Marshal(atStartup)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wire), `"text_templates"`) {
		t.Fatalf("the node config does not carry a text_templates key: %s", wire)
	}
	var atNode ComputePipesConfig
	if err := json.Unmarshal(wire, &atNode); err != nil {
		t.Fatalf("the node cannot read back the config it was handed: %v", err)
	}
	if len(atNode.TextTemplates) != 1 {
		t.Fatalf("the node sees %d text templates, want 1", len(atNode.TextTemplates))
	}

	// And the operator resolves the name against what the node decoded, which is
	// the step that failed silently for prompt templates.
	if err := buildRenderPipe(t, &atNode, renderTestConfig("briefing"), nil); err != nil {
		t.Fatalf("the operator cannot resolve its template on the node: %v", err)
	}
}

// TestTextTemplateSpecCarriesTheDocument keeps the copied document fields in
// step with the engine's `Spec`.
//
// **The two types exist separately for one reason**: `template.Spec` decodes
// strictly, so it admits no `comment`, and every element of this configuration
// carries one. The cost of the copy is that a field added to the engine's
// document reaches no pipeline configuration, silently -- which is
// `node_config_fields_test.go`'s failure one level over, and gets the same
// answer: the omission has to be written down as a decision.
func TestTextTemplateSpecCarriesTheDocument(t *testing.T) {
	omitted := map[string]string{}
	engine := reflect.TypeOf(template.Spec{})
	config := reflect.TypeOf(TextTemplateSpec{})
	for i := 0; i < engine.NumField(); i++ {
		name := engine.Field(i).Name
		field, ok := config.FieldByName(name)
		if !ok {
			if _, said := omitted[name]; !said {
				t.Errorf("template.Spec has %s and TextTemplateSpec does not, and nothing says why not. "+
					"A document field the configuration cannot express reaches no pipeline.", name)
			}
			continue
		}
		if field.Type != engine.Field(i).Type {
			t.Errorf("%s is %s on template.Spec and %s on TextTemplateSpec", name, engine.Field(i).Type, field.Type)
		}
		if got, want := field.Tag.Get("json"), engine.Field(i).Tag.Get("json"); got != want {
			t.Errorf("%s is tagged %q on TextTemplateSpec and %q on template.Spec; a document that "+
				"round trips through the node config must decode the same either way", name, got, want)
		}
	}
	// And Document() must carry every one of them across.
	spec := &TextTemplateSpec{Key: "k", Width: 78, Empty: "nothing to report",
		Elements: []*template.Element{{Text: "x"}}}
	doc := spec.Document()
	if doc.Key != spec.Key || doc.Width != spec.Width || doc.Empty != spec.Empty ||
		len(doc.Elements) != len(spec.Elements) {
		t.Errorf("Document() dropped a field: %+v", doc)
	}
}

// ---------------------------------------------------------------------------
// Criterion 86 — the six sites, and the pair that must agree
// ---------------------------------------------------------------------------

// TestRenderIsWiredIntoTheErrorChannelSynthesis asserts the half of criterion 86
// a test can assert: `reportsRowLevelFailures` and `errorChannelConfig` name the
// same operator, and the synthesis places a channel a render operator can find.
//
// **The criterion is graded by reading both switches and this test does not
// replace that.** What it does catch is the asymmetric case the comment on
// `reportsRowLevelFailures` warns about -- an operator that gets a channel
// nothing can find, or finds a channel nothing gave it.
func TestRenderIsWiredIntoTheErrorChannelSynthesis(t *testing.T) {
	if !reportsRowLevelFailures(RenderOperatorType) {
		t.Fatal("reportsRowLevelFailures does not name render")
	}
	spec := &TransformationSpec{Type: RenderOperatorType, RenderConfig: renderTestConfig("briefing")}
	ec := &OutputChannelConfig{Name: "jets_process_errors.render.0.0", SpecName: DefaultErrorChannelSpecName}
	if !setErrorChannelConfig(spec, ec) {
		t.Fatal("setErrorChannelConfig has no render arm")
	}
	if got := errorChannelConfig(spec); got != ec {
		t.Fatalf("errorChannelConfig returned %v; the two switches must name the same operators", got)
	}
	setDefaultMaxErrorCount(spec, 7)
	if spec.RenderConfig.MaxErrorCount != 7 {
		t.Errorf("setDefaultMaxErrorCount has no render arm: max_error_count is %d",
			spec.RenderConfig.MaxErrorCount)
	}
	// An author's explicit number still wins over the deployment's.
	spec.RenderConfig.MaxErrorCount = 3
	setDefaultMaxErrorCount(spec, 7)
	if spec.RenderConfig.MaxErrorCount != 3 {
		t.Errorf("the deployment cap overrode an explicit max_error_count")
	}
	// And an operator with no config struct is skipped rather than given one.
	if setErrorChannelConfig(&TransformationSpec{Type: RenderOperatorType}, ec) {
		t.Error("setErrorChannelConfig placed a channel on a render operator with no render_config")
	}
}

// TestRenderIsValidatedAtStartup covers the sixth site: `ValidatePipeSpecConfig`
// recognises `render_config` and reports what it can before a node is launched.
func TestRenderIsValidatedAtStartup(t *testing.T) {
	cases := []struct {
		name   string
		config *RenderSpec
		says   string
	}{
		{"no render_config", nil, "missing render_config"},
		{"no template_name", &RenderSpec{InputColumn: "a", OutputColumn: "b"}, "must specify a template_name"},
		{"no columns", &RenderSpec{TemplateName: "briefing"}, "input_column and output_column"},
	}
	// A document nobody names is compiled too, so a second entry carrying a
	// broken construct fails at startup rather than on the day a step is pointed
	// at it.
	t.Run("an unreferenced template that does not compile", func(t *testing.T) {
		args := &CpipesStartup{}
		cpConfig := renderTestCpConfig(renderTestTemplate("briefing"),
			TextTemplateSpec{Key: "not_used_yet", Width: 78,
				Elements: []*template.Element{{Text: "{{each xs[] sep: ', ' last: ' and '}}x"}}})
		err := args.ValidatePipeSpecConfig(cpConfig, nil)
		if err == nil {
			t.Fatal("the configuration validated with a broken text template in it")
		}
		if !strings.Contains(err.Error(), "text_templates[1]") {
			t.Errorf("the error is %q, want it to name the entry", err)
		}
	})
	t.Run("two templates sharing a key", func(t *testing.T) {
		args := &CpipesStartup{}
		cpConfig := renderTestCpConfig(renderTestTemplate("briefing"), renderTestTemplate("briefing"))
		err := args.ValidatePipeSpecConfig(cpConfig, nil)
		if err == nil || !strings.Contains(err.Error(), "more than one entry with the key") {
			t.Fatalf("the error is %v", err)
		}
	})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := &CpipesStartup{}
			cpConfig := renderTestCpConfig(renderTestTemplate("briefing"))
			pipeConfig := []PipeSpec{{
				Type:         "fan_out",
				InputChannel: InputChannelConfig{Name: "briefings.in"},
				Apply: []TransformationSpec{{
					Type:          RenderOperatorType,
					RenderConfig:  tc.config,
					OutputChannel: OutputChannelConfig{Name: "briefings.out", SpecName: "briefings"},
				}},
			}}
			err := args.ValidatePipeSpecConfig(cpConfig, pipeConfig)
			if err == nil {
				t.Fatal("the configuration validated")
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("the error is %q, want it to mention %q", err, tc.says)
			}
		})
	}
}
