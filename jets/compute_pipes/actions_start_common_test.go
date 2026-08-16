package compute_pipes

import (
	"testing"
)

func TestMergeTransformationSpec(t *testing.T) {
	// Test merging two TransformationSpec structs
	host := &TransformationSpec{
		Type: "partition_writer",
		PartitionWriterConfig: &PartitionWriterSpec{
			DeviceWriterType: "S3",
			PartitionSize:    50,
		},
	}
	override := &TransformationSpec{
		Type: "", // Empty type means we want to merge fields, not replace
		PartitionWriterConfig: &PartitionWriterSpec{
			DeviceWriterType: "Parquet",
			PartitionSize:    500,
		},
	}

	err := MergeTransformationSpec(host, override)
	if err != nil {
		t.Fatalf("MergeTransformationSpec failed: %v", err)
	}
	if host.Type != "partition_writer" {
		t.Errorf("Expected Type 'partition_writer', got '%s'", host.Type)
	}
	if host.PartitionWriterConfig.DeviceWriterType != "Parquet" {
		t.Errorf("Expected DeviceWriterType 'Parquet', got '%s'", host.PartitionWriterConfig.DeviceWriterType)
	}
	if host.PartitionWriterConfig.PartitionSize != 500 {
		t.Errorf("Expected PartitionSize 500, got '%d'", host.PartitionWriterConfig.PartitionSize)
	}

	// Test merging with additional fields in override
	host = &TransformationSpec{
		Type: "partition_writer",
		PartitionWriterConfig: &PartitionWriterSpec{
			DeviceWriterType: "S3",
			PartitionSize:    50,
		},
	}
	override = &TransformationSpec{
		Columns: []TransformationColumnSpec{
			{Name: "col1", Type: "type1"},
			{Name: "col2", Type: "type2"},
		},
	}
	err = MergeTransformationSpec(host, override)
	if err != nil {
		t.Fatalf("MergeTransformationSpec failed: %v", err)
	}

	if host.Type != "partition_writer" {
		t.Errorf("Expected Type 'partition_writer', got '%s'", host.Type)
	}
	if host.PartitionWriterConfig.DeviceWriterType != "S3" {
		t.Errorf("Expected DeviceWriterType 'S3', got '%s'", host.PartitionWriterConfig.DeviceWriterType)
	}
	if len(host.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(host.Columns))
	} else {
		if host.Columns[0].Name != "col1" || host.Columns[0].Type != "type1" {
			t.Errorf("Unexpected first column: %+v", host.Columns[0])
		}
		if host.Columns[1].Name != "col2" || host.Columns[1].Type != "type2" {
			t.Errorf("Unexpected second column: %+v", host.Columns[1])
		}
	}

	// Test replacing the entire TransformationSpec when Type is set
	host = &TransformationSpec{
		Type: "partition_writer",
		PartitionWriterConfig: &PartitionWriterSpec{
			DeviceWriterType: "S3",
			PartitionSize:    50,
		},
	}
	override = &TransformationSpec{
		Type: "map", // Non-empty type means replace
		MapRecordConfig: &MapRecordSpec{
			FileMappingTableName: "my_mapping_table_name",
		},
		Columns: []TransformationColumnSpec{
			{Name: "col1", Type: "type1"},
			{Name: "col2", Type: "type2"},
		},
	}

	err = MergeTransformationSpec(host, override)
	if err != nil {
		t.Fatalf("MergeTransformationSpec failed: %v", err)
	}
	if host.Type != "map" {
		t.Errorf("Expected Type 'map', got '%s'", host.Type)
	}
	if host.MapRecordConfig == nil || host.MapRecordConfig.FileMappingTableName != "my_mapping_table_name" {
		t.Errorf("Expected MapRecordConfig with FileMappingTableName 'my_mapping_table_name', got %+v", host.MapRecordConfig)
	}
	if host.PartitionWriterConfig != nil {
		t.Errorf("Expected PartitionWriterConfig to be nil, got %+v", host.PartitionWriterConfig)
	}
	if len(host.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(host.Columns))
	} else {
		if host.Columns[0].Name != "col1" || host.Columns[0].Type != "type1" {
			t.Errorf("Unexpected first column: %+v", host.Columns[0])
		}
		if host.Columns[1].Name != "col2" || host.Columns[1].Type != "type2" {
			t.Errorf("Unexpected second column: %+v", host.Columns[1])
		}
	}
}

func TestApplyAllConditionalTransformationSpec(t *testing.T) {
	// Test applying conditional transformation specs
	envSettings := map[string]any{
		"$USE_S3_WRITER": "true",
		"count":          5,
	}

	pipeConfig := []PipeSpec{
		{
			Apply: []TransformationSpec{
				{
					Type: "partition_writer",
					PartitionWriterConfig: &PartitionWriterSpec{
						DeviceWriterType: "S3",
						PartitionSize:    100,
					},
					ConditionalConfig: []*ConditionalTransformationSpec{
						{
							When: ExpressionNode{
								Lhs: &ExpressionNode{
									Type: "select",
									Expr: "$USE_S3_WRITER",
								},
								Op: "==",
								Rhs: &ExpressionNode{
									Type: "value",
									Expr: "'true'",
								},
							},
							Then: TransformationSpec{
								PartitionWriterConfig: &PartitionWriterSpec{
									DeviceWriterType: "S3",
									PartitionSize:    500,
								},
								MapRecordConfig: &MapRecordSpec{
									FileMappingTableName: "my_mapping_table_name",
								},
							},
						},
						{
							When: ExpressionNode{
								Lhs: &ExpressionNode{
									Type: "select",
									Expr: "count",
								},
								Op: ">",
								Rhs: &ExpressionNode{
									Type: "value",
									Expr: "1",
								},
							},
							Then: TransformationSpec{
								PartitionWriterConfig: &PartitionWriterSpec{
									DeviceWriterType: "Parquet",
									PartitionSize:    200,
								},
							},
						},
					},
				},
			},
		},
	}

	err := ApplyAllConditionalTransformationSpec(pipeConfig, envSettings)
	if err != nil {
		t.Fatalf("ApplyAllConditionalTransformationSpec failed: %v", err)
	}
	// Note all conditions are true, they will be applied successively
	// Final result should reflect the cummulative conditions applied
	if len(pipeConfig) == 0 || len(pipeConfig[0].Apply) == 0 {
		t.Fatalf("No transformations found after applying conditions")
	}
	transformation := pipeConfig[0].Apply[0]
	if transformation.MapRecordConfig == nil || transformation.MapRecordConfig.FileMappingTableName != "my_mapping_table_name" {
		t.Errorf("Expected MapRecordConfig with FileMappingTableName 'my_mapping_table_name', got %+v", transformation.MapRecordConfig)
	}
	if transformation.Type != "partition_writer" {
		t.Errorf("Expected Type 'partition_writer', got '%s'", transformation.Type)
	}
	if transformation.PartitionWriterConfig.DeviceWriterType != "Parquet" {
		t.Errorf("Expected DeviceWriterType 'Parquet', got '%s'", transformation.PartitionWriterConfig.DeviceWriterType)
	}
	if transformation.PartitionWriterConfig.PartitionSize != 200 {
		t.Errorf("Expected PartitionSize 200, got '%d'", transformation.PartitionWriterConfig.PartitionSize)
	}
}
func TestValidatePipeSpecConfigRequiresInputChannelName(t *testing.T) {
	// The pipe reads from its input channel by name, resolved in the channel
	// registry at DAG-build time, so validation must reject an unnamed one
	// rather than let it fail inside a running task.
	makePipe := func(inputName string) []PipeSpec {
		return []PipeSpec{{
			Type:         "fan_out",
			InputChannel: InputChannelConfig{Name: inputName},
			Apply: []TransformationSpec{{
				Type:          "map_record",
				OutputChannel: OutputChannelConfig{Name: "mapped_row"},
			}},
		}}
	}
	startup := &CpipesStartup{}

	if err := startup.ValidatePipeSpecConfig(&ComputePipesConfig{}, makePipe("input_row")); err != nil {
		t.Fatalf("named input channel rejected: %v", err)
	}
	if err := startup.ValidatePipeSpecConfig(&ComputePipesConfig{}, makePipe("")); err == nil {
		t.Error("expected an unnamed input channel to be rejected")
	}
	// An absent input_channel decodes to the zero value, so it must fail the
	// same way.
	noChannel := []PipeSpec{{
		Type: "fan_out",
		Apply: []TransformationSpec{{
			Type:          "map_record",
			OutputChannel: OutputChannelConfig{Name: "mapped_row"},
		}},
	}}
	if err := startup.ValidatePipeSpecConfig(&ComputePipesConfig{}, noChannel); err == nil {
		t.Error("expected a pipe with no input_channel to be rejected")
	}
}

func TestValidatePipeSpecConfigRequiresSortKey(t *testing.T) {
	// A sort with no key silently emits records in arrival order, which is how
	// two production configs ran with their key in a dead `expr` field.
	makePipe := func(sortConfig *SortSpec) []PipeSpec {
		return []PipeSpec{{
			Type:         "fan_out",
			InputChannel: InputChannelConfig{Name: "input_row"},
			Apply: []TransformationSpec{{
				Type:          "sort",
				SortConfig:    sortConfig,
				OutputChannel: OutputChannelConfig{Name: "sorted_row"},
			}},
		}}
	}
	startup := &CpipesStartup{}

	if err := startup.ValidatePipeSpecConfig(&ComputePipesConfig{},
		makePipe(&SortSpec{SortByColumn: []string{"key1"}})); err != nil {
		t.Fatalf("sort_by sort rejected: %v", err)
	}
	if err := startup.ValidatePipeSpecConfig(&ComputePipesConfig{},
		makePipe(&SortSpec{DomainKey: "dk"})); err != nil {
		t.Fatalf("domain_key sort rejected: %v", err)
	}
	if err := startup.ValidatePipeSpecConfig(&ComputePipesConfig{},
		makePipe(&SortSpec{IsDebug: true})); err == nil {
		t.Error("expected a sort_config with neither domain_key nor sort_by to be rejected")
	}
	if err := startup.ValidatePipeSpecConfig(&ComputePipesConfig{},
		makePipe(nil)); err == nil {
		t.Error("expected a sort operator without sort_config to be rejected")
	}
}

func TestValidatePipeSpecConfigOriginalHeadersStagePath(t *testing.T) {
	// The final file of an output channel carrying both use_original_headers and
	// put_headers_on_first_partition is a concatenation of the stage part files
	// written upstream (s3 multipart copy), so its header line is the one written
	// on the first stage partition — the stage channels leading to the output
	// channel must carry both flags too.
	makeStagePath := func(stageUseOriginal, stagePutHeaders, outUseOriginal, outPutHeaders bool) (*ComputePipesConfig, []PipeSpec) {
		writerStep := []PipeSpec{{
			Type:         "fan_out",
			InputChannel: InputChannelConfig{Name: "input_row", Type: "input"},
			Apply: []TransformationSpec{{
				Type: "map_record",
				OutputChannel: OutputChannelConfig{
					Type:               "stage",
					Name:               "staged_row",
					WriteStepId:        "staged_data",
					UseOriginalHeaders: stageUseOriginal,
					FileConfig:         FileConfig{PutHeadersOnFirstPartition: stagePutHeaders},
				},
			}},
		}}
		outputStep := []PipeSpec{{
			Type:         "fan_out",
			InputChannel: InputChannelConfig{Name: "input_row", Type: "stage", ReadStepId: "staged_data"},
			Apply: []TransformationSpec{{
				Type: "map_record",
				OutputChannel: OutputChannelConfig{
					Type:               "output",
					Name:               "final_row",
					SpecName:           "input_row",
					UseOriginalHeaders: outUseOriginal,
					FileConfig:         FileConfig{Format: "csv", PutHeadersOnFirstPartition: outPutHeaders},
				},
			}},
		}}
		cpConfig := &ComputePipesConfig{ReducingPipesConfig: [][]PipeSpec{writerStep, outputStep}}
		return cpConfig, outputStep
	}
	startup := &CpipesStartup{}

	cpConfig, outputStep := makeStagePath(true, true, true, true)
	if err := startup.ValidatePipeSpecConfig(cpConfig, outputStep); err != nil {
		t.Fatalf("stage channel carrying both flags rejected: %v", err)
	}
	cpConfig, outputStep = makeStagePath(false, true, true, true)
	if err := startup.ValidatePipeSpecConfig(cpConfig, outputStep); err == nil {
		t.Error("expected a stage channel without use_original_headers to be rejected")
	}
	cpConfig, outputStep = makeStagePath(true, false, true, true)
	if err := startup.ValidatePipeSpecConfig(cpConfig, outputStep); err == nil {
		t.Error("expected a stage channel without put_headers_on_first_partition to be rejected")
	}
	// The rule is anchored on the output channel having both flags: with only
	// use_original_headers set, the stage channels are unconstrained.
	cpConfig, outputStep = makeStagePath(false, false, true, false)
	if err := startup.ValidatePipeSpecConfig(cpConfig, outputStep); err != nil {
		t.Fatalf("output channel with a single flag must not constrain the stage path: %v", err)
	}
}
