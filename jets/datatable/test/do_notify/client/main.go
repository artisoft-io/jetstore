package main

import (
	"log"

	"github.com/artisoft-io/jetstore/jets/datatable"
)

func main() {
	var apiEndpoint string
	// apiEndpoint = "http://localhost:8090/hello"
	apiEndpointJson := `{
		"key": "endpoint",
		"ep1": "http://localhost:8090/helloEp"
	}`
	customFileKeys := []string{"custom_key"}
	fileKey := "client=Acme/endpoint=ep1/custom_key=my_key/year=2024/month=05/day=13/object_type=PharmacyClaim/org=pp/files"
	notificationTemplate := `{"my_key":"{{custom_key}}","object_type":"{{object_type}}","org":"{{org}}","status":"Running","message":"Test harness execution in {{error}}"}`
	// Test
	err := datatable.DoNotifyApiGateway(fileKey, apiEndpoint, apiEndpointJson, notificationTemplate, customFileKeys, "some error", nil)
	if err != nil {
		log.Printf("while calling DoNotifyApiGateway: %v", err)
	}
}
