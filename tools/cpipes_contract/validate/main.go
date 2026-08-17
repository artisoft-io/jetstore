// The B.15 gate: every live corpus config validates against the emitted
// cpipes schema under santhosh-tekuri/jsonschema/v6 - the same library the
// engine's Go side will use, so the schema is proven in the language that
// consumes it. A real config that fails means the schema is wrong, not the
// config (criterion 6); live means workspaces/*/pipes_config/** and excludes
// the retired documents under */data/ (I-13).
//
// Run from the JetStore repo root:
//
//	go run ./tools/cpipes_contract/validate \
//	    -schema tools/cpipes_contract/cpipes_schema.json -corpus ../..
//
// or via `python -m cpipes_contract validate`.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func main() {
	schemaPath := flag.String("schema", "tools/cpipes_contract/cpipes_schema.json", "the emitted cpipes schema")
	corpusRoot := flag.String("corpus", "../..", "root holding workspaces/")
	negatives := flag.String("negatives", "tools/cpipes_contract/negative_suite.json",
		"the B.16 negative suite; empty to skip")
	flag.Parse()

	fh, err := os.Open(*schemaPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	doc, err := jsonschema.UnmarshalJSON(fh)
	fh.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad schema json: %v\n", err)
		os.Exit(2)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("cpipes_schema.json", doc); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	schema, err := compiler.Compile("cpipes_schema.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "schema does not compile: %v\n", err)
		os.Exit(2)
	}

	var files []string
	pattern := filepath.Join(*corpusRoot, "workspaces", "*", "pipes_config")
	dirs, _ := filepath.Glob(pattern)
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".pc.json") {
				files = append(files, path)
			}
			return nil
		})
	}
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no corpus files found; check -corpus")
		os.Exit(2)
	}

	fails := 0
	for _, path := range files {
		fh, err := os.Open(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		instance, err := jsonschema.UnmarshalJSON(fh)
		fh.Close()
		if err != nil {
			fails++
			fmt.Printf("FAIL %s: bad json: %v\n", path, err)
			continue
		}
		if err := schema.Validate(instance); err != nil {
			fails++
			msg := err.Error()
			if idx := strings.IndexByte(msg, '\n'); idx > 0 {
				lines := strings.Split(msg, "\n")
				if len(lines) > 6 {
					msg = strings.Join(lines[:6], "\n") + "\n  ..."
				}
			}
			fmt.Printf("FAIL %s:\n  %s\n", path, strings.ReplaceAll(msg, "\n", "\n  "))
		}
	}
	fmt.Printf("%d/%d validate under santhosh-tekuri/jsonschema/v6\n", len(files)-fails, len(files))

	// The B.16 negative suite: every case marked invalid must fail, and a
	// negative that validates is a schema hole worth failing the build for.
	if *negatives != "" {
		nfh, err := os.Open(*negatives)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		var suite struct {
			Cases []struct {
				Name   string          `json:"name"`
				Class  string          `json:"class"`
				Expect string          `json:"expect"`
				Config json.RawMessage `json:"config"`
			} `json:"cases"`
		}
		dec := json.NewDecoder(nfh)
		if err := dec.Decode(&suite); err != nil {
			fmt.Fprintf(os.Stderr, "bad negative suite: %v\n", err)
			os.Exit(2)
		}
		nfh.Close()
		holes := 0
		for _, c := range suite.Cases {
			instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(c.Config))
			if err != nil {
				fmt.Fprintf(os.Stderr, "bad case %q: %v\n", c.Name, err)
				os.Exit(2)
			}
			valid := schema.Validate(instance) == nil
			want := c.Expect == "valid"
			if valid != want {
				holes++
				fmt.Printf("HOLE [%s] %s: expected %s, got %s\n",
					c.Class, c.Name, c.Expect, map[bool]string{true: "valid", false: "invalid"}[valid])
			}
		}
		fmt.Printf("%d/%d negative-suite cases behave as expected\n", len(suite.Cases)-holes, len(suite.Cases))
		fails += holes
	}

	if fails > 0 {
		os.Exit(1)
	}
}
