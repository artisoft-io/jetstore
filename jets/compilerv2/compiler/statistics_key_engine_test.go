//go:build engine_keys

package compiler

// THE STATISTICS KEY AGREES WITH THE ENGINE (rules_benchmark Phase 9, BY.1).
//
// The cost order looks a rule constant up in the statistics file by termKey,
// which renders it from the compiler's ResourceNode. The file's keys are the
// constant's form in the engine's graph. This test compiles a workspace, loads
// the workspace.db it wrote in the C++ engine, and compares, for every
// constant of every antecedent, the compiler's key with the key of the node
// the engine has for it: a resource or volatile resource is looked up by name
// in the engine's meta graph, which holds a node for every row of the
// resources table, and a text or int literal is made the way
// ReteMetaStoreFactory::read_resources_cb makes it from that row.
//
// It needs libjets, so it is behind a build tag:
//
//	LD_LIBRARY_PATH=<jetstore>/build/jets go test -count=1 -tags engine_keys -run TestStatisticsKeyAgreesWithTheEngine ./compilerv2/compiler/
//
// It runs on jets/jetrules/test_ws, or on the workspace JETS_ENGINE_KEYS_WORKSPACE
// names, copied to a temporary directory either way. It prints counts only, and a
// mismatch by rule set, vertex and position, never a value.

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// copyTree copies the files of src that a compile reads into dst: copies, never
// links, so nothing the test writes reaches src.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	for _, name := range []string{"workspace_control.json", "lookup.db"} {
		if _, err := os.Stat(filepath.Join(src, name)); err == nil {
			copyFile(t, filepath.Join(src, name), filepath.Join(dst, name))
		}
	}
	for _, sub := range []string{"jet_rules", "data_model", "lookups"} {
		root := filepath.Join(src, sub)
		if _, err := os.Stat(root); err != nil {
			continue
		}
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(src, p)
			if info.IsDir() {
				return os.MkdirAll(filepath.Join(dst, rel), 0o755)
			}
			if info.Mode().IsRegular() {
				copyFile(t, p, filepath.Join(dst, rel))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

// constantKey is the compiler's key for one antecedent constant.
type constantKey struct {
	key     string
	counted bool
	kind    string
}

var positionNames = [3]string{"subject", "predicate", "object"}

func TestStatisticsKeyAgreesWithTheEngine(t *testing.T) {
	saved := workspaceSchema
	workspaceSchema = filepath.Join("..", "..", "workspace_schema.sql")
	t.Cleanup(func() { workspaceSchema = saved })

	src := os.Getenv("JETS_ENGINE_KEYS_WORKSPACE")
	if src == "" {
		src = filepath.Join("..", "..", "jetrules", "test_ws")
	}
	dir := t.TempDir()
	copyTree(t, src, dir)
	wc, err := rete.LoadWorkspaceControl(filepath.Join(dir, "workspace_control.json"))
	if err != nil {
		t.Fatal(err)
	}
	ruleSets := append([]string(nil), wc.RuleSets...)
	sort.Strings(ruleSets)

	// The compiler's keys, by rule set and vertex, taken before SaveModel,
	// which strips a volatile resource's prefix from the model it saves
	compiled := make(map[string]map[int][3]*constantKey)
	for _, rs := range ruleSets {
		c := NewCompiler(dir, rs, false, false, wc.AutoAddResources)
		if err := c.Compile(); err != nil {
			t.Fatalf("%s: %v", rs, err)
		}
		byKey := make(map[int]*rete.ResourceNode)
		for _, r := range c.JetRuleModel().Resources {
			byKey[r.Key] = r
		}
		vertices := make(map[int][3]*constantKey)
		for _, rn := range c.JetRuleModel().ReteNodes {
			if rn.Type != "antecedent" {
				continue
			}
			var keys [3]*constantKey
			for i, k := range []int{rn.SubjectKey, rn.PredicateKey, rn.ObjectKey} {
				r := byKey[k]
				if r == nil || r.Type == "var" {
					continue
				}
				key, ok := termKey(r)
				keys[i] = &constantKey{key: key, counted: ok, kind: r.Type}
			}
			vertices[rn.Vertex] = keys
		}
		compiled[rs] = vertices
		if err := c.SaveModel(); err != nil {
			t.Fatalf("%s: %v", rs, err)
		}
	}

	// The engine's nodes, from the workspace.db it loads
	dbPath := filepath.Join(dir, "workspace.db")
	lookupPath := filepath.Join(dir, "lookup.db")
	if _, err := os.Stat(lookupPath); err != nil {
		lookupPath = ""
	}
	js, err := bridge.LoadJetRules("engine_keys", dbPath, lookupPath)
	if err != nil {
		t.Fatal(err)
	}
	defer js.ReleaseJetRules()
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	matched, mismatched := map[string]int{}, map[string]int{}
	notCounted := map[string]int{}
	for _, rs := range ruleSets {
		rows, err := db.Query(`
			SELECT rn.vertex, s.type, s.value, p.type, p.value, o.type, o.value
			FROM rete_nodes rn
			JOIN workspace_control wc ON wc.key = rn.source_file_key
			JOIN resources s ON s.key = rn.subject_key
			JOIN resources p ON p.key = rn.predicate_key
			JOIN resources o ON o.key = rn.object_key
			WHERE rn.type = 'antecedent' AND wc.source_file_name = ?
			ORDER BY rn.vertex`, rs)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		for rows.Next() {
			var vertex int
			var types, values [3]sql.NullString
			if err := rows.Scan(&vertex, &types[0], &values[0], &types[1], &values[1], &types[2], &values[2]); err != nil {
				t.Fatal(err)
			}
			seen++
			keys, ok := compiled[rs][vertex]
			if !ok {
				t.Errorf("%s: vertex %d is in workspace.db and not in the compiled model", rs, vertex)
				continue
			}
			for i := range 3 {
				ck := keys[i]
				if ck == nil {
					continue
				}
				if !ck.counted {
					notCounted[ck.kind]++
					continue
				}
				ek, err := engineKey(js, types[i].String, values[i].String)
				if err != nil {
					mismatched[ck.kind]++
					t.Errorf("%s: vertex %d, %s (%s): %v", rs, vertex, positionNames[i], ck.kind, err)
					continue
				}
				if ek != ck.key {
					mismatched[ck.kind]++
					t.Errorf("%s: vertex %d, %s (%s): the compiler's key differs from the engine's", rs, vertex, positionNames[i], ck.kind)
					continue
				}
				matched[ck.kind]++
			}
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		if seen != len(compiled[rs]) {
			t.Errorf("%s: %d antecedent vertices in workspace.db, %d in the compiled model", rs, seen, len(compiled[rs]))
		}
	}
	t.Logf("antecedent constants: matched %s; mismatched %s; not counted %s",
		counts(matched), counts(mismatched), counts(notCounted))
	if len(matched) == 0 {
		t.Error("no constant was compared")
	}
}

// engineKey is the key of the node the engine has for a resources row.
func engineKey(js *bridge.JetStore, typ, value string) (string, error) {
	switch typ {
	case "resource", "volatile_resource":
		name := value
		if typ == "volatile_resource" {
			name = "_0:" + value
		}
		r, err := js.GetResource(name)
		if err != nil {
			return "", err
		}
		if r.GetType() < 0 {
			return "", errNoNode
		}
		got, err := r.GetName()
		if err != nil {
			return "", err
		}
		return got, nil
	case "text":
		r, err := js.NewTextLiteral(value)
		if err != nil {
			return "", err
		}
		got, err := r.GetText()
		if err != nil {
			return "", err
		}
		return strconv.Quote(got), nil
	case "int":
		v, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return "", err
		}
		r, err := js.NewIntLiteral(v)
		if err != nil {
			return "", err
		}
		got, err := r.GetInt()
		if err != nil {
			return "", err
		}
		return strconv.Itoa(got) + "^^int", nil
	}
	return "", errNotCounted
}

type keyError string

func (e keyError) Error() string { return string(e) }

const (
	errNoNode     = keyError("the engine has no node of that name")
	errNotCounted = keyError("a kind the compiler counts and the engine test does not")
)

func counts(m map[string]int) string {
	if len(m) == 0 {
		return "0"
	}
	kinds := make([]string, 0, len(m))
	total := 0
	for k, n := range m {
		kinds = append(kinds, k+" "+strconv.Itoa(n))
		total += n
	}
	sort.Strings(kinds)
	return strconv.Itoa(total) + " (" + strings.Join(kinds, ", ") + ")"
}
