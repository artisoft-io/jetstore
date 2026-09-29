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

// Every other node runs in this process unless -go_node_url is given, in which
// case it goes to the native node Lambda image served the same way. In-process
// is the default because it is the debugging loop; the image is the check that
// what ships -- its libjets.so, its bootstrap, its workspace -- behaves as the
// working tree does. See README.md.
var goNodeURL = flag.String("go_node_url", "",
	"invocation URL of the native Go cp_node image served by the Lambda RIE, e.g. "+
		"http://localhost:9124/2015-03-31/functions/function/invocations; "+
		"when empty, Go nodes run in this process")

// nodeTarget says where one node runs: the URL of an emulated Lambda, or "" for
// this process. usePython is the reducing starter's UsePythonReducingTask, the
// flag the state machine switches on.
func nodeTarget(usePython bool, pythonURL, goURL string) (string, error) {
	if usePython {
		if pythonURL == "" {
			return "", fmt.Errorf("a use_python_node step needs -python_node_url; see local_test_driver/README.md")
		}
		return pythonURL, nil
	}
	return goURL, nil
}

// Lambda's own ceiling is 15 minutes; a little over it, so the emulator's
// timeout is the one that fires and is the one reported.
const pythonNodeTimeout = 16 * time.Minute

// The body the emulator returns when the handler raised. **It returns HTTP 200
// with this body, not an error status and not an X-Amz-Function-Error header**
// (measured 2026-09-28 against cpipes_python_lambda_jets_ws), so a check on the
// status alone would read every failed node as a success.
//
// **A runtime that exits instead is a 502 with an empty body**, and that is the
// Go node's usual failure before it serves anything: its main() panics on a
// database it cannot reach, before lambda.Start (measured the same day against
// cpipes_lambda_jets_ws). The body says nothing, so the error points at the
// container's own log, which has the panic.
type pythonNodeError struct {
	ErrorMessage string   `json:"errorMessage"`
	ErrorType    string   `json:"errorType"`
	StackTrace   []string `json:"stackTrace"`
}

// invokeLambdaNode sends one node's {id, jp, pe} -- the same three fields the
// state machine passes a node Lambda -- and waits for it to finish.
func invokeLambdaNode(ctx context.Context, url string, args compute_pipes.ComputePipesNodeArgs) error {
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
		return fmt.Errorf("node at %s: %w (is the container running? see README.md)", url, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("node at %s: reading the response: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Amz-Function-Error") != "" {
		if len(bytes.TrimSpace(out)) == 0 {
			return fmt.Errorf("node at %s: HTTP %d with no body: the runtime exited before the handler "+
				"returned; its reason is in the container's log (docker logs)", url, resp.StatusCode)
		}
		return fmt.Errorf("node at %s: HTTP %d: %s", url, resp.StatusCode, out)
	}
	var fe pythonNodeError
	if json.Unmarshal(out, &fe) == nil && fe.ErrorType != "" {
		return fmt.Errorf("node raised %s: %s\n%s",
			fe.ErrorType, fe.ErrorMessage, strings.Join(fe.StackTrace, ""))
	}
	return nil
}
