package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// An SDK API error, as far as awsi.IsNoSuchKey looks: it carries ErrorCode().
type stageAPIError struct{ code string }

func (e *stageAPIError) Error() string     { return e.code }
func (e *stageAPIError) ErrorCode() string { return e.code }

// TestStageFetchStatus is jetstore_maintenance_02 I-23: a missing stage object is
// a 404, so the client can say "no manifest for this run" without matching text,
// and every other failure stays the 400 it always was.
func TestStageFetchStatus(t *testing.T) {
	wrapped := func(code string) error {
		return fmt.Errorf("failed to download s3 file 's3://b/k': %w",
			fmt.Errorf("operation error S3: GetObject, %w", &stageAPIError{code: code}))
	}
	cases := map[string]struct {
		err  error
		want int
	}{
		"no such key":            {wrapped("NoSuchKey"), http.StatusNotFound},
		"not found":              {wrapped("NotFound"), http.StatusNotFound},
		"access denied":          {wrapped("AccessDenied"), http.StatusBadRequest},
		"a failure with no code": {errors.New("connection reset"), http.StatusBadRequest},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := stageFetchStatus(c.err); got != c.want {
				t.Errorf("stageFetchStatus = %d, want %d", got, c.want)
			}
		})
	}
}

// TestFetchFileFromStageAnswersWithStageFetchStatus ties the arm to the function
// above, so the 404 cannot be lost by an edit that writes a literal 400 back. A
// text check over the arm, which is what write_dispatch_test.go's
// TestDelegatedActionsCallTheMethodTheyClaimTo does for the delegated arms.
func TestFetchFileFromStageAnswersWithStageFetchStatus(t *testing.T) {
	arms, _ := dispatchArms(t)
	body, ok := arms["fetch_file_from_stage"]
	if !ok {
		t.Fatal("no fetch_file_from_stage arm in DoDataTableAction")
	}
	if !strings.Contains(body, "stageFetchStatus") {
		t.Error("the fetch_file_from_stage arm no longer answers a failed download with stageFetchStatus")
	}
}
