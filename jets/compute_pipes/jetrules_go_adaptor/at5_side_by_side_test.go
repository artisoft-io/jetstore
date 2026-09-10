package jetrules_go_adaptor

// The template arm and the model arm over one fact set (agentic_ai Phase 7
// `AT.5`, question Q-94).
//
// # What it is for
//
// Q-94 asks whether there is a briefing worth having that a template cannot
// write. Plan §1.8.1 makes that a controlled experiment rather than an opinion:
// `AT.3`'s template and `AT.5`'s model call take **the same jetrules fact set**
// and differ in the renderer and in nothing else, which is criterion 76. This
// file is what runs both over the 22-member evaluation population and prints
// them side by side, so the judgement criterion 75 asks for is made against
// output rather than against argument.
//
// # What makes the two arms controlled, which is the whole design
//
// One rule session per member produces one `cintel:Briefing` node. Both arms
// then read **that node**, through `JrSpecialColumnEncoding.EncodeColumnData`
// (`EncodeColumnData`, `jets/compute_pipes/jetrules_extract_entity.go:14`), which
// walks the triples with `extractAsEntity` and hands the resulting
// `map[string]any` to one of three encoders. The two arms differ in exactly two
// struct fields:
//
//	EntityEncoding:     "toon"            vs  "briefing_prose"
//	ExcludeProperties:  at5PromptExclusions   vs  (none)
//
// **That is asserted rather than claimed.** `TestTheTwoArmsDifferOnlyInTheirExclusionList`
// extracts the entity map twice, once per exclusion set, and checks that the
// model's map is the template's map with exactly the excluded property names
// removed at every depth - nothing added, nothing else lost. If a future change
// gives one arm a different traversal, a different prefix setting or a different
// subject, that test fails and criterion 76 stops being satisfiable by
// inspection. `extractAsEntity` passes `excludeProp` down its own recursion
// (`extractAsEntity`, `jets/compute_pipes/jetrules_extract_entity.go:82`), which
// is what makes the two maps comparable at all (agentic_ai F757).
//
// **The comparison is order-insensitive and that is deliberate.** A multi-valued
// property becomes a `[]any` in the order the graph container yields it, and two
// walks of one session are not obliged to agree on it. An order difference makes
// the two arms *list* things differently and does not make them see different
// facts, so the assertion canonicalises before comparing and the encodings are
// left alone.
//
// # The exclusion list is a named constant, in one place
//
// `at5PromptExclusions` is what the pipeline's `Briefing` channel carries in
// `pipes_config/patient_profile.pc.json` plus the four medication components
// `AT.1`(ii) put on the pharmacy node. Whether `cintel:Adherence` joins it is
// Q-98 and is open: §1.10.4 argues the ratio should not be in the prose and
// criterion 76 forbids withholding it from one arm alone, so the position taken
// is to leave the entity identical and **count what the model does with the
// ratio as a Q-94 datum rather than prevent it**. The list is one variable so
// that answering Q-98 is one edit, and `at5RecordedExclusions` is what the
// artefact prints, so a reading always says which list produced it.
//
// # The model arm is gated and makes no call in an ordinary run
//
// `JETS_AT5_MODEL_URL` turns it on. Unset, the arms still both run and the
// artefact still prints the template - the model column reads `(not run)` - so
// `go test ./...` never opens a socket. The request body is the one
// `vllmBackend.BuildRequest` builds for the chat api (`BuildRequest`,
// `jets/compute_pipes/pipe_transformation_vllm.go:241`): `model`, `stream:false`
// and two messages, with no sampling parameters, because the operator sends none
// unless a config supplies `options`. **The first request body is written into
// the artefact verbatim**, which is criterion 71's discipline - `AG.2` is why
// this project reads a request off the wire rather than off the code.
//
// # `-count=1`
//
// Same reason as the population file beside it, twice over: the workspace and
// the population are outside this module, and the model arm is not a function of
// anything Go can see.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// at5BriefingColumn is the column the pipeline serialises the projected entity
// into. Both arms name it, so neither is reading a different subject.
const at5BriefingColumn = "cintel:Briefing_Input"

// at5PromptExclusions is what the model sees kept out of its prompt.
//
// The first five are the `Briefing` channel's own `exclude_properties` in
// `pipes_config/patient_profile.pc.json`. The last four are `AT.1`(ii)'s
// medication components: the prompt carries one joined `cintel:Medication` value
// per drug so that a consumer which flattens cannot mispair a drug with another
// drug's indicator (agentic_ai F735), and the components are on the node beside
// it because the **template** needs them as four things (F757). Excluding them
// from the prompt is what stops the same facts appearing twice in the claim
// summary.
var at5PromptExclusions = map[string]bool{
	"jets:key":                    true,
	"rdf:type":                    true,
	"jets:source_period_sequence": true,
	"cintel:Briefing_Member_ID":   true,
	"cintel:Briefing_Disclaimer":  true,
	"hc:Drug_Name":                true,
	"cintel:Maintenance":          true,
	"cintel:Fill_Count":           true,
	"cintel:Fill_Date":            true,
}

// at5RecordedExclusions renders the list the run actually used, sorted, for the
// artefact's header. A reading of the output that does not say which properties
// were withheld is not checkable.
func at5RecordedExclusions() []string {
	out := make([]string, 0, len(at5PromptExclusions))
	for k := range at5PromptExclusions {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// at5Encode runs one arm.
func at5Encode(s *rdf.RdfSession, briefing *rdf.Node, encoding string, exclude map[string]bool) any {
	ce := &compute_pipes.JrSpecialColumnEncoding{
		Config: &compute_pipes.ColumnEncodingSpec{
			Column:              at5BriefingColumn,
			EntityEncoding:      encoding,
			RemoveModelPrefixes: true,
		},
		ExcludeProperties: exclude,
	}
	return ce.EncodeColumnData(&JetRdfSessionGo{rdfSession: s}, &RdfNodeGo{node: briefing})
}

// at5EncodeString runs one arm and insists on a string.
//
// **`EncodeColumnData` returns an `error` value rather than returning an error**,
// which is a shape worth knowing before reading a failure here: the prose branch
// returns the error as the column's value so that a record carries its own
// failure. A test has to check the type.
func at5EncodeString(t *testing.T, s *rdf.RdfSession, briefing *rdf.Node, encoding string, exclude map[string]bool) string {
	t.Helper()
	out := at5Encode(s, briefing, encoding, exclude)
	switch v := out.(type) {
	case string:
		return v
	case error:
		t.Fatalf("the %s arm failed: %v", encoding, v)
	}
	t.Fatalf("the %s arm returned %T: %v", encoding, out, out)
	return ""
}

// --- criterion 76, asserted -----------------------------------------------

// at5ExtractMap recovers the entity map each arm is built from, so the two
// **inputs** can be compared rather than the two outputs.
//
// **Through the `json` encoding rather than through `extractAsEntity` directly.**
// The walk is unexported and exporting it for a test would put a second entry
// point on the function whose single entry point is the property being asserted.
// `EncodeColumnData`'s `json` branch is the same walk with the same arguments,
// one `json.Marshal` further on, and both arms are read the same way - so a
// difference this test reports is a difference in the walk and not in how the
// two sides were recovered.
func at5ExtractMap(t *testing.T, s *rdf.RdfSession, briefing *rdf.Node, exclude map[string]bool) map[string]any {
	t.Helper()
	raw := at5EncodeString(t, s, briefing, "json", exclude)
	var entity map[string]any
	if err := json.Unmarshal([]byte(raw), &entity); err != nil {
		t.Fatalf("the json arm did not produce an object: %v\n%s", err, raw)
	}
	return entity
}

// at5Prune removes the excluded property names from a map at every depth, by the
// **unprefixed** name, because `remove_model_prefixes` is on for both arms and
// `addToEntityObj` strips the prefix before it stores the key
// (`addToEntityObj`, `jets/compute_pipes/jetrules_extract_entity.go:134`).
func at5Prune(v any, exclude map[string]bool) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if at5Excluded(k, exclude) {
				continue
			}
			out[k] = at5Prune(val, exclude)
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, e := range t {
			out = append(out, at5Prune(e, exclude))
		}
		return out
	default:
		return v
	}
}

func at5Excluded(localName string, exclude map[string]bool) bool {
	for prop := range exclude {
		if i := strings.Index(prop, ":"); i >= 0 {
			prop = prop[i+1:]
		}
		if prop == localName {
			return true
		}
	}
	return false
}

// at5Canonical renders a value as JSON with every list sorted, so two walks of
// one session compare on content rather than on container order.
func at5Canonical(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(at5Sorted(v))
	if err != nil {
		t.Fatalf("canonicalising the entity map: %v", err)
	}
	return string(b)
}

func at5Sorted(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = at5Sorted(val)
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, e := range t {
			out = append(out, at5Sorted(e))
		}
		sort.Slice(out, func(i, j int) bool {
			a, _ := json.Marshal(out[i])
			b, _ := json.Marshal(out[j])
			return string(a) < string(b)
		})
		return out
	default:
		return v
	}
}

// **Criterion 76 as a test rather than as a sentence.** Anything reported as a
// difference between the arms is attributable to the renderer only if the two
// arms saw the same facts; this asserts that they did, over every member of the
// population rather than over one.
func TestTheTwoArmsDifferOnlyInTheirExclusionList(t *testing.T) {
	requireCompiledWorkspace(t)
	p := at5LoadPopulation(t)
	for _, id := range p.memberIDs {
		t.Run(id, func(t *testing.T) {
			s, rm := ruleSessionOver(t, at5Fixture(p, id))
			pr := readProjection(t, s, rm)

			template := at5ExtractMap(t, s, pr.briefing, nil)
			model := at5ExtractMap(t, s, pr.briefing, at5PromptExclusions)

			// Non-vacuity: an empty map is a subset of everything.
			if len(template) == 0 {
				t.Fatal("the template arm's entity map is empty")
			}
			if got, want := at5Canonical(t, model), at5Canonical(t, at5Prune(template, at5PromptExclusions)); got != want {
				t.Errorf("the model arm's entity is not the template arm's minus the exclusion list.\n"+
					"model:    %s\nexpected: %s", got, want)
			}
			// And the exclusion list is doing something, or the two arms are the
			// same arm and the control is vacuous.
			if len(model) >= len(template) {
				t.Errorf("the exclusion list removed nothing: %d properties on both sides", len(model))
			}
		})
	}
}

// --- the model arm --------------------------------------------------------

// at5SystemPrompt is the prose system prompt, which is what the pipeline's
// `patient_briefing` template becomes once the structured response retires
// (agentic_ai F736). Held here as a literal rather than read out of the
// workspace, because the workspace copy is being changed by another task in the
// same track and a harness that read it would measure whichever version happened
// to be on disk.
const at5SystemPrompt = `You prepare briefings for a call-centre representative who is about to speak with a member. The representative is not a clinician and takes no clinical action.

Write a short briefing in prose - about 100 words, no headings and no lists. Answer with the briefing and nothing else.

The claim summary is TOON. A field written as ` + "`Diagnosis[3]: A,B,C`" + ` is a list of three separate values, not one value containing commas.

Every fact you write must come from the claim summary you are given. Do not add a condition, a medication, a date or a count that is not in the summary, and do not infer a diagnosis, a severity or a cause from what is there.

The briefing is informational. Do not recommend, advise or instruct.`

// at5UserPrompt is `patient_profile.pc.json`'s prompt template with the column
// substituted, byte for byte.
func at5UserPrompt(toon string) string {
	return "Claim summary in TOON:\n\n" + toon + "\n\nBriefing:\n"
}

type at5ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// at5BuildRequest mirrors `vllmBackend.BuildRequest` for the chat api with no
// `options` and no structured output, which is what an infer_config carrying
// neither produces.
func at5BuildRequest(model, toon string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"model":  model,
		"stream": false,
		"messages": []at5ChatMessage{
			{Role: "system", Content: at5SystemPrompt},
			{Role: "user", Content: at5UserPrompt(toon)},
		},
	})
}

type at5ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type at5ModelArm struct {
	url   string
	model string
	http  *http.Client
}

// at5Model returns the model arm, or nil when the gate is closed.
func at5Model() *at5ModelArm {
	url := os.Getenv("JETS_AT5_MODEL_URL")
	if url == "" {
		return nil
	}
	model := os.Getenv("JETS_AT5_MODEL")
	if model == "" {
		model = "granite4.1:3b"
	}
	return &at5ModelArm{url: url, model: model, http: &http.Client{Timeout: 10 * time.Minute}}
}

func (m *at5ModelArm) call(toon string) (body []byte, answer string, finish string, prompt, completion int, err error) {
	body, err = at5BuildRequest(m.model, toon)
	if err != nil {
		return nil, "", "", 0, 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.url, bytes.NewReader(body))
	if err != nil {
		return body, "", "", 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.http.Do(req)
	if err != nil {
		return body, "", "", 0, 0, err
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return body, "", "", 0, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return body, "", "", 0, 0, fmt.Errorf("%s: %s", resp.Status, buf.String())
	}
	var parsed at5ChatResponse
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		return body, "", "", 0, 0, err
	}
	if len(parsed.Choices) == 0 {
		return body, "", "", 0, 0, fmt.Errorf("no choice in the response: %s", buf.String())
	}
	return body, strings.TrimSpace(parsed.Choices[0].Message.Content), parsed.Choices[0].FinishReason,
		parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens, nil
}

// --- the artefact ---------------------------------------------------------

// **The artefact is the deliverable and it is not committed.** Model output is
// not a golden file: it varies with the server's sampling defaults, and a
// committed copy would invite a future run to be compared against it as though
// it were an expectation. `JETS_AT5_ARTEFACT` names where it goes.
func TestAT5TheTemplateAndTheModelOverOneFactSet(t *testing.T) {
	requireCompiledWorkspace(t)
	out := os.Getenv("JETS_AT5_ARTEFACT")
	arm := at5Model()
	if out == "" && arm == nil {
		t.Skip("neither JETS_AT5_ARTEFACT nor JETS_AT5_MODEL_URL is set; " +
			"see the header of this file")
	}
	p := at5LoadPopulation(t)

	var doc strings.Builder
	fmt.Fprintf(&doc, "# `AT.5` — the template arm and the model arm over one fact set\n\n")
	fmt.Fprintf(&doc, "Run %s.\n\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&doc, "- Population: `%s`, %d members, %d medical claims, %d pharmacy fills.\n",
		at5PopulationDir, len(p.memberIDs), p.medicalRows, p.pharmacyRows)
	fmt.Fprintf(&doc, "- Template arm: `entity_encoding: briefing_prose`, **empty** exclusion list.\n")
	fmt.Fprintf(&doc, "- Model arm: `entity_encoding: toon`, exclusion list `%s`.\n",
		strings.Join(at5RecordedExclusions(), "`, `"))
	if arm != nil {
		fmt.Fprintf(&doc, "- Backend: `%s`, model `%s`.\n", arm.url, arm.model)
	} else {
		fmt.Fprintf(&doc, "- Backend: **not run** (`JETS_AT5_MODEL_URL` unset).\n")
	}
	fmt.Fprintf(&doc, "\n**Every figure taken from this run is an upper bound.** The population's NDCs "+
		"were curated to resolve — 0 misses over %d fills — so it excludes the unresolved-NDC case that "+
		"occurs in production (criterion 80). The medical side was not curated to the same standard: 27 "+
		"of its %d claims carry a primary diagnosis (`M54.50`, `M54.51`) with no description behind it "+
		"(I-566).\n\n", p.pharmacyRows, p.medicalRows)

	var firstRequest []byte
	for _, id := range p.memberIDs {
		s, rm := ruleSessionOver(t, at5Fixture(p, id))
		pr := readProjection(t, s, rm)

		toon := at5EncodeString(t, s, pr.briefing, "toon", at5PromptExclusions)
		prose := at5EncodeString(t, s, pr.briefing, "briefing_prose", nil)

		// The fact set's own shape, read off the **session** rather than off the
		// TOON, so a reader can check a claim about either arm's output against a
		// count they did not have to tally by hand. Everything here is also in
		// the TOON printed below; what it saves is the tallying, which is where a
		// reading of 22 pairs goes wrong.
		maintenance := 0
		for ev := range pr.briefingRx {
			if textValue(t, s, rm, rm.NewResource(ev), "cintel:Maintenance") == "Y" {
				maintenance++
			}
		}
		fmt.Fprintf(&doc, "\n---\n\n## %s\n\n", id)
		fmt.Fprintf(&doc, "%d medical claims, %d pharmacy fills in the CSV; %d medical and %d pharmacy "+
			"events on the briefing; %d distinct conditions; %d of the %d medications flagged "+
			"`maintenance Y`.\n\n",
			len(p.medical[id]), len(p.pharmacy[id]), len(pr.briefingMedical), len(pr.briefingRx),
			len(values(s, rm, pr.briefing, "cintel:Condition_Summary")), maintenance, len(pr.briefingRx))
		fmt.Fprintf(&doc, "### Input — the claim summary the model is given (TOON)\n\n```\n%s\n```\n\n", toon)
		fmt.Fprintf(&doc, "### Arm (c) — the template\n\n```\n%s\n```\n\n", prose)

		if arm == nil {
			fmt.Fprintf(&doc, "### Arm (b') — the model\n\n_(not run)_\n\n")
			continue
		}
		body, answer, finish, promptTok, completionTok, err := arm.call(toon)
		if firstRequest == nil {
			firstRequest = body
		}
		if err != nil {
			t.Errorf("%s: the model arm failed: %v", id, err)
			fmt.Fprintf(&doc, "### Arm (b') — the model\n\n_(failed: %v)_\n\n", err)
			continue
		}
		fmt.Fprintf(&doc, "### Arm (b') — the model\n\n```\n%s\n```\n\n", answer)
		fmt.Fprintf(&doc, "_finish_reason `%s`, %d prompt tokens, %d completion tokens._\n\n",
			finish, promptTok, completionTok)
	}

	if firstRequest != nil {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, firstRequest, "", "  "); err != nil {
			pretty.Write(firstRequest)
		}
		fmt.Fprintf(&doc, "\n---\n\n## The captured request body\n\nThe first call of the run, verbatim, "+
			"as `at5BuildRequest` produced it — the shape `vllmBackend.BuildRequest` builds for the chat "+
			"api with no `options` and no structured output.\n\n```json\n%s\n```\n", pretty.String())
	}

	if out != "" {
		if err := os.WriteFile(out, []byte(doc.String()), 0o644); err != nil {
			t.Fatalf("writing the artefact to %s: %v", out, err)
		}
		t.Logf("wrote %d bytes to %s", doc.Len(), out)
	}
}
