package main

/*
#cgo LDFLAGS: -L./cpp -lhello
#include "cpp/hello.h"
#include <stdlib.h>
*/
import "C"

import (
    "context"
    "encoding/json"
    "fmt"
    "unsafe"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
)

type Request struct {
    Name    string `json:"name"`
    Message string `json:"message"`
}

type Response struct {
    StatusCode int               `json:"statusCode"`
    Headers    map[string]string `json:"headers"`
    Body       string            `json:"body"`
}

func callCppHello(name string) string {
    cName := C.CString(name)
    defer C.free(unsafe.Pointer(cName))
    
    cResult := C.hello_cpp(cName)
    if cResult == nil {
        return "Error calling C++ function"
    }
    defer C.free_hello_result(cResult)
    
    return C.GoString(cResult)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
    var req Request
    if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
        req.Name = "World"
        req.Message = "Default message"
    }
    
    if req.Name == "" {
        req.Name = "Lambda User"
    }
    
    cppGreeting := callCppHello(req.Name)
    
    responseBody := map[string]interface{}{
        "message":      fmt.Sprintf("Go Lambda received: %s", req.Message),
        "cppGreeting":  cppGreeting,
        "timestamp":    fmt.Sprintf("%v", ctx.Value("timestamp")),
        "requestId":    fmt.Sprintf("%v", ctx.Value("requestId")),
    }
    
    jsonBody, err := json.Marshal(responseBody)
    if err != nil {
        return Response{
            StatusCode: 500,
            Headers: map[string]string{
                "Content-Type": "application/json",
            },
            Body: `{"error": "Failed to marshal response"}`,
        }, err
    }
    
    response := Response{
        StatusCode: 200,
        Headers: map[string]string{
            "Content-Type": "application/json",
            "Access-Control-Allow-Origin": "*",
        },
        Body: string(jsonBody),
    }

    return response, nil
}

func main() {
    lambda.Start(handler)
}
