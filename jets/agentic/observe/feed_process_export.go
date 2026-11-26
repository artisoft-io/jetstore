package observe

// The export, and judging what it returns.
//
// A retrospective runs at a client whose database the process cannot reach,
// through a query tool that runs one statement at a time, returns nothing for
// a statement containing a semicolon anywhere, and is easiest to read back
// from when a statement returns one row with one text column. So the process
// is run against FeedProcessReplay, a database that answers each statement
// from results pasted back from the client, and the statements it could not
// answer are printed for the client to run.
//
// # One SQL text
//
// What the export prints is what the detectors issue: each statement the
// process sends to the replay, with its arguments substituted as literals by
// FeedProcessRender, the one function that does so. Judging the pasted results
// is the same process run again against the same replay, now able to answer,
// so the verdicts come from the detectors' own Go judgement over the client's
// own aggregates, and a retrospective measures the detectors rather than a
// second implementation of them.
//
// A statement's arguments can depend on an earlier statement's result -- the
// content checks' statement for a delivery names the delivery's session and
// its predecessor's, which the feed's registrations give -- so an export runs
// in rounds: the source configuration, then the registrations and the
// claim-set tables' columns, then one summary statement per delivery and the
// stored summaries, then, for each delivery judged, the run that read the
// session its summary names. Each round is
// what the process asked for and could not be answered; an export with
// nothing left to ask is complete.
//
// # What a result line is
//
// Each statement is wrapped so that it returns one row and one text column, a
// JSON object on one line: the statement's identifier, a hash of its rendered
// text, and its rows, each row an object whose values are read by position.
// The statements of a round are combined into as few statements as a size
// bound allows, each returning one text cell holding one line per statement.
// A result line names the statement it answers, so lines may be pasted in any
// order and a line for a statement that has since changed is ignored rather
// than misread.
//
// A wrapped statement aggregates its rows in the order its own ORDER BY gives
// them: PostgreSQL does not flatten a subquery that sorts, and json_agg has no
// parallel form, so the aggregate reads the subquery's output in order.

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// FeedProcessRender substitutes a statement's arguments as SQL literals and
// puts it on one line. It is the only place an argument becomes text.
//
// A string literal carrying a semicolon or a line break is written with chr()
// for that character, so no literal puts a semicolon in a statement. A quoted
// identifier is the statement's own and is copied as it is: a name with a
// semicolon in it stays in the statement, and the export refuses it.
func FeedProcessRender(sql string, args []any) (string, error) {
	lits := make([]string, len(args))
	for i, a := range args {
		l, err := feedProcessLiteral(a)
		if err != nil {
			return "", fmt.Errorf("argument $%d: %w", i+1, err)
		}
		lits[i] = l
	}
	var b strings.Builder
	used := make([]bool, len(args))
	space := false
	for i := 0; i < len(sql); {
		c := sql[i]
		switch {
		case c == '\'' || c == '"':
			// A string literal or a quoted identifier, copied whole; a doubled
			// quote is an escaped one.
			j := i + 1
			for j < len(sql) {
				if sql[j] == c {
					if j+1 < len(sql) && sql[j+1] == c {
						j += 2
						continue
					}
					break
				}
				j++
			}
			if j >= len(sql) {
				return "", fmt.Errorf("the statement has an unterminated %c", c)
			}
			if space {
				b.WriteByte(' ')
				space = false
			}
			b.WriteString(sql[i : j+1])
			i = j + 1
		case c == '$' && i+1 < len(sql) && sql[i+1] >= '0' && sql[i+1] <= '9':
			j := i + 1
			for j < len(sql) && sql[j] >= '0' && sql[j] <= '9' {
				j++
			}
			n, _ := strconv.Atoi(sql[i+1 : j])
			if n < 1 || n > len(args) {
				return "", fmt.Errorf("the statement names $%d and has %d argument(s)", n, len(args))
			}
			if space {
				b.WriteByte(' ')
				space = false
			}
			b.WriteString(lits[n-1])
			used[n-1] = true
			i = j
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			space = b.Len() > 0
			i++
		case c == '-' && i+1 < len(sql) && sql[i+1] == '-':
			return "", errors.New("the statement carries a line comment, which one line cannot hold")
		default:
			if space {
				b.WriteByte(' ')
				space = false
			}
			b.WriteByte(c)
			i++
		}
	}
	for i, u := range used {
		if !u {
			return "", fmt.Errorf("argument $%d is not used by the statement", i+1)
		}
	}
	return b.String(), nil
}

// feedProcessLiteral is one argument as a SQL literal.
func feedProcessLiteral(a any) (string, error) {
	if a == nil {
		return "NULL", nil
	}
	v := reflect.ValueOf(a)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "NULL", nil
		}
		return feedProcessLiteral(v.Elem().Interface())
	}
	switch x := a.(type) {
	case string:
		return feedProcessString(x), nil
	case bool:
		return strconv.FormatBool(x), nil
	case int:
		return strconv.Itoa(x), nil
	case int32:
		return strconv.FormatInt(int64(x), 10), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case []string:
		parts := make([]string, len(x))
		for i, s := range x {
			parts[i] = feedProcessString(s)
		}
		return "ARRAY[" + strings.Join(parts, ", ") + "]::text[]", nil
	}
	return "", fmt.Errorf("a %T cannot be written as a literal", a)
}

// feedProcessString is a string literal. The characters a one-line statement
// with no semicolon cannot carry are written with chr().
func feedProcessString(s string) string {
	if !strings.ContainsAny(s, ";\n\r") {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	var parts []string
	var cur strings.Builder
	flush := func() {
		parts = append(parts, "'"+strings.ReplaceAll(cur.String(), "'", "''")+"'")
		cur.Reset()
	}
	for _, r := range s {
		switch r {
		case ';', '\n', '\r':
			flush()
			parts = append(parts, fmt.Sprintf("chr(%d)", r))
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return "(" + strings.Join(parts, " || ") + ")"
}

// FeedProcessStatement is one statement the process issued, as the export
// prints it.
type FeedProcessStatement struct {
	// ID is the first 16 hex digits of the SHA-256 of Text.
	ID string
	// Label says in words what the statement reads.
	Label string
	// Text is the statement with its arguments as literals, on one line.
	Text string
}

func newFeedProcessStatement(sql string, args []any) (FeedProcessStatement, error) {
	text, err := FeedProcessRender(sql, args)
	if err != nil {
		return FeedProcessStatement{}, fmt.Errorf("while writing a statement as text: %w", err)
	}
	sum := sha256.Sum256([]byte(text))
	return FeedProcessStatement{ID: hex.EncodeToString(sum[:8]), Label: feedProcessLabel(sql, args), Text: text}, nil
}

// Wrapped is the statement as the export prints it: one row, one text column,
// a JSON object naming the statement and holding its rows.
func (s FeedProcessStatement) Wrapped() string {
	return fmt.Sprintf(`SELECT replace(json_build_object('id', '%s', 'rows', COALESCE(json_agg(q), '[]'::json))::text, E'\n', ' ') FROM (%s) q`,
		s.ID, s.Text)
}

// feedProcessLabel names what a statement reads, for the export's comments.
func feedProcessLabel(sql string, args []any) string {
	arg := func(i int) string {
		if i < len(args) {
			if l, err := feedProcessLiteral(args[i]); err == nil {
				return l
			}
		}
		return "?"
	}
	switch {
	case sql == feedProcessClientsSQL:
		return "the clients with a source configuration"
	case sql == feedDeliverySQL:
		return "the file registrations of the feeds, from which both families read their deliveries"
	case sql == claimSetColumnsSQL:
		return "the columns of claim-set table " + arg(0) + "." + arg(1)
	case strings.Contains(sql, "claimset_key"):
		return "the claim-set summary of session " + arg(0)
	case sql == ClaimSetRunStatement():
		return "the run that read session " + arg(0) + ", its retain window and spans"
	case sql == claimSetSummaryTableSQL:
		return "whether the claim-set summary table exists"
	case sql == claimSetStoredSummariesSQL:
		return "the stored claim-set summaries of the feeds"
	case sql == claimSetSketchesSQL:
		return "the stored key sketches of feed " + arg(0)
	case strings.Contains(sql, "jetsapi.source_config"):
		return "the feeds of client " + arg(0)
	}
	return "a statement"
}

// FeedProcessResults are the rows the client returned, by statement
// identifier, each row its values in column order.
type FeedProcessResults map[string][][]json.RawMessage

// ReadFeedProcessResults reads pasted result lines: each a JSON object with
// "id" and "rows", as a wrapped statement returns it. Blank lines, lines that
// are not such an object, and a second result for a statement already read are
// skipped, so a results file can be appended to round after round.
func ReadFeedProcessResults(r io.Reader) (FeedProcessResults, error) {
	out := FeedProcessResults{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<30)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		line = strings.TrimSuffix(strings.TrimPrefix(line, `"`), `"`)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var doc struct {
			ID   string            `json:"id"`
			Rows []json.RawMessage `json:"rows"`
		}
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			return nil, fmt.Errorf("result line %d is not a result: %w", n, err)
		}
		if doc.ID == "" {
			return nil, fmt.Errorf("result line %d names no statement", n)
		}
		if _, dup := out[doc.ID]; dup {
			continue
		}
		rows := make([][]json.RawMessage, len(doc.Rows))
		for i, raw := range doc.Rows {
			vals, err := feedProcessPositional(raw)
			if err != nil {
				return nil, fmt.Errorf("result line %d, row %d: %w", n, i+1, err)
			}
			rows[i] = vals
		}
		out[doc.ID] = rows
	}
	return out, sc.Err()
}

// feedProcessPositional reads a row object's values in order. Two columns may
// share a name, which a map would lose.
func feedProcessPositional(raw json.RawMessage) ([]json.RawMessage, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("a row is not an object")
	}
	var out []json.RawMessage
	for dec.More() {
		if _, err := dec.Token(); err != nil { // the column name
			return nil, err
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// FeedProcessReplay is a database that answers the process from results. A
// statement it has no result for is recorded as pending and answered with no
// rows, or with a row of zeros where one row is expected, so the process runs
// to the end and every statement it can ask for in this round is recorded.
type FeedProcessReplay struct {
	results FeedProcessResults
	pending []FeedProcessStatement
	seen    map[string]bool
	err     error
}

// NewFeedProcessReplay answers from results, which may be empty.
func NewFeedProcessReplay(results FeedProcessResults) *FeedProcessReplay {
	if results == nil {
		results = FeedProcessResults{}
	}
	return &FeedProcessReplay{results: results, seen: map[string]bool{}}
}

// Pending are the statements asked for and not answered, in the order asked.
func (r *FeedProcessReplay) Pending() []FeedProcessStatement { return r.pending }

func (r *FeedProcessReplay) lookup(sql string, args []any) ([][]json.RawMessage, bool) {
	s, err := newFeedProcessStatement(sql, args)
	if err != nil {
		r.err = errors.Join(r.err, err)
		return nil, false
	}
	rows, ok := r.results[s.ID]
	if !ok && !r.seen[s.ID] {
		r.seen[s.ID] = true
		r.pending = append(r.pending, s)
	}
	return rows, ok
}

// Query answers a statement from its result, or records it as pending.
func (r *FeedProcessReplay) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, _ := r.lookup(sql, args)
	return &feedProcessRows{rows: rows, at: -1}, nil
}

// QueryRow answers a one-row statement from its result, or records it as
// pending and answers with zeros.
func (r *FeedProcessReplay) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	rows, ok := r.lookup(sql, args)
	if !ok {
		return feedProcessZeroRow{}
	}
	if len(rows) == 0 {
		return feedProcessErrRow{pgx.ErrNoRows}
	}
	return feedProcessRow(rows[0])
}

type feedProcessErrRow struct{ err error }

func (e feedProcessErrRow) Scan(...any) error { return e.err }

type feedProcessZeroRow struct{}

func (feedProcessZeroRow) Scan(dest ...any) error {
	for _, d := range dest {
		v := reflect.ValueOf(d)
		if v.Kind() != reflect.Pointer || v.IsNil() {
			return fmt.Errorf("cannot scan into %T", d)
		}
		v.Elem().Set(reflect.Zero(v.Elem().Type()))
	}
	return nil
}

type feedProcessRow []json.RawMessage

func (row feedProcessRow) Scan(dest ...any) error {
	if len(dest) != len(row) {
		return fmt.Errorf("the result has %d column(s) and the statement reads %d", len(row), len(dest))
	}
	for i, d := range dest {
		v := reflect.ValueOf(d)
		if v.Kind() != reflect.Pointer || v.IsNil() {
			return fmt.Errorf("cannot scan into %T", d)
		}
		if err := feedProcessAssign(v.Elem(), row[i]); err != nil {
			return fmt.Errorf("column %d: %w", i+1, err)
		}
	}
	return nil
}

// feedProcessAssign sets v from one JSON value, as pgx would scan the column.
func feedProcessAssign(v reflect.Value, raw json.RawMessage) error {
	text := strings.TrimSpace(string(raw))
	if text == "null" {
		switch v.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Interface, reflect.Map:
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		return fmt.Errorf("cannot scan NULL into %s", v.Type())
	}
	switch v.Kind() {
	case reflect.Pointer:
		nv := reflect.New(v.Type().Elem())
		if err := feedProcessAssign(nv.Elem(), raw); err != nil {
			return err
		}
		v.Set(nv)
	case reflect.String:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			s = text // a number or a boolean read as text
		}
		v.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return fmt.Errorf("%s is not an integer", text)
		}
		v.SetInt(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return fmt.Errorf("%s is not a number", text)
		}
		v.SetFloat(f)
	case reflect.Bool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return fmt.Errorf("%s is not a boolean", text)
		}
		v.SetBool(b)
	case reflect.Slice:
		var elems []json.RawMessage
		if err := json.Unmarshal(raw, &elems); err != nil {
			return fmt.Errorf("%s is not an array", text)
		}
		s := reflect.MakeSlice(v.Type(), len(elems), len(elems))
		for i, e := range elems {
			if err := feedProcessAssign(s.Index(i), e); err != nil {
				return err
			}
		}
		v.Set(s)
	case reflect.Interface:
		var x any
		if err := json.Unmarshal(raw, &x); err != nil {
			return err
		}
		v.Set(reflect.ValueOf(x))
	default:
		return fmt.Errorf("cannot scan into %s", v.Type())
	}
	return nil
}

// feedProcessRows is a result as pgx.Rows.
type feedProcessRows struct {
	rows [][]json.RawMessage
	at   int
	err  error
}

func (r *feedProcessRows) Close()                                       {}
func (r *feedProcessRows) Err() error                                   { return r.err }
func (r *feedProcessRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *feedProcessRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *feedProcessRows) RawValues() [][]byte                          { return nil }
func (r *feedProcessRows) Conn() *pgx.Conn                              { return nil }
func (r *feedProcessRows) TypeMap() *pgtype.Map                         { return nil }

func (r *feedProcessRows) Next() bool {
	if r.err != nil || r.at+1 >= len(r.rows) {
		return false
	}
	r.at++
	return true
}

func (r *feedProcessRows) Scan(dest ...any) error {
	if r.at < 0 || r.at >= len(r.rows) {
		return errors.New("no current row")
	}
	if err := feedProcessRow(r.rows[r.at]).Scan(dest...); err != nil {
		r.err = err
		return err
	}
	return nil
}

func (r *feedProcessRows) Values() ([]any, error) {
	if r.at < 0 || r.at >= len(r.rows) {
		return nil, errors.New("no current row")
	}
	out := make([]any, len(r.rows[r.at]))
	for i, raw := range r.rows[r.at] {
		if err := json.Unmarshal(raw, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// FeedProcessExportRefusals names every profile name the export cannot carry:
// a claim-set table or column whose name contains a semicolon would put one in
// a statement, inside the quoted identifier, and the query tool returns
// nothing for such a statement.
func FeedProcessExportRefusals(feeds []feedprofile.Feed) []string {
	var out []string
	for _, f := range feeds {
		cs := f.Profile.ClaimSet
		if cs == nil {
			continue
		}
		check := func(field, name string) {
			if strings.Contains(name, ";") {
				out = append(out, fmt.Sprintf("feed %s: %s %q contains a semicolon", f.TableName, field, name))
			}
		}
		check("claim_set.table", cs.Table)
		for _, k := range cs.KeyColumns {
			check("claim_set.key_columns", k)
		}
		check("claim_set.service_date_column", cs.ServiceDateColumn)
		check("claim_set.member_column", cs.MemberColumn)
		for _, rf := range f.Profile.RequiredFields {
			check("required_fields.column", rf.Column)
		}
	}
	return out
}

// FeedProcessExport is one round of an export: the statements the process
// asked for and the results did not answer.
type FeedProcessExport struct {
	Statements []FeedProcessStatement
	// Answered counts the statements of the results the process read.
	Answered int
}

// Complete reports whether nothing is left to run.
func (e FeedProcessExport) Complete() bool { return len(e.Statements) == 0 }

// ExportFeedProcess runs the process against the results so far and returns
// what it still needs. clients and profiles are as for ReadFeedProcessFeeds.
func ExportFeedProcess(ctx context.Context, results FeedProcessResults, clients []string,
	profiles map[string]feedprofile.Profile, o FeedProcessOptions) (FeedProcessExport, error) {
	replay := NewFeedProcessReplay(results)
	feeds, err := ReadFeedProcessFeeds(ctx, replay, clients, profiles)
	if err != nil {
		return FeedProcessExport{}, err
	}
	if refused := FeedProcessExportRefusals(feeds.Profiled); len(refused) > 0 {
		return FeedProcessExport{}, fmt.Errorf("the export cannot print a statement that contains a semicolon, "+
			"because the query tool returns nothing for one, and these profile names would put one in a "+
			"statement:\n  %s\nRename the table or column, or leave the feed's profile out of the profiles file, "+
			"or run feed_detect against the database directly", strings.Join(refused, "\n  "))
	}
	if _, err := RunFeedProcess(ctx, replay, feeds, o); err != nil {
		return FeedProcessExport{}, err
	}
	if replay.err != nil {
		return FeedProcessExport{}, replay.err
	}
	out := FeedProcessExport{Statements: replay.Pending(), Answered: len(replay.seen) - len(replay.Pending())}
	for _, s := range out.Statements {
		if strings.Contains(s.Text, ";") {
			return FeedProcessExport{}, fmt.Errorf("the statement reading %s contains a semicolon, which the "+
				"query tool cannot run", s.Label)
		}
	}
	return out, nil
}

// JudgeFeedProcess runs the process against complete results: the verdicts
// the detectors reach on the client's aggregates. It fails, naming how many,
// when the results do not answer every statement the run asks.
func JudgeFeedProcess(ctx context.Context, results FeedProcessResults, clients []string,
	profiles map[string]feedprofile.Profile, o FeedProcessOptions) (FeedProcessResult, error) {
	replay := NewFeedProcessReplay(results)
	feeds, err := ReadFeedProcessFeeds(ctx, replay, clients, profiles)
	if err != nil {
		return FeedProcessResult{}, err
	}
	res, err := RunFeedProcess(ctx, replay, feeds, o)
	if err != nil {
		return res, err
	}
	if replay.err != nil {
		return res, replay.err
	}
	if n := len(replay.Pending()); n > 0 {
		return res, fmt.Errorf("the results answer all but %d statement(s) this run asks for; run the export "+
			"again with the same flags and these results, and run what it prints", n)
	}
	return res, nil
}

// Write prints the round's statements, combined into parts of at most
// maxBytes each, every part one statement on one line, preceded by comment
// lines that say what it reads and how to go on. resultsHint is the command
// line to give the results back to.
func (e FeedProcessExport) Write(w io.Writer, maxBytes int, resultsHint string) error {
	if e.Complete() {
		_, err := fmt.Fprintf(w, "-- The export is complete: %d statement(s) answered and nothing left to run.\n"+
			"-- Judge the results with:\n--   %s\n", e.Answered, strings.ReplaceAll(resultsHint, "-mode export", "-mode judge"))
		return err
	}
	parts := FeedProcessParts(e.Statements, maxBytes)
	fmt.Fprintf(w, "-- %d statement(s) to run, in %d part(s); %d already answered.\n", len(e.Statements), len(parts), e.Answered)
	fmt.Fprintf(w, "-- Run each part below in the query tool, one at a time. Each returns one row with one text column.\n")
	fmt.Fprintf(w, "-- Append every returned text, as it is, to the results file, then run:\n--   %s\n", resultsHint)
	fmt.Fprintf(w, "-- which prints the next round, or says the export is complete.\n")
	for i, p := range parts {
		fmt.Fprintf(w, "\n-- part %d of %d: %d statement(s)\n", i+1, len(parts), len(p.Statements))
		for _, s := range p.Statements {
			fmt.Fprintf(w, "--   %s: %s\n", s.ID, s.Label)
		}
		if _, err := fmt.Fprintln(w, p.Text); err != nil {
			return err
		}
	}
	return nil
}

// FeedProcessPart is statements combined into one: it returns one row with one
// text column, a line per statement.
type FeedProcessPart struct {
	Statements []FeedProcessStatement
	Text       string
}

// FeedProcessParts combines statements, in order, into parts of at most
// maxBytes each; a statement longer than that is a part of its own.
func FeedProcessParts(stmts []FeedProcessStatement, maxBytes int) []FeedProcessPart {
	const head = `SELECT string_agg(r, E'\n' ORDER BY n) FROM (VALUES `
	const tail = `) v(n, r)`
	var out []FeedProcessPart
	var cur FeedProcessPart
	var b strings.Builder
	flush := func() {
		if len(cur.Statements) == 0 {
			return
		}
		cur.Text = head + b.String() + tail
		out = append(out, cur)
		cur = FeedProcessPart{}
		b.Reset()
	}
	for _, s := range stmts {
		item := fmt.Sprintf("(%d, (%s))", len(cur.Statements)+1, s.Wrapped())
		if len(cur.Statements) > 0 && maxBytes > 0 && len(head)+b.Len()+2+len(item)+len(tail) > maxBytes {
			flush()
			item = fmt.Sprintf("(%d, (%s))", 1, s.Wrapped())
		}
		if len(cur.Statements) > 0 {
			b.WriteString(", ")
		}
		b.WriteString(item)
		cur.Statements = append(cur.Statements, s)
	}
	flush()
	return out
}

// FeedProcessVerdictLines are the verdicts as the judge prints it: detector,
// feed, ordinal, session, verdict and reason, tab separated.
func FeedProcessVerdictLines(r FeedProcessResult) []string {
	var out []string
	last := map[string]FeedDeliveryVerdict{}
	for i, v := range r.Delivery {
		det := r.DeliveryDetector[i]
		if det == feedprofile.FeedOverdue {
			last[v.TableName] = v // the verdict at the last step
			continue
		}
		out = append(out, strings.Join([]string{det, v.TableName, strconv.Itoa(v.Ordinal), v.SessionId,
			v.Verdict, v.Reason}, "\t"))
	}
	for _, t := range feedDeliverySortedKeys(feedProcessKeys(last)) {
		v := last[t]
		out = append(out, strings.Join([]string{feedprofile.FeedOverdue, v.TableName, strconv.Itoa(v.Ordinal),
			v.SessionId, v.Verdict, v.Reason}, "\t"))
	}
	for _, v := range r.ClaimSets {
		out = append(out, strings.Join([]string{v.Detector, v.TableName, strconv.Itoa(v.Ordinal), v.SessionID,
			v.Verdict, v.Reason}, "\t"))
	}
	return out
}

func feedProcessKeys(m map[string]FeedDeliveryVerdict) map[string]int {
	out := make(map[string]int, len(m))
	for k := range m {
		out[k] = 0
	}
	return out
}
