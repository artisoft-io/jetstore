package datatable

// Recording a job's failure.
//
// A job is work JetStore runs that is not a pipeline: the purge Lambda, the
// register keys Lambda, a workspace's custom Lambda. Until this file such a
// failure was written to the job's log and nowhere else, so nothing that reads
// the run record, the Pipelines Status screen or the agentic triage, could see
// it. A workspace's custom Lambda that did record one wrote a failed run under a
// fixed, shared session id, so every failure of every job read as one session.
//
// RecordJobFailure writes one failed pipeline_execution_status row under a
// session of its own, registered like any other, so the row shows on the
// Pipelines Status screen, the session on the Session Registry screen, and the
// retention purge deletes both together once they are old enough. The row's
// pipeline_config_key is 0, which is what marks it as a job's: no pipeline
// configuration has key 0, and a reader that needs a configuration
// (resubmission, triage) tells the two apart by it.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The jobs of JetStore itself that record their failures, by the process_name
// their row carries. A workspace's custom Lambda records under a name of its
// own.
const (
	// JobPurgeData is the purge Lambda (jets/purge_database).
	JobPurgeData = "purge_data"
	// JobRegisterKeys is the register keys Lambda, which registers the files
	// that arrive in the bucket (cdk/jetstore_one/lambdas/register_keys).
	JobRegisterKeys = "register_keys"
)

const (
	// SessionSourceTypeJob is the session_registry source_type of a job's
	// failure. A value of its own, so nothing that reads a session's source type
	// takes the failure for a file's delivery or a pipeline's output.
	SessionSourceTypeJob = "job"
	// FailureClassJobFailed is the failure_class of a job's failure. The column
	// otherwise holds the state machine's Error field verbatim; a job has no
	// state machine, and this class says so.
	FailureClassJobFailed = "job_failed"
	// jobUserEmail is the user_email of a job's row: no person started it.
	jobUserEmail = "system"
)

// JobFailure is one failure of a job.
type JobFailure struct {
	// Job is the job's name, the row's process_name: JobPurgeData,
	// JobRegisterKeys, or a workspace Lambda's own name. Required.
	Job string
	// Details is the error, the row's failure_details. Required.
	Details string
	// FileKey is the file the job was handling, when it was handling one; the
	// row's main_input_file_key, null when empty.
	FileKey string
	// Client is the row's and the session's client. Empty, it is the file key's
	// client when FileKey carries one, as registration reads it, and empty
	// otherwise: a job that serves no client, such as the purge.
	Client string
	// Period is the session's source period. Zero, it is the file's period when
	// FileKey carries a year, as registration reads it, and today's (UTC)
	// otherwise.
	Period time.Time
}

// RecordJobFailure records a job's failure: it reserves a session as
// registration does, registers it with source type job, and writes one failed
// pipeline_execution_status row under it. It returns the session id.
//
// The three writes are not one transaction, because ReserveSessionId and
// schema.RegisterSession each take the pool. A failure part way leaves a
// reserved or registered session with no row, which the purge deletes like any
// other: by retention once registered, as an orphan before.
func RecordJobFailure(ctx context.Context, dbpool *pgxpool.Pool, f JobFailure) (string, error) {
	f.Job = strings.TrimSpace(f.Job)
	if f.Job == "" {
		return "", errors.New("error: a job's failure needs the job's name")
	}
	if strings.TrimSpace(f.Details) == "" {
		return "", fmt.Errorf("error: the failure of job %s carries no details", f.Job)
	}
	if f.Client == "" && f.FileKey != "" {
		f.Client = fileKeyClient(ctx, dbpool, f.FileKey)
	}
	if f.Period.IsZero() {
		f.Period = fileKeyPeriod(f.FileKey, time.Now().UTC())
	}
	sourcePeriodKey, err := InsertSourcePeriod(dbpool, f.Period.Year(), int(f.Period.Month()), f.Period.Day())
	if err != nil {
		return "", fmt.Errorf("while recording the failure of job %s: %v", f.Job, err)
	}
	sessionId, err := ReserveSessionId(dbpool)
	if err != nil {
		return "", fmt.Errorf("while recording the failure of job %s: %v", f.Job, err)
	}
	if err = schema.RegisterSession(dbpool, SessionSourceTypeJob, f.Client, sessionId, sourcePeriodKey); err != nil {
		return "", fmt.Errorf("while recording the failure of job %s: %v", f.Job, err)
	}
	_, err = dbpool.Exec(ctx, `INSERT INTO jetsapi.pipeline_execution_status
			(pipeline_config_key, main_input_registry_key, main_input_file_key, merged_input_registry_keys,
			 client, process_name, main_object_type, input_session_id, session_id, source_period_key,
			 status, failure_details, failure_class, failure_source, user_email)
		VALUES (0, NULL, NULLIF($1, ''), '{}', $2, $3, '', NULL, $4, $5, 'failed', $6, $7, $8, $9)`,
		f.FileKey, f.Client, f.Job, sessionId, sourcePeriodKey,
		f.Details, FailureClassJobFailed, FailureSourceJob, jobUserEmail)
	if err != nil {
		return "", fmt.Errorf("while recording the failure of job %s in pipeline_execution_status: %v", f.Job, err)
	}
	return sessionId, nil
}

// fileKeyClient is the file key's client as registration reads it: the
// client= component, in the case client_registry gives it. A client that is not
// registered, or a registry that cannot be read, keeps the case of the key; a
// key with no client= component gives "".
func fileKeyClient(ctx context.Context, dbpool *pgxpool.Pool, fileKey string) string {
	client, _ := utils.SplitFileKeyIntoComponents(map[string]any{}, &fileKey)["client"].(string)
	if client == "" {
		return ""
	}
	var registered string
	err := dbpool.QueryRow(ctx, "SELECT client FROM jetsapi.client_registry WHERE lower(client) = $1 LIMIT 1",
		strings.ToLower(client)).Scan(&registered)
	if err == nil && registered != "" {
		return registered
	}
	return client
}

// fileKeyPeriod is the file key's period as registration reads it, when the key
// carries a year: year=, with month= and day= defaulting to 1. A key with no
// year, or a date that does not exist (a month of 13, a component that is not a
// number), gives today.
func fileKeyPeriod(fileKey string, today time.Time) time.Time {
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if fileKey == "" || !utils.FileKeyCarries(fileKey, "year") {
		return today
	}
	c := utils.SplitFileKeyIntoComponents(map[string]any{}, &fileKey)
	year, _ := c["year"].(int)
	month, _ := c["month"].(int)
	day, _ := c["day"].(int)
	p := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	// time.Date normalises a month of 13 into the next year; a date that does
	// not survive the round trip is not the file's.
	if year < 1 || p.Year() != year || int(p.Month()) != month || p.Day() != day {
		return today
	}
	return p
}
