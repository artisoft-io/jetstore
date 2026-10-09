package tools

import (
	"context"
	"os"
	"testing"
)

// I-21 asked for a measurement before a fix: compile_rule_file copies the
// workspace's .jr tree on every call, which is invisible for one interactive
// compile and might not be for E.8's fan-out, where K candidates each compile
// on every repair iteration.
//
// Run with: go test -bench . -benchtime 10x ./jets/agentic/tools/
func BenchmarkCompileRuleFile(b *testing.B) {
	ws := fixture(b)
	args := []byte(`{"rule_text":"text greeting = \"hello\";\n"}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := CompileRuleFile(context.Background(), ws, args); err != nil {
			b.Fatal(err)
		}
	}
}

// The copy alone, so the compile's own cost can be subtracted.
func BenchmarkCopyRuleSourcesOnly(b *testing.B) {
	ws := fixture(b)
	dir, err := ws.LocalDir()
	if err != nil {
		b.Fatal(err)
	}
	// os.MkdirTemp with an immediate cleanup, not b.TempDir: b.TempDir
	// registers a cleanup per call and defers every removal to the end of the
	// benchmark, which accumulates N directories and charges their bookkeeping
	// to the loop. The first version of this benchmark did that and reported
	// the copy as *slower* than the compile containing it — a result that
	// cannot be true, and the tell that the harness was being measured.
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		to, err := os.MkdirTemp("", "bench_copy_")
		if err != nil {
			b.Fatal(err)
		}
		if err := copyRuleSources(dir, to); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		os.RemoveAll(to)
		b.StartTimer()
	}
}
