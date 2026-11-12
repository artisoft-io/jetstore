package registerkey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// An S3 object-created event can reach this lambda more than once for one write
// of a key: S3 delivers each event at least once, and Lambda retries an
// asynchronous invocation that returned an error or timed out, with the same
// event. Registering the file again reserves a new session and starts its
// pipelines a second time. jetsapi.s3_object_event records each event by its key
// and sequencer, which S3 sets anew for every write of a key and repeats in a
// re-delivery, so a re-delivery can be told from the file being written again.
//
// A row is started while an invocation registers the event, done once it has, and
// failed when the invocation returned an error. A delivery that finds the event
// done, or started by an invocation that may still be running, registers nothing.
// A failed row, or a started row older than an invocation can run (its invocation
// died, typically of a timeout while a pipeline start waited on the state machine
// lock), is taken over by the next delivery, and the registration is resumed:
// the row carries the input session the first invocation reserved, recorded
// before it wrote any input_registry row, so the delivery registers under that
// session, reuses those rows and starts only the pipelines with no run yet.

const (
	eventStarted = "started"
	eventDone    = "done"
	eventFailed  = "failed"

	// eventRetention is how long a row is kept. Lambda gives up on an event after
	// six hours at most, and S3 re-delivers within that, so seven days is ample.
	eventRetention = "7 days"

	// inFlightMargin is added to an invocation's own time limit to decide whether
	// a started row may still belong to a running invocation. It must stay below
	// Lambda's delay before retrying a failed invocation, about a minute, or a
	// retry after a timeout would be taken for a duplicate and skipped.
	inFlightMargin = 10 * time.Second
)

// eventDB is what the claim needs of a pgxpool.Pool.
type eventDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// claimEvent takes the event for this invocation. It returns true when this
// invocation should register it, with the input session an earlier invocation
// recorded when it takes over an unfinished registration (empty otherwise), and
// false with the existing row's status when another delivery has registered it or
// is registering it. inFlight is how long a started row is taken to belong to a
// running invocation.
func claimEvent(ctx context.Context, db eventDB, fileKey, sequencer string,
	inFlight time.Duration) (claimed bool, status, sessionId string, err error) {

	stmt := `INSERT INTO jetsapi.s3_object_event (file_key, sequencer, status, started_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (file_key, sequencer) DO UPDATE
			SET status = EXCLUDED.status, started_at = now(), completed_at = NULL
			WHERE s3_object_event.status = $5
			   OR (s3_object_event.status = $3
			       AND s3_object_event.started_at < now() - make_interval(secs => $4))
		RETURNING status, coalesce(session_id, '')`
	err = db.QueryRow(ctx, stmt, fileKey, sequencer, eventStarted, inFlight.Seconds(), eventFailed).
		Scan(&status, &sessionId)
	switch {
	case err == nil:
		return true, status, sessionId, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return false, "", "", fmt.Errorf("while recording the S3 event of %s: %w", fileKey, err)
	}
	err = db.QueryRow(ctx,
		"SELECT status FROM jetsapi.s3_object_event WHERE file_key = $1 AND sequencer = $2",
		fileKey, sequencer).Scan(&status)
	if err != nil {
		return false, "", "", fmt.Errorf("while reading the S3 event of %s: %w", fileKey, err)
	}
	return false, status, "", nil
}

// recordSession records the input session the event's registration reserved, so
// that a delivery taking the event over can resume under it.
func recordSession(ctx context.Context, db eventDB, fileKey, sequencer, sessionId string) error {
	_, err := db.Exec(ctx, `UPDATE jetsapi.s3_object_event SET session_id = $3
		WHERE file_key = $1 AND sequencer = $2`, fileKey, sequencer, sessionId)
	if err != nil {
		return fmt.Errorf("while recording the session of the S3 event of %s: %w", fileKey, err)
	}
	return nil
}

// completeEvent marks the event registered, and removes rows past retention.
func completeEvent(ctx context.Context, db eventDB, fileKey, sequencer string) error {
	_, err := db.Exec(ctx, `UPDATE jetsapi.s3_object_event SET status = $3, completed_at = now()
		WHERE file_key = $1 AND sequencer = $2`, fileKey, sequencer, eventDone)
	if err != nil {
		return fmt.Errorf("while marking the S3 event of %s registered: %w", fileKey, err)
	}
	_, err = db.Exec(ctx, "DELETE FROM jetsapi.s3_object_event WHERE started_at < now() - interval '"+eventRetention+"'")
	if err != nil {
		return fmt.Errorf("while removing old S3 events: %w", err)
	}
	return nil
}

// failEvent marks the event failed, so the next delivery, Lambda's retry, takes
// it over at once and resumes the registration.
func failEvent(ctx context.Context, db eventDB, fileKey, sequencer string) error {
	_, err := db.Exec(ctx, `UPDATE jetsapi.s3_object_event SET status = $3
		WHERE file_key = $1 AND sequencer = $2`, fileKey, sequencer, eventFailed)
	if err != nil {
		return fmt.Errorf("while marking the S3 event of %s failed: %w", fileKey, err)
	}
	return nil
}

// inFlightWindow is how long a started row may belong to a running invocation:
// this invocation's own time limit, read from its deadline, plus inFlightMargin.
// Every invocation of the lambda has the same limit, so it follows the lambda's
// configured timeout. With no deadline (outside Lambda) it is a minute.
func inFlightWindow(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		if left := time.Until(deadline); left > 0 {
			return left + inFlightMargin
		}
	}
	return time.Minute
}
