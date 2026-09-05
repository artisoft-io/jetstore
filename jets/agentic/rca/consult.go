package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/infer"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// The model arm, and what it is for.
//
// §9.8 says AC.2 is where the model earns its place, because mapping a locus to
// one of the imported ten is the step the record does not determine. It also
// says the contradicting side should be populated from the record's own
// vocabulary **before** a model is asked, so that a model's contribution is
// measurable against a floor rather than against nothing. Rank is the floor;
// Consult is the arm, and ConsultReport is the comparison.
//
// # What can be measured here without labels, and what cannot
//
// AA.2 counted the labelled population and it is **zero** (plan §10): no
// incident anywhere in reach of this repository is paired with a written cause.
// So *is the model right* is not a question this or any harness here can ask,
// and a hit rate would be a number with no denominator. What can be asked is
// whether the model's output is **admissible**, and four of those questions are
// answerable from the gate's own table:
//
//   - does it carry contradicting evidence at all, and is that side non-empty;
//   - does it cite an EvidenceSource that §9.7 found has no substrate in
//     JetStore — commit_history, infrastructure_log, prior_incident;
//   - does it name a cause class §9.5 attaches to no locus, so that no record
//     could evidence it;
//   - does it attribute a class to a locus §9.5 says cannot carry it.
//
// Each is a property of the answer rather than of the world, so each is
// countable today. **None of them says the model is useful**, and ConsultReport
// is written so that a reader cannot mistake one for the other.
//
// # The schema goes in the prompt as well as in the format
//
// I-180: infer.Client puts the schema in Ollama's `format` field and builds its
// messages from the system and user strings alone, and Phase 1's I-41 measured
// that shape at nine parseable, non-conformant fragments out of nine. The
// remedy Phase 1 recorded — the schema has to be in the prompt — was
// implemented in the Python toolchain and never ported to Go. This caller
// applies it at the call site rather than waiting for the port: instruction()
// renders the schema into the user message, so a bare Chat's known-bad shape is
// not what any figure below was measured on.

// Inferer is the one method this package needs. *infer.Client satisfies it, and
// a test substitutes its own.
type Inferer interface {
	Chat(ctx context.Context, req *infer.Request) (*infer.Response, error)
}

// ConsultReport is what one model arm produced, with the denominators that make
// each count readable. It is a report about an answer and never about a run.
//
// Report.Model is mandatory for P.1's reason (F113, R-23): a rate without its
// model is not a rate, and this machine has no GPU, so the model that answered
// is as much a property of a figure as the count is.
type ConsultReport struct {
	Model     string
	SessionId string
	// Sessions is 1 here and is carried so a caller aggregating several arms
	// does not have to reconstruct it.
	Sessions int
	// Answered is whether the model returned something the schema accepted.
	// infer.Client validates client-side against the same document it sent as
	// the format, so a false here is a refusal rather than a parse failure.
	Answered bool
	// Error is why not, when Answered is false.
	Error string

	PromptTokens, EvalTokens int

	Hypotheses int
	// WithContradicting counts hypotheses whose contradicting list is
	// non-empty. **The schema can require the key and cannot require content**,
	// which is the whole of what §A.2.8's calibration control is worth once a
	// generator is constrained rather than asked.
	WithContradicting int
	// SupportingItems and ContradictingItems are the two totals §B.3's
	// escalation trigger compares.
	SupportingItems, ContradictingItems int

	// UnsubstantiatedSources counts evidence items citing one of the three
	// EvidenceSource members §9.7 found has no substrate in JetStore. An item
	// sourced from a commit history no run names is not evidence.
	UnsubstantiatedSources int
	// ClassesWithNoLocus counts hypotheses naming a cause class §9.5 attaches
	// to no locus, so that nothing in the record could evidence one.
	ClassesWithNoLocus int
	// PairsOutsideTheTable counts hypotheses whose (locus, class) pair is not a
	// row of §9.5 — the model attributing a class to a position the gate says
	// cannot carry it.
	PairsOutsideTheTable int
	// LociNotPresent counts hypotheses at a locus triage did not find present,
	// which is the model reasoning past its evidence rather than from it.
	LociNotPresent int

	// FloorHypotheses is what the deterministic floor produced over the same
	// input, so every count above has something to be read against.
	FloorHypotheses            int
	FloorWithContradicting     int
	FloorUnsubstantiatedSource int
}

// Describe is the report in the words a reader needs, including the sentence
// that keeps it from being read as an accuracy measurement.
func (r *ConsultReport) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "model %s over session %s: ", r.Model, r.SessionId)
	if !r.Answered {
		fmt.Fprintf(&b, "no admissible answer (%s). ", r.Error)
		return b.String()
	}
	fmt.Fprintf(&b, "%d hypotheses, %d of them carrying contradicting evidence; %d supporting items "+
		"and %d contradicting. %d evidence item(s) cite a source §9.7 found has no substrate; "+
		"%d hypothesis/hypotheses name a class §9.5 attaches to no locus; %d pair a class with a locus "+
		"§9.5 does not list for it; %d sit at a locus triage did not find present. "+
		"The deterministic floor produced %d hypotheses over the same evidence, %d with contradicting "+
		"evidence and %d citing a source with no substrate. "+
		"%d prompt tokens, %d completion tokens. "+
		"**None of these counts is an accuracy**: the labelled population is zero (plan §10), so "+
		"whether either ranking is right is not a question this measures.",
		r.Hypotheses, r.WithContradicting, r.SupportingItems, r.ContradictingItems,
		r.UnsubstantiatedSources, r.ClassesWithNoLocus, r.PairsOutsideTheTable, r.LociNotPresent,
		r.FloorHypotheses, r.FloorWithContradicting, r.FloorUnsubstantiatedSource,
		r.PromptTokens, r.EvalTokens)
	return b.String()
}

// sourcesWithNoSubstrate are §9.7's three: nothing in JetStore can produce one.
var sourcesWithNoSubstrate = []string{SourceCommitHistory, SourceInfrastructureLog, SourcePriorIncident}

// Consult asks a model for a ranking over the same evidence the floor read, and
// returns the model's ranking beside a report comparing the two.
//
// The model's hypotheses go through Hypothesis.Validate exactly as the floor's
// do; one that fails is dropped and counted rather than repaired, because a
// repair loop here would measure the loop rather than the model.
func (r Ranker) Consult(ctx context.Context, client Inferer, model string, in *Input) (
	*Ranking, *ConsultReport, error) {

	if in == nil || in.Report == nil {
		return nil, nil, fmt.Errorf("rca: Consult needs a triage report")
	}
	floor := r.Rank(in)
	rep := &ConsultReport{
		Model: model, SessionId: in.Report.SessionId, Sessions: 1,
		FloorHypotheses: len(floor.Hypotheses),
	}
	for i := range floor.Hypotheses {
		if len(floor.Hypotheses[i].ContradictingEvidence) > 0 {
			rep.FloorWithContradicting++
		}
		rep.FloorUnsubstantiatedSource += unsubstantiated(&floor.Hypotheses[i])
	}

	resp, err := client.Chat(ctx, &infer.Request{
		System: systemPrompt,
		User:   instruction(in),
		Schema: json.RawMessage(responseSchema),
	})
	if err != nil {
		rep.Error = err.Error()
		return nil, rep, nil
	}
	rep.Answered = true
	rep.PromptTokens, rep.EvalTokens = resp.PromptTokens, resp.EvalTokens

	var answer struct {
		Hypotheses []struct {
			Cause                 string         `json:"cause"`
			CauseCategory         string         `json:"cause_category"`
			Locus                 string         `json:"locus"`
			Confidence            float64        `json:"confidence"`
			SupportingEvidence    []jsonEvidence `json:"supporting_evidence"`
			ContradictingEvidence []jsonEvidence `json:"contradicting_evidence"`
		} `json:"hypotheses"`
	}
	if err := json.Unmarshal([]byte(resp.Content), &answer); err != nil {
		rep.Answered = false
		rep.Error = "the answer satisfied the schema and did not decode: " + err.Error()
		return nil, rep, nil
	}

	present := map[string]bool{}
	for _, l := range in.Report.Loci() {
		present[l] = true
	}
	out := &Ranking{
		SessionId: in.Report.SessionId,
		RankerRef: fmt.Sprintf("%s+%s", r.Ref(), model),
		Basis: fmt.Sprintf("%s ranked %d hypotheses for session %s after consulting model %s over the "+
			"same evidence the deterministic floor read. The floor's ranking is the comparison and is "+
			"returned separately; nothing here is scored for correctness, the labelled population being "+
			"zero (plan §10).", r.Ref(), len(answer.Hypotheses), in.Report.SessionId, model),
	}
	for i := range answer.Hypotheses {
		a := answer.Hypotheses[i]
		h := Hypothesis{
			HypothesisId:          fmt.Sprintf("%s/%s/%s#model", in.Report.SessionId, a.Locus, a.CauseCategory),
			Cause:                 a.Cause,
			CauseCategory:         a.CauseCategory,
			Locus:                 a.Locus,
			Confidence:            clamp01(a.Confidence),
			Rank:                  len(out.Hypotheses) + 1,
			SupportingEvidence:    fromJSON(a.SupportingEvidence),
			ContradictingEvidence: fromJSON(a.ContradictingEvidence),
			RankerRef:             out.RankerRef,
		}
		if h.ContradictingEvidence == nil {
			h.ContradictingEvidence = []Evidence{}
		}
		h.Basis = fmt.Sprintf("ranked %d: proposed by model %s with %d evidence item(s) for and %d "+
			"against, at its own stated confidence of %.2f. **The number is the model's and is not "+
			"computed from the two lists**, which is how it differs from a floor hypothesis's basis.",
			h.Rank, model, len(h.SupportingEvidence), len(h.ContradictingEvidence), h.Confidence)

		rep.Hypotheses++
		rep.SupportingItems += len(h.SupportingEvidence)
		rep.ContradictingItems += len(h.ContradictingEvidence)
		if len(h.ContradictingEvidence) > 0 {
			rep.WithContradicting++
		}
		rep.UnsubstantiatedSources += unsubstantiated(&h)
		if cs := ClassesFor(h.Locus); !containsClass(cs, h.CauseCategory) {
			rep.PairsOutsideTheTable++
		}
		if evidenceabilityOf(h.CauseCategory) == None && len(lociOf(h.CauseCategory)) == 0 {
			rep.ClassesWithNoLocus++
		}
		if !present[h.Locus] {
			rep.LociNotPresent++
		}
		if err := h.Validate(); err != nil {
			// Dropped rather than repaired, and counted through the fields
			// above, which are incremented before this point on purpose: a
			// hypothesis the model produced is part of what it produced whether
			// or not this package will pass it on.
			continue
		}
		out.Hypotheses = append(out.Hypotheses, h)
	}
	// Ranks are dense over what survived validation.
	for i := range out.Hypotheses {
		out.Hypotheses[i].Rank = i + 1
	}
	return out, rep, nil
}

type jsonEvidence struct {
	Statement string `json:"statement"`
	Source    string `json:"source"`
	SourceRef string `json:"source_ref"`
}

func fromJSON(items []jsonEvidence) []Evidence {
	if items == nil {
		return nil
	}
	out := make([]Evidence, 0, len(items))
	for _, e := range items {
		out = append(out, Evidence{Statement: e.Statement, Source: e.Source, SourceRef: e.SourceRef})
	}
	return out
}

func unsubstantiated(h *Hypothesis) int {
	n := 0
	for _, e := range h.SupportingEvidence {
		if slices.Contains(sourcesWithNoSubstrate, e.Source) {
			n++
		}
	}
	for _, e := range h.ContradictingEvidence {
		if slices.Contains(sourcesWithNoSubstrate, e.Source) {
			n++
		}
	}
	return n
}

func containsClass(cs []CauseClass, name string) bool {
	for i := range cs {
		if cs[i].Name == name {
			return true
		}
	}
	return false
}

func lociOf(name string) []string {
	for i := range causeClasses {
		if causeClasses[i].Name == name {
			return causeClasses[i].Loci
		}
	}
	return nil
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

const systemPrompt = `You are a diagnostic assistant for JetStore, a data pipeline platform.

You are given what a deterministic triage step found about one pipeline run: nine predicates over the
execution record, each with a verdict of present, absent or not_evaluable, and the basis on which the
verdict was reached. A verdict says WHERE in the record evidence sits. It never says what caused the
failure; that step is yours.

Propose causal hypotheses. Every hypothesis must carry evidence FOR it and evidence AGAINST it. The
evidence against is not optional and it is not a formality: it is what makes the output advisory
rather than assertive, and a hypothesis whose case against is empty is one you have not examined.

Rules you must follow:
- Quote the evidence you were given. Do not invent measurements, log lines, commits or prior incidents.
- Use only the evidence sources listed in the schema. Three of them - commit_history,
  infrastructure_log and prior_incident - have no substrate in this system at all; citing one is a
  claim about data that does not exist.
- A locus whose verdict is not_evaluable was not checked. It is not evidence that nothing happened.
- Do not propose a cause the record cannot distinguish. Say so in the evidence against instead.`

// instruction renders the evidence and the schema into one user message. The
// schema is included here because infer.Client sends it only as Ollama's format
// field (I-180) and Phase 1 measured that shape at nine non-conformant
// fragments out of nine (I-41, F114).
func instruction(in *Input) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Run: session %s.\n\nTriage verdicts (%s):\n\n",
		in.Report.SessionId, in.Report.Findings[0].ClassifierRef)
	for i := range in.Report.Findings {
		f := &in.Report.Findings[i]
		fmt.Fprintf(&b, "- locus %s: %s\n  basis: %s\n", f.Locus, f.Verdict, f.Basis)
		if len(f.Confounders) > 0 {
			fmt.Fprintf(&b, "  the classifier could not rule out: %s\n", strings.Join(f.Confounders, ", "))
		}
	}
	if len(in.Anomalies) > 0 {
		b.WriteString("\nWhat the detectors reported for this run:\n\n")
		for _, a := range in.Anomalies {
			fmt.Fprintf(&b, "- %s on %s %s: observed %s, compared against %s (detector %s)\n",
				a.SignalType, a.SubjectType, a.SubjectRef, a.ObservedValue, a.ExpectedBasis, a.DetectorRef)
			if len(a.Confounders) > 0 {
				fmt.Fprintf(&b, "  the detector could not rule out: %s\n", strings.Join(a.Confounders, ", "))
			}
		}
	}
	b.WriteString("\nThe cause classes you may use, and what this system's execution record can " +
		"evidence about each:\n\n")
	for _, c := range causeClasses {
		where := "no locus in the record can carry it"
		if len(c.Loci) > 0 {
			where = "carried by locus " + strings.Join(c.Loci, ", ")
		}
		fmt.Fprintf(&b, "- %s (%s): %s. %s\n", c.Name, c.Evidenceability, where, c.Note)
	}
	fmt.Fprintf(&b, "\nThe loci, in the order the failure would occur: %s\n",
		strings.Join(triage.Loci, ", "))
	b.WriteString("\nAnswer with JSON conforming to this schema, and nothing else:\n\n")
	b.WriteString(responseSchema)
	b.WriteString("\n")
	return b.String()
}

// responseSchema is both the format sent to the server and the document the
// client validates against. It is built once at init from the two vocabularies,
// so a regeneration that changes either moves the prompt with it.
var responseSchema = buildSchema()

func buildSchema() string {
	enum := func(vals []string) string {
		q := make([]string, 0, len(vals))
		for _, v := range vals {
			q = append(q, `"`+v+`"`)
		}
		return "[" + strings.Join(q, ", ") + "]"
	}
	return `{
  "type": "object",
  "properties": {
    "hypotheses": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "cause": {"type": "string"},
          "cause_category": {"type": "string", "enum": ` + enum(CauseCategories) + `},
          "locus": {"type": "string", "enum": ` + enum(triage.Loci) + `},
          "confidence": {"type": "number"},
          "supporting_evidence": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "statement": {"type": "string"},
                "source": {"type": "string", "enum": ` + enum(EvidenceSources) + `},
                "source_ref": {"type": "string"}
              },
              "required": ["statement", "source"]
            }
          },
          "contradicting_evidence": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "statement": {"type": "string"},
                "source": {"type": "string", "enum": ` + enum(EvidenceSources) + `},
                "source_ref": {"type": "string"}
              },
              "required": ["statement", "source"]
            }
          }
        },
        "required": ["cause", "cause_category", "locus", "confidence", "supporting_evidence", "contradicting_evidence"]
      }
    }
  },
  "required": ["hypotheses"]
}`
}
