package compute_pipes

// An env key must not be the prefix of another env key.
//
// prepareCpipesEnv and CoordinateComputePipes both carry the rule as a comment
// (*make sure a key is not the prefix of another key*), and nothing enforced
// it, so `$TOTAL_FILE_SIZE` and `$TOTAL_FILE_SIZE_GB` were both set by the
// starters. parseValue resolves a `value` node by ranging over the env map and
// taking the first key the expression *contains*, and Go randomises map order,
// so `$TOTAL_FILE_SIZE_GB` resolved to the size in bytes on some runs. That is
// the `use_ecs_tasks_when` of cedargate's deid and anonymize_file pipelines,
// where `> 30.0` against bytes is true for any input over 30 bytes. Found
// 2026-09-27 when the Python node, which checks this rule at startup, refused
// the env outright; the keys are `${TOTAL_FILE_SIZE}` and
// `${TOTAL_FILE_SIZE_GB}` since, and the closing brace is what makes them
// prefix-free.
//
// **It covers every key, not only the `$` ones.** ReplaceEnvVars replaces every
// key of the map, whatever it looks like, and the Python node checks all of
// them. This test first matched `$` keys only - the same scope as the sweep
// that found the pair above - and so passed while `total_file_size` was a
// prefix of `total_file_size_gb`, which the next deployed run refused
// (2026-09-27). The bytes key is `total_file_size_bytes` since.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Literal env keys assigned anywhere in this package, e.g.
// `EnvSettings["${TOTAL_FILE_SIZE}"] = ...`, `envSettings["$SHARD_ID"] = ...`
// or `EnvSettings["nbr_partitions"] = ...`.
var envKeyAssignment = regexp.MustCompile(`[eE]nv(?:Settings)?\[\s*"([^"]+)"\s*\]\s*=`)

func TestEnvKeysAssignedByThisPackageArePrefixFree(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	where := map[string]string{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range envKeyAssignment.FindAllStringSubmatch(string(src), -1) {
			if _, ok := where[m[1]]; !ok {
				where[m[1]] = f
			}
		}
	}
	if len(where) < 10 {
		t.Fatalf("found only %d env keys; the pattern no longer matches how this package assigns them", len(where))
	}
	keys := make([]string, 0, len(where))
	for k := range where {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, a := range keys {
		for _, b := range keys[i+1:] {
			if strings.HasPrefix(b, a) {
				t.Errorf("env key %q (%s) is a prefix of %q (%s): substitution is textual, so %q can resolve to %q's value",
					a, where[a], b, where[b], b, a)
			}
		}
	}
}

// The resolution the rename exists for. Map order is random, so one lookup
// proves nothing; enough of them make a wrong answer certain to show if it can.
func TestTotalFileSizeGbResolvesToGbEveryTime(t *testing.T) {
	const bytes, gb = int64(64424509440), 60.0
	env := ExprBuilderContext{
		"${TOTAL_FILE_SIZE}":    bytes,
		"${TOTAL_FILE_SIZE_GB}": gb,
		"total_file_size_bytes": bytes,
		"total_file_size_gb":    gb,
	}
	for i := 0; i < 500; i++ {
		expr := "${TOTAL_FILE_SIZE_GB}"
		v, err := env.parseValue(&expr, 0)
		if err != nil {
			t.Fatal(err)
		}
		if v != gb {
			t.Fatalf("iteration %d: ${TOTAL_FILE_SIZE_GB} resolved to %v (%T), want %v", i, v, v, gb)
		}
	}
}
