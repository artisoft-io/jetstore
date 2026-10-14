package main

import (
	"testing"
	"time"
)

// startupCompileVersion decides which workspace the pipelines read after a deployment,
// through SyncComputePipesWorkspace's equality test against the image's JETS_VERSION.
func TestStartupCompileVersion(t *testing.T) {
	const image = "1791917630" // a deployment version, 2026-10-13 18:53 UTC
	after := time.Unix(1791918540, 0)

	if got := startupCompileVersion(image, false, after); got != image {
		t.Errorf("no override applied: got %s, want the image's version %s, so the pipelines may use their image's workspace", got, image)
	}
	got := startupCompileVersion(image, true, after)
	if got == image {
		t.Fatalf("override applied: got the image's version %s, so the pipelines would use the image's workspace without the override", got)
	}
	if got != "1791918540" {
		t.Errorf("override applied: got %s, want the compile time 1791918540, as the UI's compile stamps it", got)
	}
	if !(got > image) {
		t.Errorf("override applied: %s must compare above %s as a string, or the next start recompiles", got, image)
	}
	// The next release's image is built later, so its version compares above this one
	// and triggers the next start-up compile, as it does today.
	if next := "1792090000"; !(next > got) {
		t.Errorf("a later release %s must compare above %s", next, got)
	}
}

// A JETS_VERSION the time cannot compare above keeps the old behaviour rather than
// making every later start recompile.
func TestStartupCompileVersionKeepsAVersionTheTimeCannotPass(t *testing.T) {
	for _, image := range []string{"1891917630", "v4.0.3"} {
		if got := startupCompileVersion(image, true, time.Unix(1791918540, 0)); got != image {
			t.Errorf("JETS_VERSION %q: got %s, want %s kept", image, got, image)
		}
	}
}
