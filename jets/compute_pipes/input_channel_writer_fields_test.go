package compute_pipes

// The four writer settings of FileConfig -- nbr_rows_in_record,
// put_headers_on_first_partition, quote_all_records and write_date_layout -- stay off
// input channels.
//
// syncInputChannelWithSchemaProvider used to copy them onto an input channel from its
// schema provider, where no reader consults them. They then travelled in the runtime
// document a starter hands a worker, and the Python cp_node's contract refuses them on a
// stage input channel: the first merge_files step routed to the Python node
// (healthcare_corpus, use_python_node_when) failed on nbr_rows_in_record and
// put_headers_on_first_partition before merging anything. A scan of every pipeline in
// workspaces/*/pipes_config and of this repository's fixtures found no input channel
// authoring any of the four, so the sync lost nothing by dropping them.

import (
	"encoding/json"
	"strings"
	"testing"
)

// writerProvider is a schema provider carrying all four writer settings beside three
// reader settings, so one fixture shows both halves of the sync.
func writerProvider() *SchemaProviderSpec {
	return &SchemaProviderSpec{
		Key:  "sp",
		Type: "default",
		FileConfig: FileConfig{
			NbrRowsInRecord:            75000,
			PutHeadersOnFirstPartition: true,
			QuoteAllRecords:            true,
			WriteDateLayout:            "2006-01-02",
			Delimiter:                  '|',
			Format:                     "csv",
			ReadDateLayout:             "01/02/2006",
		},
	}
}

func TestInputSyncCarriesNoWriterFields(t *testing.T) {
	sp := writerProvider()
	ic := InputChannelConfig{Name: "parts", Type: "stage"}
	syncInputChannelWithSchemaProvider(&ic, sp)

	if fields := inputChannelWriterFields(&ic); len(fields) > 0 {
		t.Errorf("the input sync put writer settings on an input channel: %v", fields)
	}
	// The reader settings still sync: the change is to four fields, not to the sync.
	if ic.Delimiter != '|' || ic.Format != "csv" || ic.ReadDateLayout != "01/02/2006" {
		t.Errorf("reader settings did not sync: delimiter %q, format %q, read_date_layout %q",
			ic.Delimiter, ic.Format, ic.ReadDateLayout)
	}
	// And the provider keeps its own values for the output channels that read them.
	if sp.NbrRowsInRecord != 75000 || !sp.PutHeadersOnFirstPartition || !sp.QuoteAllRecords ||
		sp.WriteDateLayout != "2006-01-02" {
		t.Errorf("the input sync changed the provider's writer settings: %+v", sp.FileConfig)
	}
}

func TestOutputSyncStillCarriesWriterFields(t *testing.T) {
	// The other half of the argument: the four are dropped from the input sync because
	// the output sync is where they belong, so it must go on carrying them.
	oc := OutputChannelConfig{Name: "table", Type: "stage"}
	syncOutputChannelWithSchemaProvider(&oc, writerProvider())
	if oc.NbrRowsInRecord != 75000 || !oc.PutHeadersOnFirstPartition || !oc.QuoteAllRecords ||
		oc.WriteDateLayout != "2006-01-02" {
		t.Errorf("the output sync no longer carries the writer settings: %+v", oc.FileConfig)
	}
}

func TestValidatePipeSpecConfigRefusesWriterFieldsOnInputChannels(t *testing.T) {
	cleanPipe := func(input InputChannelConfig) []PipeSpec {
		return []PipeSpec{{
			Type:         "fan_out",
			InputChannel: input,
			Apply: []TransformationSpec{{
				Type:          "map_record",
				OutputChannel: OutputChannelConfig{Name: "mapped_row"},
			}},
		}}
	}
	withWriterField := map[string]func(*InputChannelConfig){
		"nbr_rows_in_record":             func(ic *InputChannelConfig) { ic.NbrRowsInRecord = 75000 },
		"put_headers_on_first_partition": func(ic *InputChannelConfig) { ic.PutHeadersOnFirstPartition = true },
		"quote_all_records":              func(ic *InputChannelConfig) { ic.QuoteAllRecords = true },
		"write_date_layout":              func(ic *InputChannelConfig) { ic.WriteDateLayout = "2006-01-02" },
	}
	startup := &CpipesStartup{}

	current := cleanPipe(InputChannelConfig{Name: "input_row"})
	clean := &ComputePipesConfig{PipesConfig: current}
	if err := startup.ValidatePipeSpecConfig(clean, current); err != nil {
		t.Fatalf("a document with no writer settings on its input channels was refused: %v", err)
	}

	for field, set := range withWriterField {
		// On the input channel of the step that is starting.
		ic := InputChannelConfig{Name: "input_row"}
		set(&ic)
		pipe := cleanPipe(ic)
		err := startup.ValidatePipeSpecConfig(&ComputePipesConfig{PipesConfig: pipe}, pipe)
		if err == nil || !strings.Contains(err.Error(), field) {
			t.Errorf("%s on the starting step's input channel: want a refusal naming it, got %v", field, err)
		}

		// On one of its merge_channels.
		merged := InputChannelConfig{Name: "other_row"}
		set(&merged)
		pipe = cleanPipe(InputChannelConfig{Name: "input_row", MergeChannels: []InputChannelConfig{merged}})
		err = startup.ValidatePipeSpecConfig(&ComputePipesConfig{PipesConfig: pipe}, pipe)
		if err == nil || !strings.Contains(err.Error(), field) {
			t.Errorf("%s on a merge_channels entry: want a refusal naming it, got %v", field, err)
		}

		// On a later step's input channel, while the starting step is clean: a document
		// fails at its first step rather than part way through a run.
		later := InputChannelConfig{Name: "parts"}
		set(&later)
		doc := &ComputePipesConfig{
			PipesConfig: current,
			ConditionalPipesConfig: []ConditionalPipeSpec{{
				StepName:    "later",
				PipesConfig: cleanPipe(later),
			}},
		}
		err = startup.ValidatePipeSpecConfig(doc, current)
		if err == nil || !strings.Contains(err.Error(), field) {
			t.Errorf("%s on a later step's input channel: want a refusal naming it, got %v", field, err)
		}
	}
}

func TestAStageInputFedByAWriterProviderMarshalsWithoutWriterFields(t *testing.T) {
	// The deployed failure, as a Go test: a stage input channel whose schema provider
	// carries the four, validated as a starter validates it and then marshalled as a
	// starter marshals it into the runtime document.
	pipe := []PipeSpec{{
		Type:         "fan_out",
		InputChannel: InputChannelConfig{Name: "parts", Type: "stage", SchemaProvider: "sp"},
		Apply: []TransformationSpec{{
			Type:          "map_record",
			OutputChannel: OutputChannelConfig{Name: "mapped_row"},
		}},
	}}
	doc := &ComputePipesConfig{SchemaProviders: []*SchemaProviderSpec{writerProvider()}, PipesConfig: pipe}
	if err := (&CpipesStartup{}).ValidatePipeSpecConfig(doc, pipe); err != nil {
		t.Fatalf("validation refused a clean stage input channel: %v", err)
	}
	raw, err := json.Marshal(pipe[0].InputChannel)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"nbr_rows_in_record", "put_headers_on_first_partition", "quote_all_records", "write_date_layout"} {
		if strings.Contains(string(raw), `"`+key+`"`) {
			t.Errorf("the runtime input channel carries %s: %s", key, raw)
		}
	}
	if !strings.Contains(string(raw), `"format":"csv"`) {
		t.Errorf("the reader settings did not reach the runtime input channel: %s", raw)
	}
}
