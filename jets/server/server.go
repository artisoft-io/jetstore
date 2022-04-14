package main

import (
	// "bufio"
	// "context"
	// "encoding/csv"
	"errors"
	"flag"
	"fmt"
	// "io"
	"log"
	"os"
	// "path/filepath"
	// "strings"

	// "github.com/jackc/pgx/v4"
	// "github.com/jackc/pgx/v4/pgxpool"
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

// var inFile = flag.String("in_file", "/work/input.csv", "the input csv file name")
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
func processFile() error {
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