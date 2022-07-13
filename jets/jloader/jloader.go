package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
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

var inFile             = flag.String("in_file", "", "the input json file name")
// var dropTable          = flag.Bool("d", false, "drop table if it exists, default is false")
// var dsnList            = flag.String("dsn", "", "comma-separated list of database connection string, order matters and should always be the same (required)")
// var tblName            = flag.String("table", "", "table name to load the data into (required)")
// var groupingColumn     = flag.String("groupingColumn", "", "Grouping column used in server process. This will add an index to the input_table for that column")
// var nbrShards          = flag.Int   ("nbrShards", 1, "Number of shards to use in sharding the input file")
// var sessionId          = flag.String("sessionId", "", "Process session ID, is needed as -inSessionId for the server process (must be unique), default based on timestamp.")
var sep_flag chartype = '|'
func init() {
	flag.Var(&sep_flag, "sep", "Field separator for output csv, default is pipe ('|')")
}

const (
	e_none = iota
	e_start_array
	e_end_array
	e_start_struct
	e_end_struct
	e_string
)

// type Record struct {
// 	billingCode string
// 	npi []string
// 	tinType string
// 	tinValue string
// }


func readToken(dec *json.Decoder) (int, json.Token, error) {
	t, err := dec.Token()
	if err != nil {
		return e_none, t, err
	}

	switch v := t.(type) { 
	case json.Delim:
		switch fmt.Sprintf("%v", t) {
		case "[":
				return e_start_array, t, nil
		case "]":
				return e_end_array, t, nil
		case "{":
				return e_start_struct, t, nil
		case "}":
				return e_end_struct, t, nil
		default:
			return e_none, t, fmt.Errorf("error, unknown delimit %v",t)
		}
		case string:
			return e_string, t, nil
		default:
			return e_none, t, fmt.Errorf("error, unexpected type %T in json", v)
		} 
}
// skip to the struct key keyName, does not read the value
func skipTo(dec *json.Decoder, keyName string) error {
	for {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		if fmt.Sprintf("%s", t) == keyName {
			return nil
		}
		_, err = dec.Token()
		if err != nil {
			return err
		}
	}
}

func expectStartStruct(dec *json.Decoder) error {
	d, t, err := readToken(dec)
	if err != nil {
		return err
	}
	if d != e_start_struct {
		return fmt.Errorf("error, expecting '{' got '%v'", t)
	}
	return nil
}

func expectStartArray(dec *json.Decoder) error {
	d, t, err := readToken(dec)
	if err != nil {
		return err
	}
	if d != e_start_array {
		return fmt.Errorf("error, expecting '[' got '%v'", t)
	}
	return nil
}

func expectString(dec *json.Decoder) (string, error) {
	d, t, err := readToken(dec)
	if err != nil {
		return "", err
	}
	if d != e_string {
		return "", fmt.Errorf("error, expecting a string got '%v'", t)
	}
	return t.(string), nil
}

// processFile
// --------------------------------------------------------------------------------------
func processFile() error {
	// open json file
	file, err := os.Open(*inFile)
	if err != nil {
		return fmt.Errorf("error while opening json file: %v", err)
	}
	defer file.Close()

	// open and read first token
	dec := json.NewDecoder(file)
	err = expectStartStruct(dec)
	if err != nil {
			return err
	}

	err = skipTo(dec, "in_network")
	if err != nil {
		return err
	}
	err = expectStartArray(dec)
	if err != nil {
			return err
	}
	fmt.Println("OK, got to in-network and ready to process the array!")
  // while the array contains values
	for dec.More() {
		err = expectStartStruct(dec)
		if err != nil {
				return err
		}	
		err = skipTo(dec, "billing_code")
		if err != nil {
			return err
		}
		val, err := expectString(dec)
		if err != nil {
			return err
		}
		fmt.Println("Finally we got billing_code:", val)
	}

	return nil
}

func main() {
	flag.Parse()
	hasErr := false
	var errMsg []string
	if *inFile == "" {
		hasErr = true
		errMsg = append(errMsg, "Input file is required. (-inFile)")
	}
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			fmt.Println("**",msg)
		}
		os.Exit((1))
	}

	fmt.Println("jloader argument:")
	fmt.Println("----------------")
	fmt.Println("Got argument: inFile",*inFile)
	fmt.Println("Got argument: sep_flag",sep_flag)

	err := processFile()
	if err != nil {
		flag.Usage()
		log.Fatal(err)
	}
}