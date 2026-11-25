package triage

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/errorcode"
)

// # The per-record locus's error codes
//
// Locus per_record_failures_reported reads the run's process_errors rows with
// their operator type and message, and gives each row its error code
// (errorcode.Catalogue.Match): the code of the catalogue row or JetStore fixed
// entry whose match prefix reaches furthest into the message, or
// "<operator>: uncatalogued". **A code is built only from a catalogue row's
// literal and JetStore's constants, never from a message's tail**, so grouping
// by code carries no record value, and the message itself is read, matched and
// dropped: nothing here keeps it.
//
// Each code has a threshold, the stored rows of the code in one run at or above
// which it reports. A code reports when its stored rows reach its threshold and
// its kind is not debug; otherwise it is quiet for the run. The locus is present
// when any code reports. Every code is counted in the finding's basis, quiet and
// debug ones included, so nothing below a threshold is hidden.
//
// The rows are capped per operator instance (errorcode.DefaultMaxErrorCount
// unless configured), so a code's stored rows are a lower bound on its failures.

// The states of a code in one run.
const (
	// CodeReports: its stored rows reach its threshold and it is not debug.
	CodeReports = "reports"
	// CodeQuiet: its stored rows are below its threshold.
	CodeQuiet = "quiet"
	// CodeDebug: a debug raise, counted and never reported.
	CodeDebug = "debug"
)

// CodeCount is one error code's rows in one run.
type CodeCount struct {
	Code string
	// Kind is errorcode.KindError or errorcode.KindDebug.
	Kind string
	// Operator is the operator type the rows were read as: their own, or the
	// one inferred from the message when theirs was null.
	Operator string
	// StoredRows is how many of the run's rows have the code: a lower bound.
	StoredRows int64
	// Threshold is the code's reporting threshold, in stored rows per run.
	Threshold int
	// State is CodeReports, CodeQuiet or CodeDebug.
	State string
	// Uncatalogued reports that no entry matched the rows' messages.
	Uncatalogued bool
}

// Reports says whether the code reports in the run.
func (c CodeCount) Reports() bool { return c.State == CodeReports }

// Describe renders the code's count for a basis. It names the code, never a
// message.
func (c CodeCount) Describe() string {
	rows := plural(int(c.StoredRows), "stored row", "stored rows")
	switch c.State {
	case CodeDebug:
		return fmt.Sprintf("%q %d %s, a debug raise, counted and never reported", c.Code, c.StoredRows, rows)
	case CodeQuiet:
		return fmt.Sprintf("%q %d %s, quiet: below its threshold of %d", c.Code, c.StoredRows, rows, c.Threshold)
	}
	return fmt.Sprintf("%q %d %s, reported at its threshold of %d", c.Code, c.StoredRows, rows, c.Threshold)
}

// MessageCount is a message's rows in one run, as process_errors holds them.
type MessageCount struct {
	OperatorType string
	Message      string
	Rows         int64
}

// CodesOf groups messages by their code under the catalogue, most stored rows
// first and then by code. It keeps no message.
func CodesOf(cat *errorcode.Catalogue, messages []MessageCount) []CodeCount {
	byCode := map[string]*CodeCount{}
	var order []string
	for _, m := range messages {
		match := cat.Match(m.OperatorType, m.Message)
		c := byCode[match.Code()]
		if c == nil {
			c = &CodeCount{Code: match.Code(), Kind: match.Entry.Kind, Operator: match.OperatorType,
				Threshold: match.Threshold, Uncatalogued: match.Uncatalogued}
			if c.Kind == "" {
				c.Kind = errorcode.KindError
			}
			if c.Operator == "" {
				c.Operator = errorcode.UnknownOperator
			}
			byCode[c.Code] = c
			order = append(order, c.Code)
		}
		c.StoredRows += m.Rows
	}
	out := make([]CodeCount, 0, len(order))
	for _, code := range order {
		c := byCode[code]
		switch {
		case c.Kind == errorcode.KindDebug:
			c.State = CodeDebug
		case c.StoredRows >= int64(c.Threshold):
			c.State = CodeReports
		default:
			c.State = CodeQuiet
		}
		out = append(out, *c)
	}
	slices.SortFunc(out, func(a, b CodeCount) int {
		if a.StoredRows != b.StoredRows {
			if a.StoredRows > b.StoredRows {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Code, b.Code)
	})
	return out
}

// catalogueAbsent is logged once per process: a database without the catalogue
// is named rather than failed on.
var catalogueAbsent sync.Once

// ReadCatalogue reads the workspace's error code catalogue, or an empty one
// when jetsapi.error_code_catalogue is absent (deployed false, or a read that
// finds the table missing), and says which. The absence is logged once.
func ReadCatalogue(ctx context.Context, db observe.DB, deployed bool) (*errorcode.Catalogue, bool, error) {
	if deployed {
		rows, err := db.Query(ctx, errorcode.SelectCatalogueSQL)
		if err == nil {
			entries, err := errorcode.ReadCatalogue(rows)
			rows.Close()
			if err == nil {
				return errorcode.NewCatalogue(entries), true, nil
			}
			if !errorcode.IsMissingTable(err) {
				return nil, false, fmt.Errorf("while reading %s: %w", errorcode.TableName, err)
			}
		} else if !errorcode.IsMissingTable(err) {
			return nil, false, fmt.Errorf("while reading %s: %w", errorcode.TableName, err)
		}
	}
	catalogueAbsent.Do(func() {
		log.Printf("triage: %s is absent from this database, so every workspace message reads "+
			"uncatalogued at threshold 1 and only JetStore's fixed codes apply; `update_db -migrateDb` installs it",
			errorcode.TableName)
	})
	return errorcode.NewCatalogue(nil), false, nil
}

const errorMessagesSQL = `SELECT coalesce(operator_type, ''), coalesce(error_message, ''), count(*)
  FROM jetsapi.process_errors WHERE session_id = $1
 GROUP BY 1, 2`

// Without operator_type, every row's operator is inferred from its message.
const errorMessagesNoOperatorSQL = `SELECT '', coalesce(error_message, ''), count(*)
  FROM jetsapi.process_errors WHERE session_id = $1
 GROUP BY 2`

// ReadErrorCodes reads the run's process_errors messages, gives each its code
// under the workspace's catalogue, and sets s.Codes, s.WithOperatorType and
// s.Catalogue. The messages are not kept.
func ReadErrorCodes(ctx context.Context, db observe.DB, extent *Extent, s *ErrorSummary) error {
	cat, read, err := ReadCatalogue(ctx, db, extent.ErrorCatalogue)
	if err != nil {
		return err
	}
	s.Catalogue = read
	stmt := errorMessagesSQL
	if !extent.ErrorOperatorType {
		stmt = errorMessagesNoOperatorSQL
	}
	rows, err := db.Query(ctx, stmt, s.SessionId)
	if err != nil {
		return fmt.Errorf("while reading the process_errors messages of session %q: %w", s.SessionId, err)
	}
	defer rows.Close()
	var messages []MessageCount
	for rows.Next() {
		var m MessageCount
		if err := rows.Scan(&m.OperatorType, &m.Message, &m.Rows); err != nil {
			return fmt.Errorf("while scanning a process_errors message of session %q: %w", s.SessionId, err)
		}
		if m.OperatorType != "" {
			s.WithOperatorType += m.Rows
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("while reading the process_errors messages of session %q: %w", s.SessionId, err)
	}
	s.Codes = CodesOf(cat, messages)
	return nil
}
