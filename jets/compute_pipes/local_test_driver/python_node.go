package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// A step whose use_python_node is true runs on the Python cp_node, which this
// process cannot host. The driver hands each of that step's nodes to one served
// over HTTP by the Lambda Runtime Interface Emulator that every AWS Lambda base
// image carries -- normally the site image itself, run with `docker run`, so the
// run exercises the artefact a deployment ships. See README.md.
var pythonNodeURL = flag.String("python_node_url", "",
	"invocation URL of a Python cp_node served by the Lambda RIE, e.g. "+
		"http://localhost:9123/2015-03-31/functions/function/invocations; "+
		"required when a step has use_python_node")

// Lambda's own ceiling is 15 minutes; a little over it, so the emulator's
// timeout is the one that fires and is the one reported.
const pythonNodeTimeout = 16 * time.Minute

// The body the emulator returns when the handler raised. **It returns HTTP 200
// with this body, not an error status and not an X-Amz-Function-Error header**
// (measured 2026-09-28 against cpipes_python_lambda_jets_ws), so a check on the
// status alone would read every failed node as a success.
type pythonNodeError struct {
	ErrorMessage string   `json:"errorMessage"`
	ErrorType    string   `json:"errorType"`
	StackTrace   []string `json:"stackTrace"`
}

// invokePythonNode sends one node's {id, jp, pe} -- the same three fields the
// state machine passes the Python node Lambda -- and waits for it to finish.
func invokePythonNode(ctx context.Context, url string, args compute_pipes.ComputePipesNodeArgs) error {
	body, err := json.Marshal(args)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: pythonNodeTimeout}).Do(req)
	if err != nil {
		return fmt.Errorf("python node at %s: %w (is the container running? see README.md)", url, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("python node at %s: reading the response: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Amz-Function-Error") != "" {
		return fmt.Errorf("python node at %s: HTTP %d: %s", url, resp.StatusCode, out)
	}
	var fe pythonNodeError
	if json.Unmarshal(out, &fe) == nil && fe.ErrorType != "" {
		return fmt.Errorf("python node raised %s: %s\n%s",
			fe.ErrorType, fe.ErrorMessage, strings.Join(fe.StackTrace, ""))
	}
	return nil
}
