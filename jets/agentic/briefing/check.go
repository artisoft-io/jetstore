package briefing

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// Check reports every field of the briefing that its input entity does not
// support, and returns the source references that ground the rest.
//
// `entity` is the input entity as it reached the prompt - the decoded json or
// toon `EncodeColumnData` wrote into the record's column. `brief` is the model's
// answer, decoded. **Both are plain decoded documents on purpose**: a flat map
// of the columns `output_mapping` set is as valid an argument as the whole
// response object, so this package does not have to know which of the two
// `AK.2` chooses, and can be built before it chooses.
//
// The error return is a schema that does not compile. It is separate from the
// findings because the two are addressed to different people: a finding is for
// whoever reads the briefing, a schema error is for whoever wrote the contract.
func Check(s *Schema, entity, brief map[string]any) (*Result, error) {
	if s == nil {
		return nil, fmt.Errorf("no provenance schema: a briefing checked against nothing is a briefing nobody checked")
	}
	if f := s.compile(); len(wsvalidate.ErrorsOnly(f)) > 0 {
		return nil, fmt.Errorf("the provenance schema %q does not compile: %s", s.Key, f[0].Message)
	}
	res := &Result{Refs: map[string][]string{}}
	covered := map[string]bool{}
	for i := range s.Rules {
		r := &s.Rules[i]
		for _, loc := range r.field.Resolve(brief) {
			covered[loc.Pointer] = true
			if f, refs := applyRule(r, entity, loc); f != nil {
				res.Findings = append(res.Findings, *f)
			} else if len(refs) > 0 {
				res.Refs[loc.Pointer] = refs
			}
		}
	}

	// The closure. Every populated leaf the rules did not reach is an assertion
	// nobody declared an obligation for, which is what criterion 52 refuses.
	var found []Located
	leaves(brief, "", &found)
	for _, l := range found {
		if covered[l.Pointer] {
			continue
		}
		res.Findings = append(res.Findings, Finding{Finding: wsvalidate.Finding{
			Severity: wsvalidate.Error,
			Code:     CodeNoRule,
			Path:     l.Pointer,
			Message: fmt.Sprintf(
				"field %s is populated and the provenance schema %q has no rule for it; "+
					"an undeclared field is an unchecked assertion", l.Pointer, s.Key),
		}})
	}

	// A stable order, not document order: pointers sort lexicographically, so
	// /events/10 precedes /events/2. Determinism is what is wanted here - phase
	// 3's F118 measured what happens to a harness whose output reshuffles - and
	// claiming document order would be a claim this comparison does not make.
	sort.SliceStable(res.Findings, func(i, j int) bool {
		if res.Findings[i].Path != res.Findings[j].Path {
			return res.Findings[i].Path < res.Findings[j].Path
		}
		return res.Findings[i].Code < res.Findings[j].Code
	})
	return res, nil
}

// CheckJSON is Check over two undecoded documents, which is what a pipeline
// operator holds: the entity column is a json string and the response is a json
// string. toon is not decoded here - nothing in the tree reads toon back, and a
// checker that silently accepted a format it cannot parse would report a clean
// briefing for every record.
func CheckJSON(s *Schema, entityJSON, briefJSON string) (*Result, error) {
	var entity, brief map[string]any
	if err := json.Unmarshal([]byte(entityJSON), &entity); err != nil {
		return nil, fmt.Errorf("while reading the input entity as json: %w", err)
	}
	if err := json.Unmarshal([]byte(briefJSON), &brief); err != nil {
		return nil, fmt.Errorf("while reading the briefing as json: %w", err)
	}
	return Check(s, entity, brief)
}

func applyRule(r *FieldRule, entity map[string]any, loc Located) (*Finding, []string) {
	fail := func(code, msg string) (*Finding, []string) {
		return &Finding{
			Finding: wsvalidate.Finding{
				Severity: wsvalidate.Error, Code: code, Path: loc.Pointer, Message: msg,
			},
			Sources: append([]string(nil), r.Sources...),
		}, nil
	}
	switch r.Kind {
	case KindUngrounded:
		return nil, nil

	case KindGrounded:
		want := normalise(loc.Value)
		if want == "" {
			// An empty string asserts nothing, so there is nothing to ground.
			return nil, nil
		}
		var refs []string
		for _, src := range r.sources {
			for _, cand := range src.Resolve(entity) {
				if matches(want, normalise(cand.Value), r.Match) {
					refs = append(refs, cand.Pointer)
				}
			}
		}
		if len(refs) == 0 {
			return fail(CodeUngrounded, fmt.Sprintf(
				"%s is %q, which appears nowhere in the input entity under %s",
				loc.Pointer, fmt.Sprint(loc.Value), strings.Join(r.Sources, ", ")))
		}
		return nil, refs

	case KindWithinSpan:
		at, ok := asDate(loc.Value)
		if !ok {
			return fail(CodeUnparseableDate, fmt.Sprintf(
				"%s is %q and a within_span rule cannot read it as a date", loc.Pointer, fmt.Sprint(loc.Value)))
		}
		var lo, hi time.Time
		var loRef, hiRef string
		for _, src := range r.sources {
			for _, cand := range src.Resolve(entity) {
				d, ok := asDate(cand.Value)
				if !ok {
					continue
				}
				if loRef == "" || d.Before(lo) {
					lo, loRef = d, cand.Pointer
				}
				if hiRef == "" || d.After(hi) {
					hi, hiRef = d, cand.Pointer
				}
			}
		}
		if loRef == "" {
			// An empty span admits every date. Reporting it is the difference
			// between "this date is supported" and "nothing said otherwise".
			return fail(CodeNoSpan, fmt.Sprintf(
				"%s is a date and the input entity has no observed span under %s to place it in",
				loc.Pointer, strings.Join(r.Sources, ", ")))
		}
		if at.Before(lo) || at.After(hi) {
			return fail(CodeOutOfSpan, fmt.Sprintf(
				"%s is %s, outside the observed span %s to %s of %s",
				loc.Pointer, at.Format(dateOnly), lo.Format(dateOnly), hi.Format(dateOnly),
				strings.Join(r.Sources, ", ")))
		}
		return nil, []string{loRef, hiRef}

	case KindCountOf:
		want, ok := asNumber(loc.Value)
		if !ok {
			return fail(CodeNotANumber, fmt.Sprintf(
				"%s is %q and a count_of rule cannot read it as a number", loc.Pointer, fmt.Sprint(loc.Value)))
		}
		var refs []string
		for _, src := range r.sources {
			for _, cand := range src.Resolve(entity) {
				refs = append(refs, cand.Pointer)
			}
		}
		if want != float64(len(refs)) {
			return fail(CodeWrongCount, fmt.Sprintf(
				"%s says %s and the input entity has %d under %s",
				loc.Pointer, trimNumber(want), len(refs), strings.Join(r.Sources, ", ")))
		}
		return nil, refs

	case KindDerived:
		var got []Located
		for _, src := range r.sources {
			got = append(got, src.Resolve(entity)...)
		}
		if len(got) != 1 {
			return fail(CodeNotDerived, fmt.Sprintf(
				"%s is derived from %s, which resolves to %d values in the input entity rather than one",
				loc.Pointer, strings.Join(r.Sources, ", "), len(got)))
		}
		if normalise(loc.Value) != normalise(got[0].Value) {
			return fail(CodeNotDerived, fmt.Sprintf(
				"%s is %q and the value it is derived from is %q; a derived field is populated deterministically "+
					"and a difference means the model wrote over it",
				loc.Pointer, fmt.Sprint(loc.Value), fmt.Sprint(got[0].Value)))
		}
		return nil, []string{got[0].Pointer}
	}
	// Unreachable: compile refuses an unknown kind and Check refuses a schema
	// that does not compile. Answered rather than panicked, because a guardrail
	// that takes the process down is one somebody removes.
	return fail("briefing_rule_kind_not_applied", fmt.Sprintf("rule kind %q was not applied", r.Kind))
}

func matches(want, cand string, m Match) bool {
	if cand == "" {
		return false
	}
	if m == MatchSubstring {
		return strings.Contains(cand, want)
	}
	return cand == want
}

// normalise renders a scalar for comparison: trimmed, case folded, and with runs
// of whitespace collapsed. A number is rendered without a trailing ".0", because
// json decodes every number as float64 and an entity holding 3 would otherwise
// never equal a briefing holding 3.
func normalise(v any) string {
	var s string
	switch vv := v.(type) {
	case nil:
		return ""
	case string:
		s = vv
	case float64:
		s = trimNumber(vv)
	case json.Number:
		s = vv.String()
	case time.Time:
		s = vv.Format(dateOnly)
	default:
		s = fmt.Sprint(vv)
	}
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func trimNumber(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func asNumber(v any) (float64, bool) {
	switch vv := v.(type) {
	case float64:
		return vv, true
	case int:
		return float64(vv), true
	case int64:
		return float64(vv), true
	case json.Number:
		f, err := vv.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(vv), 64)
		return f, err == nil
	}
	return 0, false
}

const dateOnly = "2006-01-02"

// dateLayouts is what a date can look like on the way in.
//
// The padded pair is what the entity writer emits: `EncodeColumnData`
// (`jets/compute_pipes/jetrules_extract_entity.go:13`) formats a time as
// `2006-01-02` when it has no clock and `2006-01-02T15:04:05` when it does. The
// **unpadded** layouts are here because the one worked briefing input in the
// tree does not use the padded ones - `prompt.md`'s events carry `2025-8-14` and
// `2025-1-17` - so a checker that accepted only what the writer emits would fail
// every date in the only sample anybody has.
var dateLayouts = []string{
	dateOnly,
	"2006-01-02T15:04:05",
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-1-2",
	"2006-1-2T15:04:05",
}

func asDate(v any) (time.Time, bool) {
	switch vv := v.(type) {
	case time.Time:
		return vv, true
	case string:
		s := strings.TrimSpace(vv)
		for _, l := range dateLayouts {
			if t, err := time.Parse(l, s); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}
