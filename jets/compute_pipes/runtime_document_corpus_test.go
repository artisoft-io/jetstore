package compute_pipes

// A node's document must carry nothing its author did not write, beyond the
// three fields a starter fills in.
//
// **The Python node validates the document it is handed against the contract
// model, and the contract model forbids extra keys.** A starter builds that
// document by unmarshalling the authored .pc.json into ComputePipesConfig and
// marshalling it back (actions_start_sharding_cp.go, actions_start_reducing_cp.go),
// so every field whose json tag lacks omitempty or omitzero comes back as a key
// the author never wrote -- `"type": ""` on a channel or a conditional `then`,
// `"apply": null` on merge_files, an empty `output_channel` on jetrules. Go
// reads those back as the zero values they are, which is why no Go node ever
// noticed; the first deployed run of a Python node refused its document
// outright, 2026-09-27. Measured the same day, before the tags were fixed:
// **194 of 194** starter-shaped documents over the corpus were refused.
//
// So this asserts the invariant rather than the tags: marshal each step the way
// a starter does and require that the only keys it adds are the runtime ones.
// A new field with a bare tag goes red here, naming the file and the path, on
// the day it is added rather than on the day a Python step first reaches it.
//
// Skips unless JETS_PC_CORPUS_DIR is set; see error_channel_default_corpus_test.go.
// It reads a submodule, so run it with -count=1.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// The keys a starter adds that no author writes. Everything under one of these
// prefixes is the starter's own and is not compared.
var runtimeOnlyKeys = []string{
	"common_runtime_args",
	"cluster_config.sharding_info",
}

// keyPaths lists every object key in a decoded JSON value as a dotted path,
// with array positions kept, so two documents compare element by element.
func keyPaths(v any, prefix string, out map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			out[p] = true
			keyPaths(child, p, out)
		}
	case []any:
		for i, child := range x {
			keyPaths(child, fmt.Sprintf("%s.%d", prefix, i), out)
		}
	}
}

func isRuntimeOnly(path string) bool {
	for _, k := range runtimeOnlyKeys {
		if path == k || strings.HasPrefix(path, k+".") {
			return true
		}
	}
	return false
}

func TestCorpusRuntimeDocumentAddsOnlyRuntimeKeys(t *testing.T) {
	dir := corpusDir(t)
	checked := 0
	for _, path := range corpusFiles(t, dir) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		var authored map[string]any
		if err := json.Unmarshal(raw, &authored); err != nil {
			t.Errorf("%s: not JSON: %v", path, err)
			continue
		}
		var cfg ComputePipesConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Errorf("%s: does not unmarshal into ComputePipesConfig: %v", path, err)
			continue
		}

		// The authored steps, generic and typed, in the same order.
		type step struct {
			generic any
			typed   []PipeSpec
		}
		steps := []step{}
		if v, ok := authored["pipes_config"]; ok {
			steps = append(steps, step{v, cfg.PipesConfig})
		}
		if v, ok := authored["reducing_pipes_config"].([]any); ok {
			for i := range v {
				steps = append(steps, step{v[i], cfg.ReducingPipesConfig[i]})
			}
		}
		if v, ok := authored["conditional_pipes_config"].([]any); ok {
			for i := range v {
				steps = append(steps, step{v[i].(map[string]any)["pipes_config"], cfg.ConditionalPipesConfig[i].PipesConfig})
			}
		}

		for i, s := range steps {
			// What the author wrote for this step, in a node's shape.
			want := map[string]any{}
			for k, v := range authored {
				want[k] = v
			}
			delete(want, "reducing_pipes_config")
			delete(want, "conditional_pipes_config")
			want["pipes_config"] = s.generic

			// What a starter writes: the same, marshalled from the struct, with
			// the runtime fields filled in.
			node := cfg
			if node.ClusterConfig == nil {
				node.ClusterConfig = &ClusterSpec{}
			} else {
				cs := *node.ClusterConfig
				node.ClusterConfig = &cs
			}
			node.ClusterConfig.ShardingInfo = &ClusterShardingInfo{TotalFileSize: 1, NbrPartitions: 1}
			node.CommonRuntimeArgs = &ComputePipesCommonArgs{CpipesMode: "sharding", SessionId: "s"}
			node.ReducingPipesConfig, node.ConditionalPipesConfig = nil, nil
			node.PipesConfig = s.typed
			b, err := json.Marshal(node)
			if err != nil {
				t.Fatalf("%s step %d: %v", path, i, err)
			}
			var got any
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("%s step %d: %v", path, i, err)
			}

			wantKeys, gotKeys := map[string]bool{}, map[string]bool{}
			keyPaths(want, "", wantKeys)
			keyPaths(got, "", gotKeys)
			added := []string{}
			for k := range gotKeys {
				if !wantKeys[k] && !isRuntimeOnly(k) {
					added = append(added, k)
				}
			}
			sort.Strings(added)
			if len(added) > 0 {
				t.Errorf("%s step %d: the node's document carries keys the author did not write:\n  %s",
					strings.TrimPrefix(path, dir), i, strings.Join(added, "\n  "))
			}
			checked++
		}
	}
	t.Logf("%d steps checked", checked)
}
