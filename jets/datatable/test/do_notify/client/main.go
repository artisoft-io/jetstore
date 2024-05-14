package main

import (
	"log"

	"github.com/artisoft-io/jetstore/jets/datatable"
)

func main() {
	apiEndpoint := "http://localhost:8090/hello"
	customFileKeys := []string{"custom_key"}
	fileKey := "client=Acme/custom_key=my_key/year=2024/month=05/day=13/object_type=PharmacyClaim/org=pp/files"
	notificationTemplate := `{"my_key":"$custom_key","object_type":"$object_type","org":"$org","status":"Running","message":"Test harness execution in progress"}`
	// Test
	err := datatable.DoNotifyApiGateway(fileKey, apiEndpoint, notificationTemplate, customFileKeys, "")
	if err != nil {
		log.Printf("while calling DoNotifyApiGateway: %v", err)
	}
}
