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

var inFile = flag.String("in_file", "/work/input.csv", "the input csv file name")
var dropTable = flag.Bool("d", false, "drop table if it exists, default is false")
var dsnList = flag.String("dsn", "", "comma-separated list of database connection string, order matters and should always be the same (required)")
var tblName = flag.String("table", "", "table name to load the data into (required)")
var client = flag.String("client", "", "Client associated with the source location (required)")
var objectType = flag.String("objectType", "", "Source location of the file (required)")
var userEmail = flag.String("userEmail", "", "User identifier to register the load (required)")
var groupingColumn = flag.String("groupingColumn", "", "Grouping column used in server process. This will add an index to the table_name for that column")
var nbrShards = flag.Int("nbrShards", 1, "Number of shards to use in sharding the input file")
var sessionId = flag.String("sessionId", "", "Process session ID, is needed as -inSessionId for the server process (must be unique), default based on timestamp.")
var sep_flag chartype = '|'

func init() {
	flag.Var(&sep_flag, "sep", "Field separator, default is pipe ('|')")
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

func registerCurrentLoad(copyCount int64, badRowCount int, dbpool *pgxpool.Pool, nodeId int) error {
	stmt := `INSERT INTO jetsapi.input_loader_status (
		object_type, table_name, client, file_key, session_id, 
		load_count, bad_row_count, node_id, user_email) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := dbpool.Exec(context.Background(), stmt, 
		*objectType, *tblName, *client, *inFile, *sessionId, copyCount, badRowCount, nodeId, *userEmail)
	if err != nil {
		return fmt.Errorf("error inserting in jetsapi.input_loader_status table: %v", err)
	}
	stmt = `INSERT INTO jetsapi.input_registry (
		client, object_type, file_key, table_name, source_type, session_id, user_email) 
		VALUES ($1, $2, $3, $4, 'file', $5, $6)`
	_, err = dbpool.Exec(context.Background(), stmt, 
		*client, *objectType, *inFile, *tblName, *sessionId, *userEmail)
	if err != nil {
		return fmt.Errorf("error inserting in jetsapi.input_registry table: %v", err)
	}
	return nil
}

type writeResult struct {
	count  int64
	errMsg string
}

// processFile
// --------------------------------------------------------------------------------------
func processFile() error {
	// open csv file
	file, err := os.Open(*inFile)
	if err != nil {
		return fmt.Errorf("error while opening csv file: %v", err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = rune(sep_flag)

	// open err file where we'll put the bad rows
	dp, fn := filepath.Split(*inFile)
	badRowsfile, err := os.Create(dp + "err_" + fn)
	if err != nil {
		return fmt.Errorf("error while opening output csv err file: %v", err)
	}
	defer badRowsfile.Close()
	badRowsWriter := bufio.NewWriter(badRowsfile)
	defer badRowsWriter.Flush()

	// read the headers, put them in err file and make them valid for db
	rawHeaders, err := reader.Read()
	if err == io.EOF {
		return errors.New("input csv file is empty")
	} else if err != nil {
		return fmt.Errorf("while reading csv headers: %v", err)
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
		return fmt.Errorf("error: grouping column '%s' not found in input file %s", *groupingColumn, *inFile)
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
			badRowsWriter.WriteRune(reader.Comma)
		}
		badRowsWriter.WriteString(rawHeaders[i])
	}
	_, err = badRowsWriter.WriteRune('\n')
	if err != nil {
		return fmt.Errorf("while writing csv headers to err file: %v", err)
	}

	// open db connections
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

		// validate table name
		tblExists, err = tableExists(dbpool[i], "public", *tblName)
		if err != nil {
			return fmt.Errorf("while validating table name: %v", err)
		}
		if !tblExists || *dropTable {
			err = createTable(dbpool[i], headers)
			if err != nil {
				return fmt.Errorf("while creating table: %v", err)
			}
		}
	}

	// read the rest of the file
	var badRowsPos []int
	var rowid int64
	nshards64 := int64(*nbrShards)
	inputRows := make([][][]interface{}, nbrNodes)
	for {
		record, err := reader.Read()
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
				return fmt.Errorf("unknown error while reading csv records: %v", err)
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
	var copyCount int64
	var badRowCount int
	hasErrors := false
	if nbrNodes == 1 {
		// everything is in shard 0
		copyCount, err = dbpool[0].CopyFrom(context.Background(), pgx.Identifier{*tblName}, headers, pgx.CopyFromRows(inputRows[0]))
		if err != nil {
			return fmt.Errorf("while copy csv to table: %v", err)
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
		return fmt.Errorf("error(s) while writing to database nodes")
	}
	log.Println("Inserted", copyCount, "rows in database!")
	badRowCount = len(badRowsPos)
	if len(badRowsPos) > 0 {
		log.Println("Got", len(badRowsPos), "bad rows in input file, copying them to the error file.")
		file, err := os.Open(*inFile)
		if err != nil {
			return fmt.Errorf("error while re-opening csv file: %v", err)
		}
		defer file.Close()
		reader := bufio.NewReader(file)
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
						return fmt.Errorf("error while fetching bad rows from csv file: %v", err)
					}
				}
				filePos += 1
			}
			_, err = badRowsWriter.WriteString(line)
			if err != nil {
				return fmt.Errorf("error while writing a bad csv row to err file: %v", err)
			}
		}
	}
	// registering the load
	for i := 0; i < nbrNodes; i++ {
		err = registerCurrentLoad(copyCount, badRowCount, dbpool[i], i)
		if err != nil {
			return fmt.Errorf("error while registering the load: %v", err)
		}
	}

	return nil
}

func main() {
	flag.Parse()
	hasErr := false
	var errMsg []string
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
		os.Exit((1))
	}
	sessId := ""
	if *sessionId == "" {
		sessId = strconv.FormatInt(time.Now().UnixMilli(), 10)
		sessionId = &sessId
		log.Println("sessionId is set to", *sessionId)
	}

	fmt.Println("Loader argument:")
	fmt.Println("----------------")
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
	fmt.Println("Got argument: sep_flag", sep_flag)

	err := processFile()
	if err != nil {
		flag.Usage()
		log.Fatal(err)
	}
}
