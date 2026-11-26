// Package errorcode gives every row of jetsapi.process_errors a short error code,
// derived when the row is read rather than stored on it.
//
// **It imports nothing outside the standard library**, like jets/wsvalidate: the
// operators that write the messages (jets/compute_pipes), the workspace compile
// that checks a workspace's codes (jets/workspace) and the readers that group
// failures by code all import it, and none of them has to import another.
//
// # Two sources of codes
//
//   - **JetStore's own messages** have fixed codes, the entries Fixed returns:
//     the jetrules worker's messages, the rule engine's, and the per-record
//     forms of map_record, render and the inference operators. They need no
//     row anywhere and are kept beside the code that writes the messages, so a
//     test can hold the two together.
//   - **A workspace's codes** are rows of jetsapi.error_code_catalogue, which
//     the workspace's base init script (process_config/base__workspace_init_db.sql)
//     fills, as it fills every other jetsapi table it configures. A rule raises
//     an exception by asserting (?state jets:exception "text" + ?value); the row
//     names the text's literal head. The workspace compile lists every raise
//     and compares it with the rows (Check).
//
// # How a message gets its code (Catalogue.Match)
//
// The operator's fixed prefix is stripped first (StripPrefix): the rule
// exception prefix "jets:exception caught: " for jetrules, "map_record: " for
// map_record. The code is then the one of the entry whose match prefix reaches
// furthest into what is left: the **longest** match wins, so a row for
// "error SKU not found" beats a row for "error ". A message nothing matches
// reads "<operator>: uncatalogued".
//
// A row's match prefix is literal. A fixed entry's may hold the wildcard '%',
// any run of characters as in SQL LIKE, because three of JetStore's messages put
// a value before their distinguishing text (a property name, an inference model).
//
// **A null operator type** (an error channel declared before process_errors
// carried operator_type, which is still common for jetrules) is inferred from
// the message's fixed prefix when it has one, and otherwise every operator's
// entries are tried and the longest match wins. A message that still matches
// nothing reads "unknown_operator: uncatalogued".
//
// # Thresholds (Catalogue.Match, Match.Threshold)
//
// A threshold counts the **stored rows of one code in one run**. It is a lower
// bound on the failures, because an operator instance stops storing rows at its
// max_error_count (DefaultMaxErrorCount by default). A code's threshold is its
// own report_threshold, else the workspace default row's (operator type "*",
// empty match prefix), else 1. A fixed code reports at 1 unless a catalogue row
// with source "jetstore" and the fixed entry's operator type and match prefix
// overrides its threshold or its kind; an uncatalogued message reports at 1.
//
// A code of kind "debug" is a raise a workspace makes only while debugging; it is
// counted and never reported.
package errorcode

import (
	"database/sql"
	"errors"
	"sort"
	"strings"
	"unicode"
)

// Kinds of code.
const (
	KindError = "error"
	KindDebug = "debug"
)

// Sources of a code.
const (
	SourceRules          = "rules"
	SourcePipelineConfig = "pipeline_config"
	SourceJetStore       = "jetstore"
)

// Operator types as process_errors.operator_type stores them.
const (
	OperatorJetrules  = "jetrules"
	OperatorMapRecord = "map_record"
	OperatorRender    = "render"
	OperatorOllama    = "ollama"
	OperatorEmbed     = "embed"
	OperatorVllm      = "vllm"
)

// DefaultOperator is the operator type of the workspace default row, whose
// match prefix is empty and whose report_threshold applies to every workspace
// code with none of its own. It matches no message.
const DefaultOperator = "*"

// UnknownOperator stands for a null operator type that the message does not
// name either, in the code of an uncatalogued message.
const UnknownOperator = "unknown_operator"

// DefaultThreshold is the threshold of a fixed code, of an uncatalogued
// message, and of a workspace code when the workspace has no default row.
const DefaultThreshold = 1

// DefaultMaxErrorCount is the number of rows one jetrules or map_record operator
// instance stores before it stops, when its configuration sets no
// max_error_count. A run stores up to this many rows per instance; a run with
// several workers or nodes stores more.
const DefaultMaxErrorCount = 20

// RuleExceptionPrefix starts every process_errors row the jetrules worker writes
// for a jets:exception, whether a rule or the rule engine asserted it.
const RuleExceptionPrefix = "jets:exception caught: "

// MapRecordPrefix starts every process_errors row map_record writes. Rows
// written before it did carry the same text without it.
const MapRecordPrefix = "map_record: "

// Wildcard in a fixed entry's match prefix: any run of characters.
const Wildcard = "%"

// MaxSlugLength is the length Slug cuts a code at.
const MaxSlugLength = 48

// Entry is one error code: a row of jetsapi.error_code_catalogue, or one of
// JetStore's fixed codes.
type Entry struct {
	// OperatorType is the operator whose messages the entry matches, as
	// process_errors.operator_type spells it, or DefaultOperator.
	OperatorType string
	// MatchPrefix is the head a message starts with once the operator's fixed
	// prefix is stripped. Literal in a row; a fixed entry's may hold Wildcard.
	MatchPrefix string
	// Code is the short name shown for the message. Empty in a row, it is
	// DefaultCode(MatchPrefix).
	Code string
	// Kind is KindError or KindDebug. Empty in a row, it is KindError.
	Kind string
	// ReportThreshold is the stored rows per run at or above which the code is
	// reported; nil takes the default (see the package comment).
	ReportThreshold *int
	// Source is SourceRules, SourcePipelineConfig or SourceJetStore.
	Source string
	// SourceRef names where the code comes from: the rule file and rule name,
	// the pipeline configuration key, or the JetStore writer.
	SourceRef string
	// Description is optional text for the person reading the failure.
	Description string
}

// IsDefaultRow says whether e is the workspace default row.
func (e Entry) IsDefaultRow() bool {
	return e.OperatorType == DefaultOperator && e.MatchPrefix == ""
}

// Key is the table's key, (operator_type, match_prefix).
func (e Entry) Key() string {
	return e.OperatorType + "\x00" + e.MatchPrefix
}

// Match is the code a message was given.
type Match struct {
	// Entry is the entry that matched, with any catalogue override of a fixed
	// code applied. For an uncatalogued message it is a synthetic entry whose
	// Code is "<operator>: uncatalogued" and whose MatchPrefix is empty.
	Entry Entry
	// OperatorType is the operator the message was read as: the row's, or the
	// one inferred when the row's was null.
	OperatorType string
	// Threshold is the resolved threshold.
	Threshold int
	// Uncatalogued is true when no entry matched.
	Uncatalogued bool
	// Overridden is true when a catalogue row changed a fixed code's threshold,
	// kind or description.
	Overridden bool
}

// Code is m.Entry.Code.
func (m Match) Code() string { return m.Entry.Code }

// IsDebug says whether the code is of kind debug, and so is never reported.
func (m Match) IsDebug() bool { return m.Entry.Kind == KindDebug }

// UncataloguedCode is the code of a message no entry matches.
func UncataloguedCode(operatorType string) string {
	if operatorType == "" {
		operatorType = UnknownOperator
	}
	return operatorType + ": uncatalogued"
}

// DefaultCode is a row's code when it names none: its match prefix trimmed of
// trailing ':', ',' and spaces.
func DefaultCode(matchPrefix string) string {
	return strings.TrimRightFunc(matchPrefix, func(r rune) bool {
		return r == ':' || r == ',' || unicode.IsSpace(r)
	})
}

// Slug is the form of a code an identifier can carry: lowercased, every run of
// characters outside [a-z0-9] made one '_', trimmed of '_', and cut at
// MaxSlugLength (and trimmed again, so a cut never leaves a trailing '_').
// Two codes of one workspace must not share a slug; the compile's check says
// when they do.
func Slug(code string) string {
	var b strings.Builder
	pendingSep := false
	for _, r := range strings.ToLower(code) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingSep && b.Len() > 0 {
				b.WriteByte('_')
			}
			pendingSep = false
			b.WriteRune(r)
			continue
		}
		pendingSep = true
	}
	s := b.String()
	if len(s) > MaxSlugLength {
		s = strings.TrimRight(s[:MaxSlugLength], "_")
	}
	return s
}

// StripPrefix removes the operator's fixed prefix from message, and says
// whether it was there. For jetrules it is RuleExceptionPrefix, which the
// worker's own messages (ExecuteRules, max loop, multiple values) do not carry;
// for map_record it is MapRecordPrefix. Other operators have none.
func StripPrefix(operatorType, message string) (string, bool) {
	var prefix string
	switch operatorType {
	case OperatorJetrules:
		prefix = RuleExceptionPrefix
	case OperatorMapRecord:
		prefix = MapRecordPrefix
	default:
		return message, false
	}
	if strings.HasPrefix(message, prefix) {
		return message[len(prefix):], true
	}
	return message, false
}

// InferOperator is the operator a message's fixed prefix names, or "".
func InferOperator(message string) string {
	switch {
	case strings.HasPrefix(message, RuleExceptionPrefix):
		return OperatorJetrules
	case strings.HasPrefix(message, MapRecordPrefix):
		return OperatorMapRecord
	}
	return ""
}

// mappingColumnHead starts map_record's mapping-column message, before and
// after the message named the column:
//
//	error mapping column at output position <n>: <detail>
//	error mapping column <name> at output position <n>: <detail>
//
// The detail is the map expression's configured err_msg, or the cleansing
// function's message.
const mappingColumnHead = "error mapping column"

const mappingColumnPosition = "at output position "

// mappingDetailOffset is where the detail of a mapping-column message starts
// in body, or -1 when body is not one.
func mappingDetailOffset(body string) int {
	if !strings.HasPrefix(body, mappingColumnHead) {
		return -1
	}
	i := strings.Index(body, mappingColumnPosition)
	if i < 0 {
		return -1
	}
	j := i + len(mappingColumnPosition)
	k := j
	for k < len(body) && body[k] >= '0' && body[k] <= '9' {
		k++
	}
	if k == j || !strings.HasPrefix(body[k:], ": ") {
		return -1
	}
	return k + 2
}

// reach is how far into body pattern matches from its start, or -1 when it
// does not. A literal prefix reaches its length; a pattern with wildcards
// reaches the end of its last literal part, each part found leftmost after the
// previous one.
func reach(pattern, body string, wildcards bool) int {
	if !wildcards || !strings.Contains(pattern, Wildcard) {
		if strings.HasPrefix(body, pattern) {
			return len(pattern)
		}
		return -1
	}
	parts := strings.Split(pattern, Wildcard)
	if !strings.HasPrefix(body, parts[0]) {
		return -1
	}
	pos := len(parts[0])
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		i := strings.Index(body[pos:], part)
		if i < 0 {
			return -1
		}
		pos += i + len(part)
	}
	return pos
}

// Catalogue is a workspace's catalogue rows and JetStore's fixed codes, ready to
// match messages. Build it once per read with NewCatalogue; it is read-only
// afterwards and safe for concurrent use.
type Catalogue struct {
	// rows are the catalogue rows that match messages, overrides and the
	// default row excluded
	rows []Entry
	// fixed are JetStore's fixed codes, overrides applied
	fixed []Entry
	// overridden marks the fixed entries a row changed, by key
	overridden       map[string]bool
	defaultThreshold *int
	operators        []string
}

// NewCatalogue prepares rows (as ReadCatalogue returns them) and the fixed
// codes for matching. A row with source "jetstore" whose operator type and
// match prefix are a fixed entry's overrides that entry's threshold, kind or
// description; it keeps the fixed code. The default row sets the default
// threshold. Every other row, except one of operator type "*", matches
// messages, its prefix read literally.
func NewCatalogue(rows []Entry) *Catalogue {
	c := &Catalogue{overridden: map[string]bool{}}
	fixedByKey := map[string]int{}
	fixed := Fixed()
	for i := range fixed {
		fixedByKey[fixed[i].Key()] = i
	}
	ops := map[string]bool{}
	for _, f := range fixed {
		ops[f.OperatorType] = true
	}
	for _, r := range rows {
		r = normalize(r)
		if r.IsDefaultRow() {
			if r.ReportThreshold != nil {
				t := *r.ReportThreshold
				c.defaultThreshold = &t
			}
			continue
		}
		if r.OperatorType == DefaultOperator {
			continue
		}
		if i, ok := fixedByKey[r.Key()]; ok && r.Source == SourceJetStore {
			f := &fixed[i]
			if r.ReportThreshold != nil {
				t := *r.ReportThreshold
				f.ReportThreshold = &t
			}
			if r.Kind != "" {
				f.Kind = r.Kind
			}
			if r.Description != "" {
				f.Description = r.Description
			}
			c.overridden[f.Key()] = true
			continue
		}
		c.rows = append(c.rows, r)
		ops[r.OperatorType] = true
	}
	c.fixed = fixed
	for op := range ops {
		c.operators = append(c.operators, op)
	}
	sort.Strings(c.operators)
	// A deterministic order, so a tie between operators is broken the same way
	// every time: by operator, then prefix
	sort.SliceStable(c.rows, func(i, j int) bool {
		if c.rows[i].OperatorType != c.rows[j].OperatorType {
			return c.rows[i].OperatorType < c.rows[j].OperatorType
		}
		return c.rows[i].MatchPrefix < c.rows[j].MatchPrefix
	})
	return c
}

// normalize fills a row's defaults: the code from its prefix, the kind error.
func normalize(r Entry) Entry {
	r.OperatorType = strings.TrimSpace(r.OperatorType)
	if r.Code == "" {
		r.Code = DefaultCode(r.MatchPrefix)
	}
	if r.Kind == "" {
		r.Kind = KindError
	}
	return r
}

// DefaultRowThreshold is the workspace default row's threshold, if it has one.
func (c *Catalogue) DefaultRowThreshold() (int, bool) {
	if c.defaultThreshold == nil {
		return 0, false
	}
	return *c.defaultThreshold, true
}

// Match gives message the code of the entry that reaches furthest into it,
// operatorType being process_errors.operator_type ("" for null).
func (c *Catalogue) Match(operatorType, message string) Match {
	operatorType = strings.TrimSpace(operatorType)
	ops := []string{operatorType}
	inferred := operatorType
	if operatorType == "" {
		if op := InferOperator(message); op != "" {
			ops = []string{op}
			inferred = op
		} else {
			ops = c.operators
		}
	}
	var best Entry
	bestReach := -1
	for _, op := range ops {
		body, _ := StripPrefix(op, message)
		detail := -1
		if op == OperatorMapRecord {
			detail = mappingDetailOffset(body)
		}
		for _, f := range c.fixed {
			if f.OperatorType != op {
				continue
			}
			if n := reach(f.MatchPrefix, body, true); n > bestReach {
				best, bestReach = f, n
			}
		}
		for _, r := range c.rows {
			if r.OperatorType != op {
				continue
			}
			if n := reach(r.MatchPrefix, body, false); n > bestReach {
				best, bestReach = r, n
			}
			// A configured map expression message comes after the column
			// and position, so a row names it there
			if detail >= 0 {
				if n := reach(r.MatchPrefix, body[detail:], false); n >= 0 && detail+n > bestReach {
					best, bestReach = r, detail+n
				}
			}
		}
	}
	if bestReach < 0 {
		return Match{
			Entry: Entry{
				OperatorType: inferred,
				Code:         UncataloguedCode(inferred),
				Kind:         KindError,
			},
			OperatorType: inferred,
			Threshold:    DefaultThreshold,
			Uncatalogued: true,
		}
	}
	return Match{
		Entry:        best,
		OperatorType: best.OperatorType,
		Threshold:    c.threshold(best),
		Overridden:   c.overridden[best.Key()] && best.Source == SourceJetStore,
	}
}

// threshold resolves an entry's threshold (see the package comment).
func (c *Catalogue) threshold(e Entry) int {
	if e.ReportThreshold != nil {
		return *e.ReportThreshold
	}
	if e.Source != SourceJetStore && c.defaultThreshold != nil {
		return *c.defaultThreshold
	}
	return DefaultThreshold
}

// MatchMessage is NewCatalogue(rows).Match(operatorType, message), for a single
// message. A reader of many messages builds the Catalogue once.
func MatchMessage(rows []Entry, operatorType, message string) Match {
	return NewCatalogue(rows).Match(operatorType, message)
}

// Columns of jetsapi.error_code_catalogue, in the order SelectCatalogueSQL
// reads them and ReadCatalogue scans them.
var Columns = []string{
	"operator_type", "match_prefix", "code", "kind", "report_threshold",
	"source", "source_ref", "description",
}

// TableName is the catalogue's table.
const TableName = "jetsapi.error_code_catalogue"

// SelectCatalogueSQL reads every row of the catalogue, for ReadCatalogue.
const SelectCatalogueSQL = `SELECT operator_type, match_prefix, code, kind, report_threshold,
       source, source_ref, description
  FROM jetsapi.error_code_catalogue
 ORDER BY operator_type, match_prefix`

// Rows is what ReadCatalogue scans: pgx.Rows and *sql.Rows both are one.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

// ReadCatalogue scans the rows of SelectCatalogueSQL. It does not close rows.
func ReadCatalogue(rows Rows) ([]Entry, error) {
	var out []Entry
	for rows.Next() {
		var op, prefix, code, kind, source, ref, desc sql.NullString
		var threshold sql.NullInt64
		if err := rows.Scan(&op, &prefix, &code, &kind, &threshold, &source, &ref, &desc); err != nil {
			return nil, err
		}
		e := Entry{
			OperatorType: op.String,
			MatchPrefix:  prefix.String,
			Code:         code.String,
			Kind:         kind.String,
			Source:       source.String,
			SourceRef:    ref.String,
			Description:  desc.String,
		}
		if threshold.Valid {
			t := int(threshold.Int64)
			e.ReportThreshold = &t
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// IsMissingTable says whether err is PostgreSQL's undefined_table (42P01), as a
// read of the catalogue returns on a database not yet migrated to it. A reader
// treats the catalogue as empty then: every workspace message reads
// uncatalogued, and the reader says so once.
func IsMissingTable(err error) bool {
	var state interface{ SQLState() string }
	return errors.As(err, &state) && state.SQLState() == "42P01"
}
