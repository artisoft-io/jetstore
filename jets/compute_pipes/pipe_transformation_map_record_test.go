package compute_pipes

import (
	"testing"
)

func doMapRecordBuildTest(config *MapRecordSpec) (*MapRecordTransformationPipe, error) {
	spec := &TransformationSpec{
		Type:            "map_record",
		MapRecordConfig: config,
	}
	source := &InputChannel{
		Name:    "in",
		Columns: &map[string]int{"key1": 0},
		Config:  &ChannelSpec{Name: "in"},
	}
	outputCh := &OutputChannel{
		Channel: make(chan []any),
		Columns: &map[string]int{"out_col": 0},
		Config:  &ChannelSpec{Name: "out"},
	}
	ctx := &BuilderContext{
		done: make(chan struct{}),
	}
	return ctx.NewMapRecordTransformationPipe(source, outputCh, spec)
}

func TestMapRecordOnErrorDefaults(t *testing.T) {
	pipe, err := doMapRecordBuildTest(&MapRecordSpec{})
	if err != nil {
		t.Fatalf("empty map_record_config rejected: %v", err)
	}
	if pipe.onError != OnErrorPassThrough {
		t.Errorf("expecting on_error default %s, got %s", OnErrorPassThrough, pipe.onError)
	}
	if pipe.maxErrorCount != 20 {
		t.Errorf("expecting max_error_count default 20, got %d", pipe.maxErrorCount)
	}
}

func TestMapRecordFailOnErrorLegacy(t *testing.T) {
	// fail_on_error is the legacy spelling of on_error: fail
	pipe, err := doMapRecordBuildTest(&MapRecordSpec{FailOnError: true})
	if err != nil {
		t.Fatalf("fail_on_error config rejected: %v", err)
	}
	if pipe.onError != OnErrorFail {
		t.Errorf("expecting fail_on_error to map to on_error %s, got %s", OnErrorFail, pipe.onError)
	}
	// an explicit on_error wins over fail_on_error
	pipe, err = doMapRecordBuildTest(&MapRecordSpec{FailOnError: true, OnError: "drop"})
	if err != nil {
		t.Fatalf("on_error drop config rejected: %v", err)
	}
	if pipe.onError != OnErrorDrop {
		t.Errorf("expecting explicit on_error to win, got %s", pipe.onError)
	}
}

func TestMapRecordRejectsUnknownOnError(t *testing.T) {
	if _, err := doMapRecordBuildTest(&MapRecordSpec{OnError: "bogus"}); err == nil {
		t.Error("expecting an unknown map_record on_error to be rejected")
	}
}

func TestJetrulesRejectsUnknownOnError(t *testing.T) {
	// The on_error validation runs before any rule-engine wiring, so the
	// builder rejects a bad value with no further context needed.
	spec := &TransformationSpec{
		Type:           "jetrules",
		JetrulesConfig: &JetrulesSpec{OnError: "bogus"},
	}
	ctx := &BuilderContext{done: make(chan struct{})}
	if _, err := ctx.NewJetrulesTransformationPipe(&InputChannel{Name: "in"}, nil, spec); err == nil {
		t.Error("expecting an unknown jetrules on_error to be rejected")
	}
}
