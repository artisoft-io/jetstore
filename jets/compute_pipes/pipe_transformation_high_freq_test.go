package compute_pipes

import (
	"testing"
)

func doHighFreqBuildTest(columnName string) error {
	spec := &TransformationSpec{
		Type: "high_freq",
		HighFreqColumns: []*HighFreqSpec{
			{Name: columnName},
		},
	}
	columns := &map[string]int{
		"key1":  0,
		"key2":  1,
		"value": 2,
	}
	source := &InputChannel{
		Name:    "in",
		Columns: columns,
	}
	outputCh := &OutputChannel{
		Channel: make(chan []any),
	}
	ctx := &BuilderContext{
		done: make(chan struct{}),
	}
	_, err := ctx.NewHighFreqTransformationPipe(source, outputCh, spec)
	return err
}

func TestHighFreqRejectsUnknownColumn(t *testing.T) {
	// An unchecked lookup used to map an unknown high_freq column name to
	// position 0, silently tracking the wrong column; the builder must reject it.
	if err := doHighFreqBuildTest("not_a_column"); err == nil {
		t.Error("expecting a high_freq column that is not an input column to be rejected")
	}
	// A known column still builds.
	if err := doHighFreqBuildTest("key2"); err != nil {
		t.Fatalf("known high_freq column rejected: %v", err)
	}
}
