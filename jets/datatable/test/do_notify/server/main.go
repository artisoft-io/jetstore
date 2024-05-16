package main

import (
	"fmt"
	"io"
	"net/http"
)

func hello(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, "Could not read the body\n")
		return
	}
  fmt.Fprintln(w, "Got it!")
  fmt.Println("Hello!")
  fmt.Println(string(body))
}

func helloEp(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, "Could not read the body\n")
		return
	}
  fmt.Fprintln(w, "Hello EP!")
  fmt.Println("Hello EP!")
  fmt.Println(string(body))
}

func headers(w http.ResponseWriter, req *http.Request) {
    for name, headers := range req.Header {
        for _, h := range headers {
            fmt.Fprintf(w, "%v: %v\n", name, h)
        }
    }
}

func main() {

    http.HandleFunc("/hello", hello)
    http.HandleFunc("/helloEp", helloEp)
    http.HandleFunc("/headers", headers)

    http.ListenAndServe(":8090", nil)
}
// $ go run server