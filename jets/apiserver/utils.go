package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// sanitizeOrigin validates an untrusted Origin request header before it is
// echoed back in the Access-Control-Allow-Origin response header. It strips any
// CR/LF (and other control) characters to prevent HTTP response splitting /
// header injection, and ensures the value conforms to the expected
// "scheme://host[:port]" origin format. It returns the validated origin, or an
// empty string when the value is missing or malformed (in which case the caller
// must not set the response header).
func sanitizeOrigin(origin string) string {
	if origin == "" {
		return ""
	}
	// Reject any value containing CR, LF or other control characters to prevent
	// header injection / response splitting.
	if strings.ContainsFunc(origin, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return ""
	}
	// The "null" origin is a valid opaque origin per the Fetch spec.
	if origin == "null" {
		return "null"
	}
	// Validate the value conforms to the expected origin format:
	// scheme://host[:port] with no path, query, fragment or user info.
	u, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	if u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	// Reconstruct from parsed components so only the sanitized form is returned.
	return u.Scheme + "://" + u.Host
}

// Utility Funtions
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		fmt.Fprintf(w, "%s", err.Error())
	}
}

// version with argument already encoded json
func JSONB(w http.ResponseWriter, statusCode int, data []byte) {
	w.WriteHeader(statusCode)
	_, err := w.Write(data)
	if err != nil {
		fmt.Fprintf(w, "%s", err.Error())
	}
}

func ERROR(w http.ResponseWriter, statusCode int, err error) {
	if err != nil {
		JSON(w, statusCode, struct {
			Error string `json:"error"`
		}{
			Error: err.Error(),
		})
		return
	}
	JSON(w, http.StatusBadRequest, nil)
}
