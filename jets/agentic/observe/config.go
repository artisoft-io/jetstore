package observe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
)

// The five confounders of the Anomaly vocabulary that are properties of a
// pipeline's configuration rather than of its execution record. The package
// comment says why the extraction cannot establish them; this file is the
// narrow, deliberate excursion across that boundary, and it is here rather
// than in extract.go so the boundary stays legible in the file listing.
//
// The only place a run's configuration survives the run is
// jetsapi.cpipes_execution_status.cpipes_config_json, and three limits come
// with it:
//
//   - It holds at most ONE step's config. The sharding start inserts the
//     sharding config (actions_start_sharding_cp.go:385) and every reducing
//     step overwrites it with its own (actions_start_reducing_cp.go:289), so
//     what is there at read time is the last step to have started, not the
//     step an anomaly is about.
//   - A run that failed at sharding validation leaves no row at all, because
//     the insert is downstream of the validation.
//   - The row is purged by neither the RETENTION_DAYS clock nor the header's,
//     so a config can outlive the worker rows it describes.
//
// A detector therefore reads this as evidence about a confounder and never as
// evidence about its absence, which is why ConfigConfounders carries Read and
// Note and why a caller that skips the read has to say so in the basis.

// ConfigConfounders is what a run's stored configuration says about the five
// confounders the execution record cannot supply. It is deliberately a report
// rather than a list: an empty Found under Read == false and an empty Found
// under Read == true are different claims, and collapsing them is exactly the
// silent omission I-168 says a detector must not make.
type ConfigConfounders struct {
	SessionId string
	// Read is true when a cpipes_execution_status row was found and its config
	// parsed. False means the question was asked and not answered.
	Read bool
	// Found is the subset of the five this config mentions. It is meaningful
	// only when Read is true.
	Found []string
	// Note says what the read covers or why it failed, in the words a triage
	// step needs. It is written into an Anomaly's expected_basis.
	Note string
}

// Has reports whether c positively found a confounder. It is false for an
// unread config, which is the safe direction: an unread config contributes its
// Note rather than a false negative.
func (c *ConfigConfounders) Has(name string) bool {
	return c != nil && c.Read && slices.Contains(c.Found, name)
}

// Bearing returns the confounders in want that this config found, in want's
// order. A detector passes the confounders that bear on its own signal rather
// than everything the config mentions: parquet_input is a real property of a
// run and is not a reason an output count collapsed, and an anomaly that
// lists it says less rather than more.
func (c *ConfigConfounders) Bearing(want []string) []string {
	var out []string
	for _, w := range want {
		if c.Has(w) {
			out = append(out, w)
		}
	}
	return out
}

const configJsonSQL = `SELECT cpipes_config_json FROM jetsapi.cpipes_execution_status
  WHERE session_id = $1`

// ReadConfigConfounders reads the run's stored configuration and reports which
// of the five config-sourced confounders it mentions. It returns a report and
// no error for the ordinary "there is nothing to read" cases — a run that
// failed at sharding validation is not a failure of this function — and an
// error only when the database itself would not answer.
func ReadConfigConfounders(ctx context.Context, db DB, sessionId string) (*ConfigConfounders, error) {
	c := &ConfigConfounders{SessionId: sessionId}
	if sessionId == "" {
		c.Note = "no session id, so no configuration was read"
		return c, nil
	}
	var raw string
	err := db.QueryRow(ctx, configJsonSQL, sessionId).Scan(&raw)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		c.Note = fmt.Sprintf("no cpipes_execution_status row for session %q, which is what a run that "+
			"failed at sharding validation leaves; the five configuration confounders were not checked", sessionId)
		return c, nil
	case err != nil:
		return nil, fmt.Errorf("while reading the configuration of session %q: %w", sessionId, err)
	}
	if raw == "" || raw == "{}" {
		c.Note = fmt.Sprintf("session %q has an empty cpipes_config_json; the five configuration "+
			"confounders were not checked", sessionId)
		return c, nil
	}
	var doc any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		c.Note = fmt.Sprintf("session %q has a cpipes_config_json that does not parse (%v); the five "+
			"configuration confounders were not checked", sessionId, err)
		return c, nil
	}
	c.Read = true
	c.Found = scanConfigConfounders(doc)
	c.Note = fmt.Sprintf("configuration read from cpipes_execution_status for session %q; it holds at "+
		"most one step's config — the sharding step's, overwritten by each reducing step — so it may "+
		"not be the step this anomaly is about", sessionId)
	return c, nil
}

// scanConfigConfounders walks the parsed document for the five keys rather
// than unmarshalling into the engine's own config types, and that is a choice
// with a stated cost on each side.
//
// Against: a key-name scan cannot tell two uses of one key apart, and there is
// exactly one such collision — sampling_max_count is a record cap on an input
// channel and on a partition writer (pipes_model.go:782, :1013) and is the
// number of rows used to guess a date format on a parse_date config
// (ParseDateSpec, pipes_model.go:946). The scan reports the confounder for
// both. That over-reports, which is the safe direction for a confounder: the
// field says what could not be ruled out, so a spurious member costs an
// operator one look and a missing one costs a wrong conclusion.
//
// For: unmarshalling would bind this package to ComputePipesConfig, a type
// that gains a field whenever the engine gains an operator, and a detector
// that fails to build because a new operator landed is worse than one that
// scans for a key. The engine's own error sites are the authority on where
// these keys may appear and there are more of them than any struct — max_input_count
// alone is declared on three specs (pipes_model.go:1176, :1288, :1344).
func scanConfigConfounders(doc any) []string {
	found := map[string]bool{}
	var walk func(node any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			for k, v := range n {
				switch k {
				case "on_error":
					// map_record's OnErrorDrop (pipe_transformation_map_record.go:91).
					// The other two arms, pass_through and fail, are not a silent drop.
					if s, ok := v.(string); ok && s == "drop" {
						found[ConfounderOnErrorDrop] = true
					}
				case "max_input_count":
					if positiveNumber(v) {
						found[ConfounderMaxInputCount] = true
					}
				case "sampling_max_count":
					if positiveNumber(v) {
						found[ConfounderSamplingCap] = true
					}
				case "device_writer_type":
					// An output written through a device writer contributes
					// nothing to output_records_count (section 12.2 row 2), so
					// its presence anywhere in the config is a reason a count
					// can be low without a row having been lost.
					if s, ok := v.(string); ok && s != "" {
						found[ConfounderDeviceWriterOutput] = true
					}
				case "format":
					// The reader's inputFormat switch (actions_load_main_input.go:142);
					// its parquet arm is what F49 measured.
					if s, ok := v.(string); ok && (s == "parquet" || s == "parquet_select") {
						found[ConfounderParquetInput] = true
					}
				}
				walk(v)
			}
		case []any:
			for i := range n {
				walk(n[i])
			}
		}
	}
	walk(doc)

	// Returned in the vocabulary's own order so two runs' lists are comparable
	// without sorting at the reader.
	var out []string
	for _, name := range []string{ConfounderOnErrorDrop, ConfounderMaxInputCount,
		ConfounderSamplingCap, ConfounderDeviceWriterOutput, ConfounderParquetInput} {
		if found[name] {
			out = append(out, name)
		}
	}
	return out
}

// positiveNumber is written against encoding/json's own decoding of a number
// into any, which is float64 — not against what the field is declared as in
// pipes_model.go, since the scan never sees that declaration.
func positiveNumber(v any) bool {
	f, ok := v.(float64)
	return ok && f > 0
}
