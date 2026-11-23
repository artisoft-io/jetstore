package stack

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A rate-limited meta request is answered 403 with a message and no "git" key. The synth
// panicked on a type assertion there; it must report GitHub's answer and when the limit resets.
func TestFetchGithubGitCidrsRateLimited(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "" {
			t.Errorf("no token was given, yet the request carries Authorization")
		}
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1795397226")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"API rate limit exceeded for 203.0.113.7.","documentation_url":"https://docs.github.com"}`)
	}))
	defer srv.Close()

	_, err := fetchGithubGitCidrs(srv.Client(), srv.URL, "", 3, time.Millisecond)
	if err == nil {
		t.Fatal("a 403 must be an error")
	}
	for _, want := range []string{"403", "API rate limit exceeded", "resets at 2026-", "GITHUB_TOKEN"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if calls != 1 {
		t.Errorf("a rate-limited request is not retried, got %d calls", calls)
	}
}

// A 5xx is tried again; a token, when given, is sent.
func TestFetchGithubGitCidrsRetriesServerErrors(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q", got)
		}
		if calls < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, `{"git":["192.30.252.0/22","2a0a:a440::/29","140.82.112.0/20"]}`)
	}))
	defer srv.Close()

	got, err := fetchGithubGitCidrs(srv.Client(), srv.URL, "tok", 3, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "192.30.252.0/22,140.82.112.0/20" {
		t.Errorf("got %v, want the two IPv4 CIDRs", got)
	}
	if calls != 3 {
		t.Errorf("got %d calls, want 3", calls)
	}
}

func TestParseGithubGitCidrsCapsAt51(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"git":[`)
	for i := 0; i < 60; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `"10.0.%d.0/24"`, i)
	}
	b.WriteString(`]}`)
	got, err := parseGithubGitCidrs([]byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != maxGithubCidrs {
		t.Errorf("got %d CIDRs, want %d", len(got), maxGithubCidrs)
	}
}

func TestParseGithubGitCidrsRefusesAMissingKey(t *testing.T) {
	_, err := parseGithubGitCidrs([]byte(`{"message":"Not Found"}`))
	if err == nil || !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("err = %v, want one naming the message", err)
	}
}
