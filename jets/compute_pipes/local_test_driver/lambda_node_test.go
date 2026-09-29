package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

func emulator(t *testing.T, status int, body string, got *string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*got = string(b)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

var node = compute_pipes.ComputePipesNodeArgs{NodeId: 2, JetsPartitionLabel: "0002P", PipelineExecKey: 9}

func TestTheNodeIsHandedExactlyTheThreeFieldsTheStateMachinePasses(t *testing.T) {
	var sent string
	url := emulator(t, 200, "null", &sent)
	if err := invokeLambdaNode(context.Background(), url, node); err != nil {
		t.Fatal(err)
	}
	// NodeArgs refuses an unknown field, so anything more would fail the node.
	if sent != `{"id":2,"jp":"0002P","pe":9}` {
		t.Fatalf("sent %s", sent)
	}
}

// The emulator reports a handler exception as HTTP 200 with the error in the
// body. This is the shape it returned on 2026-09-28 for a node that could not
// read its secret.
func TestAHandlerErrorReturnedWithStatus200IsAnError(t *testing.T) {
	var sent string
	url := emulator(t, 200, `{"errorMessage": "no secret", "errorType": "ClientError",
		"requestId": "r", "stackTrace": ["  File \"/var/task/handler.py\", line 138\n"]}`, &sent)
	err := invokeLambdaNode(context.Background(), url, node)
	if err == nil || !strings.Contains(err.Error(), "ClientError: no secret") ||
		!strings.Contains(err.Error(), "handler.py") {
		t.Fatalf("got %v", err)
	}
}

func TestANon200ResponseIsAnError(t *testing.T) {
	var sent string
	url := emulator(t, 502, "bad gateway", &sent)
	if err := invokeLambdaNode(context.Background(), url, node); err == nil ||
		!strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("got %v", err)
	}
}

func TestNoEmulatorListeningIsAnErrorNamingTheURL(t *testing.T) {
	err := invokeLambdaNode(context.Background(), "http://127.0.0.1:1/x", node)
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Fatalf("got %v", err)
	}
}

// The Go node's usual failure before it serves anything: main() panics, the
// runtime exits, and the emulator answers 502 with nothing in the body.
func TestAnEmptyBodied502PointsAtTheContainerLog(t *testing.T) {
	var sent string
	url := emulator(t, 502, "", &sent)
	err := invokeLambdaNode(context.Background(), url, node)
	if err == nil || !strings.Contains(err.Error(), "docker logs") {
		t.Fatalf("got %v", err)
	}
}

func TestAPythonStepGoesToThePythonNodeAndNeedsItsURL(t *testing.T) {
	if got, err := nodeTarget(true, "http://py", "http://go"); err != nil || got != "http://py" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := nodeTarget(true, "", "http://go"); err == nil {
		t.Fatal("a use_python_node step with no -python_node_url must be refused, not run on the Go node")
	}
}

func TestAGoStepRunsInProcessUnlessGoNodeURLIsGiven(t *testing.T) {
	if got, err := nodeTarget(false, "http://py", ""); err != nil || got != "" {
		t.Fatalf("got %q, %v", got, err)
	}
	if got, err := nodeTarget(false, "http://py", "http://go"); err != nil || got != "http://go" {
		t.Fatalf("got %q, %v", got, err)
	}
}
