package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
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

// Processing Functions
// --------------------------------------------------------------------------------------
func tableExists(dbpool *pgxpool.Pool) ( exists bool, err error) {
	err = dbpool.QueryRow(context.Background(), "select exists (select from pg_tables where schemaname = 'public' and tablename = $1)", *tblName).Scan(&exists)
	if err != nil {
		err = fmt.Errorf("QueryRow failed: %w", err)
	}
	return exists, err
}
func isValidName(name string) bool {
	return !strings.ContainsAny(name, "=;\t\n ")
}
func makeValid(name string) string {
	dropInvalid := func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		}
		return -1
	}
	return strings.Map(dropInvalid, name)
}

func createTable(dbpool *pgxpool.Pool, headers []string) (err error) {
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", *tblName)
	//*
	fmt.Println("Drop table statement:")
	fmt.Println(stmt)
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while droping table: %w", err)
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
	//*
	fmt.Println("Create table statement:")
	fmt.Println(stmt)

	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while creating table: %w", err)
	}
	return err
}

func processFile() error {
	// open csv file
	file, err := os.Open(*inFile)
	if err != nil {
		return fmt.Errorf("error while opening csv file: %w", err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = rune(sep_flag)

	// read the headers
	rawHeaders, err := reader.Read()
	if err == io.EOF {
		return errors.New("input csv file is empty")
	} else if err != nil {
		return fmt.Errorf("while reading csv headers: %w", err)
	}
	var headers []string
	for _, header := range rawHeaders {
		headers = append(headers, makeValid(header))
	}
	//*
	fmt.Println("Header:")
	fmt.Println(rawHeaders)
	fmt.Println("Validated Header:")
	fmt.Println(headers)
	fmt.Println()

	// open db connection
	fmt.Println("Connecting using ",*dsn)
	dbpool, err := pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %w", err)
	}
	defer dbpool.Close()

	// validate table name
	tblExists, err := tableExists(dbpool)
	if err != nil {
		return fmt.Errorf("while validating table name: %w", err)
	}
	if tblExists && !(*appendTable || *dropTable) {
			return fmt.Errorf("table already exist, must specify -a or -d option")
	}

	if !tblExists || *dropTable {
		err = createTable(dbpool, headers)
		if err != nil {
			return fmt.Errorf("while creating table: %w", err)
		}
	}

	// read the rest of the file
	var inputRows [][]interface{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("while reading csv records: %w", err)
		}
		copyRec := make([]interface{}, len(record))
		for i, v := range record {
			copyRec[i] = v
		}
		inputRows = append(inputRows, copyRec)
	}
	copyCount, err := dbpool.CopyFrom(context.Background(), pgx.Identifier{*tblName}, headers, pgx.CopyFromRows(inputRows))
	if err != nil {
		return fmt.Errorf("while copy csv to table: %w", err)
	}
	//*
	fmt.Println("Inserted",copyCount,"rows in database!")
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
	fmt.Printf("Got sep: %#U\n",sep_flag)
	fmt.Println("Got input file name:", *inFile)
	fmt.Println("Got table name:", *tblName)
	fmt.Println("Got append file to table:", *appendTable)
	fmt.Println("Got drop table:", *dropTable)

	err := processFile()
	if err != nil {
		log.Fatal(err)
	}
}