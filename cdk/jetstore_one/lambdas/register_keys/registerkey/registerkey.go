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
	"log"
	"net/url"
	"os"
	"strings"

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

func processMessage(ctx context.Context, dbpool *pgxpool.Pool, record events.S3EventRecord) error {
	fileKey, err := url.QueryUnescape(record.S3.Object.Key)
	if err != nil {
		return fmt.Errorf("while unescaping file key: %v", err)
	}
	fileSize := record.S3.Object.Size
	log.Printf("S3 event: key: %s, size: %d\n", fileKey, fileSize)
	if strings.HasSuffix(fileKey, "/") {
		// bailing out
		return nil
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
		return doFileKey(ctx, dbpool, dtCtx, fileKey, fileSize, token)
	case strings.HasPrefix(fileKey, s3SchemaPrefix):
		// File Schema
		return doFileSchema(dbpool, dtCtx, fileKey, fileSize, token)
	default:
		// untracked file
		log.Printf("Register Key v2: got untracked file?? %s", fileKey)
		log.Printf("Note: s3InputPrefix: %s, s3SchemaPrefix: %s", s3InputPrefix, s3SchemaPrefix)
		return nil
	}
}

func doFileKey(ctx context.Context, _ *pgxpool.Pool, dtCtx *datatable.DataTableContext,
	fileKey string, fileSize int64, token string) error {

	// Extract processing date from file key inFile
	fileKeyComponents := make(map[string]any)
	fileKeyComponents = utils.SplitFileKeyIntoComponents(fileKeyComponents, &fileKey)
	fileKeyComponents["size"] = fileSize

	// The site's opportunity to decorate the registration, or to take the event
	// over. done == true is the return: there is no fall-through to a stock
	// registration the hook has already handled.
	var isSchemaEvent bool
	if siteHook != nil {
		var done bool
		var err error
		isSchemaEvent, done, err = siteHook.BeforeRegister(ctx, dtCtx, fileKey, fileKeyComponents, token)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}

	registerFileKeyAction := datatable.RegisterFileKeyAction{
		Action:        "register_keys",
		Data:          []map[string]any{fileKeyComponents},
		IsSchemaEvent: isSchemaEvent,
	}
	return registerFileKeys(dtCtx, &registerFileKeyAction, token)
}

func doFileSchema(dbpool *pgxpool.Pool, dtCtx *datatable.DataTableContext, fileKey string, fileSize int64, token string) error {

	// pre-allocate in memory buffer, where n is the object size
	buf := make([]byte, int(fileSize))
	// wrap with aws.WriteAtBuffer
	w := manager.NewWriteAtBuffer(buf)
	_, err := awsi.DownloadFromS3WithRetry(downloader, awsBucket, fileKey, nil, w)
	if err != nil {
		return fmt.Errorf("while downloading file schema from s3: %v", err)
	}
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
			err = dtCtx.RegisterSchemaEvent(dbpool, schemaInfo, token)
			if err != nil {
				return fmt.Errorf("while processing schema event from json in RegisterFileKeyV2 lambda: %v", err)
			}
		}
		return nil
	}

	return dtCtx.RegisterSchemaEvent(dbpool, &schemaInfo, token)
}
