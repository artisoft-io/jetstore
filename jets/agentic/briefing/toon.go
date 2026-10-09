package briefing

import (
	"fmt"

	togo "github.com/toon-format/toon-go"
)

// CheckTOON is Check over the encoding the one live briefing pipeline actually
// writes: the input entity as toon, the model's answer as json.
//
// # Why the checker learns to read toon rather than the projection emitting json
//
// `ColumnEncodingSpec` (`ColumnEncodingSpec`,
// `jets/compute_pipes/pipes_model.go:291`) admits both, and
// `workspaces/jets_ws/pipes_config/patient_profile.pc.json` chooses toon. The
// phase-5 plan §9.7 left `AK.2` two branches: the projection emits json, or
// something learns to read toon. **The second is right and it is not a
// convenience.**
//
// toon exists to cut prompt tokens, and A§8.2 measures prompt tokens dominating
// this workload roughly 9:1 - the minimum-necessary projection is argued for on
// exactly that arithmetic. A guardrail that forced json would be paid for out of
// the budget the thing it guards was built to protect, which is the wrong party
// paying. And the entity column is one document: the prompt template
// interpolates the same column this reads, so a checker that insisted on a json
// re-encoding would be checking a document the model was never shown. That is a
// second encoder to drift, and this repository has measured what a second
// encoder costs (P2 M.5).
//
// What it cost is one library call. `github.com/toon-format/toon-go` is already
// in go.mod for `EncodeColumnData`'s `Marshal`
// (`EncodeColumnData`, `jets/compute_pipes/jetrules_extract_entity.go:13`) and
// exports the decoder beside it, at the same version.
//
// # What a round trip does and does not preserve
//
// The decoder's own contract is that numbers come back as float64, objects as
// map[string]any and arrays as []any - the shape Check wants. What does not come
// back is **type**: the encoder formats a `time.Time` as text
// (`2006-01-02`, or `2006-01-02T15:04:05` when it carries a clock) and the
// decoder returns that text as a string. This costs nothing here, because
// `asDate` parses both forms and every comparison in this package is over
// normalised text. It is the same asymmetry the phase-5 plan's §10.3 records for
// a decoder that has to assert back into a rule session, where it is expensive;
// a checker is the case where it is free, and the difference is that a checker
// compares rather than reasons.
func CheckTOON(s *Schema, entityTOON, briefJSON string) (*Result, error) {
	entity, err := DecodeTOONEntity(entityTOON)
	if err != nil {
		return nil, err
	}
	brief, err := decodeJSONObject(briefJSON, "the briefing")
	if err != nil {
		return nil, err
	}
	return Check(s, entity, brief)
}

// DecodeTOONEntity reads the entity column of a record as toon.
//
// A document whose root is not an object is refused rather than wrapped:
// `EncodeColumnData` always writes a map, so a scalar or a list at the root is a
// column holding something other than an encoded entity, and grounding a
// briefing against it would report a clean record for the wrong reason.
func DecodeTOONEntity(content string) (map[string]any, error) {
	v, err := togo.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("while reading the input entity as toon: %w", err)
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf(
			"while reading the input entity as toon: the document is a %T rather than an object; "+
				"an encoded entity is always an object", v)
	}
	return obj, nil
}

// CheckEncoded is Check over an entity column whose encoding is named rather
// than known, which is what a caller holding a `ColumnEncodingSpec` has.
//
// The empty encoding is json, matching `ColumnEncodingSpec`'s own default. An
// encoding this package does not read is an **error rather than a pass**: a
// checker that shrugged at a format it cannot parse would report a clean
// briefing for every record, which is the worst failure available to a
// guardrail.
func CheckEncoded(s *Schema, encoding, entity, briefJSON string) (*Result, error) {
	switch encoding {
	case "", "json":
		return CheckJSON(s, entity, briefJSON)
	case "toon":
		return CheckTOON(s, entity, briefJSON)
	default:
		return nil, fmt.Errorf(
			"the input entity is encoded as %q and this checker reads json and toon; "+
				"a briefing cannot be checked against an entity nothing can read", encoding)
	}
}
