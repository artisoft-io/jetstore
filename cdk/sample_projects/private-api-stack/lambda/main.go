package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type Response struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

type RequestBody struct {
	Message string `json:"message"`
	Data    string `json:"data"`
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	log.Printf("Received request: %+v", request)

	// Handle different HTTP methods
	switch request.HTTPMethod {
	case "GET":
		return handleGet(request)
	case "POST":
		return handlePost(request)
	default:
		return Response{
			StatusCode: 405,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "Method not allowed"}`,
		}, nil
	}
}

func handleGet(request events.APIGatewayProxyRequest) (Response, error) {
	// Extract path parameters
	resource := request.Resource
	pathParams := request.PathParameters

	responseBody := map[string]interface{}{
		"message":        "Hello from private Lambda API!",
		"method":         "GET",
		"resource":       resource,
		"pathParameters": pathParams,
		"queryParams":    request.QueryStringParameters,
		"requestId":      request.RequestContext.RequestID,
	}

	body, err := json.Marshal(responseBody)
	if err != nil {
		return Response{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "Internal server error"}`,
		}, err
	}

	return Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func handlePost(request events.APIGatewayProxyRequest) (Response, error) {
	var requestBody RequestBody
	
	if err := json.Unmarshal([]byte(request.Body), &requestBody); err != nil {
		return Response{
			StatusCode: 400,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "Invalid JSON body"}`,
		}, nil
	}

	responseBody := map[string]interface{}{
		"message":     "Data received successfully",
		"method":      "POST",
		"received":    requestBody,
		"resource":    request.Resource,
		"requestId":   request.RequestContext.RequestID,
		"processedAt": fmt.Sprintf("%d", request.RequestContext.RequestTimeEpoch),
	}

	body, err := json.Marshal(responseBody)
	if err != nil {
		return Response{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "Internal server error"}`,
		}, err
	}

	return Response{
		StatusCode: 201,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}
