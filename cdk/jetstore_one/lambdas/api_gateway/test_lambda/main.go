package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Examples of test event structure
// {
//   "method": "POST",
//   "path": "/jets-private-api",
//   "body": {
//     "method": "jets:status",
//     "params": {
//       "jetstore_session_id": "1751533857515"
//     }
//   }
// }
// example of GET
// {
//   "method": "GET",
//   "path": "/jets-private-api?method=jets:status&jetstore_session_id=1751533857515"
// }
type TestEvent struct {
	Method string                 `json:"method"`
	Path   string                 `json:"path"`
	Body   map[string]interface{} `json:"body,omitempty"`
}

type TestResponse struct {
	StatusCode  int                    `json:"statusCode"`
	Body        string                 `json:"body"`
	Headers     map[string]string      `json:"headers"`
	ApiResponse map[string]interface{} `json:"apiResponse,omitempty"`
	Error       string                 `json:"error,omitempty"`
	AssumedRole string                 `json:"assumedRole,omitempty"`
	RequestInfo map[string]string      `json:"requestInfo,omitempty"`
}

func handler(ctx context.Context, event TestEvent) (TestResponse, error) {
	log.Printf("Received test event: %+v", event)

	// Get environment variables
	apiEndpoint := os.Getenv("API_ENDPOINT")
	vpcEndpointId := os.Getenv("VPC_ENDPOINT_ID")
	region := os.Getenv("AWS_REGION")
	systemRoleArn := os.Getenv("SYSTEM_ROLE_ARN")

	if apiEndpoint == "" {
		return TestResponse{
			StatusCode: 500,
			Body:       "API_ENDPOINT environment variable not set",
			Error:      "Missing API_ENDPOINT",
		}, nil
	}

	if systemRoleArn == "" {
		return TestResponse{
			StatusCode: 500,
			Body:       "SYSTEM_ROLE_ARN environment variable not set",
			Error:      "Missing SYSTEM_ROLE_ARN",
		}, nil
	}

	// Set default values if not provided in event
	method := "GET"
	path := "/"
	if event.Method != "" {
		method = event.Method
	}
	if event.Path != "" {
		path = event.Path
	}

	// Construct the full URL
	url := strings.TrimSuffix(apiEndpoint, "/") + path

	log.Printf("Making %s request to: %s", method, url)
	log.Printf("VPC Endpoint ID: %s", vpcEndpointId)
	log.Printf("Region: %s", region)
	log.Printf("System Role ARN: %s", systemRoleArn)

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return TestResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to load AWS config: %v", err),
			Error:      err.Error(),
		}, err
	}

	// Create STS client
	stsClient := sts.NewFromConfig(cfg)

	// Assume the system role
	log.Printf("Assuming role: %s", systemRoleArn)
	assumeRoleProvider := stscreds.NewAssumeRoleProvider(stsClient, systemRoleArn, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = "test-lambda-session"
		o.Duration = 15 * time.Minute
	})

	// Create new config with assumed role credentials
	assumedCfg := cfg.Copy()
	assumedCfg.Credentials = assumeRoleProvider

	// Get caller identity to verify the assumed role
	callerIdentity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}, func(o *sts.Options) {
		o.Credentials = assumeRoleProvider
	})
	if err != nil {
		return TestResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to get caller identity with assumed role: %v", err),
			Error:      err.Error(),
		}, err
	}

	log.Printf("Successfully assumed role. Caller identity: %s", *callerIdentity.Arn)

	// Prepare request body
	var requestBody []byte
	if event.Body != nil && (method == "POST" || method == "PUT") {
		requestBody, err = json.Marshal(event.Body)
		if err != nil {
			return TestResponse{
				StatusCode: 400,
				Body:       fmt.Sprintf("Failed to marshal request body: %v", err),
				Error:      err.Error(),
			}, nil
		}
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(requestBody))
	if err != nil {
		return TestResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to create request: %v", err),
			Error:      err.Error(),
		}, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", req.URL.Host)

	// Sign the request with SigV4 using assumed role credentials
	signer := v4.NewSigner()
	payloadHash := sha256.Sum256(requestBody)

	// Retrieve concrete AWS credentials from the provider before signing
	creds, credErr := assumedCfg.Credentials.Retrieve(ctx)
	if credErr != nil {
		return TestResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to retrieve credentials: %v", credErr),
			Error:      credErr.Error(),
		}, credErr
	}

	err = signer.SignHTTP(ctx, creds, req, hex.EncodeToString(payloadHash[:]), "execute-api", region, time.Now())
	if err != nil {
		return TestResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to sign request: %v", err),
			Error:      err.Error(),
		}, err
	}

	log.Printf("Request headers: %+v", req.Header)

	// Make the HTTP request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return TestResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to make request: %v", err),
			Error:      err.Error(),
		}, err
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return TestResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to read response: %v", err),
			Error:      err.Error(),
		}, err
	}

	log.Printf("Response status: %d", resp.StatusCode)
	log.Printf("Response headers: %+v", resp.Header)
	log.Printf("Response body: %s", string(responseBody))

	// Parse API response if it's JSON
	var apiResponse map[string]interface{}
	if resp.Header.Get("Content-Type") == "application/json" {
		json.Unmarshal(responseBody, &apiResponse)
	}

	// Convert response headers to map
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Create request info for debugging
	requestInfo := map[string]string{
		"method":        method,
		"url":           url,
		"region":        region,
		"vpcEndpointId": vpcEndpointId,
		"systemRoleArn": systemRoleArn,
	}

	return TestResponse{
		StatusCode:  resp.StatusCode,
		Body:        string(responseBody),
		Headers:     headers,
		ApiResponse: apiResponse,
		AssumedRole: *callerIdentity.Arn,
		RequestInfo: requestInfo,
	}, nil
}

func main() {
	lambda.Start(handler)
}
