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
// A row is started while an invocation registers the event and done once it has.
// A delivery that finds the event done, or started by an invocation that may
// still be running, registers nothing. A started row older than an invocation can
// run belongs to one that died (a timeout), and the next delivery takes it over,
// since the dead invocation may have stopped before starting every pipeline. An
// invocation that fails with an error deletes its row, so Lambda's retry
// registers the event.

const (
	eventStarted = "started"
	eventDone    = "done"

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
// invocation should register it, and false with the existing row's status when
// another delivery has registered it or is registering it. inFlight is how long
// a started row is taken to belong to a running invocation.
func claimEvent(ctx context.Context, db eventDB, fileKey, sequencer string,
	inFlight time.Duration) (claimed bool, status string, err error) {

	stmt := `INSERT INTO jetsapi.s3_object_event (file_key, sequencer, status, started_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (file_key, sequencer) DO UPDATE
			SET status = EXCLUDED.status, started_at = now(), completed_at = NULL
			WHERE s3_object_event.status = $3
			  AND s3_object_event.started_at < now() - make_interval(secs => $4)
		RETURNING status`
	err = db.QueryRow(ctx, stmt, fileKey, sequencer, eventStarted, inFlight.Seconds()).Scan(&status)
	switch {
	case err == nil:
		return true, status, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return false, "", fmt.Errorf("while recording the S3 event of %s: %w", fileKey, err)
	}
	err = db.QueryRow(ctx,
		"SELECT status FROM jetsapi.s3_object_event WHERE file_key = $1 AND sequencer = $2",
		fileKey, sequencer).Scan(&status)
	if err != nil {
		return false, "", fmt.Errorf("while reading the S3 event of %s: %w", fileKey, err)
	}
	return false, status, nil
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

// releaseEvent deletes the event's row, so the next delivery registers it.
func releaseEvent(ctx context.Context, db eventDB, fileKey, sequencer string) error {
	_, err := db.Exec(ctx, "DELETE FROM jetsapi.s3_object_event WHERE file_key = $1 AND sequencer = $2",
		fileKey, sequencer)
	if err != nil {
		return fmt.Errorf("while releasing the S3 event of %s: %w", fileKey, err)
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
