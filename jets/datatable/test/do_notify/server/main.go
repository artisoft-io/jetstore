package main

import (
	"fmt"
	"html"
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
	// Reflecting user-controlled header values: force a non-HTML content type,
	// disable MIME sniffing, and HTML-escape the values to prevent XSS.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Fprintf(w, "%v: %v\n", html.EscapeString(name), html.EscapeString(h))
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
