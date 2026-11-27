package compute_pipes

// The `claim_set_summary` operator: it summarises a delivery's mapped claim
// records into the row jetsapi.claim_set_summary holds for that delivery, the
// row the feed detection writes from the claim-set table, so a pipeline that
// does not load its claims into the database keeps the claim-set checks.
//
// # What it reads, and why from the feed's profile
//
// The columns are the ones the feed's profile names (claim_set.key_columns,
// service_date_column and member_column), read from source_config when the
// operator is built, and the operator has no configuration of its own. That is
// a decision, for two reasons. The feed detection keeps a stored summary only
// while it was computed under the profile's claim set, lag and span as they
// are now; a summary computed under any other declaration is stale the day it
// is written, and with the rows not loaded the checks it cannot answer are not
// judged. And a second declaration of the same columns, in the pipeline, is
// one that can disagree with the first. So the operator reads the declaration
// the checks are judged against, and a change to the profile reaches the next
// run without a change to the pipeline.
//
// The feed and the delivery are the run's own (claimset.Resolve): the main
// input's registration names the feed (input_registry.table_name) and is the
// delivery when it is the first registration of its file, as the feed
// detection reads deliveries. The row is written for that delivery, under this
// run's session, and is deleted with it. The summary is computed by package
// claimset, which holds it to the row the feed detection computes from the
// table.
//
// # What it summarises, and when
//
// The records it reads, which are the rows the table would hold only if the
// operator reads what the load would have written: after the mapping, and
// after any filter that would have kept a record out of the table. It writes
// the row when its step has read every record. A step that fails writes none;
// a later step that fails leaves the row in place, and a run of the same
// delivery again replaces it. The feed detection writes a summary of an
// unfinished load only after the load completes; the operator's summary is
// never partial, since it is written once every record has been read, so it
// does not wait for the run to complete.
//
// # One node sees the whole delivery
//
// A summary's exact counts (the rows, the distinct keys, the distinct members)
// cannot be added across partitions, so the operator is valid only in a step
// that runs on one node: the sharding step when it is not sharded, or a
// reducing step whose input is one partition. Both starters refuse any other
// step before a node runs (validateClaimSetSummaryNodes), as merge_files is
// refused, and ValidatePipeSpecConfig refuses the operator under a splitter,
// which builds one per split value.
//
// # It never fails the pipeline for the data
//
// Records pass through to the output channel unchanged; the output channel
// must carry the input's columns. Where the operator cannot write a summary
// that is the feed detection's own (no usable profile, a declared column the
// channel lacks, a run that re-processes a delivery, a value it cannot render
// as the database would, the summary table not deployed), it writes none and
// logs why, and the feed detection judges the delivery as it would without
// the operator. A database error is an error.
//
// # The text of a value
//
// The key and member columns are compared as text, so a value is rendered as
// PostgreSQL renders the column it would be loaded into (claimSetSummaryText):
// text as it is, integers in decimal, a double as float8 prints it, a date as
// YYYY-MM-DD, a time with a time of day as a timestamp. A date of service is a
// date, or an ISO date as text.

import (
	"context"
	"fmt"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/claimset"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ClaimSetSummaryOperatorType is the operator's type as the configuration
// spells it.
const ClaimSetSummaryOperatorType = "claim_set_summary"

// ClaimSetSummaryTransformationPipe summarises the records it forwards.
type ClaimSetSummaryTransformationPipe struct {
	source   *InputChannel
	outputCh *OutputChannel
	dbpool   *pgxpool.Pool
	doneCh   chan struct{}
	target   *claimset.Target
	// skip is why no summary will be written, "" while one will.
	skip       string
	summariser *claimset.Summariser
	keyPos     []int
	memberPos  int
	datePos    int
	nbrColumns int
	// key and present are reused from record to record: the summariser reads
	// them and keeps neither.
	key     []*string
	present []bool
}

// Apply forwards the record and counts it.
func (ctx *ClaimSetSummaryTransformationPipe) Apply(input *[]any) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in ClaimSetSummaryTransformationPipe")
	}
	if ctx.skip == "" {
		if err := ctx.add(*input); err != nil {
			ctx.stop(err.Error())
		}
	}
	select {
	case ctx.outputCh.Channel <- *input:
	case <-ctx.doneCh:
		log.Printf("ClaimSetSummaryTransformationPipe writing to '%s' interrupted", ctx.outputCh.Name)
	}
	return nil
}

// stop gives up the summary, and the memory it held.
func (ctx *ClaimSetSummaryTransformationPipe) stop(why string) {
	ctx.skip = why
	ctx.summariser = nil
}

func (ctx *ClaimSetSummaryTransformationPipe) add(record []any) error {
	at := func(i int) any {
		if i < len(record) {
			return record[i]
		}
		return nil
	}
	cs := ctx.target.Feed.Profile.ClaimSet
	r := claimset.Record{Key: ctx.key[:0], Present: ctx.present}
	for i, p := range ctx.keyPos {
		text, err := claimSetSummaryText(at(p))
		if err != nil {
			return fmt.Errorf("the key column '%s' %v", cs.KeyColumns[i], err)
		}
		r.Key = append(r.Key, text)
	}
	member, err := claimSetSummaryText(at(ctx.memberPos))
	if err != nil {
		return fmt.Errorf("the member column '%s' %v", cs.MemberColumn, err)
	}
	r.Member = member
	if r.ServiceDate, err = claimSetSummaryDate(at(ctx.datePos)); err != nil {
		return fmt.Errorf("the date of service column '%s' %v", cs.ServiceDateColumn, err)
	}
	for i := range r.Present {
		r.Present[i] = claimSetSummaryPresent(at(i))
	}
	ctx.summariser.Add(r)
	return nil
}

// Done writes the summary, unless the run was interrupted, which would leave
// it partial.
func (ctx *ClaimSetSummaryTransformationPipe) Done() error {
	select {
	case <-ctx.doneCh:
		log.Println("claim_set_summary: the run was interrupted, no summary is written")
		return nil
	default:
	}
	if ctx.skip != "" {
		log.Printf("claim_set_summary: no summary is written: %s", ctx.skip)
		return nil
	}
	t := ctx.target
	s := ctx.summariser.Summary(t.DeliverySession, t.Session, &t.Arrival, time.Now().UTC())
	if s == nil {
		log.Printf("claim_set_summary: no record was read, no summary of delivery %s is written", t.DeliverySession)
		return nil
	}
	if err := claimset.Write(context.Background(), ctx.dbpool, s); err != nil {
		return fmt.Errorf("claim_set_summary: %v", err)
	}
	log.Printf("claim_set_summary: wrote the summary of delivery %s of feed %s: %d rows, %d distinct keys",
		t.DeliverySession, t.Feed.TableName, s.Rows, s.DistinctKeys)
	return nil
}

func (ctx *ClaimSetSummaryTransformationPipe) Finally() {}

// NewClaimSetSummaryTransformationPipe builds the operator: it resolves the
// feed, the delivery and the columns from the database.
func (ctx *BuilderContext) NewClaimSetSummaryTransformationPipe(source *InputChannel, outputCh *OutputChannel,
	spec *TransformationSpec) (*ClaimSetSummaryTransformationPipe, error) {
	if outputCh == nil {
		return nil, fmt.Errorf("configuration error: claim_set_summary operator has no output channel")
	}
	if source.Config == nil || outputCh.Config == nil {
		return nil, fmt.Errorf("error: claim_set_summary operator: input or output channel has no channel spec")
	}
	// The records pass through unchanged, so the two channels carry the same
	// columns, as for the operators that augment a record in place.
	if err := validateInferChannels(source, outputCh, "claim_set_summary operator"); err != nil {
		return nil, err
	}
	pipe := &ClaimSetSummaryTransformationPipe{source: source, outputCh: outputCh, dbpool: ctx.dbpool,
		doneCh: ctx.done}
	if ctx.dbpool == nil {
		pipe.stop("there is no database connection")
		return pipe, nil
	}
	target, why, err := claimset.Resolve(context.Background(), ctx.dbpool, ctx.peKey)
	if err != nil {
		return nil, fmt.Errorf("claim_set_summary operator: %v", err)
	}
	if why != "" {
		pipe.stop(why)
		log.Printf("claim_set_summary: no summary will be written: %s", why)
		return pipe, nil
	}
	pipe.target = target
	columns := claimSetSummaryColumns(*source.Columns)
	pos := func(c string) int {
		if p, ok := (*source.Columns)[c]; ok {
			return p
		}
		return -1
	}
	cs := target.Feed.Profile.ClaimSet
	var absent []string
	for _, c := range append(slices.Clone(cs.KeyColumns), cs.MemberColumn, cs.ServiceDateColumn) {
		if pos(c) < 0 && !slices.Contains(absent, c) {
			absent = append(absent, c)
		}
	}
	for _, c := range cs.KeyColumns {
		pipe.keyPos = append(pipe.keyPos, pos(c))
	}
	pipe.memberPos, pipe.datePos = pos(cs.MemberColumn), pos(cs.ServiceDateColumn)
	if len(absent) > 0 {
		pipe.stop(fmt.Sprintf("the profile of feed %s names the column(s) %s, which the input channel '%s' does not have",
			target.Feed.TableName, strings.Join(absent, ", "), source.Name))
		log.Printf("claim_set_summary: no summary will be written: %s", pipe.skip)
		return pipe, nil
	}
	pipe.nbrColumns = len(columns)
	pipe.key = make([]*string, 0, len(cs.KeyColumns))
	pipe.present = make([]bool, len(columns))
	pipe.summariser = claimset.NewSummariser(target.Feed, columns, claimset.DefaultMinMembers)
	log.Printf("claim_set_summary: summarising delivery %s of feed %s, read under session %s",
		target.DeliverySession, target.Feed.TableName, target.Session)
	return pipe, nil
}

// claimSetSummaryColumns are a channel's columns in position order.
func claimSetSummaryColumns(columns map[string]int) []string {
	out := make([]string, len(columns))
	for c, i := range columns {
		if i >= 0 && i < len(out) {
			out[i] = c
		}
	}
	return out
}

// validateClaimSetSummaryPipes refuses the operator under a splitter, which
// builds one operator per split value, so each would summarise part of the
// delivery. ValidatePipeSpecConfig calls it; how many nodes the step runs on
// is not known there, and the starters check it (validateClaimSetSummaryNodes).
func validateClaimSetSummaryPipes(pipeConfig []PipeSpec) error {
	for i := range pipeConfig {
		if pipeConfig[i].Type != "splitter" {
			continue
		}
		for j := range pipeConfig[i].Apply {
			if pipeConfig[i].Apply[j].Type == ClaimSetSummaryOperatorType {
				return fmt.Errorf("configuration error: the claim_set_summary operator summarises a whole " +
					"delivery and cannot run under a splitter, which builds one per split value; use a fan_out")
			}
		}
	}
	return nil
}

// claimSetSummaryDocumentFindings warns, when a pipeline configuration is
// saved, of each pipe that carries a claim_set_summary operator under a
// splitter: the start of its step refuses it (validateClaimSetSummaryPipes).
func claimSetSummaryDocumentFindings(cfg *ComputePipesConfig) []wsvalidate.Finding {
	var out []wsvalidate.Finding
	check := func(pipes []PipeSpec, path string) {
		for i := range pipes {
			if err := validateClaimSetSummaryPipes(pipes[i : i+1]); err != nil {
				out = append(out, wsvalidate.Finding{Severity: wsvalidate.Warning,
					Code: "cpipes_config_claim_set_summary_splitter", Message: "the step will not start: " + err.Error(),
					Path: fmt.Sprintf("%s/%d", path, i)})
			}
		}
	}
	check(cfg.PipesConfig, "/pipes_config")
	for i := range cfg.ConditionalPipesConfig {
		check(cfg.ConditionalPipesConfig[i].PipesConfig, fmt.Sprintf("/conditional_pipes_config/%d/pipes_config", i))
	}
	return out
}

// validateClaimSetSummaryNodes refuses a step carrying a claim_set_summary
// operator that would run on more than one node: each node would summarise
// part of the delivery, and the last to finish would write its part as the
// whole. It is called by both starters once the step's node count is known.
func validateClaimSetSummaryNodes(pipeConfig []PipeSpec, nbrNodes int, stepName string) error {
	if nbrNodes <= 1 {
		return nil
	}
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			if pipeConfig[i].Apply[j].Type == ClaimSetSummaryOperatorType {
				return fmt.Errorf("configuration error: step '%s' carries a claim_set_summary operator, which "+
					"summarises a whole delivery and so requires a step that runs on one node; this step has %d",
					stepName, nbrNodes)
			}
		}
	}
	return nil
}

// claimSetSummaryText is a value's text as PostgreSQL casts the column it is
// loaded into to text, or nil for a null.
func claimSetSummaryText(v any) (*string, error) {
	var s string
	switch x := v.(type) {
	case nil:
		return nil, nil
	case string:
		s = x
	case int:
		s = strconv.FormatInt(int64(x), 10)
	case int8:
		s = strconv.FormatInt(int64(x), 10)
	case int16:
		s = strconv.FormatInt(int64(x), 10)
	case int32:
		s = strconv.FormatInt(int64(x), 10)
	case int64:
		s = strconv.FormatInt(x, 10)
	case uint:
		s = strconv.FormatUint(uint64(x), 10)
	case uint8:
		s = strconv.FormatUint(uint64(x), 10)
	case uint16:
		s = strconv.FormatUint(uint64(x), 10)
	case uint32:
		s = strconv.FormatUint(uint64(x), 10)
	case uint64:
		s = strconv.FormatUint(x, 10)
	case float64:
		s = claimSetSummaryFloat(x)
	case bool:
		s = strconv.FormatBool(x)
	case time.Time:
		if x.Hour() == 0 && x.Minute() == 0 && x.Second() == 0 && x.Nanosecond() == 0 {
			s = x.Format(time.DateOnly)
		} else {
			s = x.Format("2006-01-02 15:04:05.999999")
		}
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			t, err := claimSetSummaryText(e)
			if err != nil {
				return nil, err
			}
			parts[i] = claimSetSummaryArrayElement(t, e)
		}
		s = "{" + strings.Join(parts, ",") + "}"
	default:
		return nil, fmt.Errorf("carries a %T, which has no text the database would give it", v)
	}
	return &s, nil
}

// claimSetSummaryFloat is a double as float8 prints it: the shortest text that
// reads back to the same value, in positional notation for a decimal exponent
// from -4 to 14 and in exponent notation otherwise, the exponent of at least
// two digits.
func claimSetSummaryFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	e := strconv.FormatFloat(f, 'e', -1, 64)
	exp, _ := strconv.Atoi(e[strings.IndexByte(e, 'e')+1:])
	if f != 0 && (exp < -4 || exp >= 15) {
		return e
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// claimSetSummaryArrayElement is one element of an array's text, quoted as
// array_out quotes it.
func claimSetSummaryArrayElement(t *string, v any) string {
	if t == nil {
		return "NULL"
	}
	s := *t
	_, isText := v.(string)
	needs := s == "" || (isText && strings.EqualFold(s, "NULL")) || strings.ContainsAny(s, "{}\",\\ \t\n\r\v\f")
	if !needs {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}

// claimSetSummaryDate is a date of service, at midnight UTC, or nil.
func claimSetSummaryDate(v any) (*time.Time, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case time.Time:
		d := time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, time.UTC)
		return &d, nil
	case string:
		s := strings.TrimSpace(x)
		if len(s) >= 10 && (len(s) == 10 || s[10] == ' ' || s[10] == 'T') {
			if d, err := time.Parse(time.DateOnly, s[:10]); err == nil {
				return &d, nil
			}
		}
		return nil, fmt.Errorf("carries the text %q, which is not an ISO date", x)
	default:
		return nil, fmt.Errorf("carries a %T, which is not a date", v)
	}
}

// claimSetSummaryPresent says whether a value is present as the summary
// counts it: not null and, as text, not blank once spaces are trimmed. Only
// text can be blank.
func claimSetSummaryPresent(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return strings.Trim(x, " ") != ""
	default:
		return true
	}
}
