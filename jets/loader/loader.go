package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

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
var dsn = flag.String("dsn", "", "database connection string (required)")
var tblName = flag.String("table", "", "table name to load the data into, must not exist unless -a or -d is provided (required)")
var appendTable = flag.Bool("a", false, "append file to existing table, default is false")
var dropTable = flag.Bool("d", false, "drop table if it exists, default is false")
var sep_flag chartype = '|'
func init() {
	flag.Var(&sep_flag, "sep", "Field separator, default is pipe ('|')")
}

// Support Functions
// --------------------------------------------------------------------------------------
func tableExists(dbpool *pgxpool.Pool) ( exists bool, err error) {
	err = dbpool.QueryRow(context.Background(), "select exists (select from pg_tables where schemaname = 'public' and tablename = $1)", *tblName).Scan(&exists)
	if err != nil {
		err = fmt.Errorf("QueryRow failed: %v", err)
	}
	return exists, err
}
func isValidName(name string) bool {
	return !strings.ContainsAny(name, "=;\t\n ")
}
func makeValid(name string) (string, error) {
	pos := 0
	badName := false
	dropInvalid := func(r rune) rune {
		pos += 1
		switch {
		case r >= 'A' && r <= 'Z':
			return r
		case r >= 'a' && r <= 'z':
			return r
		case pos>1 && r >= '0' && r <= '9':
			return r
		case r == '_':
			return r
		}
		if pos == 1 {
			badName = true
		}
		return -1
	}
	resultval := strings.Map(dropInvalid, name)
	if badName {
		return resultval, errors.New("bad header name")
	}
	return resultval, nil
}

func createTable(dbpool *pgxpool.Pool, headers []string) (err error) {
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", *tblName)
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while droping table: %v", err)
	}
	var buf strings.Builder
	buf.WriteString("CREATE TABLE IF NOT EXISTS ")
	buf.WriteString(*tblName)
	buf.WriteString("(")
	for i, header := range headers {
		if i > 0 {
		buf.WriteString(", ")
		}
		buf.WriteString(header)
		buf.WriteString(" TEXT")
	}
	buf.WriteString(");")
	stmt = buf.String()
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while creating table: %v", err)
	}
	return err
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
	badRowsfile, err := os.Create(dp + "err_"+fn)
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
	var headers []string
	for i, header := range rawHeaders {
		if i > 0 {
			badRowsWriter.WriteRune(reader.Comma)
		}
		badRowsWriter.WriteString(header)
		validHeader, err := makeValid(header)
		if err != nil {
			return fmt.Errorf("input csv file contains an invalid header: %s", header)
		}
		headers = append(headers, validHeader)
	}
	_, err = badRowsWriter.WriteRune('\n')
	if err != nil {
		return fmt.Errorf("while writing csv headers to err file: %v", err)
	}

	// open db connection
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer dbpool.Close()

	// validate table name
	tblExists, err := tableExists(dbpool)
	if err != nil {
		return fmt.Errorf("while validating table name: %v", err)
	}
	if tblExists && !(*appendTable || *dropTable) {
			return fmt.Errorf("table already exist, must specify -a or -d option")
	}

	if !tblExists || *dropTable {
		err = createTable(dbpool, headers)
		if err != nil {
			return fmt.Errorf("while creating table: %v", err)
		}
	}

	// read the rest of the file
	var badRowsPos []int
	var inputRows [][]interface{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			// get the details of the error
			var details *csv.ParseError
			if errors.As(err, &details) {
				log.Printf("while reading csv records: %v", err)
				for i:=details.StartLine; i<=details.Line; i++ {
					badRowsPos = append(badRowsPos, i)
				}
			} else {
				return fmt.Errorf("unknown error while reading csv records: %v", err)
			}
		} else {
			copyRec := make([]interface{}, len(record))
			for i, v := range record {
				copyRec[i] = v
			}
			inputRows = append(inputRows, copyRec)
		}
	}
	copyCount, err := dbpool.CopyFrom(context.Background(), pgx.Identifier{*tblName}, headers, pgx.CopyFromRows(inputRows))
	if err != nil {
		return fmt.Errorf("while copy csv to table: %v", err)
	}
	fmt.Println("Inserted",copyCount,"rows in database!")
	if len(badRowsPos) > 0 {
		fmt.Println("Got",len(badRowsPos),"bad rows in input file, copying them to the error file.")
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

	return nil
}

func main() {
	flag.Parse()
	hasErr := false
	var errMsg []string
	if *tblName == "" {
		hasErr = true
		errMsg = append(errMsg, "Table name must be provided.")
	}
	if !isValidName(*tblName) {
		hasErr = true
		errMsg = append(errMsg, "Table name is not valid.")
	}
	if *dsn == "" {
		hasErr = true
		errMsg = append(errMsg, "Connection string must be provided.")
	}
	if *appendTable && *dropTable {
		hasErr = true
		errMsg = append(errMsg, "Cannot specify both -a and -d options.")
	}
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			fmt.Println("**",msg)
		}
		os.Exit((1))
	}
	// fmt.Printf("Got sep: %#U\n",sep_flag)
	// fmt.Println("Got input file name:", *inFile)
	// fmt.Println("Got table name:", *tblName)
	// fmt.Println("Got append file to table:", *appendTable)
	// fmt.Println("Got drop table:", *dropTable)

	err := processFile()
	if err != nil {
		flag.Usage()
		log.Fatal(err)
	}
}