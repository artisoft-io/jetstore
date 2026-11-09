package main

import (
	"errors"
	"strings"
	"testing"
)

// JM-088: a DSN given directly must not need JETS_REGION, which is only for
// reading the secret from AWS.

func noSecret(t *testing.T) func(string, bool, int) (string, error) {
	return func(string, bool, int) (string, error) {
		t.Fatal("the secret must not be read when a dsn is given")
		return "", nil
	}
}

func fromJsonOK(string, bool, int) (string, error) { return "postgresql://from-json", nil }

func TestResolveDsnUriNeedsNoRegion(t *testing.T) {
	dsn, errMsg := resolveDsn(dsnSources{dsn: "postgresql://given"}, fromJsonOK, noSecret(t))
	if len(errMsg) > 0 {
		t.Fatalf("unexpected errors: %v", errMsg)
	}
	if dsn != "postgresql://given" {
		t.Fatalf("dsn = %q", dsn)
	}
}

func TestResolveDsnJsonNeedsNoRegion(t *testing.T) {
	dsn, errMsg := resolveDsn(dsnSources{dsnJson: `{"k":"v"}`}, fromJsonOK, noSecret(t))
	if len(errMsg) > 0 {
		t.Fatalf("unexpected errors: %v", errMsg)
	}
	if dsn != "postgresql://from-json" {
		t.Fatalf("dsn = %q", dsn)
	}
}

func TestResolveDsnSecretNeedsRegion(t *testing.T) {
	dsn, errMsg := resolveDsn(dsnSources{awsDsnSecret: "some-secret"}, fromJsonOK, noSecret(t))
	if dsn != "" {
		t.Fatalf("dsn = %q, want empty", dsn)
	}
	if len(errMsg) == 0 || !strings.Contains(errMsg[0], "JETS_REGION") {
		t.Fatalf("want an error naming JETS_REGION, got %v", errMsg)
	}
}

func TestResolveDsnSecretWithRegion(t *testing.T) {
	called := false
	fromSecret := func(secret string, _ bool, _ int) (string, error) {
		called = true
		if secret != "some-secret" {
			t.Errorf("secret = %q", secret)
		}
		return "postgresql://from-secret", nil
	}
	dsn, errMsg := resolveDsn(dsnSources{awsDsnSecret: "some-secret", awsRegion: "us-east-1"},
		fromJsonOK, fromSecret)
	if !called || len(errMsg) > 0 || dsn != "postgresql://from-secret" {
		t.Fatalf("called=%v dsn=%q errors=%v", called, dsn, errMsg)
	}
}

func TestResolveDsnBadJsonFallsBackToSecret(t *testing.T) {
	badJson := func(string, bool, int) (string, error) { return "", errors.New("bad json") }
	fromSecret := func(string, bool, int) (string, error) { return "postgresql://from-secret", nil }
	dsn, errMsg := resolveDsn(dsnSources{dsnJson: "{", awsRegion: "us-east-1"}, badJson, fromSecret)
	if len(errMsg) > 0 || dsn != "postgresql://from-secret" {
		t.Fatalf("dsn=%q errors=%v", dsn, errMsg)
	}
}
