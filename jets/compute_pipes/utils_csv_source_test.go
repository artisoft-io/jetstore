package compute_pipes

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// The `csv_file` discriminator, and the agreement between what the contract
// describes and what the builder accepts.
//
// `CsvSourceSpec/csv_file` was a token of three static artefacts -- the Go
// contract table in `cpipes_contract_data.go`, the Pydantic model and the
// emitted json schema -- and of no arm of `NewCsvSourceS3`. A document
// declaring it passed all three and failed inside a running worker when the
// lookup table was built. The failure was a configuration error reported at run
// time, which is the class rather than the case: nothing static could have
// caught it, because every static artefact said yes.
//
// So the tests here are about *agreement* rather than about the arm.
// `TestCsvSourceTokensAreTheBuilderArms` is the durable one: it fails when a
// token is added to the contract with no arm behind it, which is the shape that
// produced this track.
//
// It does not catch the inverse -- an arm added with no token -- and saying so
// is worth more than the test pretending otherwise. The contract's tokens are
// enumerable and the builder's arms are not: a type switch offers nothing to
// range over, so the only way to ask whether an arm exists is to name a token
// and probe it. That direction is the cheaper failure of the two in any case,
// because an arm no document can reach is dead rather than misleading.

// A lookup table declaring the token, as a document rather than as a struct
// literal -- the gates read json, so the thing held against them is json.
const csvFileLookupDocument = `{
  "key": "thresholds",
  "type": "s3_csv_lookup",
  "csv_source": {
    "type": "csv_file",
    "csv_source_file_key": "$SESSIONID/thresholds.csv",
    "format": "csv",
    "compression": "none"
  },
  "lookup_key": ["field_id"],
  "lookup_values": ["authored_rate"]
}`

func csvSourceContractKeys(t *testing.T, token string) map[string]ContractField {
	t.Helper()
	entry, ok := CpipesContract["CsvSourceSpec/"+token]
	if !ok {
		t.Fatalf("the contract carries no CsvSourceSpec/%s token", token)
	}
	return entry
}

// A document declaring `csv_file` is described by the contract and built by the
// engine, on the same bytes. Removing the arm from NewCsvSourceS3 fails the
// second half; removing the token from the contract fails the first.
func TestCsvSourceCsvFileRunsWhatTheContractDescribes(t *testing.T) {
	var spec LookupSpec
	if err := json.Unmarshal([]byte(csvFileLookupDocument), &spec); err != nil {
		t.Fatalf("while decoding the csv_file lookup document: %v", err)
	}
	if spec.CsvSource == nil {
		t.Fatal("the document decoded with no csv_source")
	}

	// Every key the document authors under csv_source is applicable to the
	// token, per the contract. A key the contract does not carry would be one
	// the engine is about to read and no gate would refuse.
	var authored map[string]any
	if err := json.Unmarshal([]byte(csvFileLookupDocument), &authored); err != nil {
		t.Fatalf("while decoding the document as a map: %v", err)
	}
	authoredSource, ok := authored["csv_source"].(map[string]any)
	if !ok {
		t.Fatal("the document's csv_source is not an object")
	}
	contract := csvSourceContractKeys(t, "csv_file")
	for key := range authoredSource {
		if _, ok := contract[key]; !ok {
			t.Errorf("the document authors csv_source.%s and the contract does not carry it", key)
		}
	}

	env := map[string]any{"$SESSIONID": "123456789"}
	source, err := NewCsvSourceS3(spec.CsvSource, env)
	if err != nil {
		t.Fatalf("NewCsvSourceS3 refused a document every static gate accepts: %v", err)
	}
	if source.fileKey == nil {
		t.Fatal("NewCsvSourceS3 returned an empty source for a spec that does not ask for one")
	}
	if got, want := source.fileKey.key, "123456789/thresholds.csv"; got != want {
		t.Errorf("file key: got %q, want %q -- csv_source_file_key is resolved against env", got, want)
	}
	// The zero byte range is what makes the object download in full;
	// DownloadS3Object only sends a Range header when `end > 0`.
	if source.fileKey.end != 0 || source.fileKey.start != 0 {
		t.Errorf("file key carries a byte range (%d-%d); a named object is downloaded whole",
			source.fileKey.start, source.fileKey.end)
	}
}

// The contract marks `csv_source_file_key` required for this token and nothing
// enforced that claim. Required means the builder refuses without it.
func TestCsvSourceCsvFileRequiresItsFileKey(t *testing.T) {
	contract := csvSourceContractKeys(t, "csv_file")
	if !contract["csv_source_file_key"].Required {
		t.Fatal("the contract no longer marks csv_source_file_key required for csv_file; " +
			"this test is the engine's half of that claim and has nothing to hold")
	}
	_, err := NewCsvSourceS3(&CsvSourceSpec{Type: "csv_file"}, nil)
	if err == nil {
		t.Fatal("NewCsvSourceS3 accepted a csv_file source with no csv_source_file_key")
	}
	if !strings.Contains(err.Error(), "csv_source_file_key") {
		t.Errorf("the refusal does not name the missing field: %v", err)
	}
}

// A key that is nothing but an unresolved variable resolves to the empty
// string, and an empty key downloads the bucket root rather than a file.
func TestCsvSourceCsvFileRefusesAKeyThatResolvesToNothing(t *testing.T) {
	_, err := NewCsvSourceS3(
		&CsvSourceSpec{Type: "csv_file", CsvSourceFileKey: "$MISSING"},
		map[string]any{"$MISSING": ""})
	if err == nil {
		t.Fatal("NewCsvSourceS3 accepted a csv_file source whose key resolves to an empty string")
	}
	if !strings.Contains(err.Error(), "empty key") {
		t.Errorf("the refusal does not say what is wrong: %v", err)
	}
}

// The defaults the arm applies, stated so that a change to them is a change to
// a test rather than a surprise in a reader. They are the inverse of the cpipes
// arm's, which defaults to headerless_csv and snappy because that is what a
// stage file is.
func TestCsvSourceCsvFileDefaultsToAnUncompressedCsv(t *testing.T) {
	spec := &CsvSourceSpec{Type: "csv_file", CsvSourceFileKey: "some/key.csv"}
	if _, err := NewCsvSourceS3(spec, nil); err != nil {
		t.Fatalf("NewCsvSourceS3: %v", err)
	}
	if spec.Format != "csv" {
		t.Errorf("format default: got %q, want %q", spec.Format, "csv")
	}
	if spec.Compression != "none" {
		t.Errorf("compression default: got %q, want %q", spec.Compression, "none")
	}
	// Both defaults are inside the contract's value range for the token.
	contract := csvSourceContractKeys(t, "csv_file")
	assertInRange(t, contract, "format", spec.Format)
	assertInRange(t, contract, "compression", spec.Compression)
}

func assertInRange(t *testing.T, contract map[string]ContractField, key, value string) {
	t.Helper()
	values := contract[key].Values
	if len(values) == 0 {
		t.Fatalf("the contract records no value range for %s; this assertion has nothing to hold", key)
	}
	for _, v := range values {
		if v == value {
			return
		}
	}
	t.Errorf("the engine defaults %s to %q, which is outside the contract's range %v", key, value, values)
}

// The durable half: the tokens the contract describes are exactly the arms the
// builder accepts.
//
// "Accepts" here means *recognises the discriminator*, not *succeeds*: the
// cpipes arm needs a read_step_id and reaches s3, so it is probed with a spec
// it will refuse for its own reason, and what is asserted is that the refusal
// is not "unknown CsvSourceS3 type". That is the only property this test is
// about, and it is the one that was false.
func TestCsvSourceTokensAreTheBuilderArms(t *testing.T) {
	const unknownType = "unknown CsvSourceS3 type"

	tokens := make([]string, 0, 2)
	for key := range CpipesContract {
		if rest, ok := strings.CutPrefix(key, "CsvSourceSpec/"); ok {
			tokens = append(tokens, rest)
		}
	}
	sort.Strings(tokens)
	if len(tokens) == 0 {
		t.Fatal("the contract carries no CsvSourceSpec token at all")
	}

	for _, token := range tokens {
		// Deliberately a bare spec: every arm refuses it, and the refusal says
		// which kind of refusal it is.
		_, err := NewCsvSourceS3(&CsvSourceSpec{Type: token}, nil)
		if err == nil {
			t.Errorf("token %q: a bare spec was accepted, so this probe proves nothing", token)
			continue
		}
		if strings.Contains(err.Error(), unknownType) {
			t.Errorf("token %q is in the contract and has no arm in NewCsvSourceS3: %v", token, err)
		}
	}

	// The converse, so that the assertion above cannot be met by a builder that
	// accepts everything: a token the contract does not carry is refused as
	// unknown.
	_, err := NewCsvSourceS3(&CsvSourceSpec{Type: "not_a_token"}, nil)
	if err == nil || !strings.Contains(err.Error(), unknownType) {
		t.Errorf("a token outside the contract was not refused as unknown: %v", err)
	}
}
