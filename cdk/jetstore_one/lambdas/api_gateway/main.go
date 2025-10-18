package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/cdk/jetstore_one/lambdas/dbc"
	"github.com/aws/aws-lambda-go/events"
)
var dbConnection *dbc.DbConnection

type RequestBody struct {
	Method string      `json:"method"`
	Params *JetsParams `json:"params,omitzero"`
	Id     int         `json:"id,omitempty"`
}

type ResponseBody struct {
	Result JetsResult `json:"result"`
	Id     int        `json:"id,omitempty"`
}

type JetsResult struct {
	Status        string `json:"status,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
	SessionId     string `json:"jetstore_session_id,omitempty"`
}

// JetsParams defines the parameters for the request
// These requests use jets: prefix for the method name.
// See readme.md for more details
type JetsParams struct {
	SessionId   string          `json:"jetstore_session_id,omitempty"`
}

// JetsHandler handles JetStore general related API requests
type JetsHandler struct{
	prefix string
}
func (h *JetsHandler) Prefix() string {
	return h.prefix
}

func (h *JetsHandler) HandleGet(ctx context.Context, request events.APIGatewayProxyRequest) (awsi.Response, error) {
	// Extract query string parameters
	queryParams := request.QueryStringParameters
	method := queryParams["method"]
	sessionId := queryParams["jetstore_session_id"]
	var responseBody ResponseBody

	switch method {
	case "jets:status":
		// Simulate status check
		log.Println("Checking status for session:", sessionId)
		responseBody = ResponseBody{
			Result: JetsResult{
				Status:    "completed",
				SessionId: sessionId,
			},
		}
	default:
		return response(400, fmt.Sprintf(`{"error": "Invalid method for GET: %s"}`, method))
	}

	body, err := json.Marshal(responseBody)
	if err != nil {
		return response(500, `{"error": "Internal server error"}`)
	}
	return response(200, string(body))
}

func (h *JetsHandler) HandlePost(ctx context.Context, request events.APIGatewayProxyRequest) (awsi.Response, error) {
	var requestBody RequestBody
	var responseBody ResponseBody

	if err := json.Unmarshal([]byte(request.Body), &requestBody); err != nil {
		return response(400, `{"error": "Invalid JSON body"}`)
	}
	if requestBody.Params == nil {
		return response(400, `{"error": "Missing params in request body"}`)
	}

	switch requestBody.Method {
	case "jets:status":
		// Simulate status check
		sessionId := requestBody.Params.SessionId
		log.Println("Checking status for session:", sessionId)
		if len(sessionId) == 0 {
			return response(400, `{"error": "Missing jetstore_session_id in params"}`)
		}
		responseBody = ResponseBody{
			Result: JetsResult{
				Status:    "completed",
				SessionId: sessionId,
			},
		}
	default:
		return response(400, fmt.Sprintf(`{"error": "Invalid method for POST: %s"}`, requestBody.Method))
	}

	body, err := json.Marshal(responseBody)
	if err != nil {
		return response(500, `{"error": "Internal server error"}`)
	}
	return response(201, string(body))
}

func response(statusCode int, body string) (awsi.Response, error) {
	return awsi.Response{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: body,
	}, nil
}

func main() {

	// open db connection
	var err error
	dbConnection, err = dbc.NewDbConnection(5)
	if err != nil {
		log.Panicf("while opening db connection: %v", err)
	}
	defer dbConnection.ReleaseConnection()

	handlers := map[string]awsi.JetsHandler{
		"jets": &JetsHandler{prefix: "jets"},
	}
	controller := awsi.NewJetController(handlers)
	controller.StartHandler()
}
