package awsi

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// This file contains integration functions for handling API Gateway requests in AWS Lambda.
// The main feature is to maintain multiple handlers for different methods specified using prefixes.

var prefixRe *regexp.Regexp = regexp.MustCompile(`"method"\s*:\s*"(.*?):`)

type JetController struct {
	handlers map[string]JetsHandler
}

func NewJetController(handlers map[string]JetsHandler) *JetController {
	if len(handlers) == 0 {
		log.Fatalf("At least one JetsHandler must be provided")
	}
	return &JetController{
		handlers: handlers,
	}
}

// Response represents the structure of the HTTP response returned by the API Gateway.
type Response struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

type JetsHandler interface {
	Prefix() string
	HandleGet(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error)
	HandlePost(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error)
}

func (h *JetController) handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	// log.Printf("***Received request: %+v\n", request)

	// Handle different HTTP methods
	switch request.HTTPMethod {
	case "GET":
		queryParams := request.QueryStringParameters
		handler := h.handlers[getPrefix(queryParams["method"])]
		if handler == nil {
			return Response{
				StatusCode: 405,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       fmt.Sprintf(`{"error": "No handler found for method prefix: %s"}`, getPrefix(queryParams["method"])),
			}, nil
		}
		return handler.HandleGet(ctx, request)

	case "POST":
		prefix := findPrefix(request.Body)
		handler := h.handlers[prefix]
		if handler == nil {
			return Response{
				StatusCode: 405,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       fmt.Sprintf(`{"error": "No handler found for method prefix in body: %s"}`, prefix),
			}, nil
		}
		return handler.HandlePost(ctx, request)

	default:
		return Response{
			StatusCode: 405,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `{"error": "Method not allowed"}`,
		}, nil
	}
}

// StartHandler starts the AWS Lambda handler for the JetController, this method blocks indefinitely.
func (h *JetController) StartHandler() {
	log.Printf("Starting JetController...")
	lambda.Start(h.handleRequest)
}

// getPrefix extracts the prefix from the method field in the query parameters.
// Returns the `:` at the end to be consistent with rdf notation.
func getPrefix(method string) string {
	for i, char := range method {
		if char == ':' {
			return method[:i+1]
		}
	}
	return ""
}

// findPrefix extracts the prefix from the method field in the JSON body.
// Returns the `:` at the end to be consistent with rdf notation.
func findPrefix(body string) string {
	matches := prefixRe.FindStringSubmatch(body)
	if len(matches) > 1 {
		return matches[1] + ":"
	}
	return ""
}
