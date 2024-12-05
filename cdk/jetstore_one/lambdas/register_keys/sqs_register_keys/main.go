package main

// Test lambda that register file keys from sqs events

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// func handler(event events.SQSEvent) error {
func handler(event map[string]any) error {
	log.Println("event received:")
	b, _ := json.MarshalIndent(event, "", " ")
	log.Println(string(b))
	// for _, record := range event.Records {
	// 	err := processMessage(record)
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	fmt.Println("done")
	return nil
}

func processMessage(record events.SQSMessage) error {
	fmt.Printf("Processed message %s\n", record.Body)
	//*** Test calling the endpoint in the record body
	info, err := getInfo(record.Body)
	if err != nil {
		log.Printf("while calling getInfo(\"%s\"): %v", record.Body, err)
	}
	//** print the response
	log.Println("*** Response from endpoint:", info)
	return nil
}

func main() {
	lambda.Start(handler)
}

func getInfo(requestUrl string) (string, error) {
	retry := 0
do_retry:
	resp, err := http.Get(requestUrl)
	if err != nil {
		if retry < 10 {
			log.Printf("Endpoint response with err %v, retrying\n", err)
			time.Sleep(1 * time.Second)
			retry++
			goto do_retry
		}
		return "", fmt.Errorf("failed go get info from Endpoint: %v", err)
	}
	if resp.StatusCode != 200 {
		if retry < 10 {
			log.Printf("Endpoint response status code is %d, retrying\n", resp.StatusCode)
			resp.Body.Close()
			time.Sleep(1 * time.Second)
			retry++
			goto do_retry
		}
		return "", fmt.Errorf("failed go get info from Endpoint, bad status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("while reading the response body from Endpoint: %v", err)
	}
	return string(body), err
}
