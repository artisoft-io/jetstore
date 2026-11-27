package compute_pipes

// The `claim_set_summary` operator: it summarises a delivery's mapped claim
// records into the row jetsapi.claim_set_summary holds for that delivery, the
// row the feed detection writes from the claim-set table, so a pipeline that
// does not load its claims into the database keeps the claim-set checks.
//
// # Two modes, two steps
//
// The operator exists for deliveries too large to load into the database, which
// run on many nodes, so it summarises in two steps (claim_set_summary_config's
// mode):
//
//   - partial, on every node of a step whose input is sharded by member: it
//     summarises the node's records and, once it has read the last, emits one
//     partial record (claimset.PartialColumns) on its output channel. It
//     forwards no record;
//   - merge, in a later step on one node: it reads every partial of the run
//     from its input channel, adds them (claimset.Partial.Add), applies the
//     summary's rules to the whole (claimset.Partial.Summary) and writes the
//     row. It has no output channel, and is the only node that writes.
//
// The partials reach the merge step through a partition_writer with a fixed
// jets_partition_key, so the merge step has one partition and one node. Both
// starters refuse a merge on more than one node (validateClaimSetSummaryNodes),
// and ValidatePipeSpecConfig refuses one under a splitter, which builds one per
// split value: either would write a part of the delivery as the whole.
//
// # The preconditions
//
// The partial mode runs on mapped claims, in the canonical model, in a step
// whose input is sharded by member, and each claim key belongs to one member.
// Every shard then holds disjoint members and disjoint keys, so every count is
// exact per shard and additive: the rows, the distinct keys and their
// fingerprint (a sum), the distinct members, the rows per date of service and
// the present counts; the sketches merge by register. The merge checks this
// rather than assuming it (claimset.Partial.Overlap): where the distinct keys
// summed over the shards exceed the merged key sketch's estimate of the union
// by more than claimset.SplitBound, keys were counted on more than one shard,
// and the same of the members with the members sketch. Then it writes nothing
// and logs which.
//
// The intended placement is beside the sort in the step that maps and sorts
// the claims: a second entry of the fan_out on the mapped channel, before the
// rules engine, so the summary measures what the source sent, not merged or
// augmented claims.
//
// # What it reads, and why from the feed's profile
//
// The columns are the ones the feed's profile names (claim_set.key_columns,
// service_date_column and member_column), and the operator declares none. That
// is a decision, for two reasons. The feed detection keeps a stored summary
// only while it was computed under the profile's claim set, lag and span as
// they are now; a summary computed under any other declaration is stale the
// day it is written. And a second declaration of the same columns, in the
// pipeline, is one that can disagree with the first.
//
// The feed and the delivery are the run's own (claimset.Resolve): the main
// input's registration names the feed (input_registry.table_name) and is the
// delivery by the feed detection's rule, a deleted first registration counted
// (purged_delivery). The start of the step resolves them once
// (resolveClaimSetSummaries) and passes them to every node in the operator's
// configuration (ClaimSetSummarySpec.Resolved), so no partial node reads the
// database. The row is written for that delivery, under this run's session,
// is deleted with it, and says it was computed by a pipeline (computed_by).
//
// # It never fails the pipeline for the data
//
// Where the operator cannot write a summary that is the feed detection's own
// (no usable profile, a declared column the channel lacks, a run that
// re-processes a delivery, a value it cannot render as the database would, a
// shard that gave up, shards that were not disjoint, the summary table not
// deployed), it writes none and logs why, and the feed detection judges the
// delivery as it would without the operator. A database error is an error, and
// so is a configuration the operator cannot run.
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
	"encoding/json"
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

// The operator's modes, claim_set_summary_config.mode.
const (
	ClaimSetSummaryPartial = "partial"
	ClaimSetSummaryMerge   = "merge"
)

// ClaimSetSummaryTransformationPipe summarises a shard's records (partial) or
// a run's partials (merge).
type ClaimSetSummaryTransformationPipe struct {
	mode     string
	source   *InputChannel
	outputCh *OutputChannel
	dbpool   *pgxpool.Pool
	doneCh   chan struct{}
	target   *claimset.Target
	// skip is why no summary will be written, "" while one will.
	skip string

	// The partial mode.
	summariser *claimset.Summariser
	keyPos     []int
	memberPos  int
	datePos    int
	// key and present are reused from record to record: the summariser reads
	// them and keeps neither.
	key     []*string
	present []bool

	// The merge mode.
	merged   *claimset.Partial
	partials int
}

// Apply counts the record: a claim in the partial mode, a partial in the merge
// mode.
func (ctx *ClaimSetSummaryTransformationPipe) Apply(input *[]any) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in ClaimSetSummaryTransformationPipe")
	}
	if ctx.skip != "" {
		return nil
	}
	var err error
	if ctx.mode == ClaimSetSummaryMerge {
		err = ctx.addPartial(*input)
	} else {
		err = ctx.add(*input)
	}
	if err != nil {
		ctx.stop(err.Error())
	}
	return nil
}

// stop gives up the summary, and the memory it held.
func (ctx *ClaimSetSummaryTransformationPipe) stop(why string) {
	ctx.skip = why
	ctx.summariser = nil
	ctx.merged = nil
}

func recordAt(record []any, i int) any {
	if i >= 0 && i < len(record) {
		return record[i]
	}
	return nil
}

func (ctx *ClaimSetSummaryTransformationPipe) add(record []any) error {
	cs := ctx.target.Feed.Profile.ClaimSet
	r := claimset.Record{Key: ctx.key[:0], Present: ctx.present}
	for i, p := range ctx.keyPos {
		text, err := claimSetSummaryText(recordAt(record, p))
		if err != nil {
			return fmt.Errorf("the key column '%s' %v", cs.KeyColumns[i], err)
		}
		r.Key = append(r.Key, text)
	}
	member, err := claimSetSummaryText(recordAt(record, ctx.memberPos))
	if err != nil {
		return fmt.Errorf("the member column '%s' %v", cs.MemberColumn, err)
	}
	r.Member = member
	if r.ServiceDate, err = claimSetSummaryDate(recordAt(record, ctx.datePos)); err != nil {
		return fmt.Errorf("the date of service column '%s' %v", cs.ServiceDateColumn, err)
	}
	for i := range r.Present {
		r.Present[i] = claimSetSummaryPresent(recordAt(record, i))
	}
	ctx.summariser.Add(r)
	return nil
}

func (ctx *ClaimSetSummaryTransformationPipe) addPartial(record []any) error {
	columns := *ctx.source.Columns
	p, err := claimset.PartialOf(func(c string) any { return recordAt(record, columns[c]) })
	if err != nil {
		return fmt.Errorf("partial %d does not read: %v", ctx.partials+1, err)
	}
	ctx.partials++
	ctx.merged.Add(p)
	return nil
}

// Done emits the node's partial, or writes the delivery's summary, unless the
// run was interrupted, which would leave it partial.
func (ctx *ClaimSetSummaryTransformationPipe) Done() error {
	select {
	case <-ctx.doneCh:
		log.Println("claim_set_summary: the run was interrupted, no summary is written")
		return nil
	default:
	}
	if ctx.mode == ClaimSetSummaryMerge {
		return ctx.write()
	}
	return ctx.emit()
}

// emit sends the node's partial: its counts, or why it has none.
func (ctx *ClaimSetSummaryTransformationPipe) emit() error {
	p := &claimset.Partial{Skip: ctx.skip}
	if ctx.skip == "" {
		p = ctx.summariser.Partial()
		log.Printf("claim_set_summary: the partial of this node of delivery %s: %d rows, %d distinct keys, "+
			"%d distinct members", ctx.target.DeliverySession, p.Rows, p.Keys.Count, p.Members)
	} else {
		log.Printf("claim_set_summary: this node's partial carries no summary: %s", ctx.skip)
	}
	values, err := p.Values()
	if err != nil {
		return fmt.Errorf("claim_set_summary: %v", err)
	}
	record := make([]any, len(*ctx.outputCh.Columns))
	for c, v := range values {
		record[(*ctx.outputCh.Columns)[c]] = v
	}
	select {
	case ctx.outputCh.Channel <- record:
	case <-ctx.doneCh:
		log.Printf("ClaimSetSummaryTransformationPipe writing to '%s' interrupted", ctx.outputCh.Name)
	}
	return nil
}

// write writes the delivery's summary from the partials, where they make one.
func (ctx *ClaimSetSummaryTransformationPipe) write() error {
	why := ctx.skip
	switch {
	case why != "":
	case ctx.partials == 0:
		why = "no partial was read"
	case ctx.merged.Skip != "":
		why = "a node gave up its partial: " + ctx.merged.Skip
	default:
		why = ctx.merged.Overlap()
	}
	if why != "" {
		log.Printf("claim_set_summary: no summary is written: %s", why)
		return nil
	}
	t := ctx.target
	s := ctx.merged.Summary(t.Feed, claimset.DefaultMinMembers, t.DeliverySession, t.Session, &t.Arrival,
		time.Now().UTC())
	if s == nil {
		log.Printf("claim_set_summary: no record was read, no summary of delivery %s is written", t.DeliverySession)
		return nil
	}
	if err := claimset.Write(context.Background(), ctx.dbpool, s); err != nil {
		return fmt.Errorf("claim_set_summary: %v", err)
	}
	log.Printf("claim_set_summary: wrote the summary of delivery %s of feed %s from %d partials: %d rows, "+
		"%d distinct keys, %d distinct members", t.DeliverySession, t.Feed.TableName, ctx.partials, s.Rows,
		s.DistinctKeys, s.Members)
	return nil
}

func (ctx *ClaimSetSummaryTransformationPipe) Finally() {}

// NewClaimSetSummaryTransformationPipe builds the operator from the target the
// start of the step resolved (ClaimSetSummarySpec.Resolved). It reads nothing
// from the database.
func (ctx *BuilderContext) NewClaimSetSummaryTransformationPipe(source *InputChannel, outputCh *OutputChannel,
	spec *TransformationSpec) (*ClaimSetSummaryTransformationPipe, error) {
	config := spec.ClaimSetSummaryConfig
	if config == nil {
		return nil, fmt.Errorf("configuration error: missing claim_set_summary_config for claim_set_summary operator")
	}
	if source.Config == nil {
		return nil, fmt.Errorf("error: claim_set_summary operator: the input channel has no channel spec")
	}
	pipe := &ClaimSetSummaryTransformationPipe{mode: config.Mode, source: source, outputCh: outputCh,
		dbpool: ctx.dbpool, doneCh: ctx.done}
	switch config.Mode {
	case ClaimSetSummaryPartial:
		if outputCh == nil || outputCh.Config == nil {
			return nil, fmt.Errorf("configuration error: the claim_set_summary operator in partial mode emits its " +
				"partial on its output channel, and has none")
		}
		if absent := claimSetSummaryAbsent(*outputCh.Columns); len(absent) > 0 {
			return nil, fmt.Errorf("configuration error: the output channel '%s' of the claim_set_summary operator "+
				"lacks the partial column(s) %s", outputCh.Name, strings.Join(absent, ", "))
		}
	case ClaimSetSummaryMerge:
		if outputCh != nil {
			return nil, fmt.Errorf("configuration error: the claim_set_summary operator in merge mode writes the " +
				"summary row and no record; it takes no output_channel")
		}
		if absent := claimSetSummaryAbsent(*source.Columns); len(absent) > 0 {
			return nil, fmt.Errorf("configuration error: the input channel '%s' of the claim_set_summary operator "+
				"in merge mode lacks the partial column(s) %s", source.Name, strings.Join(absent, ", "))
		}
	default:
		return nil, fmt.Errorf("configuration error: claim_set_summary_config.mode is '%s', not partial or merge",
			config.Mode)
	}

	var resolution claimset.Resolution
	if len(config.Resolved) == 0 {
		resolution.Skip = "the start of the step resolved no feed and no delivery for it"
	} else if err := json.Unmarshal(config.Resolved, &resolution); err != nil {
		return nil, fmt.Errorf("claim_set_summary operator: the resolved target does not read: %v", err)
	}
	if resolution.Skip == "" && resolution.Target == nil {
		resolution.Skip = "the start of the step resolved no feed and no delivery for it"
	}
	if resolution.Skip != "" {
		pipe.stop(resolution.Skip)
		log.Printf("claim_set_summary: no summary will be written: %s", pipe.skip)
		return pipe, nil
	}
	target := resolution.Target
	pipe.target = target
	cs := target.Feed.Profile.ClaimSet
	if cs == nil {
		pipe.stop(fmt.Sprintf("feed %s has no usable profile declaring a claim_set", target.Feed.TableName))
		return pipe, nil
	}

	if config.Mode == ClaimSetSummaryMerge {
		if ctx.dbpool == nil {
			pipe.stop("there is no database connection")
			log.Printf("claim_set_summary: no summary will be written: %s", pipe.skip)
			return pipe, nil
		}
		pipe.merged = &claimset.Partial{Days: map[int64]int64{}, Present: map[string]int64{}}
		log.Printf("claim_set_summary: merging the partials of delivery %s of feed %s, read under session %s",
			target.DeliverySession, target.Feed.TableName, target.Session)
		return pipe, nil
	}

	columns := claimSetSummaryColumns(*source.Columns)
	pos := func(c string) int {
		if p, ok := (*source.Columns)[c]; ok {
			return p
		}
		return -1
	}
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
	pipe.key = make([]*string, 0, len(cs.KeyColumns))
	pipe.present = make([]bool, len(columns))
	pipe.summariser = claimset.NewSummariser(target.Feed, columns, claimset.DefaultMinMembers)
	log.Printf("claim_set_summary: summarising this node's records of delivery %s of feed %s, read under session %s",
		target.DeliverySession, target.Feed.TableName, target.Session)
	return pipe, nil
}

// claimSetSummaryOutputChannels is the output channel of an operator, as the
// channel registry registers it: none for a claim_set_summary in merge mode,
// which writes the summary row and no record, and the operator's own for any
// other.
func claimSetSummaryOutputChannels(spec *TransformationSpec) []*OutputChannelConfig {
	if spec.Type == ClaimSetSummaryOperatorType && spec.OutputChannel.Name == "" {
		return nil
	}
	return []*OutputChannelConfig{&spec.OutputChannel}
}

// claimSetSummaryAbsent are the partial columns a channel's columns lack.
func claimSetSummaryAbsent(columns map[string]int) []string {
	var absent []string
	for _, c := range claimset.PartialColumns {
		if _, ok := columns[c]; !ok {
			absent = append(absent, c)
		}
	}
	return absent
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

// resolveClaimSetSummaries resolves, once for the step, what the run's
// summary is for, and gives it to every claim_set_summary operator of the step
// in its configuration (ClaimSetSummarySpec.Resolved), which reaches the nodes
// with the rest of the step's configuration. Both starters call it; a step
// without the operator reads nothing. It returns an error only when the
// database fails.
func resolveClaimSetSummaries(ctx context.Context, dbpool *pgxpool.Pool, peKey int, pipeConfig []PipeSpec) error {
	var resolved json.RawMessage
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			spec := &pipeConfig[i].Apply[j]
			if spec.Type != ClaimSetSummaryOperatorType || spec.ClaimSetSummaryConfig == nil {
				continue
			}
			if resolved == nil {
				var r claimset.Resolution
				if dbpool == nil {
					r.Skip = "there is no database connection at the start of the step"
				} else {
					target, why, err := claimset.Resolve(ctx, dbpool, peKey)
					if err != nil {
						return fmt.Errorf("claim_set_summary operator: %v", err)
					}
					r.Target, r.Skip = target, why
				}
				if r.Skip != "" {
					log.Printf("claim_set_summary: no summary will be written: %s", r.Skip)
				}
				var err error
				if resolved, err = json.Marshal(r); err != nil {
					return err
				}
			}
			config := *spec.ClaimSetSummaryConfig
			config.Resolved = resolved
			spec.ClaimSetSummaryConfig = &config
		}
	}
	return nil
}

// validateClaimSetSummaryOperator is ValidatePipeSpecConfig's check of a
// claim_set_summary operator (validateClaimSetSummarySpec); it checks nothing
// of another operator. A merge writes the summary row and no record, so it
// clears the output channel the caller validates next.
func validateClaimSetSummaryOperator(cpConfig *ComputePipesConfig, pipe *PipeSpec, spec *TransformationSpec,
	outputChConfig **OutputChannelConfig) error {
	if spec.Type != ClaimSetSummaryOperatorType {
		return nil
	}
	if err := validateClaimSetSummarySpec(cpConfig, pipe, spec); err != nil {
		return err
	}
	if spec.ClaimSetSummaryConfig.Mode == ClaimSetSummaryMerge {
		*outputChConfig = nil
	}
	return nil
}

// validateClaimSetSummarySpec checks one claim_set_summary operator of a pipe
// against what is answerable without the channels' records: its mode, that a
// partial has an output channel carrying the partial columns, and that a merge
// has none and is not under a splitter, which builds one per split value.
// ValidatePipeSpecConfig calls it, and the .pc.json save warns of what it
// refuses (claimSetSummaryDocumentFindings).
func validateClaimSetSummarySpec(cpConfig *ComputePipesConfig, pipe *PipeSpec, spec *TransformationSpec) error {
	config := spec.ClaimSetSummaryConfig
	if config == nil {
		return fmt.Errorf("configuration error: missing claim_set_summary_config for claim_set_summary operator")
	}
	switch config.Mode {
	case ClaimSetSummaryPartial:
		if spec.OutputChannel.Name == "" {
			return fmt.Errorf("configuration error: the claim_set_summary operator in partial mode emits its partial " +
				"on its output_channel, which it lacks")
		}
		if chSpec := cpConfig.GetChannelSpec(spec.OutputChannel.SpecName); chSpec != nil {
			if absent := claimSetSummaryAbsentColumns(chSpec.Columns); len(absent) > 0 {
				return fmt.Errorf("configuration error: the channel spec '%s' of the claim_set_summary operator's "+
					"output channel lacks the partial column(s) %s", chSpec.Name, strings.Join(absent, ", "))
			}
		}
	case ClaimSetSummaryMerge:
		if spec.OutputChannel.Name != "" || spec.OutputChannel.SpecName != "" {
			return fmt.Errorf("configuration error: the claim_set_summary operator in merge mode writes the " +
				"summary row and no record; it takes no output_channel")
		}
		if pipe.Type == "splitter" {
			return fmt.Errorf("configuration error: the claim_set_summary operator in merge mode writes the whole " +
				"delivery's summary and cannot run under a splitter, which builds one per split value; use a fan_out")
		}
		if pipe.InputChannel.Type == "stage" {
			if chSpec := cpConfig.GetChannelSpec(pipe.InputChannel.Name); chSpec != nil {
				if absent := claimSetSummaryAbsentColumns(chSpec.Columns); len(absent) > 0 {
					return fmt.Errorf("configuration error: the channel spec '%s' the claim_set_summary operator "+
						"merges from lacks the partial column(s) %s", chSpec.Name, strings.Join(absent, ", "))
				}
			}
		}
	default:
		return fmt.Errorf("configuration error: claim_set_summary_config.mode is '%s', not partial or merge",
			config.Mode)
	}
	return nil
}

func claimSetSummaryAbsentColumns(columns []string) []string {
	m := make(map[string]int, len(columns))
	for i, c := range columns {
		m[c] = i
	}
	return claimSetSummaryAbsent(m)
}

// claimSetSummaryDocumentFindings warns, when a pipeline configuration is
// saved, of each claim_set_summary operator its step's start would refuse
// (validateClaimSetSummarySpec). A warning, since the step may be one whose
// when is never true.
func claimSetSummaryDocumentFindings(cfg *ComputePipesConfig) []wsvalidate.Finding {
	var out []wsvalidate.Finding
	check := func(pipes []PipeSpec, path string) {
		for i := range pipes {
			for j := range pipes[i].Apply {
				if pipes[i].Apply[j].Type != ClaimSetSummaryOperatorType {
					continue
				}
				if err := validateClaimSetSummarySpec(cfg, &pipes[i], &pipes[i].Apply[j]); err != nil {
					out = append(out, wsvalidate.Finding{Severity: wsvalidate.Warning,
						Code: "cpipes_config_claim_set_summary", Message: "the step will not start: " + err.Error(),
						Path: fmt.Sprintf("%s/%d/apply/%d", path, i, j)})
				}
			}
		}
	}
	check(cfg.PipesConfig, "/pipes_config")
	for i := range cfg.ConditionalPipesConfig {
		check(cfg.ConditionalPipesConfig[i].PipesConfig, fmt.Sprintf("/conditional_pipes_config/%d/pipes_config", i))
	}
	for i := range cfg.ReducingPipesConfig {
		check(cfg.ReducingPipesConfig[i], fmt.Sprintf("/reducing_pipes_config/%d", i))
	}
	return out
}

// validateClaimSetSummaryNodes refuses a step carrying a claim_set_summary
// operator in merge mode that would run on more than one node: each node would
// merge part of the run's partials, and the last to finish would write its
// part as the whole. A partial is valid on any number of nodes. It is called by
// both starters once the step's node count is known.
func validateClaimSetSummaryNodes(pipeConfig []PipeSpec, nbrNodes int, stepName string) error {
	if nbrNodes <= 1 {
		return nil
	}
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			spec := &pipeConfig[i].Apply[j]
			if spec.Type == ClaimSetSummaryOperatorType && spec.ClaimSetSummaryConfig != nil &&
				spec.ClaimSetSummaryConfig.Mode == ClaimSetSummaryMerge {
				return fmt.Errorf("configuration error: step '%s' carries a claim_set_summary operator in merge mode, "+
					"which writes the whole delivery's summary and so requires a step that runs on one node; this "+
					"step has %d. Write the partials with a partition_writer whose jets_partition_key is fixed",
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
