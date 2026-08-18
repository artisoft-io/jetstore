package infer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The tests run against a stub server rather than a live Ollama. That is a
// deliberate limit and worth naming: it exercises the envelope, the retry
// policy and the validation, and says nothing about whether a real model
// honours `format`. The thing it cannot test is exactly the thing client-side
// validation exists to catch, which is why validation is not optional in the
// client.

const objSchema = `{"type":"object","properties":{"name":{"type":"string"}},` +
	`"required":["name"],"additionalProperties":false}`

func stub(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Client{Host: srv.URL, Model: "test-model", RetryWait: time.Millisecond}
}

func reply(w http.ResponseWriter, content string, prompt, eval int) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"model":             "test-model:v1",
		"message":           map[string]string{"content": content},
		"prompt_eval_count": prompt,
		"eval_count":        eval,
	})
}

func TestChat_ValidAnswerIsParsedAndCounted(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("posted to %s, want /api/chat", r.URL.Path)
		}
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		// The schema must reach the server as `format`, or generation is not
		// constrained and only the client-side half of Rung 1 is in play.
		if len(req.Format) == 0 {
			t.Error("the request carries no format; generation would be unconstrained")
		}
		if req.Stream {
			t.Error("stream must be false; the client reads one whole answer")
		}
		reply(w, `{"name":"ok"}`, 11, 7)
	})

	resp, err := c.Chat(context.Background(), &Request{
		System: "s", User: "u", Schema: json.RawMessage(objSchema),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Tokens() != 18 {
		t.Errorf("Tokens() = %d, want 18 — this is what AgentRun.token_spend accumulates", resp.Tokens())
	}
	if resp.ModelName != "test-model:v1" {
		t.Errorf("ModelName = %q; the server's answer is what gets recorded, not what was asked for", resp.ModelName)
	}
	if resp.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", resp.Attempts)
	}
	if m, ok := resp.Value.(map[string]any); !ok || m["name"] != "ok" {
		t.Errorf("Value = %#v, want the parsed object", resp.Value)
	}
}

// The failure mode tasks_go documents and vLLM's guided decoding would remove:
// the server ignores `format` and answers something else. Client-side
// validation is the only thing standing between that and the loop.
func TestChat_AnswerViolatingTheSchemaIsASchemaError(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(w, `{"nickname":"wrong shape"}`, 3, 4)
	})
	_, err := c.Chat(context.Background(), &Request{User: "u", Schema: json.RawMessage(objSchema)})
	if err == nil {
		t.Fatal("expected a schema violation to be reported")
	}
	var se *SchemaError
	if !errors.As(err, &se) {
		t.Fatalf("expected a *SchemaError so the repair loop can act on it, got %T: %v", err, err)
	}
	// The repair prompt quotes the answer back, so it has to be carried.
	if !strings.Contains(se.Content, "nickname") {
		t.Errorf("the raw answer is not on the error; a repair prompt has nothing to quote: %q", se.Content)
	}
}

func TestChat_NonJSONAnswerIsAlsoASchemaError(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(w, "I am afraid I cannot do that", 3, 4)
	})
	_, err := c.Chat(context.Background(), &Request{User: "u", Schema: json.RawMessage(objSchema)})
	var se *SchemaError
	if !errors.As(err, &se) {
		t.Fatalf("prose where JSON was required is the same class of failure, got %T", err)
	}
}

// Retries cover the call, not the answer.
func TestChat_RetriesTransportFailuresButNotSchemaFailures(t *testing.T) {
	t.Run("server error is retried", func(t *testing.T) {
		var calls atomic.Int32
		c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			reply(w, `{"name":"eventually"}`, 1, 1)
		})
		resp, err := c.Chat(context.Background(), &Request{User: "u", Schema: json.RawMessage(objSchema)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Attempts != 3 {
			t.Errorf("Attempts = %d, want 3", resp.Attempts)
		}
	})

	t.Run("schema failure is not retried", func(t *testing.T) {
		var calls atomic.Int32
		c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			reply(w, `{"nickname":"wrong"}`, 1, 1)
		})
		_, _ = c.Chat(context.Background(), &Request{User: "u", Schema: json.RawMessage(objSchema)})
		// Resending an identical prompt is unlikely to produce a different
		// answer, and spending the budget on it is the strategy the plan
		// rejects in favour of structured repair.
		if n := calls.Load(); n != 1 {
			t.Errorf("made %d calls; a schema failure must not be retried, it is the repair loop's job", n)
		}
	})

	t.Run("a 4xx is not retried", func(t *testing.T) {
		var calls atomic.Int32
		c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusBadRequest)
		})
		if _, err := c.Chat(context.Background(), &Request{User: "u", Schema: json.RawMessage(objSchema)}); err == nil {
			t.Fatal("expected an error")
		}
		if n := calls.Load(); n != 1 {
			t.Errorf("made %d calls; a bad request produces the same bad request", n)
		}
	})
}

// A cancelled context is the caller's decision, not a transient failure. This
// is what makes the run-level wall-clock cap (D.5) able to stop a run: if the
// client retried through cancellation, the cap would not bind.
func TestChat_CancelledContextIsNotRetriedInto(t *testing.T) {
	var calls atomic.Int32
	c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		reply(w, `{"name":"late"}`, 1, 1)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.Chat(ctx, &Request{User: "u", Schema: json.RawMessage(objSchema)}); err == nil {
		t.Fatal("expected the cancelled context to end the call")
	}
	if n := calls.Load(); n > 1 {
		t.Errorf("made %d calls after cancellation; the wall-clock cap could not bind", n)
	}
}

func TestChat_RejectsRequestsItCannotConstrain(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the server must not be reached")
	})
	if _, err := c.Chat(context.Background(), &Request{User: "u"}); err == nil {
		t.Error("a request with no schema must be refused: the schema is what constrains and what validates")
	}
	if _, err := c.Chat(context.Background(), &Request{
		User: "u", Schema: json.RawMessage(`{"type": not json}`),
	}); err == nil {
		t.Error("a schema that does not parse must be refused before a token is spent")
	}
	nomodel := &Client{Host: c.Host}
	if _, err := nomodel.Chat(context.Background(), &Request{User: "u", Schema: json.RawMessage(objSchema)}); err == nil {
		t.Error("a client with no model must say so")
	}
}

// An error reported inside a 200 body is the server declining, not a transport
// blip; retrying it burns budget for nothing.
func TestChat_ServerReportedErrorIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "model not found"})
	})
	_, err := c.Chat(context.Background(), &Request{User: "u", Schema: json.RawMessage(objSchema)})
	if err == nil || !strings.Contains(err.Error(), "model not found") {
		t.Fatalf("the server's own message should reach the caller, got %v", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("made %d calls; a declined request is not transient", n)
	}
}
