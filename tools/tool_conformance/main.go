// J.2: measure whether the local model can drive the tool catalogue.
//
// **What A§10 asks for is "drivable by both Claude and the local model", and
// only half of that has ever been true.** Claude drives the catalogue over
// stdio through jets/agentic/mcp, doing its own selection and argument
// population. Nothing in jets/agentic has ever asked the local model to select
// a tool: jets/agentic/infer is an /api/chat client with a JSON Schema in
// `format`, which is structured generation. The Phase-1 loop does not close the
// gap either — it dispatches task.Verifier, a field set in Go (I-63). So this
// tool builds the path that did not exist rather than instrumenting one that
// did (I-64).
//
// **The catalogue comes from the registry and the schemas from the adapter.**
// tools.DefaultRegistry is the inventory an MCP client is served, and
// mcpadapter.WireSchema is the exact relaxation it is served through — a
// harness that rebuilt either would measure a catalogue no client sees, and the
// discrepancy would not show up in the result.
//
// **The prompts are hand-written probes, not a corpus**, and the report says so
// on every rendering. There is no corpus of people asking JetStore for things;
// inventing one and calling it a measurement would be the more expensive
// mistake.
//
//	go run ./tools/tool_conformance
//	go run ./tools/tool_conformance -repeats 3 -model granite4.1:3b
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/eval"
	mcpadapter "github.com/artisoft-io/jetstore/jets/agentic/mcp"
	"github.com/artisoft-io/jetstore/jets/agentic/tools"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed cases.json
var casesJSON []byte

// fidelitySpec says what a payload-preserving call must carry. Substrings
// rather than equality on purpose: a model may legitimately reformat
// whitespace or reorder a JSON object's keys, and neither is a corruption.
// What it may not do is drop an import line or a rule label.
type fidelitySpec struct {
	Arg         string   `json:"arg"`
	MustContain []string `json:"must_contain"`
}

type testCase struct {
	ID string `json:"id"`
	// Expect is the tool the prompt should reach for, or "" when no catalogue
	// tool answers it.
	Expect   string        `json:"expect"`
	Prompt   string        `json:"prompt"`
	Fidelity *fidelitySpec `json:"fidelity,omitempty"`
}

type caseFile struct {
	Cases []testCase `json:"cases"`
}

type ollamaTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type toolCall struct {
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

type chatRequest struct {
	Model    string         `json:"model"`
	Stream   bool           `json:"stream"`
	Messages []chatMessage  `json:"messages"`
	Tools    []ollamaTool   `json:"tools,omitempty"`
	Options  map[string]any `json:"options,omitempty"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Message struct {
		Content   string     `json:"content"`
		ToolCalls []toolCall `json:"tool_calls"`
	} `json:"message"`
}

func main() {
	host := flag.String("host", "http://localhost:11434", "inference server")
	model := flag.String("model", "granite4.1:3b", "model tag")
	repeats := flag.Int("repeats", 1, "how many times to run each case")
	timeout := flag.Duration("timeout", 180*time.Second, "per-call timeout")
	verbose := flag.Bool("v", false, "print every trial")
	flag.Parse()

	if err := run(*host, *model, *repeats, *timeout, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "tool_conformance:", err)
		os.Exit(1)
	}
}

func run(host, model string, repeats int, timeout time.Duration, verbose bool) error {
	reg, err := tools.DefaultRegistry()
	if err != nil {
		return fmt.Errorf("while building the registry: %w", err)
	}
	sigs := reg.List()

	var wire []ollamaTool
	validators := map[string]*jsonschema.Schema{}
	for _, sig := range sigs {
		relaxed, err := mcpadapter.WireSchema(sig.InputSchema)
		if err != nil {
			return fmt.Errorf("while relaxing %s's schema: %w", sig.Name, err)
		}
		var t ollamaTool
		t.Type = "function"
		t.Function.Name = sig.Name
		t.Function.Description = sig.Description
		t.Function.Parameters = relaxed
		wire = append(wire, t)

		compiled, err := compileSchema(sig.Name, relaxed)
		if err != nil {
			return fmt.Errorf("while compiling %s's wire schema: %w", sig.Name, err)
		}
		validators[sig.Name] = compiled
	}

	var cf caseFile
	if err := json.Unmarshal(casesJSON, &cf); err != nil {
		return fmt.Errorf("while reading cases.json: %w", err)
	}
	for _, c := range cf.Cases {
		if c.Expect != "" && validators[c.Expect] == nil {
			return fmt.Errorf("case %s expects %q, which the registry does not carry", c.ID, c.Expect)
		}
	}

	results := map[string]*eval.ToolResult{}
	for _, sig := range sigs {
		results[sig.Name] = &eval.ToolResult{Tool: sig.Name}
	}
	var abst eval.AbstentionResult
	served := model

	client := &http.Client{Timeout: timeout}
	for _, c := range cf.Cases {
		for i := 0; i < repeats; i++ {
			resp, err := chat(context.Background(), client, host, model, c.Prompt, wire)
			if err != nil {
				return fmt.Errorf("case %s: %w", c.ID, err)
			}
			if resp.Model != "" {
				served = resp.Model
			}
			called := map[string]json.RawMessage{}
			for _, tc := range resp.Message.ToolCalls {
				called[tc.Function.Name] = tc.Function.Arguments
			}

			// Denominators first, so every tool's OtherTrials counts this trial
			// exactly once whether or not anything was called.
			for name, r := range results {
				if name == c.Expect {
					r.Trials++
				} else {
					r.OtherTrials++
				}
			}
			if c.Expect == "" {
				abst.Trials++
				if len(called) == 0 {
					abst.Abstained++
				}
			}

			for name, args := range called {
				r := results[name]
				if r == nil {
					// A name the registry does not carry is a hallucinated tool.
					// It cannot be attributed to any row, so it is reported as a
					// line of its own rather than silently dropped.
					fmt.Printf("  ! %s invented a tool named %q\n", c.ID, name)
					continue
				}
				if name == c.Expect {
					// Exactly this tool, and nothing else, counts as selected:
					// a call accompanied by a second call is not the behaviour a
					// client can act on.
					if len(called) == 1 {
						r.Selected++
						if err := validateArgs(validators[name], args); err != nil {
							if verbose {
								fmt.Printf("  - %s args rejected: %v\n", c.ID, err)
							}
						} else {
							r.ArgsValid++
						}
						if c.Fidelity != nil {
							r.PayloadTrials++
							if missing := payloadMissing(args, c.Fidelity); len(missing) == 0 {
								r.PayloadFaithful++
							} else {
								fmt.Printf("  ~ %s dropped from %s: %v\n", c.ID, c.Fidelity.Arg, missing)
							}
						}
					}
				} else {
					r.FalsePositives++
				}
			}
			if verbose {
				fmt.Printf("  %s expect=%q called=%v\n", c.ID, c.Expect, keys(called))
			}
		}
	}

	report := &eval.ToolReport{
		Mechanism:  eval.MechanismNativeTools,
		Model:      served,
		CaseSource: "tools/tool_conformance/cases.json — hand-written probes, not a corpus",
		Repeats:    repeats,
		Abstention: abst,
	}
	for _, sig := range sigs {
		report.Tools = append(report.Tools, *results[sig.Name])
	}
	fmt.Println()
	fmt.Print(report.String())
	return nil
}

// payloadMissing returns the expected fragments the call did not carry. The
// argument is rendered back to text before searching, so a config passed as a
// JSON object and one passed as a JSON string are judged the same way — the
// question is whether the content survived, not how it was typed.
func payloadMissing(args json.RawMessage, spec *fidelitySpec) []string {
	var obj map[string]any
	if err := json.Unmarshal(args, &obj); err != nil {
		return spec.MustContain
	}
	v, ok := obj[spec.Arg]
	if !ok {
		return spec.MustContain
	}
	var text string
	if s, isString := v.(string); isString {
		text = s
	} else {
		b, err := json.Marshal(v)
		if err != nil {
			return spec.MustContain
		}
		text = string(b)
	}
	var missing []string
	for _, want := range spec.MustContain {
		if !strings.Contains(text, want) {
			missing = append(missing, want)
		}
	}
	return missing
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func compileSchema(name string, doc json.RawMessage) (*jsonschema.Schema, error) {
	var v any
	if err := json.Unmarshal(doc, &v); err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	url := "mem://" + name + ".json"
	if err := c.AddResource(url, v); err != nil {
		return nil, err
	}
	return c.Compile(url)
}

func validateArgs(s *jsonschema.Schema, args json.RawMessage) error {
	if len(args) == 0 {
		args = []byte("{}")
	}
	var v any
	if err := json.Unmarshal(args, &v); err != nil {
		return fmt.Errorf("arguments are not JSON: %w", err)
	}
	return s.Validate(v)
}

func chat(ctx context.Context, hc *http.Client, host, model, prompt string, tl []ollamaTool) (*chatResponse, error) {
	body, err := json.Marshal(chatRequest{
		Model:    model,
		Stream:   false,
		Messages: []chatMessage{{Role: "user", Content: prompt}},
		Tools:    tl,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the server answered %s", resp.Status)
	}
	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
