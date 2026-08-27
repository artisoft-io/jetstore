// Package observe is the extraction step of the anomaly detectors (Phase 3
// item 9, task N.3): it reads JetStore's execution record out of Postgres and
// hands a detector typed rows, and it writes the resulting jetsa:Anomaly back.
//
// # Why this is a Go package and not a cpipes pipeline
//
// A cpipes pipeline's main input is a file or a domain_table
// (actions_start_common.go:462), and a domain_table is a domain class a rule
// session produced (actions_start_common.go:43, :122) — so no pipeline can
// read jetsapi.pipeline_execution_details at all. Three shapes were tabled for
// getting past that: export the record to S3 and read it as a file input; add
// a third sourceType arm reading jetsapi; or skip cpipes and compute the
// detectors as SQL with a Go emitter. The third was taken (N.3, 2026-08-27).
//
// The reason is where the aggregate is computed. The execution record supports
// five derivable failure modes and every one of them is a GROUP BY or a
// predicate over rows of a relational table — no unification, no class
// hierarchy, no chain of inferences over facts a previous rule asserted. Both
// of the other two shapes exist to move those rows out of the database that
// already computes the answer, into an engine that would recompute it.
//
// The export-to-S3 shape stays as the documented upgrade path: if the detector
// set stops being a handful of predicates, or a threshold starts depending on
// what an Anomaly is *about* rather than on which column moved, the work here
// is a SQL statement to port and a source_config row to add.
//
// # What this package does and does not know
//
// It reads the record. It does not read a pipeline's configuration, and that
// boundary is load-bearing rather than tidy: five of the fourteen confounders
// the Anomaly vocabulary carries — on_error_drop, max_input_count,
// sampling_cap, device_writer_output and parquet_input — are properties of the
// config and appear nowhere in the execution record. A detector that must
// qualify itself with one of those has to find it elsewhere, and
// cpipes_execution_status.cpipes_config_json holds at most one step's config.
// The confounders this package sets are only the ones the record shows.
//
// # Two things about the record that decide the shape of every query here
//
// The worker row is self-keying. jetsapi.pipeline_execution_details carries
// client, process_name, source_period_key and session_id of its own, written
// at the insert (InsertPipelineExecutionStatus,
// jets/compute_pipes/actions_process_file.go:302), so a within-run predicate
// needs no join to the run header at all. Only main_object_type has to come
// from the header.
//
// And the header can be gone. The worker record is purged by session_id
// against RETENTION_DAYS, an environment variable with no default, while the
// header is deleted on its own clock at 30*6 days
// (DoPurgeSessions, jets/purge_database/delegate/delegate.go:122). The two
// clocks are independent, and in three of the four production environments
// measured on 2026-08-25 the worker rows outlive their headers — by nine
// months in one. So every join to the header in this package is an outer join
// and the rows it could not key are counted rather than dropped silently.
package observe

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB is the slice of pgx the extraction needs — satisfied by *pgxpool.Pool,
// *pgx.Conn and pgx.Tx alike, so a detector can read inside a caller's
// transaction.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Exec is what InsertAnomaly needs; same implementors as DB.
type Exec interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}
