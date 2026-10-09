package main

// Infer Server administration endpoint.
//
// Two kinds of action share this route. The lifecycle actions (start, stop, status) act on
// AWS through jets/awsi. The model actions proxy a request to the Ollama API running on the
// infer service.
//
// The proxy takes an action name, never a path: the client picks from inferActions below and
// the server supplies the method and the route. Accepting a caller-supplied path here would
// turn the apiserver into an open proxy to anything reachable from inside the VPC.
//
// Every action requires the infer_server_admin capability. The UI disables the screen without
// it, but that is presentation — this check is the enforcement point.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/user"
	"go.uber.org/zap"
)

// InferServerCapability is the capability required by every action on this endpoint.
// Seeded in jets/jets_init_db.sql; see also the capability list in jets/user/user.go.
const InferServerCapability = "infer_server_admin"

// inferProxyTimeout bounds a proxied Ollama call. Generous because pulling a model is one
// of the actions and that transfers gigabytes; the ALB in front of the infer service has a
// 20 minute idle timeout, so there is no point waiting longer than that.
const inferProxyTimeout = 20 * time.Minute

// inferAction is one entry of the proxy allow list.
type inferAction struct {
	method string
	path   string
}

// inferActions is the closed set of Ollama routes reachable through this endpoint. Adding a
// route here is the only way to expose one.
var inferActions = map[string]inferAction{
	"list_models":  {http.MethodGet, "/api/ps"},
	"pull_model":   {http.MethodPost, "/api/pull"},
	"show_model":   {http.MethodPost, "/api/show"},
	"delete_model": {http.MethodDelete, "/api/delete"},
}

// InferServerAction is the request envelope. Body is passed through to Ollama untouched for
// the proxy actions and ignored by the lifecycle ones.
type InferServerAction struct {
	Action string          `json:"action"`
	Body   json.RawMessage `json:"body"`
}

// DoInferServerAction ------------------------------------------------------
// Entry point function
func (server *Server) DoInferServerAction(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	token := user.ExtractToken(r)
	userEmail, _ := user.ExtractTokenID(token)
	server.AuditLogger.Info(string(body), zap.String("user", userEmail),
		zap.String("time", time.Now().Format(time.RFC3339)))

	// RBAC check, following DataTableContext.VerifyUserPermission
	jetsUser, err := user.GetUserByToken(server.dbpool, token)
	if err != nil {
		log.Printf("while GetUserByToken: %v", err)
		ERROR(w, http.StatusUnauthorized, errors.New("error: unauthorized, cannot get user info"))
		return
	}
	if !jetsUser.HasCapability(InferServerCapability) {
		log.Printf("user %s attempted an infer server action without the %s capability",
			userEmail, InferServerCapability)
		ERROR(w, http.StatusForbidden,
			errors.New("error: unauthorized, user do not have required capability"))
		return
	}

	action := InferServerAction{}
	if err = json.Unmarshal(body, &action); err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}

	var results *map[string]any
	var code int
	switch action.Action {
	case "server_status":
		results, code, err = server.GetInferServerStatus(r.Context())
	case "start_server":
		results, code, err = server.ScaleInferServer(r.Context(), true)
	case "stop_server":
		results, code, err = server.ScaleInferServer(r.Context(), false)
	default:
		results, code, err = server.ProxyInferServer(r.Context(), &action)
	}
	if err != nil {
		log.Printf("Error: %v", err)
		ERROR(w, code, err)
		return
	}
	addToken(r, results)
	JSON(w, http.StatusOK, results)
}

// GetInferServerStatus ------------------------------------------------------
// Report whether the infer server is running, so the UI can offer the right action
func (server *Server) GetInferServerStatus(ctx context.Context) (*map[string]any, int, error) {
	status, err := awsi.GetInferServerStatus(ctx, awsi.InferServerTarget{})
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while getting infer server status: %v", err)
	}
	return &map[string]any{"status": status}, http.StatusOK, nil
}

// ScaleInferServer ------------------------------------------------------
// Start or stop the infer task and the GPU instance behind it
//
// Both underlying calls are idempotent, so asking for a state the server is already in
// succeeds with changed false rather than failing. They return once the scaling has been
// requested; reaching the requested state takes minutes, which is what the returned status
// is for -- it is read back after the change so the UI does not have to guess.
func (server *Server) ScaleInferServer(ctx context.Context, start bool) (*map[string]any, int, error) {
	var changed bool
	var err error
	if start {
		changed, err = awsi.StartInferServer(ctx, awsi.InferServerTarget{})
	} else {
		changed, err = awsi.StopInferServer(ctx, awsi.InferServerTarget{})
	}
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while scaling the infer server: %v", err)
	}
	status, err := awsi.GetInferServerStatus(ctx, awsi.InferServerTarget{})
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while getting infer server status: %v", err)
	}
	return &map[string]any{"changed": changed, "status": status}, http.StatusOK, nil
}

// ProxyInferServer ------------------------------------------------------
// Forward an allow-listed request to the Ollama API and return its response verbatim
func (server *Server) ProxyInferServer(ctx context.Context, action *InferServerAction) (*map[string]any, int, error) {
	route, ok := inferActions[action.Action]
	if !ok {
		return nil, http.StatusUnprocessableEntity,
			fmt.Errorf("error: unknown infer server action: %s, expecting one of %s",
				action.Action, strings.Join(inferActionNames(), ", "))
	}
	// Absent when the stack was deployed without BUILD_INFER_SERVICE. Distinguishing this
	// from a stopped server matters: one is a deployment choice, the other a button away.
	inferUrl := os.Getenv("JETS_INFER_URL")
	if inferUrl == "" {
		return nil, http.StatusServiceUnavailable,
			errors.New("error: the infer server is not part of this deployment (JETS_INFER_URL is not set)")
	}

	var requestBody io.Reader
	if len(action.Body) > 0 {
		requestBody = bytes.NewReader(action.Body)
	}
	request, err := http.NewRequestWithContext(ctx, route.method,
		strings.TrimSuffix(inferUrl, "/")+route.path, requestBody)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while building the infer server request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: inferProxyTimeout}
	response, err := client.Do(request)
	if err != nil {
		// The common case is the server being stopped, which is not an error worth a stack
		// trace -- say so in terms the operator can act on.
		return nil, http.StatusServiceUnavailable,
			fmt.Errorf("while calling the infer server, is it running? %v", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("while reading the infer server response: %v", err)
	}
	// Ollama's errors are as useful to the operator as its successes, so they are reported
	// as a result rather than collapsed into an apiserver error.
	results := map[string]any{"statusCode": response.StatusCode}
	var parsed any
	if err = json.Unmarshal(responseBody, &parsed); err == nil {
		results["response"] = parsed
	} else {
		// Not every Ollama route answers with json, and an empty body is a valid answer
		// from /api/delete.
		results["response"] = string(responseBody)
	}
	return &results, http.StatusOK, nil
}

func inferActionNames() []string {
	names := make([]string, 0, len(inferActions)+3)
	for name := range inferActions {
		names = append(names, name)
	}
	return append(names, "server_status", "start_server", "stop_server")
}
