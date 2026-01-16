// The Go standard library comes with excellent support
// for HTTP clients and servers in the `net/http`
// package. In this example we'll use it to issue simple
// HTTP requests.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

func main() {

	// Issue an HTTP GET request to a server. `http.Get` is a
	// convenient shortcut around creating an `http.Client`
	// object and calling its `Get` method; it uses the
	// `http.DefaultClient` object which has useful default
	// settings.
	resp, err := http.Get("https://api.github.com/meta")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Print the HTTP response status.
	fmt.Println("Response status:", resp.Status)

	// Parse the response body as json
	d := json.NewDecoder(resp.Body)
	for {
		 bodyData := make(map[string]interface{})
		 err := d.Decode(&bodyData)
		 if err == io.EOF {
				 break
		 }
		 if err != nil {
				 log.Fatalf("Error decoding: %v", err)
		 }
		 // do stuff with "jsonObject"...
		 fmt.Println("Git ip in response body:")
		 for _,ipi := range bodyData["git"].([]interface{}) {
			ip := ipi.(string)
			// check for ip v6
			if strings.Contains(ip, ":") {
				fmt.Println(ip, "(ip v6 -- filter out)")
			} else {
				fmt.Println(ip)
			}			
		 }
	}	
	fmt.Println("Done!")
}
