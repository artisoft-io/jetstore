package claimset

// A partial as one record of a pipeline channel: the compute pipes operator
// claim_set_summary emits one per shard in its partial mode, and reads them
// back in its merge mode, possibly from a staged csv file in which every
// value is text.

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/keyset"
)

// The columns of a partial record.
const (
	// PartialSkip is why the shard gave up its summary, empty when it did not.
	PartialSkip = "claim_set_skip"
	// PartialRows and PartialDatedRows are the shard's rows, and those of
	// them with a date of service: integers.
	PartialRows      = "claim_set_rows"
	PartialDatedRows = "claim_set_dated_rows"
	// PartialDistinctKeys is the count of the shard's distinct keys, an
	// integer; PartialKeyFingerprint their fingerprint, the unsigned 64-bit
	// sum as decimal text; PartialKeySketch their sketch, packed
	// (keyset.Sketch.Pack) and base64.
	PartialDistinctKeys   = "claim_set_distinct_keys"
	PartialKeyFingerprint = "claim_set_key_fingerprint"
	PartialKeySketch      = "claim_set_key_sketch"
	// PartialMembers is the count of the shard's distinct members, an
	// integer; PartialMemberSketch their sketch (MemberHash), packed and
	// base64.
	PartialMembers      = "claim_set_members"
	PartialMemberSketch = "claim_set_member_sketch"
	// PartialRowsByDate is the rows per date of service, a JSON object from
	// the date, YYYY-MM-DD, to the count.
	PartialRowsByDate = "claim_set_rows_by_date"
	// PartialPresentCounts is the rows carrying a value per column, a JSON
	// object from the column to the count.
	PartialPresentCounts = "claim_set_present_counts"
)

// PartialColumns are the columns of a partial record, in the order the
// operator's documentation lists them. A channel carrying partials has every
// one of them, in any order.
var PartialColumns = []string{PartialSkip, PartialRows, PartialDatedRows, PartialDistinctKeys,
	PartialKeyFingerprint, PartialKeySketch, PartialMembers, PartialMemberSketch, PartialRowsByDate,
	PartialPresentCounts}

// Values is the partial as a record's values, by column: integers as int64,
// the rest as text.
func (p *Partial) Values() (map[string]any, error) {
	days := make(map[string]int64, len(p.Days))
	for d, n := range p.Days {
		days[dayOf(d).Format(time.DateOnly)] = n
	}
	byDate, err := json.Marshal(days)
	if err != nil {
		return nil, err
	}
	present := p.Present
	if present == nil {
		present = map[string]int64{}
	}
	counts, err := json.Marshal(present)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		PartialSkip:           p.Skip,
		PartialRows:           p.Rows,
		PartialDatedRows:      p.DatedRows,
		PartialDistinctKeys:   p.Keys.Count,
		PartialKeyFingerprint: strconv.FormatUint(p.Keys.Fingerprint, 10),
		PartialKeySketch:      base64.StdEncoding.EncodeToString(p.Keys.Sketch.Pack()),
		PartialMembers:        p.Members,
		PartialMemberSketch:   base64.StdEncoding.EncodeToString(p.MemberSketch.Pack()),
		PartialRowsByDate:     string(byDate),
		PartialPresentCounts:  string(counts),
	}, nil
}

// PartialOf reads a partial record, value gives each column's value. A
// value may be the one Values gave, or its text, as a staged csv file holds
// it.
func PartialOf(value func(column string) any) (*Partial, error) {
	text := func(c string) (string, error) {
		switch v := value(c).(type) {
		case nil:
			return "", nil
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		case int64:
			return strconv.FormatInt(v, 10), nil
		case int:
			return strconv.Itoa(v), nil
		case int32:
			return strconv.FormatInt(int64(v), 10), nil
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64), nil
		default:
			return "", fmt.Errorf("the column %s carries a %T", c, v)
		}
	}
	count := func(c string) (int64, error) {
		t, err := text(c)
		if err != nil {
			return 0, err
		}
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("the column %s holds %q, which is not a count", c, t)
		}
		return n, nil
	}
	sketch := func(c string, into *keyset.Sketch) error {
		t, err := text(c)
		if err != nil {
			return err
		}
		b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(t))
		if err != nil {
			return fmt.Errorf("the column %s is not base64: %v", c, err)
		}
		s, err := keyset.Unpack(b)
		if err != nil {
			return fmt.Errorf("the column %s: %v", c, err)
		}
		*into = *s
		return nil
	}
	p := &Partial{}
	var err error
	if p.Skip, err = text(PartialSkip); err != nil {
		return nil, err
	}
	for c, into := range map[string]*int64{PartialRows: &p.Rows, PartialDatedRows: &p.DatedRows,
		PartialDistinctKeys: &p.Keys.Count, PartialMembers: &p.Members} {
		if *into, err = count(c); err != nil {
			return nil, err
		}
	}
	fp, err := text(PartialKeyFingerprint)
	if err != nil {
		return nil, err
	}
	if p.Keys.Fingerprint, err = strconv.ParseUint(strings.TrimSpace(fp), 10, 64); err != nil {
		return nil, fmt.Errorf("the column %s holds %q, which is not an unsigned 64-bit integer",
			PartialKeyFingerprint, fp)
	}
	if err := sketch(PartialKeySketch, &p.Keys.Sketch); err != nil {
		return nil, err
	}
	if err := sketch(PartialMemberSketch, &p.MemberSketch); err != nil {
		return nil, err
	}
	byDate, err := text(PartialRowsByDate)
	if err != nil {
		return nil, err
	}
	var days map[string]int64
	if err := json.Unmarshal([]byte(byDate), &days); err != nil {
		return nil, fmt.Errorf("the column %s is not a JSON object of counts: %v", PartialRowsByDate, err)
	}
	p.Days = make(map[int64]int64, len(days))
	for d, n := range days {
		t, err := time.Parse(time.DateOnly, d)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("the column %s holds %q: %d, which is not a date and a count", PartialRowsByDate, d, n)
		}
		p.Days[epochDay(t)] += n
	}
	counts, err := text(PartialPresentCounts)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(counts), &p.Present); err != nil {
		return nil, fmt.Errorf("the column %s is not a JSON object of counts: %v", PartialPresentCounts, err)
	}
	if p.Present == nil {
		p.Present = map[string]int64{}
	}
	return p, nil
}
