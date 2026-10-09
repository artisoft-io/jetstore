package compute_pipes

import (
	"encoding/json"
	"testing"
)

// The bug this guards: a vllm operator naming a provenance_schema_name and no
// response_format of its own sent an *unconstrained* request.
//
// `vllmRequestBase` reads `response_format`, and `resolveInferProvenanceSchema` is
// what puts it there — it runs inside `newInferTransformationPipe`, after the
// backend is constructed. So a base built in the constructor read it empty, and the
// operator asked vLLM for free text while believing it had asked for a schema.
//
// **The symptom was two things that do not look related.** On records where the
// model happened to omit a required field, `applyMappings` reported it missing; and
// across enough records the unbounded generation ran the lambda out of time. One
// cause, and neither symptom names it.
func TestVllmBackendConstrainsAfterProvenanceAdoption(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["medical_event_count"],` +
		`"properties":{"medical_event_count":{"type":"integer"}}}`)

	newBackend := func() *vllmBackend {
		config := &VllmSpec{Model: "granite4.1:3b", Api: vllmApiChat}
		applyVllmDefaults(config)
		return &vllmBackend{config: config}
	}

	t.Run("a base built before adoption would be unconstrained", func(t *testing.T) {
		// The old ordering, stated as a test rather than described: prepare() first,
		// adoption second. This is what the constructor used to do.
		b := newBackend()
		if err := b.prepare(); err != nil {
			t.Fatalf("prepare: %v", err)
		}
		if b.constrained {
			t.Fatal("a config with no response_format cannot be constrained")
		}
		b.config.ResponseFormat = schema // the adoption, arriving too late
		payload, err := b.BuildRequest("hello")
		if err != nil {
			t.Fatalf("BuildRequest: %v", err)
		}
		var body map[string]any
		if err := json.Unmarshal(payload, &body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body["response_format"]; ok {
			t.Fatal("this case exists to show the old ordering sent none; it now sends one")
		}
	})

	t.Run("the shipped ordering carries the schema", func(t *testing.T) {
		b := newBackend()
		// What newInferTransformationPipe does: adopt, then prepare.
		b.config.ResponseFormat = schema
		if err := b.prepare(); err != nil {
			t.Fatalf("prepare: %v", err)
		}
		if !b.constrained {
			t.Error("the operator believes it is unconstrained after adopting a schema")
		}
		payload, err := b.BuildRequest("hello")
		if err != nil {
			t.Fatalf("BuildRequest: %v", err)
		}
		var body map[string]any
		if err := json.Unmarshal(payload, &body); err != nil {
			t.Fatal(err)
		}
		rf, ok := body["response_format"].(map[string]any)
		if !ok {
			t.Fatal("the request carries no response_format; vLLM would generate freely")
		}
		if rf["type"] != "json_schema" {
			t.Errorf(`response_format type is %v, want "json_schema"`, rf["type"])
		}
		js, ok := rf["json_schema"].(map[string]any)
		if !ok {
			t.Fatal("response_format carries no json_schema")
		}
		if js["strict"] != true {
			t.Error("strict is not set, which makes the schema a hint rather than a constraint")
		}
		// The schema that travels must be the provenance schema, not an empty shell:
		// the guardrail checks the same document the model was constrained by.
		got, err := json.Marshal(js["schema"])
		if err != nil {
			t.Fatal(err)
		}
		same, err := sameJSONDocument(json.RawMessage(got), schema)
		if err != nil {
			t.Fatal(err)
		}
		if !same {
			t.Errorf("the request carries a different schema than the one adopted:\n got %s", got)
		}
	})
}

// The backend must implement the hook, or the shared builder silently skips it and
// the bug returns with no test failing.
func TestVllmBackendImplementsThePreparer(t *testing.T) {
	var b any = &vllmBackend{}
	if _, ok := b.(inferBackendPreparer); !ok {
		t.Fatal("vllmBackend no longer implements inferBackendPreparer, so its base is never built")
	}
}
