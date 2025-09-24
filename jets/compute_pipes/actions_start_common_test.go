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
			PartitionSize:     50,
		},
	}
	override := &TransformationSpec{
		Type: "", // Empty type means we want to merge fields, not replace
		PartitionWriterConfig: &PartitionWriterSpec{
			DeviceWriterType: "Parquet",
			PartitionSize:     500,
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
			PartitionSize:     50,
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
			PartitionSize:     50,
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
		"count":           5,
	}

	pipeConfig := []PipeSpec{
		{
			Apply: []TransformationSpec{
				{
					Type: "partition_writer",
					PartitionWriterConfig: &PartitionWriterSpec{
						DeviceWriterType: "S3",
						PartitionSize:     100,
					},
					ConditionalConfig: []*ConditionalTransformationSpec{
						{
							When: ExpressionNode{
								Lhs: &ExpressionNode{
									Type:  "select",
									Expr: "$USE_S3_WRITER",
								},
								Op:  "==",
								Rhs: &ExpressionNode{
									Type:  "value",
									Expr: "'true'",
								},
							},
							Then: TransformationSpec{
								PartitionWriterConfig: &PartitionWriterSpec{
									DeviceWriterType: "S3",
									PartitionSize:     500,
								},
								MapRecordConfig: &MapRecordSpec{
									FileMappingTableName: "my_mapping_table_name",
								},
							},
						},
						{
							When: ExpressionNode{
								Lhs: &ExpressionNode{
									Type:  "select",
									Expr: "count",
								},
								Op:  ">",
								Rhs: &ExpressionNode{
									Type:  "value",
									Expr: "1",
								},
							},
							Then: TransformationSpec{
								PartitionWriterConfig: &PartitionWriterSpec{
									DeviceWriterType: "Parquet",
									PartitionSize:     200,
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