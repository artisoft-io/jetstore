package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

var bucket = flag.String("bucket", "", "Bucket having the the input csv file (aws integration)")
var inFile = flag.String("in_file", "/work/input.csv", "the input csv file name")
var dropTable = flag.Bool("d", false, "drop table if it exists, default is false")
var dsnList = flag.String("dsn", "", "comma-separated list of database connection string, order matters and should always be the same (required)")
var tblName = flag.String("table", "", "table name to load the data into (required)")
var client = flag.String("client", "", "Client associated with the source location (required)")
var objectType = flag.String("objectType", "", "The type of object contained in the file (required)")
var userEmail = flag.String("userEmail", "", "User identifier to register the load (required)")
var groupingColumn = flag.String("groupingColumn", "", "Grouping column used in server process. This will add an index to the table_name for that column and shard the data")
var nbrShards = flag.Int("nbrShards", 1, "Number of shards to use in sharding the input file")
var sessionId = flag.String("sessionId", "", "Process session ID, is needed as -inSessionId for the server process (must be unique), default based on timestamp.")
var doNotLockSessionId = flag.Bool("doNotLockSessionId", false, "Do NOT lock sessionId on sucessful completion (default is to lock the sessionId on successful completion")
var sep_flag chartype = '€'
var errOutDir string

func init() {
	flag.Var(&sep_flag, "sep", "Field separator, default is auto detect between pipe ('|') or comma (',')")
}

// Support Functions
// --------------------------------------------------------------------------------------
func compute_shard_id(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	res := int(h.Sum32()) % *nbrShards
	// log.Println("COMPUTE SHARD for key ",key,"on",*nbrShards,"shard id =",res)
	return res
}
func tableExists(dbpool *pgxpool.Pool, schema, table string) (exists bool, err error) {
	err = dbpool.QueryRow(context.Background(), "select exists (select from pg_tables where schemaname = $1 and tablename = $2)", schema, table).Scan(&exists)
	if err != nil {
		err = fmt.Errorf("QueryRow failed: %v", err)
	}
	return exists, err
}

func isValidName(name string) bool {
	return !strings.ContainsAny(name, "=;\t\n ")
}

func createTable(dbpool *pgxpool.Pool, headers []string) (err error) {
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", pgx.Identifier{*tblName}.Sanitize())
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while droping table: %v", err)
	}
	var buf strings.Builder
	buf.WriteString("CREATE TABLE IF NOT EXISTS ")
	buf.WriteString(pgx.Identifier{*tblName}.Sanitize())
	buf.WriteString("(")
	for _, header := range headers {
		switch header {
		case "session_id", "shard_id", "file_key":
		default:
			buf.WriteString(pgx.Identifier{header}.Sanitize())
			buf.WriteString(" TEXT, ")
		}
	}
	buf.WriteString(" file_key TEXT,")
	buf.WriteString(" \"jets:key\" TEXT DEFAULT gen_random_uuid ()::text NOT NULL,")
	buf.WriteString(" session_id TEXT DEFAULT '' NOT NULL,")
	buf.WriteString(" shard_id integer DEFAULT 0 NOT NULL, ")
	buf.WriteString(" last_update timestamp without time zone DEFAULT now() NOT NULL ")
	buf.WriteString(");")
	stmt = buf.String()
	log.Println(stmt)
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while creating table: %v", err)
	}
	// Add grouping column index
	if *groupingColumn != "" {
		stmt = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s  (%s ASC);`,
			pgx.Identifier{*tblName + "_grouping_idx"}.Sanitize(),
			pgx.Identifier{*tblName}.Sanitize(),
			pgx.Identifier{*groupingColumn}.Sanitize())
		log.Println(stmt)
		_, err := dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return fmt.Errorf("error while creating primary index: %v", err)
		}
	}
	return nil
}

func truncateSessionId(dbpool *pgxpool.Pool, nodeId int) error {
	stmt := `DELETE FROM jetsapi.input_loader_status 
						WHERE table_name = $1 AND session_id = $2 AND node_id = $3`
	_, err := dbpool.Exec(context.Background(), stmt, *tblName, *sessionId, nodeId)
	if err != nil {
		return fmt.Errorf("error deleting sessionId from jetsapi.input_loader_status table: %v", err)
	}
	stmt = `DELETE FROM jetsapi.input_registry WHERE table_name = $1 AND session_id = $2`
	_, err = dbpool.Exec(context.Background(), stmt, *tblName, *sessionId)
	if err != nil {
		return fmt.Errorf("error deleting sessionId from jetsapi.input_registry table: %v", err)
	}
	return nil
}

func registerCurrentLoad(copyCount int64, badRowCount int, dbpool *pgxpool.Pool, nodeId int, status string) error {
	stmt := `INSERT INTO jetsapi.input_loader_status (
		object_type, table_name, client, file_key, session_id, status,
		load_count, bad_row_count, node_id, user_email) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT ON CONSTRAINT input_loader_status_unique_cstraint
			DO UPDATE SET (status, load_count, bad_row_count, node_id, user_email, last_update) =
			(EXCLUDED.status, EXCLUDED.load_count, EXCLUDED.bad_row_count, EXCLUDED.node_id, EXCLUDED.user_email, DEFAULT)`
	_, err := dbpool.Exec(context.Background(), stmt, 
		*objectType, *tblName, *client, *inFile, *sessionId, status, copyCount, badRowCount, nodeId, *userEmail)
	if err != nil {
		return fmt.Errorf("error inserting in jetsapi.input_loader_status table: %v", err)
	}
	if status == "completed" {
		stmt = `INSERT INTO jetsapi.input_registry (
			client, object_type, file_key, table_name, source_type, session_id, user_email) 
			VALUES ($1, $2, $3, $4, 'file', $5, $6)`
		_, err = dbpool.Exec(context.Background(), stmt, 
			*client, *objectType, *inFile, *tblName, *sessionId, *userEmail)
		if err != nil {
			return fmt.Errorf("error inserting in jetsapi.input_registry table: %v", err)
		}	
	}
	return nil
}

type writeResult struct {
	count  int64
	errMsg string
}

// processFile
// --------------------------------------------------------------------------------------
func processFile(dbpool []*pgxpool.Pool, fileHd, errFileHd *os.File) (bool, error) {

	// determine the csv separator
	// ---------------------------------------
	if sep_flag == '€' {
		// auto detect the separator based on the first line
		buf := make([]byte, 2048)
		nb, err := fileHd.Read(buf)
		if err != nil {
			return false, fmt.Errorf("error while ready first few bytes of in_file %s: %v", *inFile, err)
		}
		txt := string(buf[0:nb])
		cn := strings.Count(txt, ",")
		pn := strings.Count(txt, "|")
		if cn == pn {
			return false, fmt.Errorf("error: cannot determine the csv-delimit used in file %s",*inFile)
		}
		if cn > pn {
			sep_flag = ','
		} else {
			sep_flag = '|'
		}
		_, err = fileHd.Seek(0, 0)
		if err != nil {
			return false, fmt.Errorf("error while returning to beginning of in_file %s: %v", *inFile, err)
		}
	}
	fmt.Println("Got argument: sep_flag", sep_flag)

	// Get reader / writer for input and error file resp.
	csvReader := csv.NewReader(fileHd)
	csvReader.Comma = rune(sep_flag)
	badRowsWriter := bufio.NewWriter(errFileHd)
	defer badRowsWriter.Flush()

	// read the headers, put them in err file and make them valid for db
	// ---------------------------------------
	rawHeaders, err := csvReader.Read()
	if err == io.EOF {
		return false, errors.New("input csv file is empty")
	} else if err != nil {
		return false, fmt.Errorf("while reading csv headers: %v", err)
	}
	// Add sessionId and shardId to the headers,
	// drop input column matching one of the reserve column name
	headers := make([]string, 0, len(rawHeaders)+5)
	headerPos := make([]int, 0, len(rawHeaders)+5)
	groupingColumnPos := -1
	for ipos := range rawHeaders {
		if rawHeaders[ipos] == *groupingColumn {
			groupingColumnPos = ipos
		}
		switch rawHeaders[ipos] {
		case "file_key", "jets:key", "last_update", "session_id", "shard_id":
			log.Printf("Input file contains column named '%s', this is a reserve name. Droping the column", rawHeaders[ipos])
		default:
			headers = append(headers, rawHeaders[ipos])
			headerPos = append(headerPos, ipos)
		}
	}
	// Check if we have grouping column if we should
	if *groupingColumn != "" && groupingColumnPos < 0 {
		return false, fmt.Errorf("error: grouping column '%s' not found in input file %s", *groupingColumn, *inFile)
	}
	// Adding reserve columns
	fileNamePos := len(headers)
	headers = append(headers, "file_key")
	sessionIdPos := len(headers)
	headers = append(headers, "session_id")
	shardIdPos := len(headers)
	headers = append(headers, "shard_id")
	for i := range rawHeaders {
		if i > 0 {
			badRowsWriter.WriteRune(csvReader.Comma)
		}
		badRowsWriter.WriteString(rawHeaders[i])
	}
	_, err = badRowsWriter.WriteRune('\n')
	if err != nil {
		return false, fmt.Errorf("while writing csv headers to err file: %v", err)
	}

	// prepare db connections
	// ---------------------------------------
	nbrNodes := len(dbpool)
	for i := range dbpool {
		// validate table name
		tblExists, err := tableExists(dbpool[i], "public", *tblName)
		if err != nil {
			return false, fmt.Errorf("while validating table name: %v", err)
		}
		if !tblExists || *dropTable {
			if *dropTable {
				// remove the previous input loader status associated with sessionId
				err = truncateSessionId(dbpool[i], i)
				if err != nil {
					return false, fmt.Errorf("while truncating sessionId: %v", err)
				}
			}
			err = createTable(dbpool[i], headers)
			if err != nil {
				return false, fmt.Errorf("while creating table: %v", err)
			}
		}
	}

	// read the rest of the file
	// ---------------------------------------
	var badRowsPos []int
	var rowid int64
	nshards64 := int64(*nbrShards)
	inputRows := make([][][]interface{}, nbrNodes)
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
				return false, fmt.Errorf("unknown error while reading csv records: %v", err)
			}
		} else {
			copyRec := make([]interface{}, len(headers))
			for i, ipos := range headerPos {
				copyRec[i] = record[ipos]
			}
			// Set the file_key, session_id, and shard_id
			var nodeId int
			copyRec[fileNamePos] = *inFile
			copyRec[sessionIdPos] = *sessionId
			shardId := 0
			if groupingColumnPos >= 0 {
				shardId = compute_shard_id(record[groupingColumnPos])
			} else {
				shardId = int(rowid % nshards64)
			}
			copyRec[shardIdPos] = shardId
			nodeId = shardId % nbrNodes
			inputRows[nodeId] = append(inputRows[nodeId], copyRec)
			rowid += 1
		}
	}

	// write the sharded rows to the db using go routines...
	// ---------------------------------------
	var copyCount int64
	var badRowCount int
	hasErrors := false
	if nbrNodes == 1 {
		// everything is in shard 0
		copyCount, err = dbpool[0].CopyFrom(context.Background(), pgx.Identifier{*tblName}, headers, pgx.CopyFromRows(inputRows[0]))
		if err != nil {
			return false, fmt.Errorf("while copy csv to table: %v", err)
		}
	} else {
		// create a channel to writing the insert row results
		var wg sync.WaitGroup
		resultsChan := make(chan writeResult, nbrNodes)
		wg.Add(nbrNodes)
		for i := 0; i < nbrNodes; i++ {
			go func(c chan writeResult, dbpool *pgxpool.Pool, data *[][]interface{}) {
				var errMsg string
				copyCount, err := dbpool.CopyFrom(context.Background(), pgx.Identifier{*tblName}, headers, pgx.CopyFromRows(*data))
				if err != nil {
					errMsg = fmt.Sprintf("%v", err)
				}
				c <- writeResult{count: copyCount, errMsg: errMsg}
				wg.Done()
			}(resultsChan, dbpool[i], &inputRows[i])
		}
		wg.Wait()
		log.Println("Writing to database nodes completed.")
		close(resultsChan)
		for res := range resultsChan {
			copyCount += res.count
			if len(res.errMsg) > 0 {
				log.Println("Error writing to db node: ", res.errMsg)
				hasErrors = true
			}
		}
	}
	if hasErrors {
		return false, fmt.Errorf("error(s) while writing to database nodes")
	}
	log.Println("Inserted", copyCount, "rows in database!")

	// Copy the bad rows from input file into the error file
	// ---------------------------------------
	badRowCount = len(badRowsPos)
	hasBadRows := badRowCount > 0
	if len(badRowsPos) > 0 {
		log.Println("Got", len(badRowsPos), "bad rows in input file, copying them to the error file.")
		_, err = fileHd.Seek(0, 0)
		if err != nil {
			return false, fmt.Errorf("error while returning to beginning of in_file %s to write the bad rows to error file: %v", *inFile, err)
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
						return false, fmt.Errorf("error while fetching bad rows from csv file: %v", err)
					}
				}
				filePos += 1
			}
			_, err = badRowsWriter.WriteString(line)
			if err != nil {
				return false, fmt.Errorf("error while writing a bad csv row to err file: %v", err)
			}
		}
	}

	// registering the load
	// ---------------------------------------
	status := "completed"
	if hasBadRows {
		status = "errors"
	}
	// register the session if status is completed
	if status == "completed" && !*doNotLockSessionId {
		err:= schema.RegisterSession(dbpool[0], *sessionId)
		if err != nil {
			return false, fmt.Errorf("error while registering the session id: %v", err)
		}
	}
	for i := 0; i < nbrNodes; i++ {
		err = registerCurrentLoad(copyCount, badRowCount, dbpool[i], i, status)
		if err != nil {
			return false, fmt.Errorf("error while registering the load: %v", err)
		}
	}

	return hasBadRows, nil
}

func coordinateWork() error {
	var err error
	secretName := "jetstore/pgsql"
	region := "us-east-1"
	// if len(*bucket) > 0 { }
	config, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
	 log.Fatal(err)
	}
 
	// Create Secrets Manager client
	svc := secretsmanager.NewFromConfig(config)
 
	input := &secretsmanager.GetSecretValueInput{
	 SecretId:     aws.String(secretName),
	 VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}
 
	result, err := svc.GetSecretValue(context.TODO(), input)
	if err != nil {
	 // For a list of exceptions thrown, see
	 // https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_GetSecretValue.html
	 log.Fatal(err.Error())
	}
 
	// Decrypts secret using the associated KMS key.
	var secretString string = *result.SecretString
 
	// Your code goes here.
	fmt.Println("GOT SECRET:",secretString)
 
	// open db connections
	// ---------------------------------------
	dsnSplit := strings.Split(*dsnList, ",")
	nbrNodes := len(dsnSplit)
	dbpool := make([]*pgxpool.Pool, nbrNodes)
	for i := range dsnSplit {
		dbpool[i], err = pgxpool.Connect(context.Background(), dsnSplit[i])
		if err != nil {
			return fmt.Errorf("while opening db connection: %v", err)
		}
		defer dbpool[i].Close()

		// Make sure the jetstore schema exists
		tblExists, err := tableExists(dbpool[i], "jetsapi", "input_loader_status")
		if err != nil {
			return fmt.Errorf("while verifying the jetstore schema: %v", err)
		}
		if !tblExists {
			return fmt.Errorf("error: JetStore schema does not exst in database, please run 'update_db -migrateDb'")
		}
	}

	// check the session is not already used
	// ---------------------------------------
	isInUse, err := schema.IsSessionExists(dbpool[0], *sessionId)
	if err != nil {
		return fmt.Errorf("while verifying is the session is in use: %v", err)
	}
	if isInUse {
		return fmt.Errorf("error: the session id is already used")
	}

	var fileHd, errFileHd *os.File
	var awsClient *s3.Client
	if len(*bucket) > 0 {
		// aws integration: Check if we read from bucket
		// Open input and error files
		// ---------------------------------------
		// Load the SDK's configuration from environment and shared config, and
		// create the client with this.
		// ALREADY DONE ABOVE
		// config, err := config.LoadDefaultConfig(context.TODO())
		// if err != nil {
		// 	log.Fatalf("failed to load SDK configuration, %v", err)
		// }
		awsClient = s3.NewFromConfig(config)

		// Download object using a download manager to a temp file (fileHd)
		fileHd, err = os.CreateTemp("", "jetstore")
		if err != nil {
			log.Fatalf("failed to open temp input file: %v", err)
		}
		fmt.Println("Temp input file name:", fileHd.Name())
		defer os.Remove(fileHd.Name())

		// Open the error file
		errFileHd, err = os.CreateTemp("", "jetstore_err")
		if err != nil {
			log.Fatalf("failed to open temp error file: %v", err)
		}
		fmt.Println("Temp error file name:", errFileHd.Name())
		defer os.Remove(errFileHd.Name())

		// Download the object
		downloader := manager.NewDownloader(awsClient)
		nsz, err := downloader.Download(context.TODO(), fileHd, &s3.GetObjectInput{Bucket: bucket, Key: inFile})
		if err != nil {
			log.Fatalf("failed to download input file: %v", err)
		}
		fmt.Println("downloaded", nsz,"bytes")

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
	hasBadRows, err := processFile(dbpool, fileHd, errFileHd)
	if err != nil {
		for i := 0; i < nbrNodes; i++ {
			err2 := registerCurrentLoad(0, 0, dbpool[i], i, "failed")
			if err2 != nil {
				return fmt.Errorf("error while registering the load: %v", err)
			}
		}
		return err
	}

	if len(*bucket) > 0 && hasBadRows {

		// aws integration: Copy the error file to bucket
		errFileHd.Seek(0, 0)

		// Create an uploader with the client and custom options
		uploader := manager.NewUploader(awsClient)

		// Create the error file key
		dp, fn := filepath.Split(*inFile)
		errFileKey := dp + "err_" + fn
		result, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
			Bucket: bucket,
			Key:    &errFileKey,
			Body:   bufio.NewReader(errFileHd),
		})
		if err != nil {
			log.Fatalf("failed to upload error file: %v", err)
		}
		fmt.Println("uploaded", len(result.CompletedParts),"parts to bad rows file")		
	}
	return nil
}

func main() {
	fmt.Println("CMD LINE ARGS:",os.Args[1:])
	flag.Parse()
	hasErr := false
	var errMsg []string
	// if grouping_column is '' it means it's actually empty
	if *groupingColumn == "''" {
		*groupingColumn = ""
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
	if *tblName == "" {
		hasErr = true
		errMsg = append(errMsg, "Table name must be provided.")
	}
	if !isValidName(*tblName) {
		hasErr = true
		errMsg = append(errMsg, "Table name is not valid.")
	}
	if *dsnList == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string must be provided.")
	}
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		os.Exit(1)
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
	fmt.Println("Got argument: bucket", *bucket)
	fmt.Println("Got argument: inFile", *inFile)
	fmt.Println("Got argument: dropTable", *dropTable)
	fmt.Println("Got argument: dsnList", *dsnList)
	fmt.Println("Got argument: client", *client)
	fmt.Println("Got argument: objectType", *objectType)
	fmt.Println("Got argument: userEmail", *userEmail)
	fmt.Println("Got argument: tblName", *tblName)
	fmt.Println("Got argument: groupingColumn", *groupingColumn)
	fmt.Println("Got argument: nbrShards", *nbrShards)
	fmt.Println("Got argument: sessionId", *sessionId)
	fmt.Println("Got argument: doNotLockSessionId", *doNotLockSessionId)
	fmt.Println("Loader out dir (from env LOADER_ERR_DIR):", errOutDir)
	if len(errOutDir) == 0 {
		fmt.Println("Loader error file will be in same directory as input file.")
	}
 
	err := coordinateWork()
	if err != nil {
		flag.Usage()
		log.Fatal(err)
	}
}
