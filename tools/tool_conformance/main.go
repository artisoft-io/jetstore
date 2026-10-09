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
// **T.2 added an arm axis, 2026-08-31, and it is what makes I-94's remedy
// falsifiable.** compile_rule_file and validate_cpipes_config now take a
// workspace-relative path as an alternative to their content arguments, on the
// argument that a model which never holds the text cannot mangle it. The
// obvious way for that to be wrong is that the model fails to *name* a path as
// often as it failed to *copy* a rule — so the two populations are reported as
// separate rows with their own denominators, and the comparison is within one
// model in one run rather than against J.2's granite4.1:3b figures.
//
// **The paths in the path-arm prompts do not exist anywhere**, deliberately.
// What is measured is whether the string the prompt gave survives into the
// argument — the same question the content arm asks about a rule — and the tool
// is never called, so nothing resolves them. A prompt naming a file that
// happened to exist would measure the same thing and invite the reader to think
// otherwise.
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
	Expect string `json:"expect"`
	// Arm partitions the cases for one tool into variants being compared.
	// T.2 gives compile_rule_file and validate_cpipes_config a path-valued
	// alternative to their content arguments (I-94's remedy), and the question
	// the remedy has to answer is whether the model *names* a path as
	// unreliably as it *copied* a rule. That comparison is only readable if the
	// two populations are reported apart, which is what this field is for.
	Arm      string        `json:"arm,omitempty"`
	Prompt   string        `json:"prompt"`
	Fidelity *fidelitySpec `json:"fidelity,omitempty"`
}

// row identifies one reported line: a tool, and the arm where a tool has more
// than one.
type row struct {
	tool string
	arm  string
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
	// **Added at T.2, because the rates could not be diagnosed from the rates.**
	// The path arm of one tool moved payload fidelity from 0 of 11 to 11 of 14
	// and the other's did not, and nothing in the report could say whether the
	// failing arm named a wrong path, named none, or filled the content
	// argument instead. Those are three different verdicts on I-94's remedy.
	dump := flag.String("dump", "", "append every tool call's raw arguments to this JSONL file")
	flag.Parse()

	if err := run(*host, *model, *repeats, *timeout, *verbose, *dump); err != nil {
		fmt.Fprintln(os.Stderr, "tool_conformance:", err)
		os.Exit(1)
	}
}

func run(host, model string, repeats int, timeout time.Duration, verbose bool, dumpPath string) error {
	var dumpTo *os.File
	if dumpPath != "" {
		var err error
		dumpTo, err = os.Create(dumpPath)
		if err != nil {
			return err
		}
		defer dumpTo.Close()
	}

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

	// One row per (tool, arm) the case file actually exercises, plus a bare row
	// for every tool, so a tool with no case still reports "not exercised"
	// rather than vanishing — a catalogue growing a tool nobody wrote a probe
	// for is exactly what a conformance run should say out loud.
	results := map[row]*eval.ToolResult{}
	order := []row{}
	addRow := func(r row) {
		if results[r] == nil {
			results[r] = &eval.ToolResult{Tool: r.tool, Arm: r.arm}
			order = append(order, r)
		}
	}
	for _, sig := range sigs {
		hasArmed := false
		for _, c := range cf.Cases {
			if c.Expect == sig.Name && c.Arm != "" {
				hasArmed = true
			}
		}
		if !hasArmed {
			addRow(row{tool: sig.Name})
		}
	}
	for _, c := range cf.Cases {
		if c.Expect != "" {
			addRow(row{tool: c.Expect, arm: c.Arm})
		}
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
			if dumpTo != nil {
				for name, a := range called {
					line, _ := json.Marshal(map[string]any{
						"case": c.ID, "expect": c.Expect, "arm": c.Arm,
						"called": name, "args": json.RawMessage(a),
					})
					fmt.Fprintln(dumpTo, string(line))
				}
				if len(called) == 0 {
					line, _ := json.Marshal(map[string]any{
						"case": c.ID, "expect": c.Expect, "arm": c.Arm, "called": "",
					})
					fmt.Fprintln(dumpTo, string(line))
				}
			}

			// Denominators first, so every row's OtherTrials counts this trial
			// exactly once whether or not anything was called.
			//
			// **A sibling arm's trial lands in neither denominator, and that is
			// the one judgement in this loop.** A prompt naming a path is not a
			// trial in which compile_rule_file was the wrong answer, so counting
			// it against the text arm's OtherTrials would manufacture false
			// calls out of the tool being correctly selected. The tool is the
			// unit of selection; the arm is the unit of argument population.
			for key, r := range results {
				switch {
				case key.tool == c.Expect && key.arm == c.Arm:
					r.Trials++
				case key.tool == c.Expect:
					// sibling arm: neither
				default:
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
				if validators[name] == nil {
					// A name the registry does not carry is a hallucinated tool.
					// It cannot be attributed to any row, so it is reported as a
					// line of its own rather than silently dropped.
					fmt.Printf("  ! %s invented a tool named %q\n", c.ID, name)
					continue
				}
				if name == c.Expect {
					r := results[row{tool: c.Expect, arm: c.Arm}]
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
							switch missing, present := payloadMissing(args, c.Fidelity); {
							case !present:
								// The argument is absent, so nothing was mangled:
								// the model answered through another argument of
								// the same tool. Counted apart, because folding it
								// in reads as I-68's corruption recurring.
								r.PayloadDiverted++
								fmt.Printf("  > %s sent no %s at all\n", c.ID, c.Fidelity.Arg)
							case len(missing) == 0:
								r.PayloadFaithful++
							default:
								fmt.Printf("  ~ %s dropped from %s: %v\n", c.ID, c.Fidelity.Arg, missing)
							}
						}
					}
					continue
				}
				// A false call is attributed to every row of the tool that was
				// wrongly called, because the arm is a property of the *case*
				// and a call nobody asked for belongs to none of them.
				for key, r := range results {
					if key.tool == name {
						r.FalsePositives++
					}
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
	for _, key := range order {
		report.Tools = append(report.Tools, *results[key])
	}
	fmt.Println()
	fmt.Print(report.String())
	return nil
}

// payloadMissing returns the expected fragments the call did not carry, and
// whether the argument was there at all. The argument is rendered back to text
// before searching, so a config passed as a JSON object and one passed as a JSON
// string are judged the same way — the question is whether the content survived,
// not how it was typed.
//
// **The second return value is T.2's, and it separates two things that had one
// number.** An argument that is absent is not a mangled payload; on a tool with
// a path alternative it is usually the model choosing the other argument, which
// is a different verdict on the remedy.
func payloadMissing(args json.RawMessage, spec *fidelitySpec) ([]string, bool) {
	var obj map[string]any
	if err := json.Unmarshal(args, &obj); err != nil {
		return spec.MustContain, false
	}
	v, ok := obj[spec.Arg]
	if !ok {
		return spec.MustContain, false
	}
	var text string
	if s, isString := v.(string); isString {
		text = s
	} else {
		b, err := json.Marshal(v)
		if err != nil {
			return spec.MustContain, true
		}
		text = string(b)
	}
	var missing []string
	for _, want := range spec.MustContain {
		if !strings.Contains(text, want) {
			missing = append(missing, want)
		}
	}
	return missing, true
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
