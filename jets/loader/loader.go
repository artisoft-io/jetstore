package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	// "sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Command Line Arguments
// --------------------------------------------------------------------------------------
// Single character type for csv options
type chartype rune

func (s *chartype) String() string {
	return fmt.Sprintf("%#U", *s)
}

func (s *chartype) Set(value string) error {
	r := []rune(value)
	if len(r) > 1 || r[0] == '\n' {
		return errors.New("sep must be a single char not '\\n'")
	}
	*s = chartype(r[0])
	return nil
}

// Loader env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_DSN_URI_VALUE
// JETS_DSN_JSON_VALUE
// LOADER_ERR_DIR
// JETS_DOMAIN_KEY_HASH_ALGO (values: md5, sha1, none (default: none))
// JETS_DOMAIN_KEY_HASH_SEED (required for md5 and sha1. MUST be a valid uuid )
// JETS_INPUT_ROW_JETS_KEY_ALGO (values: uuid, row_hash, domain_key (default: uuid))
// JETS_ADMIN_EMAIL (set as admin in dockerfile)
// JETSTORE_DEV_MODE Indicates running in dev mode
// AWS_API_SECRET or API_SECRET
var awsDsnSecret = flag.String("awsDsnSecret", "", "aws secret with dsn definition (aws integration) (required unless -dsn is provided)")
var awsRegion = flag.String("awsRegion", "", "aws region to connect to for aws secret and bucket (aws integration) (required if -awsDsnSecret or -awsBucket is provided)")
var awsBucket = flag.String("awsBucket", "", "Bucket having the the input csv file (aws integration)")
var usingSshTunnel = flag.Bool("usingSshTunnel", false, "Connect  to DB using ssh tunnel (expecting the ssh open)")
var inFile = flag.String("in_file", "", "the input csv file name (required)")
var dropTable = flag.Bool("d", false, "drop table if it exists, default is false")
var dsn = flag.String("dsn", "", "Database connection string (required unless -awsDsnSecret is provided)")
var client = flag.String("client", "", "Client associated with the source location (required)")
var objectType = flag.String("objectType", "", "The type of object contained in the file (required)")
var userEmail = flag.String("userEmail", "", "User identifier to register the load (required)")
var nbrShards = flag.Int("nbrShards", 1, "Number of shards to use in sharding the input file")
var sessionId = flag.String("sessionId", "", "Process session ID, is needed as -inSessionId for the server process (must be unique), default based on timestamp.")
var doNotLockSessionId = flag.Bool("doNotLockSessionId", false, "Do NOT lock sessionId on sucessful completion (default is to lock the sessionId on successful completion")
var completedMetric = flag.String("loaderCompletedMetric", "loaderCompleted", "Metric name to register the loader successfull completion (default: loaderCompleted)")
var failedMetric = flag.String("loaderFailedMetric", "loaderFailed", "Metric name to register the load failure [success load metric: loaderCompleted] (default: loaderFailed)")
var tableName string
var domainKeysJson string
var sep_flag chartype = '€'
var errOutDir string
var jetsInputRowJetsKeyAlgo string
var clientOrg string
var sourcePeriodKey int
var inputRegistryKey []int
var devMode bool
var adminEmail string

func init() {
	flag.Var(&sep_flag, "sep", "Field separator, default is auto detect between pipe ('|') or comma (',')")
}


func truncateSessionId(dbpool *pgxpool.Pool) error {
	stmt := `DELETE FROM jetsapi.input_loader_status 
						WHERE table_name = $1 AND session_id = $2`
	_, err := dbpool.Exec(context.Background(), stmt, tableName, *sessionId)
	if err != nil {
		return fmt.Errorf("error deleting sessionId from jetsapi.input_loader_status table: %v", err)
	}
	stmt = `DELETE FROM jetsapi.input_registry WHERE table_name = $1 AND session_id = $2`
	_, err = dbpool.Exec(context.Background(), stmt, tableName, *sessionId)
	if err != nil {
		return fmt.Errorf("error deleting sessionId from jetsapi.input_registry table: %v", err)
	}
	return nil
}

func registerCurrentLoad(copyCount int64, badRowCount int, dbpool *pgxpool.Pool, 
	dkInfo *schema.HeadersAndDomainKeysInfo, status string, errMessage string) error {
	stmt := `INSERT INTO jetsapi.input_loader_status (
		object_type, table_name, client, org, file_key, session_id, source_period_key, status, error_message,
		load_count, bad_row_count, user_email) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT ON CONSTRAINT input_loader_status_unique_cstraint
			DO UPDATE SET (status, error_message, load_count, bad_row_count, user_email, last_update) =
			(EXCLUDED.status, EXCLUDED.error_message, EXCLUDED.load_count, EXCLUDED.bad_row_count, EXCLUDED.user_email, DEFAULT)`
	_, err := dbpool.Exec(context.Background(), stmt, 
		*objectType, tableName, *client, clientOrg, *inFile, *sessionId, sourcePeriodKey, status, errMessage, copyCount, badRowCount, *userEmail)
	if err != nil {
		return fmt.Errorf("error inserting in jetsapi.input_loader_status table: %v", err)
	}
	log.Println("Updated input_loader_status table with main object type:", *objectType,"client", *client, "org", clientOrg)
	if status == "completed" && dkInfo != nil {
		inputRegistryKey = make([]int, len(dkInfo.DomainKeysInfoMap))
		ipos := 0
		for objType := range dkInfo.DomainKeysInfoMap {
			log.Println("Registering staging table with object type:", objType,"client", *client, "org", clientOrg)
			stmt = `INSERT INTO jetsapi.input_registry (
				client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email) 
				VALUES ($1, $2, $3, $4, $5, $6, 'file', $7, $8) RETURNING key`
			err = dbpool.QueryRow(context.Background(), stmt, 
				*client, clientOrg, objType, *inFile, sourcePeriodKey, tableName, *sessionId, *userEmail).Scan(&inputRegistryKey[ipos])
			if err != nil {
				return fmt.Errorf("error inserting in jetsapi.input_registry table: %v", err)
			}
		}
		// Check for any process that are ready to kick off
		context := datatable.NewContext(dbpool, devMode, *usingSshTunnel, nil, *nbrShards, &adminEmail)
		token, err := user.CreateToken(*userEmail)
		if err != nil {
			return fmt.Errorf("error creating jwt token: %v", err)
		}
		context.StartPipelineOnInputRegistryInsert(&datatable.RegisterFileKeyAction{
			Action: "register_keys",
			Data: []map[string]interface{}{{
				"input_registry_keys": inputRegistryKey,
				"source_period_key": sourcePeriodKey,
				"file_key": *inFile,
			}},
		}, token)
	}
	return nil
}

// type writeResult struct {
// 	count  int64
// 	errMsg string
// }

// processFile
// --------------------------------------------------------------------------------------
func processFile(dbpool *pgxpool.Pool, fileHd, errFileHd *os.File) (*schema.HeadersAndDomainKeysInfo, int64, int, error) {

	// determine the csv separator
	// ---------------------------------------
	if sep_flag == '€' {
		// auto detect the separator based on the first line
		buf := make([]byte, 2048)
		nb, err := fileHd.Read(buf)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("error while ready first few bytes of in_file %s: %v", *inFile, err)
		}
		txt := string(buf[0:nb])
		cn := strings.Count(txt, ",")
		pn := strings.Count(txt, "|")
		tn := strings.Count(txt, "\t")
		switch {
		case (cn > pn) && (cn > tn):
			sep_flag = ','
		case (pn > cn) && (pn > tn):
			sep_flag = '|'
		case (tn > cn) && (tn > pn):
			sep_flag = '\t'
		default:
			return nil, 0, 0, fmt.Errorf("error: cannot determine the csv-delimit used in file %s",*inFile)
		}
		_, err = fileHd.Seek(0, 0)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("error while returning to beginning of in_file %s: %v", *inFile, err)
		}
	}
	fmt.Println("Got argument: sep_flag", sep_flag)

	// Get reader / writer for input and error file resp.
	csvReader := csv.NewReader(fileHd)
	csvReader.Comma = rune(sep_flag)
	csvReader.ReuseRecord = true
	badRowsWriter := bufio.NewWriter(errFileHd)
	defer badRowsWriter.Flush()

	// Read the headers, put them in err file and make them valid for db
	// Contruct the domain keys based on domainKeysJson
	// ---------------------------------------
	headersDKInfo, err := schema.NewHeadersAndDomainKeysInfo(tableName)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("while calling NewHeadersAndDomainKeysInfo: %v", err)
	}

	rawHeaders, err := csvReader.Read()
	if err == io.EOF {
		return nil, 0, 0, errors.New("input csv file is empty")
	} else if err != nil {
		return nil, 0, 0, fmt.Errorf("while reading csv headers: %v", err)
	}
	err = headersDKInfo.InitializeStagingTable(rawHeaders, *objectType, &domainKeysJson)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("while calling InitializeStagingTable: %v", err)
	}

	// // for development
	// fmt.Println("Domain Keys Info for table", tableName)
	// fmt.Println(headersDKInfo)

	// Write raw header to error file
	for i := range headersDKInfo.RawHeaders {
		if i > 0 {
			badRowsWriter.WriteRune(csvReader.Comma)
		}
		badRowsWriter.WriteString(headersDKInfo.RawHeaders[i])
	}
	_, err = badRowsWriter.WriteRune('\n')
	if err != nil {
		return nil, 0, 0, fmt.Errorf("while writing csv headers to err file: %v", err)
	}

	// prepare db connections
	// ---------------------------------------
	// validate table name
	tblExists, err := schema.TableExists(dbpool, "public", tableName)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("while validating table name: %v", err)
	}
	if !tblExists || *dropTable {
		if *dropTable {
			// remove the previous input loader status associated with sessionId
			err = truncateSessionId(dbpool)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("while truncating sessionId: %v", err)
			}
		}
		err = headersDKInfo.CreateStagingTable(dbpool, tableName)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("while creating table: %v", err)
		}
	}

	// read the rest of the file
	// ---------------------------------------
	var badRowsPos []int
	var rowid int64
	headerPos := headersDKInfo.GetHeaderPos()
	fileKeyPos := headersDKInfo.HeadersPosMap["file_key"]
	sessionIdPos := headersDKInfo.HeadersPosMap["session_id"]
	jetsKeyPos := headersDKInfo.HeadersPosMap["jets:key"]
	lastUpdatePos := headersDKInfo.HeadersPosMap["last_update"]
	lastUpdate := time.Now().UTC()

	// Get the list of ObjectType from domainKeysJson if it's an elm, detault to *objectType
	objTypes, err := schema.GetObjectTypesFromDominsKeyJson(domainKeysJson, *objectType)
	if err != nil {
		return nil, 0, 0, err
	}
	inputRows := make([][]interface{}, 0)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			// get the details of the error
			var details *csv.ParseError
			if errors.As(err, &details) {
				log.Printf("while reading csv records: %v", err)
				for i := details.StartLine; i <= details.Line; i++ {
					badRowsPos = append(badRowsPos, i)
				}
			} else {
				return nil, 0, 0, fmt.Errorf("unknown error while reading csv records: %v", err)
			}
		} else {
			copyRec := make([]interface{}, len(headersDKInfo.Headers))
			for i, ipos := range headerPos {
				if ipos < len(record) {
					copyRec[i] = record[ipos]
				}
			}
			// Set the file_key, session_id, and shard_id
			copyRec[fileKeyPos] = *inFile
			copyRec[sessionIdPos] = *sessionId
			jetsKeyStr := uuid.New().String()
			copyRec[lastUpdatePos] = lastUpdate
			var mainDomainKey string
			var mainDomainKeyPos int
			for _,ot := range *objTypes {
				groupingKey, shardId, err := headersDKInfo.ComputeGroupingKey(*nbrShards, &ot, &record, &jetsKeyStr)
				if err != nil {
					return nil, 0, 0, err
				}
				domainKeyPos := headersDKInfo.DomainKeysInfoMap[ot].DomainKeyPos
				if ot == *objectType {
					mainDomainKey = groupingKey
					mainDomainKeyPos = domainKeyPos
				}
				copyRec[domainKeyPos] = groupingKey
				shardIdPos := headersDKInfo.DomainKeysInfoMap[ot].ShardIdPos
				copyRec[shardIdPos] = shardId			
			}
			var buf strings.Builder
			switch jetsInputRowJetsKeyAlgo {
			case "row_hash":
				for i := range record {
					if !headersDKInfo.ReservedColumns[headersDKInfo.Headers[i]] {
						// fmt.Println("row_hash with column",headersDKInfo.Headers[i])
						buf.WriteString(record[i])
					}
				}
				jetsKeyStr = uuid.NewSHA1(headersDKInfo.HashingSeed, []byte(buf.String())).String()
				// fmt.Println("row_hash jetsKeyStr",jetsKeyStr)
			case "domain_key":
				jetsKeyStr = mainDomainKey
			}
			if headersDKInfo.IsDomainKeyIsJetsKey(objectType) {
				copyRec[mainDomainKeyPos] = jetsKeyStr
			}
			copyRec[jetsKeyPos] = jetsKeyStr
			inputRows = append(inputRows, copyRec)
			rowid += 1
		}
	}

	// write the sharded rows to the db using go routines...
	// ---------------------------------------
	var copyCount int64
	copyCount, err = dbpool.CopyFrom(context.Background(), pgx.Identifier{tableName}, headersDKInfo.Headers, pgx.CopyFromRows(inputRows))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("while copy csv to table: %v", err)
	}
	// // create a channel to writing the insert row results
	// //* EXAMPLE/STARTING POINT TO HAVE CONCURRENT DB WRITTERS
	// hasErrors := false
	// var wg sync.WaitGroup
	// resultsChan := make(chan writeResult, nbrNodes)
	// wg.Add(nbrNodes)
	// for i := 0; i < nbrNodes; i++ {
	// 	go func(c chan writeResult, dbpool *pgxpool.Pool, data *[][]interface{}) {
	// 		var errMsg string
	// 		copyCount, err := dbpool.CopyFrom(context.Background(), pgx.Identifier{tableName}, headers, pgx.CopyFromRows(*data))
	// 		if err != nil {
	// 			errMsg = fmt.Sprintf("%v", err)
	// 		}
	// 		c <- writeResult{count: copyCount, errMsg: errMsg}
	// 		wg.Done()
	// 	}(resultsChan, dbpool[i], &inputRows[i])
	// }
	// wg.Wait()
	// log.Println("Writing to database nodes completed.")
	// close(resultsChan)
	// for res := range resultsChan {
	// 	copyCount += res.count
	// 	if len(res.errMsg) > 0 {
	// 		log.Println("Error writing to db node: ", res.errMsg)
	// 		hasErrors = true
	// 	}
	// if hasErrors {
	// 	return nil, 0, 0, fmt.Errorf("error(s) while writing to database nodes")
	// }
	log.Println("Inserted", copyCount, "rows in database!")

	// Copy the bad rows from input file into the error file
	// ---------------------------------------
	badRowCount := len(badRowsPos)
	if len(badRowsPos) > 0 {
		log.Println("Got", len(badRowsPos), "bad rows in input file, copying them to the error file.")
		_, err = fileHd.Seek(0, 0)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("error while returning to beginning of in_file %s to write the bad rows to error file: %v", *inFile, err)
		}
		reader := bufio.NewReader(fileHd)
		filePos := 0
		var line string
		for _, errLinePos := range badRowsPos {
			for filePos < errLinePos {
				line, err = reader.ReadString('\n')
				if len(line) == 0 {
					if err == io.EOF {
						log.Panicf("Bug: reached EOF before getting to bad row %d", errLinePos)
					}
					if err != nil {
						return nil, 0, 0, fmt.Errorf("error while fetching bad rows from csv file: %v", err)
					}
				}
				filePos += 1
			}
			_, err = badRowsWriter.WriteString(line)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("error while writing a bad csv row to err file: %v", err)
			}
		}
	}

	return headersDKInfo, copyCount, badRowCount, nil
}

// processFileAndReportStatus is a wrapper around processFile to report error
func processFileAndReportStatus(dbpool *pgxpool.Pool, fileHd, errFileHd *os.File) (bool, error) {

	headersDKInfo, copyCount, badRowCount, err := processFile(dbpool, fileHd, errFileHd)

	// registering the load
	// ---------------------------------------
	status := "completed"
	if badRowCount > 0 || err != nil  {
		status = "errors"
	}
	// register the session if status is completed
	if status == "completed" && !*doNotLockSessionId {
		err2 := schema.RegisterSession(dbpool, *sessionId)
		if err2 != nil {
			err = fmt.Errorf("error while registering the session id: %v", err2)
		}
	}
	var errMessage string
	if err != nil {
		errMessage = fmt.Sprintf("%v", err)
	}
	dimentions := &map[string]string {
		"client": *client,
		"object_type": *objectType,
	}
	if status == "completed" {
		awsi.LogMetric(*completedMetric, dimentions, 1)
	} else {
		awsi.LogMetric(*failedMetric, dimentions, 1)
	}
	err = registerCurrentLoad(copyCount, badRowCount, dbpool, headersDKInfo, status, errMessage)
	if err != nil {
		return false, fmt.Errorf("error while registering the load: %v", err)
	}

	return badRowCount > 0, err
}

func coordinateWork() error {
	// open db connections
	// ---------------------------------------
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		dsnStr, err := awsi.GetDsnFromSecret(*awsDsnSecret, *awsRegion, *usingSshTunnel, 10)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
		dsn = &dsnStr
	}
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// Make sure the jetstore schema exists
	// ---------------------------------------
	tblExists, err := schema.TableExists(dbpool, "jetsapi", "input_loader_status")
	if err != nil {
		return fmt.Errorf("while verifying the jetstore schema: %v", err)
	}
	if !tblExists {
		return fmt.Errorf("error: JetStore schema does not exst in database, please run 'update_db -migrateDb'")
	}

	// check the session is not already used
	// ---------------------------------------
	isInUse, err := schema.IsSessionExists(dbpool, *sessionId)
	if err != nil {
		return fmt.Errorf("while verifying is the session is in use: %v", err)
	}
	if isInUse {
		return fmt.Errorf("error: the session id is already used")
	}

	// Get org and source_period_key from file_key_staging
	// ---------------------------------------
	err = dbpool.QueryRow(context.Background(), 
		"SELECT org, source_period_key FROM jetsapi.file_key_staging WHERE file_key=$1", 
		*inFile).Scan(&clientOrg, &sourcePeriodKey)
	if err != nil {
		return fmt.Errorf("query org, source_period_key from jetsapi.file_key_staging failed: %v", err)
	}
	fmt.Println("Got org",clientOrg,"and sourcePeriodKey",sourcePeriodKey,"from file_key_staging")

	// Get the DomainKeysJson and tableName from source_config table
	// ---------------------------------------
	var dkJson sql.NullString
	err = dbpool.QueryRow(context.Background(), 
		"SELECT table_name, domain_keys_json FROM jetsapi.source_config WHERE client=$1 AND org=$2 AND object_type=$3", 
		*client, clientOrg, *objectType).Scan(&tableName, &dkJson)
	if err != nil {
		return fmt.Errorf("query table_name, domain_keys_json from jetsapi.source_config failed: %v", err)
	}
	if dkJson.Valid {
		domainKeysJson = dkJson.String
	}

	var fileHd, errFileHd *os.File
	if len(*awsBucket) > 0 {

		// Download object using a download manager to a temp file (fileHd)
		fileHd, err = os.CreateTemp("", "jetstore")
		if err != nil {
			return fmt.Errorf("failed to open temp input file: %v", err)
		}
		fmt.Println("Temp input file name:", fileHd.Name())
		defer os.Remove(fileHd.Name())

		// Open the error file
		errFileHd, err = os.CreateTemp("", "jetstore_err")
		if err != nil {
			return fmt.Errorf("failed to open temp error file: %v", err)
		}
		fmt.Println("Temp error file name:", errFileHd.Name())
		defer os.Remove(errFileHd.Name())

		// Download the object
		nsz, err := awsi.DownloadFromS3(*awsBucket, *awsRegion, *inFile, fileHd)
		if err != nil {
			return fmt.Errorf("failed to download input file: %v", err)
		}
		fmt.Println("downloaded", nsz,"bytes from s3")

		// Get ready to read the file
		fileHd.Seek(0, 0)
	
	} else {

		// open csv file
		fileHd, err := os.Open(*inFile)
		if err != nil {
			return fmt.Errorf("error while opening csv file: %v", err)
		}
		defer fileHd.Close()

		// open the error file
		dp, fn := filepath.Split(*inFile)
		if len(errOutDir) == 0 {
			errFileHd, err = os.Create(dp + "err_" + fn)
		} else {
			errFileHd, err = os.Create(fmt.Sprintf("%s/err_%s", errOutDir, fn))
		}
		if err != nil {
			return fmt.Errorf("error while opening output csv err file: %v", err)
		}
		defer errFileHd.Close()
	}

	// Process the downloaded file
	hasBadRows, err := processFileAndReportStatus(dbpool, fileHd, errFileHd)
	if err != nil {
		return err
	}

	if len(*awsBucket) > 0 && hasBadRows {

		// aws integration: Copy the error file to awsBucket
		errFileHd.Seek(0, 0)

		// Create the error file key
		dp, fn := filepath.Split(*inFile)
		errFileKey := dp + "err_" + fn
		err = awsi.UploadToS3(*awsBucket, *awsRegion, errFileKey, errFileHd)
		if err != nil {
			return fmt.Errorf("failed to upload error file: %v", err)
		}
	}
	return nil
}

func main() {
	fmt.Println("CMD LINE ARGS:",os.Args[1:])
	flag.Parse()
	hasErr := false
	var errMsg []string
	var err error
	switch os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO") {
	case "uuid", "":
		jetsInputRowJetsKeyAlgo = "uuid"
	case "row_hash":
		jetsInputRowJetsKeyAlgo = "row_hash"
	case "domain_key":
		jetsInputRowJetsKeyAlgo = "domain_key"
	default:
		hasErr = true
		errMsg = append(errMsg, 
			fmt.Sprintf("env var JETS_INPUT_ROW_JETS_KEY_ALGO has invalid value: %s, must be one of uuid, row_hash, domain_key (default: uuid if empty)",
			os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO")))
	}
	if *inFile == "" {
		hasErr = true
		errMsg = append(errMsg, "Input file name must be provided (-in_file).")
	}
	if *client == "" {
		hasErr = true
		errMsg = append(errMsg, "Client name must be provided (-client).")
	}
	if *userEmail == "" {
		hasErr = true
		errMsg = append(errMsg, "User email must be provided (-userEmail).")
	}
	if *objectType == "" {
		hasErr = true
		errMsg = append(errMsg, "Object type of the input file must be provided (-objectType).")
	}
	if *dsn == "" && *awsDsnSecret == "" {
		*dsn = os.Getenv("JETS_DSN_URI_VALUE")
		if *dsn == "" {
			*dsn, err = awsi.GetDsnFromJson(os.Getenv("JETS_DSN_JSON_VALUE"), *usingSshTunnel, 20)
			if err != nil {
				log.Printf("while calling GetDsnFromJson: %v", err)
				*dsn = ""
			}
		}
		*awsDsnSecret = os.Getenv("JETS_DSN_SECRET")
		if *dsn == "" && *awsDsnSecret == "" {
			hasErr = true
			errMsg = append(errMsg, "Connection string must be provided using either -awsDsnSecret or -dsn.")	
		}
	}
	if *awsBucket == "" {
		*awsBucket = os.Getenv("JETS_BUCKET")
	}
	if *awsRegion == "" {
		*awsRegion = os.Getenv("JETS_REGION")
	}
	if (*awsBucket != "" || *awsDsnSecret != "") && *awsRegion == "" {
		hasErr = true
		errMsg = append(errMsg, "aws region must be provided when using either -awsDsnSecret or -awsBucket.")
	}


	errOutDir = os.Getenv("LOADER_ERR_DIR")
	adminEmail = os.Getenv("JETS_ADMIN_EMAIL")
	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")
	// Initialize user module -- for token generation
	user.AdminEmail = adminEmail
	// Get secret to sign jwt tokens
	awsApiSecret := os.Getenv("AWS_API_SECRET")
	apiSecret := os.Getenv("API_SECRET")
	if apiSecret == "" && awsApiSecret != "" {
		apiSecret, err = awsi.GetSecretValue(awsApiSecret, *awsRegion)
		if err != nil {
			hasErr = true
			errMsg = append(errMsg, fmt.Sprintf("while getting apiSecret from aws secret: %v", err))
		}
	}
	user.ApiSecret = apiSecret
	user.TokenExpiration = 60

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid arguments")
	}
	sessId := ""
	if *sessionId == "" {
		sessId = strconv.FormatInt(time.Now().UnixMilli(), 10)
		sessionId = &sessId
		log.Println("sessionId is set to", *sessionId)
	}
	
	fmt.Println("Loader argument:")
	fmt.Println("----------------")
	fmt.Println("Got argument: awsDsnSecret", *awsDsnSecret)
	fmt.Println("Got argument: awsBucket", *awsBucket)
	fmt.Println("Got argument: awsRegion", *awsRegion)
	fmt.Println("Got argument: inFile", *inFile)
	fmt.Println("Got argument: dropTable", *dropTable)
	fmt.Println("Got argument: len(dsn)", len(*dsn))
	fmt.Println("Got argument: client", *client)
	fmt.Println("Got argument: objectType", *objectType)
	fmt.Println("Got argument: userEmail", *userEmail)
	fmt.Println("Got argument: nbrShards", *nbrShards)
	fmt.Println("Got argument: sessionId", *sessionId)
	fmt.Println("Got argument: doNotLockSessionId", *doNotLockSessionId)
	fmt.Println("Got argument: usingSshTunnel", *usingSshTunnel)
	fmt.Println("Got argument: loaderCompletedMetric", *completedMetric)
	fmt.Println("Got argument: loaderFailedMetric", *failedMetric)
	fmt.Println("Loader out dir (from env LOADER_ERR_DIR):", errOutDir)
	if len(errOutDir) == 0 {
		fmt.Println("Loader error file will be in same directory as input file.")
	}
	if *dsn != "" && *awsDsnSecret != "" {
		fmt.Println("Both -awsDsnSecret and -dsn are provided, will use argument -awsDsnSecret only")
	}
	fmt.Println("ENV JETS_DOMAIN_KEY_HASH_ALGO:",os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO"))
	fmt.Println("ENV JETS_DOMAIN_KEY_HASH_SEED:",os.Getenv("JETS_DOMAIN_KEY_HASH_SEED"))
	fmt.Println("ENV JETS_INPUT_ROW_JETS_KEY_ALGO:",os.Getenv("JETS_INPUT_ROW_JETS_KEY_ALGO"))
	fmt.Println("ENV AWS_API_SECRET:",os.Getenv("AWS_API_SECRET"))
	if devMode {
		fmt.Println("Running in DEV MODE")
		fmt.Println("Nbr Shards in DEV MODE: nbrShards", nbrShards)
	}

	err = coordinateWork()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
