package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v5/pgxpool"
)

// # Reading only the latest session of each period
//
// A stage folder such as `$CLIENT/<ObjectType>/${WEEK_PERIOD}W` holds one session
// folder per run that wrote to it. A channel reading the folder through file_key
// reads every one of them. When a feed is remapped (after a model change, or to
// correct a mapping), the new session folder lands beside the old one, and the read
// takes both: duplicated records, and records in the old column layout read with the
// new one.
//
// latest_session_only on the input channel keeps, in each period folder of the
// read, the objects of one session folder: the latest whose run completed. The
// other session folders are skipped and logged; nothing is deleted.
//
//   - Latest is the largest session id, compared as numbers (sessionAfter): when the
//     session was mapped, not when the file was delivered. The option is therefore
//     for a feed with one delivery per period. A period with two real deliveries
//     would lose one of them.
//   - A completed run is one with a session_registry row and a
//     pipeline_execution_status row in a status whose outputs are usable (completed,
//     recovered, errors). A failed run is registered too, and a running one has
//     written part of its folder, so neither is picked. When a period holds data and
//     no session of it completed, the read is refused rather than reading nothing.
//   - Sessions a user deleted are dropped first (dropDeletedSessions), so deleting
//     the latest session makes the one before it the latest.
//   - With a cumulative object type (source_object_type), the latest session of each
//     period is kept first, then latestDelivery keeps the latest period's: one
//     session in all.
//
// The registry is read when the folder is listed: by the sharding starter, or by
// the reducing starter (AssignStageFileKeys), which hands the nodes their files.

// completedSessionsFn returns which of the session ids are sessions whose run
// completed.
type completedSessionsFn func(ids []string) (map[string]bool, error)

// completedSessionsOf reads the completed runs among ids from the database.
func completedSessionsOf(ctx context.Context, dbpool *pgxpool.Pool) completedSessionsFn {
	return func(ids []string) (map[string]bool, error) {
		completed := make(map[string]bool)
		if len(ids) == 0 {
			return completed, nil
		}
		stmt := `
			SELECT DISTINCT pe.session_id
			FROM jetsapi.pipeline_execution_status pe
			JOIN jetsapi.session_registry sr ON sr.session_id = pe.session_id
			WHERE pe.session_id = ANY($1)
			  AND pe.status IN ('completed', 'recovered', 'errors')`
		rows, err := dbpool.Query(ctx, stmt, ids)
		if err != nil {
			return nil, fmt.Errorf("while reading the completed runs of %d session(s): %v", len(ids), err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				return nil, fmt.Errorf("while scanning the completed runs: %v", err)
			}
			completed[id] = true
		}
		if err = rows.Err(); err != nil {
			return nil, fmt.Errorf("while reading the completed runs: %v", err)
		}
		return completed, nil
	}
}

// stageReadOptions says how a file_key stage read narrows what it lists.
type stageReadOptions struct {
	sourceObjectType string
	// completed is set when the channel reads only the latest session of each
	// period; nil reads every session.
	completed completedSessionsFn
}

// stageReadOptionsOf builds the options of a file_key stage channel. A channel with
// latest_session_only needs the database, and a nil dbpool is refused.
func stageReadOptionsOf(ctx context.Context, dbpool *pgxpool.Pool, ic *InputChannelConfig) (stageReadOptions, error) {
	opts := stageReadOptions{sourceObjectType: ic.SourceObjectType}
	if ic.LatestSessionOnly {
		if dbpool == nil {
			return opts, fmt.Errorf(
				"input channel %s sets latest_session_only, which reads session_registry, but this read has no database connection",
				ic.Name)
		}
		opts.completed = completedSessionsOf(ctx, dbpool)
	}
	return opts, nil
}

// ListStageChannel lists the stage objects a channel reading the stage through
// file_key reads: its lookback periods (or its one folder), less the sessions a
// user deleted, narrowed to the latest completed session of each period when the
// channel sets latest_session_only, and to the latest delivery when its object type
// is cumulative.
func ListStageChannel(ctx context.Context, dbpool *pgxpool.Pool, bucket string, ic *InputChannelConfig,
	env map[string]any) ([]*awsi.S3Object, error) {
	opts, err := stageReadOptionsOf(ctx, dbpool, ic)
	if err != nil {
		return nil, err
	}
	fileKey := fmt.Sprintf("%s/%s", awsi.JetStoreStagePrefix(), ic.FileKey)
	return listStageObjects(bucket, fileKey, ic.LookbackPeriods, opts, env)
}

// keepLatestCompletedSessions keeps, in each period, the objects of its latest
// session folder whose run completed. A period with no data is kept as is. A
// period holding data with no completed session is an error, as is an object
// with no session folder below the period's prefix. The periods passed in are not
// modified.
func keepLatestCompletedSessions(periods []lookbackPeriodObjects, completed completedSessionsFn) ([]lookbackPeriodObjects, error) {
	out := make([]lookbackPeriodObjects, len(periods))
	for i := range periods {
		p := &periods[i]
		bySession := make(map[string][]*awsi.S3Object)
		for _, obj := range p.objects {
			session, ok := sessionSegment(p.prefix, obj.Key)
			if !ok {
				return nil, fmt.Errorf(
					"latest_session_only: s3 key %s has no session folder below %s, so its session cannot be told apart",
					obj.Key, p.prefix)
			}
			bySession[session] = append(bySession[session], obj)
		}
		var withData []string
		for session, objs := range bySession {
			if hasDataObject(objs) {
				withData = append(withData, session)
			}
		}
		out[i] = lookbackPeriodObjects{periodId: p.periodId, prefix: p.prefix}
		if len(withData) == 0 {
			continue
		}
		done, err := completed(withData)
		if err != nil {
			return nil, err
		}
		var latest string
		for _, session := range withData {
			if done[session] && (latest == "" || sessionAfter(session, latest)) {
				latest = session
			}
		}
		if latest == "" {
			slices.SortFunc(withData, func(a, b string) int { return compareSessions(b, a) })
			return nil, fmt.Errorf(
				"latest_session_only: none of the %d session folder(s) holding data in %s is a completed run (%s): "+
					"resubmit the run or delete its session",
				len(withData), p.prefix, strings.Join(withData, ", "))
		}
		out[i].objects = bySession[latest]
		var skipped, notCompleted []string
		for _, session := range slices.SortedFunc(maps.Keys(bySession), func(a, b string) int { return compareSessions(b, a) }) {
			if session == latest {
				continue
			}
			skipped = append(skipped, session)
			if hasDataObject(bySession[session]) && !done[session] {
				notCompleted = append(notCompleted, session)
			}
		}
		if len(skipped) > 0 {
			log.Printf("Latest session only: %s keeps session %s; skipping %d session folder(s): %s (not a completed run: [%s])",
				p.prefix, latest, len(skipped), strings.Join(skipped, ", "), strings.Join(notCompleted, ", "))
		}
	}
	return out, nil
}

// compareSessions orders session ids as sessionAfter does.
func compareSessions(a, b string) int {
	switch {
	case sessionAfter(a, b):
		return 1
	case sessionAfter(b, a):
		return -1
	}
	return 0
}

// keepLatestCompletedSessionOfFolder applies latest_session_only to a stage folder
// listed directly rather than through a lookback (a file_key with no
// lookback_periods read by the sharding step). With no option set, the objects
// are returned unchanged.
func keepLatestCompletedSessionOfFolder(prefix string, objects []*awsi.S3Object, opts stageReadOptions) ([]*awsi.S3Object, error) {
	if opts.completed == nil {
		return objects, nil
	}
	periods, err := keepLatestCompletedSessions([]lookbackPeriodObjects{{prefix: prefix, objects: objects}}, opts.completed)
	if err != nil {
		return nil, err
	}
	return periods[0].objects, nil
}
