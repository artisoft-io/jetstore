package compute_pipes

import (
	"log"
	"testing"

	"github.com/aws/jsii-runtime-go"
)

// This file contains test cases for MapReduceColumnEval

// Full end-to-end tests
func TestMapReduceColumnEvalFull01(t *testing.T) {
	ctx := &BuilderContext{
		cpConfig: &ComputePipesConfig{
			ClusterConfig: &ClusterSpec{
				ShardingInfo: &ClusterShardingInfo{
					MaxNbrPartitions: 400,
					NbrPartitions:    131,
				},
			},
		},
	}
	inputColumns := map[string]int{
		"key":    0,
		"name":   1,
		"gender": 2,
		"dob":    3,
	}
	outputColumns := map[string]int{
		"carrying_more_records": 0,
	}
	// Build the Column Transformation Evaluator
	trsfEvaluator, err := ctx.BuildMapReduceTCEvaluator(
		&InputChannel{
			Name:    "input",
			Columns: &inputColumns,
			Config:  &ChannelSpec{},
		},
		&OutputChannel{
			Name:    "output",
			Columns: &outputColumns,
			Config:  &ChannelSpec{},
		},
		&TransformationColumnSpec{
			Type:  "map_reduce",
			MapOn: jsii.String("key"),
			ApplyMap: []TransformationColumnSpec{
				{
					Name: "record_count",
					Type: "count",
					Expr: jsii.String("*"),
				},
			},
			ApplyReduce: []TransformationColumnSpec{
				{
					Name: "carrying_more_records",
					Type: "count",
					Expr: jsii.String("*"),
					Where: &ExpressionNode{
						Lhs: &ExpressionNode{
							Type: "select",
							Expr: *jsii.String("record_count"),
						},
						Op: ">",
						Rhs: &ExpressionNode{
							Type: "value",
							Expr: *jsii.String("1"),
						},
					},
				},
			},
		},
	)
	if err != nil {
		t.Errorf("while calling BuildMapReduceTCEvaluator: %v", err)
	}

	// Evaluate the column transformation operator
	currentOutputValue := make([]any, len(outputColumns))
	inputValues := &[][]any{
		{"1", "NAME1", "M", "1969-01-01"},
		{"2", "NAME1", "M", "1969-01-01"},
		{"3", "NAME1", "M", "1969-01-01"},
		{"3", "NAME1", "M", "1969-01-01"},
		{"4", "NAME1", "M", "1969-01-01"},
		{"4", "NAME1", "M", "1969-01-01"},
		{nil, "NAME1", "M", "1969-01-01"},
		{nil, "NAME1", "M", "1969-01-01"},
	}
	var expectedValue int64 = 2
	for _, inputRow := range *inputValues {
		err = trsfEvaluator.Update(&currentOutputValue, &inputRow)
		if err != nil {
			t.Errorf("while calling Update: %v", err)
		}
	}
	trsfEvaluator.Done(&currentOutputValue)
	log.Printf("Got: %v (%T)", currentOutputValue[0], currentOutputValue[0])
	if expectedValue != currentOutputValue[0] {
		t.Errorf("value failed")
	}

	// t.Error("done")
}
