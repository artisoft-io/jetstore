package main

import "testing"

// The listen address defaults to what it has always been, and -listenAddr or
// JETS_LISTEN_ADDR move it without changing whether TLS is used.
func TestResolveServerAddr(t *testing.T) {
	cases := []struct {
		flag, env string
		tunnel    bool
		want      string
	}{
		{"", "", true, ":8080"},
		{"", "", false, ":8443"},
		{"", "127.0.0.1:18080", true, "127.0.0.1:18080"},
		{"", ":9443", false, ":9443"},
		{":18081", "127.0.0.1:18080", true, ":18081"},
		{":18081", "", false, ":18081"},
	}
	for _, c := range cases {
		if got := resolveServerAddr(c.flag, c.env, c.tunnel); got != c.want {
			t.Errorf("resolveServerAddr(%q, %q, %v) = %q, want %q", c.flag, c.env, c.tunnel, got, c.want)
		}
	}
}
