package registerkey

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/artisoft-io/jetstore/jets/datatable"
)

// The registerFileKeys seam, captured. Every test in this file goes through
// doFileKey and asserts on what it built, rather than on what a database ended up
// holding: RegisterFileKeys needs sqlInsertStmts, a pgx pool and JETS_ env, and
// none of that says anything about the Data map this package is responsible for.
type capturedCall struct {
	dtCtx  *datatable.DataTableContext
	action *datatable.RegisterFileKeyAction
	token  string
}

// withCapturedRegistration replaces the seam for the duration of one test and
// returns the slice the calls land in. It restores the production function, so a
// test that panics does not leak its stub into the next one.
func withCapturedRegistration(t *testing.T) *[]capturedCall {
	t.Helper()
	saved := registerFileKeys
	calls := make([]capturedCall, 0, 1)
	registerFileKeys = func(dtCtx *datatable.DataTableContext,
		action *datatable.RegisterFileKeyAction, token string) error {
		calls = append(calls, capturedCall{dtCtx: dtCtx, action: action, token: token})
		return nil
	}
	t.Cleanup(func() { registerFileKeys = saved })
	return &calls
}

// withHook installs a site Hook for the duration of one test.
func withHook(t *testing.T, h Hook) {
	t.Helper()
	saved := siteHook
	siteHook = h
	t.Cleanup(func() { siteHook = saved })
}

// The file key shape the cedargate deployment actually delivers: client, vendor,
// object_type and the three date components as path segments, then the file name.
const testFileKey = "client=CGT/vendor=ACME/object_type=eligibility/" +
	"year=2026/month=09/day=16/eligibility_20260916.csv"

const testFileSize int64 = 4096

// TestDoFileKeyNilHookRegistersTheStockAction is the check behind the claim that
// extracting this package leaves register_keys_v2 unchanged (AC.4). It is the only
// test in the change covering the path every object type in the deployment takes,
// and it is spelled out by value rather than by re-calling
// utils.SplitFileKeyIntoComponents, so that a change to what doFileKey puts in the
// Data map fails here instead of agreeing with itself.
func TestDoFileKeyNilHookRegistersTheStockAction(t *testing.T) {
	calls := withCapturedRegistration(t)
	withHook(t, nil)

	dtCtx := &datatable.DataTableContext{}
	err := doFileKey(context.Background(), nil, dtCtx, testFileKey, testFileSize, "a-token")
	if err != nil {
		t.Fatalf("doFileKey returned %v, want nil", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("got %d registrations, want 1", len(*calls))
	}
	call := (*calls)[0]
	if call.dtCtx != dtCtx {
		t.Errorf("registration got a different DataTableContext than doFileKey was given")
	}
	if call.token != "a-token" {
		t.Errorf("token = %q, want %q", call.token, "a-token")
	}
	if call.action.Action != "register_keys" {
		t.Errorf("Action = %q, want %q", call.action.Action, "register_keys")
	}
	if call.action.IsSchemaEvent {
		t.Errorf("IsSchemaEvent = true, want false for the stock path")
	}
	if call.action.NoAutomatedLoad {
		t.Errorf("NoAutomatedLoad = true, want false for the stock path")
	}
	if len(call.action.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(call.action.Data))
	}

	want := map[string]any{
		"client":      "CGT",
		"vendor":      "ACME",
		"org":         "ACME",
		"object_type": "eligibility",
		"year":        2026,
		"month":       9,
		"day":         16,
		"file_key":    testFileKey,
		"size":        testFileSize,
	}
	got := call.action.Data[0]
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Data[0] mismatch\n got: %#v\nwant: %#v", got, want)
	}
	// Spelled out again key by key, because DeepEqual on map[string]any is also a
	// type assertion and a failure above does not say which key or which type.
	for k, w := range want {
		g, ok := got[k]
		switch {
		case !ok:
			t.Errorf("Data[0][%q] missing, want %#v", k, w)
		case g != w:
			t.Errorf("Data[0][%q] = %#v (%T), want %#v (%T)", k, g, g, w, w)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("Data[0] has unexpected key %q = %#v", k, got[k])
		}
	}
}

// stubHook records the arguments it was handed and returns what it was told to.
type stubHook struct {
	isSchemaEvent bool
	done          bool
	err           error
	// added is written into components before returning, standing in for the
	// "schema_provider_json" entry a site adds.
	added map[string]any

	calls         int
	gotCtx        context.Context
	gotDtCtx      *datatable.DataTableContext
	gotFileKey    string
	gotComponents map[string]any
	gotToken      string
}

func (h *stubHook) BeforeRegister(ctx context.Context, dtCtx *datatable.DataTableContext,
	fileKey string, components map[string]any,
	token string) (bool, bool, error) {
	h.calls++
	h.gotCtx = ctx
	h.gotDtCtx = dtCtx
	h.gotFileKey = fileKey
	h.gotComponents = components
	h.gotToken = token
	for k, v := range h.added {
		components[k] = v
	}
	return h.isSchemaEvent, h.done, h.err
}

// TestDoFileKeyHookDoneRegistersNothing is the first half of AC.5: done == true is
// the return, and there is no fall-through to a stock registration the hook has
// already handled.
func TestDoFileKeyHookDoneRegistersNothing(t *testing.T) {
	calls := withCapturedRegistration(t)
	hook := &stubHook{done: true}
	withHook(t, hook)

	err := doFileKey(context.Background(), nil, &datatable.DataTableContext{},
		testFileKey, testFileSize, "a-token")
	if err != nil {
		t.Fatalf("doFileKey returned %v, want nil", err)
	}
	if hook.calls != 1 {
		t.Fatalf("hook called %d times, want 1", hook.calls)
	}
	if len(*calls) != 0 {
		t.Errorf("got %d registrations, want 0 when the hook reports done", len(*calls))
	}
}

// TestDoFileKeyHookErrorRegistersNothing covers the other way a site reports a
// failure of its own.
func TestDoFileKeyHookErrorRegistersNothing(t *testing.T) {
	calls := withCapturedRegistration(t)
	wantErr := errors.New("the site said no")
	withHook(t, &stubHook{err: wantErr})

	err := doFileKey(context.Background(), nil, &datatable.DataTableContext{},
		testFileKey, testFileSize, "a-token")
	if !errors.Is(err, wantErr) {
		t.Fatalf("doFileKey returned %v, want %v", err, wantErr)
	}
	if len(*calls) != 0 {
		t.Errorf("got %d registrations, want 0 when the hook errors", len(*calls))
	}
}

// TestDoFileKeyHookSchemaEventCarriesAdditions is the second half of AC.5: the flag
// reaches the action, and the entries a hook adds to components arrive in the Data
// map unchanged and alongside -- not instead of -- the ones the stock path put
// there.
func TestDoFileKeyHookSchemaEventCarriesAdditions(t *testing.T) {
	calls := withCapturedRegistration(t)
	hook := &stubHook{
		isSchemaEvent: true,
		added: map[string]any{
			"schema_provider_json": `{"schema_name":"eligibility"}`,
			"request_id":           "req-7",
		},
	}
	withHook(t, hook)

	ctx := context.Background()
	dtCtx := &datatable.DataTableContext{}
	err := doFileKey(ctx, nil, dtCtx, testFileKey, testFileSize, "a-token")
	if err != nil {
		t.Fatalf("doFileKey returned %v, want nil", err)
	}

	// The hook sees the split key with size already set, so it neither re-splits
	// nor invents a size.
	if hook.gotFileKey != testFileKey {
		t.Errorf("hook fileKey = %q, want %q", hook.gotFileKey, testFileKey)
	}
	if hook.gotToken != "a-token" {
		t.Errorf("hook token = %q, want %q", hook.gotToken, "a-token")
	}
	if hook.gotDtCtx != dtCtx {
		t.Errorf("hook got a different DataTableContext than doFileKey was given")
	}
	if hook.gotCtx != ctx {
		t.Errorf("hook got a different context.Context than doFileKey was given")
	}
	if hook.gotComponents["size"] != testFileSize {
		t.Errorf("hook components[\"size\"] = %#v, want %#v",
			hook.gotComponents["size"], testFileSize)
	}
	if hook.gotComponents["file_key"] != testFileKey {
		t.Errorf("hook components[\"file_key\"] = %#v, want %#v",
			hook.gotComponents["file_key"], testFileKey)
	}

	if len(*calls) != 1 {
		t.Fatalf("got %d registrations, want 1", len(*calls))
	}
	action := (*calls)[0].action
	if !action.IsSchemaEvent {
		t.Errorf("IsSchemaEvent = false, want true when the hook says so")
	}
	if action.NoAutomatedLoad {
		t.Errorf("NoAutomatedLoad = true, want false -- no hook sets it")
	}
	if len(action.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(action.Data))
	}

	want := map[string]any{
		"client":               "CGT",
		"vendor":               "ACME",
		"org":                  "ACME",
		"object_type":          "eligibility",
		"year":                 2026,
		"month":                9,
		"day":                  16,
		"file_key":             testFileKey,
		"size":                 testFileSize,
		"schema_provider_json": `{"schema_name":"eligibility"}`,
		"request_id":           "req-7",
	}
	if got := action.Data[0]; !reflect.DeepEqual(got, want) {
		t.Errorf("Data[0] mismatch\n got: %#v\nwant: %#v", got, want)
	}
}
