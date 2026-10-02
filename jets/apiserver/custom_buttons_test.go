package main

import (
	"encoding/json"
	"os"
	"sync"
	"testing"
)

// cedargateButtons is the value build_jetstore_scripts sets for cedargate_ws
// (internal/workspaces/cedargate_ws.sh, read 2026-10-01), verbatim.
const cedargateButtons = `[{"type":"fetch_stage_to_clipboard","key":"analysis_report_to_clipboard","description":"Get Analysis Report from JetStore stage s3 location to clipboard","label":"Analysis Report","replace_text":"|","replace_with":",","fsk_params":["process_name","session_id"],"file_path":"process_name={{process_name}}/session_id={{session_id}}/step_id=analysis_lookup/jets_partition=analysis_data/part0000-0000001.csv"}]`

func TestParseCustomButtons(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want int
	}{
		"cedargate's value":                       {cedargateButtons, 1},
		"unset, which the CDK sends as empty":     {"", 0},
		"only whitespace":                         {"  \n", 0},
		"an empty array":                          {"[]", 0},
		"truncated JSON":                          {`[{"type":`, 0},
		"an object rather than an array":          {`{"type":"fetch_stage_to_clipboard"}`, 0},
		"one good entry and one that is a string": {`[{"key":"a"}, "b"]`, 0},
		"two objects":                             {`[{"key":"a"},{"key":"b"}]`, 2},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := parseCustomButtons(c.raw)
			if got == nil {
				t.Fatal("nil, which marshals as null; the login response promises a list")
			}
			if len(got) != c.want {
				t.Errorf("got %d buttons, want %d", len(got), c.want)
			}
		})
	}
}

// The entries travel as the operator wrote them; the client is the one that
// decides what a button may say.
func TestParseCustomButtonsPassesEntriesThrough(t *testing.T) {
	got := parseCustomButtons(cedargateButtons)
	var entry map[string]any
	if err := json.Unmarshal(got[0], &entry); err != nil {
		t.Fatal(err)
	}
	if entry["label"] != "Analysis Report" || entry["replace_text"] != "|" {
		t.Errorf("entry changed in transit: %v", entry)
	}
}

// withCustomButtons substitutes the process-wide value for one test, since it is
// read once per process.
func withCustomButtons(t *testing.T, raw string) {
	t.Helper()
	previous := customButtons
	customButtons = sync.OnceValue(func() []json.RawMessage { return parseCustomButtons(raw) })
	t.Cleanup(func() { customButtons = previous })
}

// TestLoginServesCustomButtons: the buttons travel at sign-in, under the key
// jetsclient_ide/src/api/client.ts reads, as a list of objects.
func TestLoginServesCustomButtons(t *testing.T) {
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	withCustomButtons(t, cedargateButtons)
	out := loginResponse(t)
	list, ok := out["custom_buttons"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("custom_buttons = %#v, want cedargate's one button", out["custom_buttons"])
	}
	if label := list[0].(map[string]any)["label"]; label != "Analysis Report" {
		t.Errorf("label = %#v", label)
	}
}

// TestLoginWithMalformedCustomButtons: a bad value costs the buttons and not the
// sign-in, and the key is still a list.
func TestLoginWithMalformedCustomButtons(t *testing.T) {
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	withCustomButtons(t, `[{"type":`)
	out := loginResponse(t)
	list, ok := out["custom_buttons"].([]any)
	if !ok || len(list) != 0 {
		t.Errorf("custom_buttons = %#v, want an empty list", out["custom_buttons"])
	}
}
