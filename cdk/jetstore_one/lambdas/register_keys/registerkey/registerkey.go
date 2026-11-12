// Package registerkey is the body of the register_keys_v2 Lambda, extracted so a
// site can deploy its own entry point over it with one decoration Hook rather than
// a fork of the whole lambda.
//
// register_keys_v2/main.go is func main() { registerkey.Run(nil) } and nothing
// else, so the stock deployment is unchanged. A site's entry point is
// registerkey.Run(&itsOwnHook{}); everything else -- the argument validation, the
// dbc credential-refresh lifecycle, the input-versus-schema-trigger routing and
// the single-or-slice schema unmarshalling -- stays here and changes for JetStore's
// own reasons.
package registerkey

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/cdk/jetstore_one/lambdas/dbc"
	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	s3InputPrefix  string = os.Getenv("JETS_s3_INPUT_PREFIX")
	s3SchemaPrefix string = os.Getenv("JETS_s3_SCHEMA_TRIGGERS")
	systemUser     string = "system"
	awsRegion      string = os.Getenv("JETS_REGION")
	awsBucket      string = os.Getenv("JETS_BUCKET")
	downloader     *transfermanager.Client
	dbConnection   *dbc.DbConnection

	// siteHook is the Hook passed to Run, nil for the stock lambda. It is written
	// once, before lambda.Start, and read by doFileKey.
	siteHook Hook
)

// registerFileKeys is the call doFileKey makes once it has built the action. It is
// a variable because it is the seam the doFileKey tests assert through: a test
// replaces it to capture the constructed RegisterFileKeyAction, which is the only
// way to check what the stock path builds without a database. Production code must
// not reassign it.
var registerFileKeys = func(dtCtx *datatable.DataTableContext,
	action *datatable.RegisterFileKeyAction, token string) error {
	_, _, err := dtCtx.RegisterFileKeys(action, token)
	return err
}

// downloadSchemaEvent and registerSchemaEvent are the two calls doFileSchema
// makes outside this package, variables for the same reason as registerFileKeys:
// the doFileSchema tests replace them with a fake object and a capture. Production
// code must not reassign them.
var downloadSchemaEvent = func(fileKey, versionId string, w io.WriterAt) (int64, error) {
	return awsi.DownloadVersionFromS3WithRetry(downloader, awsBucket, fileKey, versionId, nil, w)
}

// claimS3Event is claimEvent, a variable so processMessage's tests can run
// without a database. Production code must not reassign it.
var claimS3Event = func(ctx context.Context, dbpool *pgxpool.Pool, fileKey, sequencer string,
	inFlight time.Duration) (bool, string, string, error) {
	return claimEvent(ctx, dbpool, fileKey, sequencer, inFlight)
}

// failS3Event marks the event failed after a failed registration, so that
// Lambda's retry takes it over and resumes. A failure is logged: the retry then
// finds the row started and takes it over once the in-flight window has passed.
var failS3Event = func(fileKey, sequencer string, dbpool *pgxpool.Pool) {
	if err := failEvent(context.Background(), dbpool, fileKey, sequencer); err != nil {
		log.Printf("WARNING %v", err)
	}
}

// recordS3Session records the input session the registration reserved. A failure
// is logged: a delivery taking the event over then registers under a new session,
// as every one did before the session was recorded.
var recordS3Session = func(fileKey, sequencer, sessionId string, dbpool *pgxpool.Pool) {
	if err := recordSession(context.Background(), dbpool, fileKey, sequencer, sessionId); err != nil {
		log.Printf("WARNING %v", err)
	}
}

// eventResume carries the duplicate check's part in a file key's registration:
// the session to resume under, and the call that records a newly reserved one.
// Its zero value registers as before the check existed.
type eventResume struct {
	sessionId  string
	onReserved func(sessionId string)
}

// completeS3Event marks the event registered. A failure is logged: the row stays
// started, and a re-delivery after the in-flight window registers the file again,
// which is what happened to every re-delivery before the record existed.
var completeS3Event = func(fileKey, sequencer string, dbpool *pgxpool.Pool) {
	if err := completeEvent(context.Background(), dbpool, fileKey, sequencer); err != nil {
		log.Printf("WARNING %v", err)
	}
}

var registerSchemaEvent = func(dtCtx *datatable.DataTableContext, dbpool *pgxpool.Pool,
	schemaInfo *compute_pipes.SchemaProviderSpec, token string) error {
	return dtCtx.RegisterSchemaEvent(dbpool, schemaInfo, token)
}

func init() {
	var err error
	downloader, err = awsi.NewDownloader(awsRegion)
	if err != nil {
		log.Fatalf("while init s3 downloader for region %s: %v", awsRegion, err)
	}
}

// Run is the whole lambda: argument checks, the dbc connection, lambda.Start, the
// S3 prefix routing, doFileSchema, and the doFileKey body. A nil hook is
// register_keys_v2 exactly as it stands today.
func Run(hook Hook) {
	siteHook = hook
	utils.UseJetStoreLogger()
	hasErr := false
	var errMsg []string
	var err error
	if os.Getenv("JETS_DSN_SECRET") == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string must be provided using env JETS_DSN_SECRET")
	}
	if awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region must be provided using env JETS_REGION")
	}
	if awsBucket == "" {
		hasErr = true
		errMsg = append(errMsg, "Bucket must be provided using env var JETS_BUCKET")
	}

	if hasErr {
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		log.Panic("Invalid argument(s)")
	}

	// Open the db connection
	dbConnection, err = dbc.NewDbConnection(5)
	if err != nil {
		log.Panicf("while opening db connection: %v", err)
	}
	defer dbConnection.ReleaseConnection()

	log.Println("Register Key v2 ready!")
	lambda.Start(handler)
}

func handler(ctx context.Context, s3Event events.S3Event) error {
	// Check if the db credential have been updated
	dbpool, err := dbConnection.GetConnection()
	if err != nil {
		return fmt.Errorf("while checking if db credential have been updated: %v", err)
	}
	// Process the records
	// log.Print("***Register Key v2 called with", s3Event)
	for _, record := range s3Event.Records {
		err := processMessage(ctx, dbpool, record)
		if err != nil {
			log.Println("Got error while processing record:", err)
			return err
		}
	}
	return nil
}

func processMessage(ctx context.Context, dbpool *pgxpool.Pool, record events.S3EventRecord) (err error) {
	fileKey, err := url.QueryUnescape(record.S3.Object.Key)
	if err != nil {
		return fmt.Errorf("while unescaping file key: %v", err)
	}
	fileSize := record.S3.Object.Size
	sequencer := record.S3.Object.Sequencer
	log.Printf("S3 event: key: %s, size: %d, sequencer: %s\n", fileKey, fileSize, sequencer)
	if strings.HasSuffix(fileKey, "/") {
		// bailing out
		return nil
	}

	// A re-delivery of an event already registered, or being registered, registers
	// nothing (event_claim.go). An event with no sequencer was not built by S3 and
	// is registered as it always was. A failure to record the event does not hold
	// up the file: it is registered unchecked, as it was before the record existed.
	//
	// A delivery that takes over an unfinished registration of a file key resumes
	// it under the session the earlier invocation recorded. A schema event is
	// registered afresh: its file can hold several registrations, and only one
	// session is recorded.
	var resume eventResume
	tracked := strings.HasPrefix(fileKey, s3InputPrefix) || strings.HasPrefix(fileKey, s3SchemaPrefix)
	if tracked && sequencer != "" {
		claimed, status, sessionId, claimErr := claimS3Event(ctx, dbpool, fileKey, sequencer, inFlightWindow(ctx))
		switch {
		case claimErr != nil:
			log.Printf("WARNING registering %s without the duplicate check: %v", fileKey, claimErr)
		case !claimed:
			log.Printf("S3 event for %s (sequencer %s) is a re-delivery of one already %s; not registered again",
				fileKey, sequencer, status)
			return nil
		default:
			if sessionId != "" {
				log.Printf("S3 event for %s (sequencer %s) was left unfinished by an earlier invocation; resuming under session %s",
					fileKey, sequencer, sessionId)
			}
			resume = eventResume{
				sessionId: sessionId,
				onReserved: func(reserved string) {
					recordS3Session(fileKey, sequencer, reserved, dbpool)
				},
			}
			defer func() {
				if r := recover(); r != nil {
					failS3Event(fileKey, sequencer, dbpool)
					panic(r)
				}
				if err != nil {
					failS3Event(fileKey, sequencer, dbpool)
					return
				}
				completeS3Event(fileKey, sequencer, dbpool)
			}()
		}
	}

	token, err := user.CreateToken(systemUser)
	if err != nil {
		return fmt.Errorf("error creating jwt token: %v", err)
	}
	dtCtx := datatable.NewDataTableContext(dbpool, false, false, nil, &systemUser)

	// Determine the event source: file key or schema file?
	switch {
	case strings.HasPrefix(fileKey, s3InputPrefix):
		// File Key Event
		return doFileKey(ctx, dbpool, dtCtx, fileKey, fileSize, token, resume)
	case strings.HasPrefix(fileKey, s3SchemaPrefix):
		// File Schema
		return doFileSchema(dbpool, dtCtx, fileKey, record.S3.Object.VersionID, fileSize, token)
	default:
		// untracked file
		log.Printf("Register Key v2: got untracked file?? %s", fileKey)
		log.Printf("Note: s3InputPrefix: %s, s3SchemaPrefix: %s", s3InputPrefix, s3SchemaPrefix)
		return nil
	}
}

func doFileKey(ctx context.Context, _ *pgxpool.Pool, dtCtx *datatable.DataTableContext,
	fileKey string, fileSize int64, token string, resume eventResume) error {

	// Extract processing date from file key inFile
	fileKeyComponents := make(map[string]any)
	fileKeyComponents = utils.SplitFileKeyIntoComponents(fileKeyComponents, &fileKey)
	fileKeyComponents["size"] = fileSize

	// The site's opportunity to decorate the registration, or to take the event
	// over. done == true is the return: there is no fall-through to a stock
	// registration the hook has already handled.
	var isSchemaEvent bool
	if siteHook != nil {
		before := make(map[string]bool, len(fileKeyComponents))
		for k := range fileKeyComponents {
			before[k] = true
		}
		var done bool
		var err error
		isSchemaEvent, done, err = siteHook.BeforeRegister(ctx, dtCtx, fileKey, fileKeyComponents, token)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		warnOfUnreadHookKeys(fileKey, before, fileKeyComponents)
	}

	registerFileKeyAction := datatable.RegisterFileKeyAction{
		Action:            "register_keys",
		Data:              []map[string]any{fileKeyComponents},
		IsSchemaEvent:     isSchemaEvent,
		ResumeSessionId:   resume.sessionId,
		OnSessionReserved: resume.onReserved,
	}
	return registerFileKeys(dtCtx, &registerFileKeyAction, token)
}

// warnOfUnreadHookKeys logs a warning naming each key the hook added to
// components that RegisterFileKeys does not read, which it would drop without a
// word: a misspelt key is otherwise a registration that silently lacks what the
// site meant to add (JM-096). Keys the file key's path put there are not the
// hook's and are not named.
func warnOfUnreadHookKeys(fileKey string, before map[string]bool, components map[string]any) {
	var unread []string
	for k := range components {
		if !before[k] && !datatable.RegisterFileKeysReads(k) {
			unread = append(unread, k)
		}
	}
	if len(unread) > 0 {
		sort.Strings(unread)
		log.Printf("WARNING register key %s: the site hook added key(s) %q that RegisterFileKeys does not read; they are dropped",
			fileKey, unread)
	}
}

// doFileSchema downloads the schema event the S3 event names and registers it.
// The version is the one the event was raised for, when the bucket is versioned
// (versionId is empty otherwise), so a key overwritten since is not read in its
// place. The event's size only sizes the first allocation: the body is what was
// downloaded, which is not that size when the object changed (I-37).
func doFileSchema(dbpool *pgxpool.Pool, dtCtx *datatable.DataTableContext, fileKey, versionId string,
	fileSize int64, token string) error {

	w := manager.NewWriteAtBuffer(make([]byte, 0, max(fileSize, 0)))
	_, err := downloadSchemaEvent(fileKey, versionId, w)
	if err != nil {
		return fmt.Errorf("while downloading file schema from s3: %v", err)
	}
	buf := w.Bytes()
	// log.Printf("*** Got file schema from s3:\n%s\n", string(buf))
	var schemaInfo compute_pipes.SchemaProviderSpec
	err = json.Unmarshal(buf, &schemaInfo)
	if err != nil {
		// check to see if we have a slice of events instead of a single event
		var schemaInfoSlice []*compute_pipes.SchemaProviderSpec
		err2 := json.Unmarshal(buf, &schemaInfoSlice)
		if err2 != nil {
			return fmt.Errorf("while unmarshalling schema info from json in RegisterFileKeyV2 lambda: %v", err)
		}
		if len(schemaInfoSlice) == 0 {
			return fmt.Errorf("while unmarshalling schema info from json in RegisterFileKeyV2 lambda: got empty slice")
		}
		// process each event in the slice
		for _, schemaInfo := range schemaInfoSlice {
			err = registerSchemaEvent(dtCtx, dbpool, schemaInfo, token)
			if err != nil {
				return fmt.Errorf("while processing schema event from json in RegisterFileKeyV2 lambda: %v", err)
			}
		}
		return nil
	}

	return registerSchemaEvent(dtCtx, dbpool, &schemaInfo, token)
}
