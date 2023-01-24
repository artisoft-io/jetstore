package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
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
	"github.com/artisoft-io/jetstore/jets/schema"
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
// JETS_DOMAIN_KEY_HASH_ALGO (values: md5, sha1, none (default))
// JETS_DOMAIN_KEY_HASH_SEED (required for md5 and sha1. MUST be a valid uuid )
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
var tableName string
var domainKeysJson string
var sep_flag chartype = '€'
var errOutDir string

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
		object_type, table_name, client, file_key, session_id, status, error_message,
		load_count, bad_row_count, user_email) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT ON CONSTRAINT input_loader_status_unique_cstraint
			DO UPDATE SET (status, error_message, load_count, bad_row_count, user_email, last_update) =
			(EXCLUDED.status, EXCLUDED.error_message, EXCLUDED.load_count, EXCLUDED.bad_row_count, EXCLUDED.user_email, DEFAULT)`
	_, err := dbpool.Exec(context.Background(), stmt, 
		*objectType, tableName, *client, *inFile, *sessionId, status, errMessage, copyCount, badRowCount, *userEmail)
	if err != nil {
		return fmt.Errorf("error inserting in jetsapi.input_loader_status table: %v", err)
	}
	if status == "completed" && dkInfo != nil {
		for objType := range dkInfo.DomainKeysInfoMap {
			log.Println("Registering staging table with object type:", objType)
			stmt = `INSERT INTO jetsapi.input_registry (
				client, object_type, file_key, table_name, source_type, session_id, user_email) 
				VALUES ($1, $2, $3, $4, 'file', $5, $6)`
			_, err = dbpool.Exec(context.Background(), stmt, 
				*client, objType, *inFile, tableName, *sessionId, *userEmail)
			if err != nil {
				return fmt.Errorf("error inserting in jetsapi.input_registry table: %v", err)
			}	
		}
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
		if cn == pn {
			return nil, 0, 0, fmt.Errorf("error: cannot determine the csv-delimit used in file %s",*inFile)
		}
		if cn > pn {
			sep_flag = ','
		} else {
			sep_flag = '|'
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
	objTypes := make([]string, 0)
	if domainKeysJson != "" {
		var f interface{}
		err = json.Unmarshal([]byte(domainKeysJson), &f)
		if err != nil {
			fmt.Println("while parsing domainKeysJson using json parser:", err)
			return nil, 0, 0, err
		}
		// Extract the domain keys structure from the json
		switch value := f.(type) {
		case map[string]interface{}:
			for k := range value {
				objTypes = append(objTypes, k)
			}		
		}
	}
	if len(objTypes) == 0 {
		objTypes = append(objTypes, *objectType)
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
			copyRec[jetsKeyPos] = jetsKeyStr
			copyRec[lastUpdatePos] = lastUpdate
			for ipos := range objTypes {
				groupingKey, shardId, err := headersDKInfo.ComputeGroupingKey(*nbrShards, &objTypes[ipos], &record, &jetsKeyStr)
				if err != nil {
					return nil, 0, 0, err
				}
				domainKeyPos := headersDKInfo.DomainKeysInfoMap[objTypes[ipos]].DomainKeyPos
				copyRec[domainKeyPos] = groupingKey
				shardIdPos := headersDKInfo.DomainKeysInfoMap[objTypes[ipos]].ShardIdPos
				copyRec[shardIdPos] = shardId
			}
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

	// Get the DomainKeysJson and tableName from source_config table
	// ---------------------------------------
	var dkJson sql.NullString
	err = dbpool.QueryRow(context.Background(), 
		"SELECT table_name, domain_keys_json FROM jetsapi.source_config WHERE client=$1 AND object_type=$2", 
		*client, *objectType).Scan(&tableName, &dkJson)
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
		errMsg = append(errMsg, "Source location must be provided (-objectType).")
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

	errOutDir = os.Getenv("LOADER_ERR_DIR")

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
	fmt.Println("Loader out dir (from env LOADER_ERR_DIR):", errOutDir)
	if len(errOutDir) == 0 {
		fmt.Println("Loader error file will be in same directory as input file.")
	}
	if *dsn != "" && *awsDsnSecret != "" {
		fmt.Println("Both -awsDsnSecret and -dsn are provided, will use argument -awsDsnSecret only")
	}
 
	err = coordinateWork()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}
