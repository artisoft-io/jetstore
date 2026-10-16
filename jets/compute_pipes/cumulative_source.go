package compute_pipes

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// # Cumulative files: reading only the latest delivery across a lookback
//
// A client's file is *cumulative* when each delivery holds the whole feed: the
// latest delivery repeats every record of the earlier ones, plus what is new.
// A pipeline that reads such a feed across lookback_periods reads every delivery
// in every period folder, so each record is read once per delivery. The flag
// that says a file is cumulative is source_config.is_cumulative, per client, org
// and object type.
//
// The pipeline that reads the deliveries is usually not the one that mapped
// them, and its main input is another object type, so the flag cannot come from
// the run's own source_config row. Two halves meet instead:
//
//   - At start (sharding initialisation), loadCumulativeObjectTypes puts the
//     client's cumulative object types in the env. The env is saved with the
//     startup and replayed by every later step, so the reducing steps see the
//     values the sharding step read.
//   - A stage input channel names the object type it reads in source_object_type.
//     When that object type is in the env's list, the lookback read keeps only
//     the latest delivery (latestDelivery).
//
// The stage folder holds no org (`$CLIENT/<ObjectType>/${PERIOD_ID}M/<session>`),
// so a client whose orgs disagree for one object type cannot be read either way:
// reading only the latest delivery would drop the incremental org's deliveries,
// and reading all would duplicate the cumulative org's. Such an object type is
// listed separately, and a cumulative read of it is refused.

const (
	// Comma-separated object types every source_config row of the client declares cumulative.
	cumulativeObjectTypesEnv = "${CUMULATIVE_OBJECT_TYPES}"
	// Comma-separated object types some, but not all, of the client's orgs declare cumulative.
	mixedCumulativeObjectTypesEnv = "${MIXED_CUMULATIVE_OBJECT_TYPES}"
)

// loadCumulativeObjectTypes reads source_config for the client and sets the two
// env entries above. Both are always set, empty when nothing is cumulative, so a
// later step can tell "none" from "not loaded".
func loadCumulativeObjectTypes(ctx context.Context, dbpool *pgxpool.Pool, client string, env map[string]any) error {
	env[cumulativeObjectTypesEnv] = ""
	env[mixedCumulativeObjectTypesEnv] = ""
	if dbpool == nil || client == "" {
		return nil
	}
	// is_cumulative is a schema "bool", which is an integer column (0 or 1), so
	// it is compared to 0 rather than handed to bool_and itself.
	stmt := `
		SELECT object_type, bool_and(is_cumulative <> 0)
		FROM jetsapi.source_config
		WHERE client = $1
		GROUP BY object_type
		HAVING bool_or(is_cumulative <> 0)
		ORDER BY object_type`
	rows, err := dbpool.Query(ctx, stmt, client)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42703" { // undefined_column
			return fmt.Errorf("source_config.is_cumulative is missing: the database schema predates this release "+
				"and needs update_db -migrateDb (%v)", err)
		}
		return fmt.Errorf("while reading the cumulative object types of client %s from source_config: %v", client, err)
	}
	defer rows.Close()
	var cumulative, mixed []string
	for rows.Next() {
		var objectType string
		var allOrgs bool
		if err = rows.Scan(&objectType, &allOrgs); err != nil {
			return fmt.Errorf("while scanning the cumulative object types of client %s: %v", client, err)
		}
		if allOrgs {
			cumulative = append(cumulative, objectType)
		} else {
			mixed = append(mixed, objectType)
		}
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("while reading the cumulative object types of client %s: %v", client, err)
	}
	env[cumulativeObjectTypesEnv] = strings.Join(cumulative, ",")
	env[mixedCumulativeObjectTypesEnv] = strings.Join(mixed, ",")
	if len(cumulative) > 0 || len(mixed) > 0 {
		log.Printf("Client %s cumulative object types: [%s]; declared cumulative by some orgs only: [%s]",
			client, env[cumulativeObjectTypesEnv], env[mixedCumulativeObjectTypesEnv])
	}
	return nil
}

// isCumulativeRead reports whether a lookback read of the channel's source object
// type keeps only the latest delivery. An empty objectType is never cumulative.
func isCumulativeRead(objectType string, env map[string]any) (bool, error) {
	objectType = strings.TrimSpace(utils.ReplaceEnvVars(objectType, env))
	if objectType == "" {
		return false, nil
	}
	if inCsvList(env[mixedCumulativeObjectTypesEnv], objectType) {
		return false, fmt.Errorf(
			"object type %s is declared cumulative for some of the client's orgs and not others (source_config.is_cumulative); "+
				"its staged deliveries hold no org, so they can be read neither as cumulative nor as incremental", objectType)
	}
	return inCsvList(env[cumulativeObjectTypesEnv], objectType), nil
}

func inCsvList(list any, item string) bool {
	s, _ := list.(string)
	if s == "" {
		return false
	}
	return slices.Contains(strings.Split(s, ","), item)
}

// lookbackPeriodObjects is one period folder of a lookback read.
type lookbackPeriodObjects struct {
	periodId int
	prefix   string // the expanded file key of the period, as listed
	objects  []*awsi.S3Object
}

// listLookbackPeriods lists the objects of each period of the lookback, the most
// recent period first.
func listLookbackPeriods(bucket, fileKey, lookbackPeriod string, env map[string]any) ([]lookbackPeriodObjects, error) {
	firstPeriodId, numPeriods, err := utils.ParseLookbackPeriod(lookbackPeriod, env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lookback_period %s: %v", lookbackPeriod, err)
	}
	log.Printf("Downloading file keys from s3 stage folder: %s, lookback first period_id: %d, num periods: %d",
		fileKey, firstPeriodId, numPeriods)
	// Make a copy of env to avoid modifying the original
	lookbackEnv := make(map[string]any, len(env)+1)
	for k, v := range env {
		lookbackEnv[k] = v
	}
	periods := make([]lookbackPeriodObjects, 0, numPeriods+1)
	// Last period in the lookback is firstPeriodId - numPeriods
	for p := range numPeriods + 1 {
		periodId := firstPeriodId - p
		lookbackEnv["${PERIOD_ID}"] = periodId
		fileKeyPrefix := utils.ReplaceEnvVars(fileKey, lookbackEnv)
		log.Printf("  Lookback period_id: %d, downloading file keys from s3 stage folder: %s", periodId, fileKeyPrefix)
		objects, err := awsi.ListS3Objects(bucket, &fileKeyPrefix)
		if err != nil {
			return nil, fmt.Errorf("failed to download list of files from s3 for lookback period_id %d: %v", periodId, err)
		}
		periods = append(periods, lookbackPeriodObjects{periodId: periodId, prefix: fileKeyPrefix, objects: objects})
	}
	return periods, nil
}

// allLookbackObjects concatenates the periods' objects, the most recent period first.
func allLookbackObjects(periods []lookbackPeriodObjects) []*awsi.S3Object {
	var all []*awsi.S3Object
	for i := range periods {
		all = append(all, periods[i].objects...)
	}
	return all
}

// latestDelivery returns the objects of the latest delivery across the lookback:
// the most recent period folder holding data, and within it the latest session
// folder. A session folder is the first path segment below the period's prefix
// (`$CLIENT/<ObjectType>/${PERIOD_ID}M/$SESSIONID/...`), and a session id is the
// session's start time in epoch milliseconds, so the largest is the latest.
//
// The period comes first, and that is deliberate: the period folder is the
// delivery's date, while the session is when it was mapped. Re-mapping an older
// delivery makes a newer session in an older period, and must not displace the
// latest delivery.
func latestDelivery(periods []lookbackPeriodObjects) ([]*awsi.S3Object, error) {
	for i := range periods {
		bySession := make(map[string][]*awsi.S3Object)
		var hasData bool
		for _, obj := range periods[i].objects {
			session, ok := sessionSegment(periods[i].prefix, obj.Key)
			if !ok {
				return nil, fmt.Errorf(
					"cumulative read: s3 key %s has no session folder below %s, so the latest delivery cannot be told apart",
					obj.Key, periods[i].prefix)
			}
			bySession[session] = append(bySession[session], obj)
			if obj.Size > 0 {
				hasData = true
			}
		}
		if !hasData {
			continue
		}
		var latest string
		for session, objs := range bySession {
			if !hasDataObject(objs) {
				continue
			}
			if latest == "" || sessionAfter(session, latest) {
				latest = session
			}
		}
		log.Printf("Cumulative read: keeping session %s of period_id %d (%d session(s) in that period); %d earlier period(s) skipped",
			latest, periods[i].periodId, len(bySession), len(periods)-i-1)
		return bySession[latest], nil
	}
	return nil, nil
}

func hasDataObject(objs []*awsi.S3Object) bool {
	for _, obj := range objs {
		if obj.Size > 0 {
			return true
		}
	}
	return false
}

// sessionSegment returns the first path segment of key below prefix, and false
// when the key has none (an object directly in the period folder).
func sessionSegment(prefix, key string) (string, bool) {
	rel, found := strings.CutPrefix(key, prefix)
	if !found {
		// ListS3Objects strips a leading '/' from the prefix
		rel, found = strings.CutPrefix(key, strings.TrimLeft(prefix, "/"))
		if !found {
			return "", false
		}
	}
	rel = strings.TrimLeft(rel, "/")
	session, _, hasMore := strings.Cut(rel, "/")
	if !hasMore || session == "" {
		return "", false
	}
	return session, true
}

// sessionAfter compares session ids numerically when both are numbers, and as
// text otherwise.
func sessionAfter(a, b string) bool {
	na, errA := strconv.ParseInt(a, 10, 64)
	nb, errB := strconv.ParseInt(b, 10, 64)
	if errA == nil && errB == nil {
		return na > nb
	}
	return a > b
}
