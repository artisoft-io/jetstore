package compute_pipes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The run manifest: what a run's document promised, and what the run wrote.
//
// Its subject is the *declaration* -- the document's own output_tables and
// output_files -- and never the channel type. An edge of the compute graph is
// how the engine got there; a deliverable is what the author asked for. The two
// are joined on output_channel, which is the same string in both:
// output_tables[].key for a table sink and the OutputFileSpec key for a file
// one (ComputePipesResult.OutputChannel, compute_pipes_results.go).
//
// It is whole or it is absent. A manifest is trustworthy because it is written
// once, at the end of the run, by something that knows every worker finished --
// so it describes every declared deliverable of the whole run or it is not
// written at all. A consumer that finds no manifest knows the run did not
// complete; a consumer that finds one and then has to ask which steps it covers
// has nothing.
//
// It is produced by StatusUpdate.CoordinateWork (jets/datatable/status_update.go)
// after that function has computed the run's terminal status, and only when
// that status is "completed". Nothing else writes it.
//
// # The type is not called RunManifest, deliberately
//
// That name belongs to healthcare_corpus's generator, for a different document
// carrying one file per table, and the two documents sit in the same bucket. A
// reader who meets "RunManifest" in a JetStore stack trace and reaches for that
// model finds fields that do not exist here.

// RunManifestSchema is the document's self-declared kind and version, written
// as the first field of every manifest.
//
// It exists because the manifest is read outside this repository, by parties
// who cannot check what produced it and who will find a second document of the
// same name beside it in the same bucket. The version is the cheap half of
// making the schema changeable later: a consumer that pins it fails loudly on a
// shape change rather than reading a field that moved underneath it.
const RunManifestSchema = "jetstore.cpipes.run_manifest/v1"

// RunManifestFileName is the object the manifest is written as, under the run's
// own stage prefix.
const RunManifestFileName = "run_manifest.json"

// Values of CpipesManifestEntry.DeclaredIn: which array of the document
// declared the entry. The observed half does not carry this -- it is the
// authored split, and keeping it per entry is what makes one entry list
// sufficient rather than two arrays mirroring the document.
const (
	DeclaredInOutputTables = "output_tables"
	DeclaredInOutputFiles  = "output_files"
)

// CpipesManifestEntry is one declared deliverable and what the run wrote for it.
//
// One shape, with Location as the discriminator: "sql://<schema>.<table>" is a
// database deliverable and carries a row count and no parts, "s3://<bucket>/<key>"
// is a file one and carries both. The scheme is a string the engine already
// writes (ComputePipesResult.OutputLocation), so nothing here is derived or
// invented.
type CpipesManifestEntry struct {
	// Channel is the declared key, and is the join between the two halves.
	Channel string `json:"channel"`
	// DeclaredIn is DeclaredInOutputTables or DeclaredInOutputFiles.
	DeclaredIn string `json:"declared_in"`
	// DeclaredName is the name the document gave the deliverable, which is not
	// the channel: most output_tables entries in the rule corpus have a key
	// differing from the table name. It may still carry an unexpanded
	// environment reference, being the document's text rather than the run's;
	// Location is the resolved one.
	DeclaredName string `json:"declared_name,omitempty"`
	// Written says whether the run wrote anything for this declaration. False
	// with no Location is a declared deliverable the run did not produce, which
	// is a fact about the run rather than a gap in the manifest.
	Written bool `json:"written"`
	// Location is where it went, as a URI with environment variables already
	// substituted. Empty when Written is false.
	Location string `json:"location,omitempty"`
	// OutputType is the engine's own sink kind: SinkDbTable or SinkOutputFile.
	// It is redundant with Location's scheme by construction, and is carried so
	// that a consumer reading one of them need not parse the other.
	OutputType string `json:"output_type,omitempty"`
	// RecordsCount is null when no row count exists, rather than 0, which is a
	// different statement: the merge_files multipart-copy path moves bytes and
	// never parses a record, and InsertChannelExecutionDetails records NULL for
	// it deliberately. A consumer reading 0 there would be reading a
	// measurement that was never made.
	RecordsCount *int64 `json:"records_count"`
	// PartsCount is null for a sql:// entry, which has no parts.
	PartsCount *int64 `json:"parts_count"`
	// SinksCount is how many sink instances the run folded into this
	// deliverable -- a splitter writing one output channel into many partitions
	// reports one each.
	SinksCount int `json:"sinks_count,omitempty"`
	// ErrorMessage carries whatever the writers reported for this edge. A
	// completed run can still have carried an error on a channel.
	ErrorMessage string `json:"error_message,omitempty"`
	// Note is the manifest's own reservation about an entry, in prose, for a
	// consumer who cannot check. Empty is the ordinary case.
	Note string `json:"note,omitempty"`
}

// CpipesRunManifest is the run's manifest: one document describing every
// declared deliverable of the whole run.
type CpipesRunManifest struct {
	Schema               string `json:"schema"`
	SessionId            string `json:"session_id"`
	ProcessName          string `json:"process_name"`
	PipelineExecutionKey int    `json:"pipeline_execution_key"`
	// Status is the run's terminal status as StatusUpdate computed it. It is
	// "completed" in every manifest that exists, and is written down so that
	// the document says what it is rather than leaving a consumer to infer it
	// from the document's own existence.
	Status string `json:"status"`
	// WrittenAt is when the manifest was produced, in UTC.
	WrittenAt time.Time             `json:"written_at"`
	Entries   []CpipesManifestEntry `json:"entries"`
}

// RunManifestFileKey builds the manifest's object key. It is run-scoped: no
// step_id= and no jets_partition= component, exactly as the sibling
// input_parquet_schema.json key is built (parquet_read_file.go).
func RunManifestFileKey(stagePrefix, processName, sessionId string) string {
	return fmt.Sprintf("%s/process_name=%s/session_id=%s/%s",
		stagePrefix, processName, sessionId, RunManifestFileName)
}

// ObservedChannel is one row of the manifest's observed half: what the run
// actually wrote on one (output_type, output_channel) edge, summed over every
// worker of every step.
type ObservedChannel struct {
	OutputType    string
	OutputChannel string
	// OutputLocation is the edge's destination. LocationCount is how many
	// distinct locations the group folded: it is 1 in every case the engine
	// produces, the location being the edge's prefix rather than the sink's
	// path, and the manifest says so on the entry when it is not.
	OutputLocation string
	LocationCount  int
	SinksCount     int
	RecordsCount   int64
	// RecordsUnknown is true when any row of the group recorded a NULL count.
	// Summing a number with a non-number gives a number that means nothing,
	// which is AggregateChannelResults' own rule one grain up.
	RecordsUnknown bool
	PartsCount     int64
	ErrMsg         string
}

// observedChannelsSQL is the observed half in one statement.
//
// One GROUP BY covers the whole run because pipeline_execution_channel_details
// carries session_id on the row: the parent key gives the worker and the
// session id gives the run. No join is needed, and no accumulation across
// reducing iterations is either.
//
// The NULL handling is the point rather than a detail. sum() ignores NULLs, so
// a group mixing measured and unmeasurable sinks would silently report the
// measured part as the total; count(*) - count(output_records_count) is how the
// manifest learns to say "not measured" instead.
const observedChannelsSQL = `
	SELECT output_type,
	       output_channel,
	       min(output_location) AS output_location,
	       count(DISTINCT output_location)::int AS location_count,
	       coalesce(sum(output_sinks_count), 0)::bigint AS sinks_count,
	       coalesce(sum(output_records_count), 0)::bigint AS records_count,
	       (count(*) - count(output_records_count))::bigint AS unmeasured_rows,
	       coalesce(sum(parts_count), 0)::bigint AS parts_count,
	       string_agg(nullif(error_message, ''), ',') AS error_message
	FROM jetsapi.pipeline_execution_channel_details
	WHERE session_id = $1
	GROUP BY output_type, output_channel
	ORDER BY output_type, output_channel`

// ReadObservedChannels reads what the run wrote, for every edge of every worker
// of every step, in one query.
func ReadObservedChannels(ctx context.Context, dbpool *pgxpool.Pool, sessionId string) ([]ObservedChannel, error) {
	rows, err := dbpool.Query(ctx, observedChannelsSQL, sessionId)
	if err != nil {
		return nil, fmt.Errorf("while reading pipeline_execution_channel_details for session %q: %v",
			sessionId, err)
	}
	defer rows.Close()
	observed := make([]ObservedChannel, 0)
	for rows.Next() {
		var o ObservedChannel
		var sinks, unmeasured int64
		var errMsg *string
		if err := rows.Scan(&o.OutputType, &o.OutputChannel, &o.OutputLocation, &o.LocationCount,
			&sinks, &o.RecordsCount, &unmeasured, &o.PartsCount, &errMsg); err != nil {
			return nil, fmt.Errorf("while scanning pipeline_execution_channel_details for session %q: %v",
				sessionId, err)
		}
		o.SinksCount = int(sinks)
		o.RecordsUnknown = unmeasured > 0
		if errMsg != nil {
			o.ErrMsg = *errMsg
		}
		observed = append(observed, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading pipeline_execution_channel_details for session %q: %v",
			sessionId, err)
	}
	return observed, nil
}

// RunManifestMeta is what the manifest knows about the run that neither of its
// two halves carries.
type RunManifestMeta struct {
	SessionId            string
	ProcessName          string
	PipelineExecutionKey int
	Status               string
	WrittenAt            time.Time
}

// BuildRunManifest joins the document's declarations to what the run wrote.
//
// The entry list is exactly the declarations. An observed edge matching no
// declaration is not a deliverable and is not in the manifest -- a
// jets_partition row is a shard, an intermediate channel never left the
// process, and D-255 makes the author's declaration the subject. That is also
// the whole of why OutputEntity is not read here: AggregateChannelResults
// blanks it when an edge folds several sinks, so the field whose name most
// sounds like "the deliverable" is empty on exactly the configurations that fan
// out, and for a jets_partition sink it holds a shard label rather than a name.
//
// It refuses rather than writing a location it knows to be a lie. A recorded
// bucket still carrying a "${...}" reference is not a bucket name, and a
// manifest exists to be believed by a consumer who cannot check.
func BuildRunManifest(cfg *ComputePipesConfig, observed []ObservedChannel,
	meta RunManifestMeta) (*CpipesRunManifest, error) {
	if cfg == nil {
		return nil, errors.New("error: cannot build a run manifest without the run's configuration")
	}
	byEdge := make(map[string]*ObservedChannel, len(observed))
	for i := range observed {
		o := &observed[i]
		byEdge[o.OutputType+"\x00"+o.OutputChannel] = o
	}
	entries := make([]CpipesManifestEntry, 0, len(cfg.OutputTables)+len(cfg.OutputFiles))
	for _, spec := range cfg.OutputTables {
		if spec == nil {
			continue
		}
		e, err := manifestEntry(spec.Key, DeclaredInOutputTables, spec.Name, SinkDbTable, byEdge)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *e)
	}
	for i := range cfg.OutputFiles {
		spec := &cfg.OutputFiles[i]
		name := spec.FileName2
		if name == "" {
			name = spec.FileName
		}
		e, err := manifestEntry(spec.Key, DeclaredInOutputFiles, name, SinkOutputFile, byEdge)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *e)
	}
	writtenAt := meta.WrittenAt
	if writtenAt.IsZero() {
		writtenAt = time.Now()
	}
	return &CpipesRunManifest{
		Schema:               RunManifestSchema,
		SessionId:            meta.SessionId,
		ProcessName:          meta.ProcessName,
		PipelineExecutionKey: meta.PipelineExecutionKey,
		Status:               meta.Status,
		WrittenAt:            writtenAt.UTC(),
		Entries:              entries,
	}, nil
}

// manifestEntry builds one entry from a declaration and the observed edge that
// matches it, if any.
func manifestEntry(channel, declaredIn, declaredName, outputType string,
	byEdge map[string]*ObservedChannel) (*CpipesManifestEntry, error) {
	e := &CpipesManifestEntry{
		Channel:      channel,
		DeclaredIn:   declaredIn,
		DeclaredName: declaredName,
	}
	o := byEdge[outputType+"\x00"+channel]
	if o == nil {
		return e, nil
	}
	note, err := checkManifestLocation(o.OutputLocation, channel)
	if err != nil {
		return nil, err
	}
	e.Written = true
	e.Location = o.OutputLocation
	e.OutputType = o.OutputType
	e.SinksCount = o.SinksCount
	e.ErrorMessage = o.ErrMsg
	e.Note = note
	if !o.RecordsUnknown {
		count := o.RecordsCount
		e.RecordsCount = &count
	}
	// A sql:// deliverable has no parts. The column is summed over its workers
	// anyway and is 0 there, and 0 parts would read as a measurement.
	if !strings.HasPrefix(o.OutputLocation, "sql://") {
		parts := o.PartsCount
		e.PartsCount = &parts
	}
	if o.LocationCount > 1 {
		e.Note = strings.TrimSpace(e.Note + fmt.Sprintf(
			" the run recorded %d distinct locations for this channel and the manifest names one of them;"+
				" the location is meant to be the edge's rather than the sink's", o.LocationCount))
	}
	return e, nil
}

// checkManifestLocation applies the resolved-bucket rule at the one place where
// the consequence is a consumer who cannot check.
//
// An unresolved bucket is an error and refuses the whole manifest: recording it
// would produce a document naming a bucket nobody can open, asserted by an
// artefact whose only job is to be believed. The sentinel is not an error --
// every writer in this tree resolves it before recording, so a location
// carrying it is a surprise rather than a failure -- but it is unresolvable by
// a consumer, so it is noted on the entry.
func checkManifestLocation(location, channel string) (string, error) {
	rest, ok := strings.CutPrefix(location, "s3://")
	if !ok {
		return "", nil
	}
	bucket, _, _ := strings.Cut(rest, "/")
	kind, err := ClassifyBucket(bucket)
	if err != nil {
		return "", fmt.Errorf(
			"error: refusing to write the run manifest: output channel %q was written to %q, whose bucket "+
				"is unresolved (%v); a manifest that records an unresolved bucket names a location no "+
				"consumer can open, and a consumer of a manifest cannot check", channel, location, err)
	}
	if kind == JetStoreBucket {
		return "the location names the deployment's own bucket by sentinel rather than by name," +
			" so it is not resolvable outside the deployment", nil
	}
	return "", nil
}

// runManifestMetaSQL reads what the manifest knows about the run, and the whole
// document it declares.
//
// cpipes_startup_json rather than cpipes_config_json: the latter holds at most
// ONE step's config -- the sharding step's, overwritten by each reducing step --
// so its output_tables is the last step to have started rather than the run's.
// cpipes_startup_json carries CpipesStartup, whose CpConfig is the whole
// document, and each reducing iteration rewrites it whole
// (actions_start_reducing_cp.go).
const runManifestMetaSQL = `
	SELECT pe.key, pe.process_name, ces.cpipes_startup_json
	FROM jetsapi.cpipes_execution_status ces, jetsapi.pipeline_execution_status pe
	WHERE ces.session_id = $1 AND ces.pipeline_execution_status_key = pe.key`

// RunManifestUploader is awsi.UploadBufToS3, indirected so that the producer's
// own test can observe the S3 half without an AWS account.
//
// It is exported because the producer is in another package: the branch that
// decides whether a manifest exists at all lives in
// StatusUpdate.CoordinateWork, so the test that drives all five run statuses
// and counts the manifests has to be able to see this half from there. Nothing
// outside a test assigns it.
var RunManifestUploader = awsi.UploadBufToS3

// WriteRunManifest builds the run's manifest and writes it to both stores: the
// object at <stage>/process_name=<p>/session_id=<s>/run_manifest.json, and
// cpipes_execution_status.run_manifest_json keyed by session_id.
//
// The caller must have established that the run's terminal status is
// "completed" before calling this, and passes it in. It is deliberately not
// recomputed here: the status is the caller's, and passing it makes the branch
// visible at the call site, which is where the error of writing a manifest for
// a run that did not complete would be made.
//
// It attempts both stores even when the first fails, and joins the errors. One
// store landing is better than neither, and a caller logging "the S3 write
// failed" while the database had quietly also failed would be reporting half a
// fact.
func WriteRunManifest(ctx context.Context, dbpool *pgxpool.Pool, sessionId, status string) error {
	var peKey int
	var processName, startupJson string
	err := dbpool.QueryRow(ctx, runManifestMetaSQL, sessionId).Scan(&peKey, &processName, &startupJson)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("error: no cpipes_execution_status row for session %q, so there is no "+
			"configuration to take the manifest's declared half from", sessionId)
	case err != nil:
		return fmt.Errorf("while reading the run's configuration for session %q: %v", sessionId, err)
	}
	var startup CpipesStartup
	if err := json.Unmarshal([]byte(startupJson), &startup); err != nil {
		return fmt.Errorf("while unmarshalling cpipes_startup_json for session %q: %v", sessionId, err)
	}
	observed, err := ReadObservedChannels(ctx, dbpool, sessionId)
	if err != nil {
		return err
	}
	manifest, err := BuildRunManifest(&startup.CpConfig, observed, RunManifestMeta{
		SessionId:            sessionId,
		ProcessName:          processName,
		PipelineExecutionKey: peKey,
		Status:               status,
		WrittenAt:            time.Now(),
	})
	if err != nil {
		return err
	}
	buf, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("while making json from the run manifest of session %q: %v", sessionId, err)
	}
	fileKey := RunManifestFileKey(awsi.JetStoreStagePrefix(), processName, sessionId)
	var errs []error
	if err := RunManifestUploader("", fileKey, buf); err != nil {
		errs = append(errs, fmt.Errorf("while uploading the run manifest to %s: %v", fileKey, err))
	}
	stmt := `UPDATE jetsapi.cpipes_execution_status SET run_manifest_json = $1 WHERE session_id = $2`
	if _, err := dbpool.Exec(ctx, stmt, string(buf), sessionId); err != nil {
		errs = append(errs, fmt.Errorf(
			"while updating run_manifest_json in jetsapi.cpipes_execution_status: %v", err))
	}
	return errors.Join(errs...)
}
