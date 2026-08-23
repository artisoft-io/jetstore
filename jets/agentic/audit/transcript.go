package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Reading a run's transcript back (task K.2, gap 11) — criterion 29's "one
// agent run's transcript is viewable from its audit record alone".
//
// **The load-bearing phrase is "alone", and it is a claim about sufficiency
// rather than about a screen.** F10 settled that the transcript *is* the audit
// store: a hash-chained per-run ordered event log with a tool_call type is what
// a transcript is, which is why gap 4 shrank. What that left unbuilt was the
// other direction — the store had a write path and no read path, so the claim
// that the record suffices had never been exercised. ReadTranscript exercises
// it: it touches `jetsapi.agent_audit` and nothing else. Not agent_run, whose
// summary it deliberately does not need; not change_proposal; not the loop's
// in-memory Result. If a run's story cannot be told from those rows, the
// criterion is not met, and the query is the proof rather than the prose.
//
// **Verification is not optional, and that is a design decision rather than a
// convenience.** ReadTranscript verifies the chain on the way out and hands the
// defects back with the events. A viewer that renders a tamper-evident log
// without checking the evidence offers the *appearance* of the property, which
// is worse than not offering it: the chain's whole value is that someone looked.
// So there is no way to obtain the events without also obtaining the verdict.
//
// **The recomputation is independent, not a re-read.** The trigger's material is
// rebuilt here from the row's own columns — including the timestamp, formatted
// in Go rather than fetched back through `to_char` — so the check does not
// consult the same expression it is checking. The one field taken verbatim is
// `payload::text`: jsonb normalises key order and whitespace on the way in, and
// re-deriving that rendering in Go would be reimplementing Postgres rather than
// verifying it. Those bytes are read, not trusted — a payload altered in place
// changes them and the hash fails.
//
// # What this proves, and the one thing it does not
//
// A verified chain says **nothing in these rows was altered, reordered or
// removed** after it was written. It does **not** say the run wrote every event
// it should have, and the gap is structural rather than a matter of care:
//
//   - the trigger assigns `seq` as `max(seq) + 1` over the run's existing rows,
//     so an append that never lands leaves the sequence contiguous; and
//   - `Loop.event` discards the append error (`jets/agentic/agent/loop.go:440`),
//     by an argument this package agrees with — killing a healthy run because
//     its transcript hiccuped is the worse trade — but the loop's Result carries
//     no field recording that it happened.
//
// So a dropped event is invisible from both ends: no gap in the record, no note
// in the run. `Defects` is silent about it because there is nothing to see, and
// a reader who takes silence for completeness has read more into it than it
// says. The seq-continuity check below therefore catches rows missing from *this
// read* — a filtered query, a partial fetch, a deletion that got past the
// append-only trigger — and not rows that were never written.

// TranscriptEvent is one row of jetsapi.agent_audit as read back, carrying the
// chain columns so the verdict can be recomputed rather than believed.
type TranscriptEvent struct {
	Seq       int
	EventType string
	Actor     string
	Tier      string // "" when the event carried none
	ToolName  string // "" when the event is not a tool call
	// Payload is `payload::text` exactly as Postgres rendered it — the bytes
	// the trigger hashed. Verification depends on them being untouched, so do
	// not re-encode this field; render a copy.
	Payload   json.RawMessage
	CreatedAt time.Time
	PrevHash  []byte // nil on the first event of a run
	RowHash   []byte
}

// DefectKind names what is wrong with a chain, kept distinct because the three
// cases have different causes and a viewer should not blur them.
type DefectKind string

const (
	// DefectGap — seq is not contiguous from 1. Rows are missing from this
	// read; see the package note on what that does and does not imply.
	DefectGap DefectKind = "gap"
	// DefectLink — prev_hash does not match the previous row's row_hash, so
	// the rows are not the sequence they claim to be.
	DefectLink DefectKind = "link"
	// DefectHash — row_hash does not recompute from the row's own columns, so
	// a field was altered after it was written.
	DefectHash DefectKind = "hash"
)

// ChainDefect is one failure found while verifying, located at the seq it was
// found at.
type ChainDefect struct {
	Seq    int
	Kind   DefectKind
	Detail string
}

func (d ChainDefect) String() string {
	return fmt.Sprintf("seq %d: %s: %s", d.Seq, d.Kind, d.Detail)
}

// Transcript is one run's record, with the verdict on it.
type Transcript struct {
	RunId  string
	Events []TranscriptEvent
	// Defects is empty on a sound chain. It is a field rather than an error
	// because a damaged transcript is still the thing you want to look at —
	// refusing to render it would destroy the evidence of what happened.
	Defects []ChainDefect
}

// Verified reports whether the chain holds. It is the question a viewer asks
// first, and the answer must be shown wherever the events are.
func (t *Transcript) Verified() bool { return len(t.Defects) == 0 }

// ErrNoTranscript is returned when a run has no audit rows at all.
//
// **Empty is not a valid transcript, and that follows from StartRun.** The run
// row and its intent event ride one transaction, so a run that exists has at
// least one event; zero rows therefore means no such run in the audit store —
// a mistyped id, or the wrong database — rather than a run that has not done
// anything yet. Reporting that as an empty log would render a blank page for a
// typo and let the reader conclude the agent did nothing.
type ErrNoTranscript struct{ RunId string }

func (e *ErrNoTranscript) Error() string {
	return fmt.Sprintf("no audit record for run %s; a started run always has its intent event, so there is no such run here", e.RunId)
}

// ReadTranscript reads one run's events, oldest first, and verifies the chain.
//
// The query names one table on purpose — criterion 29's "from its audit record
// alone" is a claim about this SELECT. `payload::text` rather than the jsonb
// value because those are the bytes the chain hashed.
func ReadTranscript(ctx context.Context, db Querier, runId string) (*Transcript, error) {
	if runId == "" {
		return nil, fmt.Errorf("cannot read a transcript without a run id")
	}
	rows, err := db.Query(ctx,
		`SELECT seq, event_type, actor, coalesce(tier, ''), coalesce(tool_name, ''),
		        payload::text, created_at, prev_hash, row_hash
		   FROM jetsapi.agent_audit
		  WHERE run_id = $1
		  ORDER BY seq`, runId)
	if err != nil {
		return nil, fmt.Errorf("while reading the transcript for run %s: %w", runId, err)
	}
	defer rows.Close()

	t := &Transcript{RunId: runId}
	for rows.Next() {
		var ev TranscriptEvent
		var payload string
		if err := rows.Scan(&ev.Seq, &ev.EventType, &ev.Actor, &ev.Tier, &ev.ToolName,
			&payload, &ev.CreatedAt, &ev.PrevHash, &ev.RowHash); err != nil {
			return nil, fmt.Errorf("while scanning an event of run %s: %w", runId, err)
		}
		ev.Payload = json.RawMessage(payload)
		t.Events = append(t.Events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the transcript for run %s: %w", runId, err)
	}
	if len(t.Events) == 0 {
		return nil, &ErrNoTranscript{RunId: runId}
	}
	t.Defects = Verify(runId, t.Events)
	return t, nil
}

// chainTimeLayout is the trigger's timestamp rendering — Postgres
// `to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')` —
// expressed in Go so the check derives the material rather than fetching it
// back through the expression under test. The trailing Z is appended rather
// than written into the layout, where a lone Z is ambiguous with Go's zone
// verbs.
const chainTimeLayout = "2006-01-02T15:04:05.000000"

func chainTime(t time.Time) string { return t.UTC().Format(chainTimeLayout) + "Z" }

// Verify recomputes the chain over events already read, oldest first, and
// returns every defect it finds.
//
// It is a pure function over the slice so that tampering is testable: the
// append-only trigger makes a real UPDATE impossible, which is the property
// criterion 9 tests, and would otherwise leave the detector itself unexercised.
// Mutating a TranscriptEvent in memory is the same input a tampered row would
// produce, and reaches the same code.
//
// Defects do not cascade. A row whose payload was altered fails its own hash
// check while the row after it still links to the stored hash, so each entry
// names a distinct problem rather than an echo of the one above it.
func Verify(runId string, events []TranscriptEvent) []ChainDefect {
	var defects []ChainDefect
	var prevRowHash []byte
	for i, ev := range events {
		if want := i + 1; ev.Seq != want {
			defects = append(defects, ChainDefect{
				Seq:  ev.Seq,
				Kind: DefectGap,
				Detail: fmt.Sprintf("expected seq %d at position %d; rows are missing from this read",
					want, i),
			})
		}
		switch {
		case i == 0 && len(ev.PrevHash) != 0:
			defects = append(defects, ChainDefect{Seq: ev.Seq, Kind: DefectLink,
				Detail: "the first event of a run links to a predecessor, and there is none"})
		case i > 0 && !bytes.Equal(ev.PrevHash, prevRowHash):
			defects = append(defects, ChainDefect{Seq: ev.Seq, Kind: DefectLink,
				Detail: fmt.Sprintf("prev_hash %s does not match the previous row_hash %s",
					shortHash(ev.PrevHash), shortHash(prevRowHash))})
		}
		if want := rowHash(runId, &ev); !bytes.Equal(ev.RowHash, want[:]) {
			defects = append(defects, ChainDefect{Seq: ev.Seq, Kind: DefectHash,
				Detail: fmt.Sprintf("row_hash %s does not recompute (%s); a column was altered after it was written",
					shortHash(ev.RowHash), shortHash(want[:]))})
		}
		prevRowHash = ev.RowHash
	}
	return defects
}

// rowHash rebuilds the trigger's material: the columns joined by the 0x1F unit
// separator, with the previous hash hex-encoded and an absent one empty.
func rowHash(runId string, ev *TranscriptEvent) [32]byte {
	prevHex := ""
	if len(ev.PrevHash) > 0 {
		prevHex = hex.EncodeToString(ev.PrevHash)
	}
	return sha256.Sum256([]byte(strings.Join([]string{
		runId,
		fmt.Sprint(ev.Seq),
		ev.EventType,
		ev.Actor,
		ev.Tier,
		ev.ToolName,
		string(ev.Payload),
		chainTime(ev.CreatedAt),
		prevHex,
	}, "\x1f")))
}

func shortHash(h []byte) string {
	if len(h) == 0 {
		return "(none)"
	}
	s := hex.EncodeToString(h)
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// Render writes the transcript as readable text — the rendering criterion 29's
// "viewable" asks for, and the one an IDE screen (K.3) or an operator at a
// terminal reads the same way.
//
// **The verdict leads, and the defects are shown where they happened.** A log
// whose integrity is reported in a footer invites reading the events and
// stopping; putting it in the first line makes the reader see it before the
// content it qualifies.
func (t *Transcript) Render(w io.Writer) error {
	verdict := "chain verified"
	if !t.Verified() {
		verdict = fmt.Sprintf("CHAIN NOT VERIFIED - %d defect(s)", len(t.Defects))
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "run %s  -  %d event(s)  -  %s\n", t.RunId, len(t.Events), verdict)
	fmt.Fprintln(b, strings.Repeat("=", 78))

	bySeq := map[int][]ChainDefect{}
	for _, d := range t.Defects {
		bySeq[d.Seq] = append(bySeq[d.Seq], d)
	}
	for _, ev := range t.Events {
		fmt.Fprintf(b, "%4d  %s  %-9s  %s", ev.Seq, chainTime(ev.CreatedAt), ev.EventType, ev.Actor)
		if ev.Tier != "" {
			fmt.Fprintf(b, "  %s", ev.Tier)
		}
		if ev.ToolName != "" {
			fmt.Fprintf(b, "  tool=%s", ev.ToolName)
		}
		fmt.Fprintln(b)
		for _, d := range bySeq[ev.Seq] {
			fmt.Fprintf(b, "      !! %s: %s\n", d.Kind, d.Detail)
		}
		for _, line := range strings.Split(indentPayload(ev.Payload), "\n") {
			fmt.Fprintf(b, "      %s\n", line)
		}
	}
	// A defect at a seq that is not in the events cannot be shown beside its
	// row, and dropping it silently would hide the one thing worth seeing.
	for _, d := range t.Defects {
		if _, shown := bySeq[d.Seq]; !shown {
			fmt.Fprintf(b, "  !! %s\n", d)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// indentPayload pretty-prints for reading. It works on a copy: the bytes in
// TranscriptEvent.Payload are chain material and re-encoding them in place
// would invalidate the hash they were verified against.
func indentPayload(payload json.RawMessage) string {
	var out bytes.Buffer
	if err := json.Indent(&out, payload, "", "  "); err != nil {
		// A payload that will not re-indent is shown as it was stored. It came
		// out of a jsonb column so this should not happen, and printing it
		// anyway is better than printing nothing about it.
		return string(payload)
	}
	return out.String()
}

// String renders to a string, for the common case of showing one transcript.
func (t *Transcript) String() string {
	b := &strings.Builder{}
	_ = t.Render(b)
	return b.String()
}
