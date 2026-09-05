package compute_pipes

// Per-field provenance on an inference operator (agentic_ai gap 16 and gap 18,
// AK.5): the model's answer checked against the entity the prompt was built
// from, by the rule table of jets/agentic/briefing rather than by a sentence in
// the system prompt.
//
// # Why this can be a pure function of one record
//
// The inference operator augments its input record **in place**, so its input
// and output channels must share a ChannelSpec (validateInferChannels). The
// serialised entity is a column of that spec, written by a column_encodings
// entry (ColumnEncodingSpec, jets/compute_pipes/pipes_model.go), and the model's
// answer arrives on the same row. So the two operands of a provenance check are
// two columns of one record: no join, no history and no second grain, which is
// not true of anything else this repository checks and is what makes the check
// affordable at all.
//
// # The disposition is pass_through, and it is a sequencing decision
//
// A finding does not fail the record and does not drop it: the briefing is
// delivered with its findings reported on the error channel. Nobody has a
// refusal rate for this checker against a model's answer, and a guardrail set to
// `fail` on an unmeasured rate is one that gets removed rather than tuned -
// pass_through is the only disposition that produces the rate the other two
// would need. **It is not the terminal answer**: a briefing a member may hear
// should not stay deliverable ungrounded once the rate is known.
//
// It is deliberately **not** routed through failedRecord, and that is the same
// distinction failedRecord's own comment makes about a defaulted on_error. A
// provenance finding is not a failed call: the model answered and the mappings
// applied. Sending it through failedRecord would give it whatever on_error the
// author chose for *call* failures, so an operator set to `fail` would stop the
// pipeline on an ungrounded field - which is precisely the disposition this
// decision declined.
//
// # What is checked, and what is deliberately not
//
// The checked unit is the **model's answer**, not the columns output_mapping
// filled. applyMappings re-encodes a json object or array as text on its way
// into a column, so over the mapped columns a briefing's arrays collapse to one
// opaque leaf and the closure that carries the guarantee would be total over a
// document with one field. Checking the columns is not a weaker version of
// checking the answer; it is a different check.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// provenanceSchemaDir is the workspace directory a provenance_schema_name
// resolves in, and provenanceSchemaSuffix is the suffix of the file. They match
// the .pv.json row of workspaceFileValidators
// (workspaceFileValidators, jets/datatable/workspace_file_validators.go), so a
// document this operator loads is a document the Workspace IDE validated on
// save.
const (
	provenanceSchemaDir    = "provenance"
	provenanceSchemaSuffix = ".pv.json"
)

// inferProvenanceCheck is a resolved provenance_schema_name: the schema, and
// where on the record to find the entity it grounds against.
type inferProvenanceCheck struct {
	name         string
	schema       *briefing.Schema
	entityColumn string
	entityColPos int
	// encoding is ColumnEncodingSpec.EntityEncoding verbatim, including the
	// empty string, which CheckEncoded reads as json exactly as the encoder
	// does.
	encoding string
}

// resolveInferProvenanceSchema loads and validates the named provenance schema
// and locates the entity column it will be run against.
//
// Everything it can refuse, it refuses **here** rather than per record: a schema
// that does not parse, a schema whose rules do not cover the declared briefing,
// a response_format that disagrees with the operator's, a channel that
// serialises no entity or serialises two. A guardrail whose configuration fails
// on the first record is a guardrail discovered in production.
//
// It returns nil, nil when no schema is named, which is every operator that
// existed before this.
func resolveInferProvenanceSchema(common *InferCommonSpec, source *InputChannel,
	configName string) (*inferProvenanceCheck, error) {

	name := strings.TrimSpace(common.ProvenanceSchemaName)
	if len(name) == 0 {
		return nil, nil
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return nil, fmt.Errorf(
			"error: %s provenance_schema_name '%s' is a name rather than a path, "+
				"it resolves to %s/<name>%s of the workspace",
			configName, name, provenanceSchemaDir, provenanceSchemaSuffix)
	}
	content, err := readProvenanceSchema(name)
	if err != nil {
		return nil, fmt.Errorf("error: %s provenance_schema_name '%s': %v", configName, name, err)
	}
	schema, findings := briefing.ParseSchema(content)
	if schema == nil {
		return nil, fmt.Errorf("error: %s provenance schema '%s' is not usable: %s",
			configName, name, provenanceFindingsText(wsvalidate.ErrorsOnly(findings)))
	}
	// Warnings travel rather than block, which is wsvalidate's own contract and
	// AK.3's for a declared prose surface. They are logged once at build so a
	// deployment is not asked to read the workspace to learn it has one.
	for _, f := range findings {
		if f.Severity != wsvalidate.Error {
			log.Printf("%s provenance schema '%s' %s: %s", configName, name, f.Severity, f.Message)
		}
	}
	// The key is compared rather than assumed: a file renamed without its key
	// being updated is a briefing checked against a contract nobody can name in
	// the audit record, which is what the key is for.
	if schema.Key != name {
		return nil, fmt.Errorf(
			"error: %s provenance schema '%s%s' declares key '%s'; the file name and the key must agree, "+
				"since the key is what says which contract a briefing was checked against",
			configName, name, provenanceSchemaSuffix, schema.Key)
	}
	if err := reconcileResponseFormat(common, schema, configName, name); err != nil {
		return nil, err
	}
	column, encoding, err := entityColumnOf(source, configName)
	if err != nil {
		return nil, err
	}
	pos, ok := (*source.Columns)[column]
	if !ok {
		return nil, fmt.Errorf(
			"error: %s the column_encodings column '%s' is not a column of the input channel '%s'",
			configName, column, source.Name)
	}
	return &inferProvenanceCheck{
		name:         name,
		schema:       schema,
		entityColumn: column,
		entityColPos: pos,
		encoding:     encoding,
	}, nil
}

// readProvenanceSchema reads provenance/<name>.pv.json of the active workspace.
// The path is composed the way every other workspace artefact's is, from
// WORKSPACES_HOME and WORKSPACE (workspaceHome, jets/compute_pipes/actions_start_common.go).
func readProvenanceSchema(name string) (string, error) {
	path := fmt.Sprintf("%s/%s/%s/%s%s", workspaceHome, wsPrefix, provenanceSchemaDir, name, provenanceSchemaSuffix)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("while reading the provenance schema at %s: %v", path, err)
	}
	return string(content), nil
}

// reconcileResponseFormat settles which of the two copies of the briefing's
// shape the model is constrained by, and refuses the case where they disagree.
//
// **This is the whole of what closes agentic_ai I-438.** Before it, the
// response_format the model is given lived in the pipeline config's
// prompt_templates and the one the guardrail's rules are checked against lived
// in the provenance schema, they were identical because somebody compared them
// by hand, and nothing compared them on any run. A difference between them is a
// field generated and never checked, which is the failure the whole check is
// about arriving through the seam rather than through the model.
//
// Adopting when absent lets a configuration keep one copy; refusing when they
// differ lets it keep two and still be safe.
func reconcileResponseFormat(common *InferCommonSpec, schema *briefing.Schema,
	configName, name string) error {

	if len(schema.Response) == 0 {
		// A schema with no declared shape is the partial contract AK.1 shipped
		// and Cover declines to check. It still grounds every field a briefing
		// carries, by the runtime closure, so it is accepted rather than
		// refused.
		return nil
	}
	if len(common.ResponseFormat) == 0 {
		common.ResponseFormat = schema.Response
		return nil
	}
	same, err := sameJSONDocument(common.ResponseFormat, schema.Response)
	if err != nil {
		return fmt.Errorf("error: %s while comparing response_format with the provenance schema '%s': %v",
			configName, name, err)
	}
	if !same {
		return fmt.Errorf(
			"error: %s the response_format it uses and the one declared by the provenance schema '%s' differ; "+
				"the shape the model is constrained by and the shape the guardrail checks must be the same document, "+
				"so remove one of the two copies",
			configName, name)
	}
	return nil
}

// entityColumnOf finds the column carrying the serialised entity on the channel
// spec the operator's input and output share.
//
// Exactly one is required. **Zero is a configuration error rather than a silent
// pass**: a provenance check with nothing to ground against would report every
// field unsupported on every record, which reads as a broken model rather than
// as a broken configuration. **Two is a configuration error rather than a
// guess**: which of them the prompt was built from is a question the operator
// cannot answer, and grounding against the wrong one is a clean report for the
// wrong reason.
func entityColumnOf(source *InputChannel, configName string) (column, encoding string, err error) {
	if source.Config == nil || len(source.Config.ColumnEncodings) == 0 {
		return "", "", fmt.Errorf(
			"error: %s names a provenance schema and the channel spec of its input channel '%s' has no "+
				"column_encodings; the check grounds the model's answer against the serialised entity the "+
				"prompt was built from, and nothing on this channel carries one",
			configName, source.Name)
	}
	if len(source.Config.ColumnEncodings) > 1 {
		names := make([]string, 0, len(source.Config.ColumnEncodings))
		for _, ce := range source.Config.ColumnEncodings {
			names = append(names, ce.Column)
		}
		return "", "", fmt.Errorf(
			"error: %s names a provenance schema and the channel spec of its input channel '%s' encodes %d "+
				"columns (%s); the check needs to know which one the prompt was built from",
			configName, source.Name, len(names), strings.Join(names, ", "))
	}
	ce := source.Config.ColumnEncodings[0]
	return ce.Column, ce.EntityEncoding, nil
}

// checkProvenance grounds the model's answer against the record's entity column
// and reports what the entity does not support.
//
// It never changes the record and never changes the record's fate. The return
// value is the number of findings, for the run counter.
func (w *inferWorker) checkProvenance(record *[]any, resp inferResponse) {
	check := w.provenance
	if check == nil {
		return
	}
	answer := w.answerText(resp)
	var entity string
	if check.entityColPos < len(*record) {
		entity = inferToString((*record)[check.entityColPos])
	}
	result, err := briefing.CheckEncoded(check.schema, check.encoding, entity, answer)
	switch {
	case err != nil:
		// A check that could not run is reported rather than passed. An
		// unreadable entity or an answer that is not a json object are both
		// "this briefing was not checked", and a guardrail silent about that is
		// one that reports clean for the wrong reason.
		w.reportProvenance(record, fmt.Sprintf(
			"the briefing could not be checked against provenance schema '%s': %v", check.name, err))
	case !result.OK():
		w.pm.provenanceFindingCount.Add(int64(len(result.Findings)))
		w.reportProvenance(record, fmt.Sprintf("provenance schema '%s': %s", check.name, result.String()))
	}
}

// reportProvenance writes one process_errors row for a record whose briefing
// carried findings, and leaves the record alone.
//
// It has its own counter against the same max_error_count rather than sharing
// the operator's. The two populations are different - a call that failed and a
// briefing that was answered and is ungrounded - and one budget spent on either
// would silence the other. The cap therefore means at most N reports *of each
// kind*, and the message that announces reaching it says which.
func (w *inferWorker) reportProvenance(record *[]any, message string) {
	nbr := w.pm.provenanceErrorCount.Add(1)
	maxErrors := int64(w.common.MaxErrorCount)
	err := fmt.Errorf("%s: %s", w.labels.ErrPrefix, message)
	switch {
	case nbr <= maxErrors:
		log.Println(err)
		if w.errorOutputCh == nil {
			return
		}
		peRow := w.builderContext.NewProcessError(w.labels.Type)
		peRow.ErrorMessage = err.Error()
		// The entity column rather than a briefing field: input_column names
		// what the report is about on the *input* side, and a finding's own
		// pointers into the briefing travel in the message.
		peRow.InputColumn = sql.NullString{String: w.provenance.entityColumn, Valid: true}
		if w.rowKeyPos >= 0 && w.rowKeyPos < len(*record) {
			peRow.RowJetsKey = sql.NullString{
				String: inferToString((*record)[w.rowKeyPos]), Valid: true}
		}
		peRow.write2Chan(w.errorOutputCh, w.doneCh)
	case nbr == maxErrors+1:
		log.Printf("%s: reached max_error_count (%d) for provenance findings, stop reporting them",
			w.labels.Operator, maxErrors)
	}
}

// sameJSONDocument compares two json documents by value rather than by bytes,
// so that indentation, key order and whitespace do not make two identical
// contracts look different. The comparison has to be forgiving about *form* and
// exact about *content*: two copies of a response_format are written by hand in
// two files, and refusing them for a trailing newline would teach an author to
// delete the check rather than the copy.
func sameJSONDocument(a, b json.RawMessage) (bool, error) {
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return false, fmt.Errorf("the operator's response_format is not valid json: %v", err)
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false, fmt.Errorf("the schema's response_format is not valid json: %v", err)
	}
	return reflect.DeepEqual(va, vb), nil
}

// provenanceFindingsText renders schema findings for a configuration error.
func provenanceFindingsText(findings []wsvalidate.Finding) string {
	if len(findings) == 0 {
		return "no findings reported"
	}
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		if len(f.Path) > 0 {
			parts = append(parts, fmt.Sprintf("%s: %s", f.Path, f.Message))
			continue
		}
		parts = append(parts, f.Message)
	}
	return strings.Join(parts, "; ")
}
