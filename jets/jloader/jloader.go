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
	e_number
)

var eTok []string

func init() {
	eTok = []string{
		"none",
		"[",
		"]",
		"{",
		"}",
		"string",
		"number",
	}
}

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
		case float64:
			return e_number, t, nil
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

func expectDelimitToken(dec *json.Decoder, tok int) error {
	d, t, err := readToken(dec)
	if err != nil {
		return err
	}
	if d != tok {
		return fmt.Errorf("error, expecting '%s' got '%v'", eTok[tok], t)
	}
	return nil
}

func ToArray(dec *json.Decoder, number_format string) ([]string, error) {
	result := make([]string, 0)
	for dec.More() {
		str, err := expectString(dec, number_format)
		if err != nil {
			return result, err
		}
		result = append(result, str)
	}
	err := expectDelimitToken(dec, e_end_array)
	if err != nil {
		return result, err
	}
	return result, nil
}

func ToString(token_type int, tok json.Token, number_format string) (string, error) {
	switch token_type {
	case e_number:
		return fmt.Sprintf(number_format, tok), nil
	case e_string:
		return tok.(string), nil
	default:
		return "", fmt.Errorf("error, expecting a string got '%v'", tok)
	}
}

func expectString(dec *json.Decoder, number_formatter string) (string, error) {
	d, t, err := readToken(dec)
	if err != nil {
		return "", err
	}
	return ToString(d, t, number_formatter)
}

// Skip next entity: string, struct and array
// error otherwise
func skipEntity(dec *json.Decoder) error {
	d, _, err := readToken(dec)
	if err != nil {
		return err
	}
	switch d {
	case e_start_array, e_start_struct:
		// skip the whole array/struct
		for dec.More() {
			skipEntity(dec)
		}
		dd := e_end_array
		if d == e_start_struct {
			dd = e_end_struct
		}
		err = expectDelimitToken(dec, dd)
		if err != nil {
			return err
		}
		return nil
	case e_end_array, e_end_struct:
		return fmt.Errorf("error while skipping entity, unexpected %s",eTok[d])
	default:
		return nil
	}
}

type Path struct {
	components []string
}
func (p *Path) isMatch (basePath []string, token string) bool {
	// fmt.Println("IsMatch(",basePath,token,") on p",p)
	l := len(basePath)
	if l < len(p.components) {
		for i := range basePath {
			if p.components[i] != basePath[i] {
				return false
			}
		}
		if p.components[l] == token {
			return true
		}
	}
	return false
}
func (p *Path) isComplete (level int) bool {
	return level == len(p.components) -1 
}

type PathExtractor struct {
	paths []Path
}
// basePath indicate segments of path between root and current position
func (pe *PathExtractor) extractPaths(dec *json.Decoder, basePath []string, cb func (int, int, json.Token, error) ) error {
	level := len(basePath)
	for dec.More() {
		key, err := expectString(dec, "%.f")
		if err != nil {
			return err
		}
		// fmt.Println("\npathExtractor on:",key)
		matchFound := false
		matchConsumed := false	// indicated value consumed, no need to skip or visit
		for i := range pe.paths {
			if pe.paths[i].isMatch(basePath, key) {
				matchFound = true
				if pe.paths[i].isComplete(level) {
					matchConsumed = true
					// fmt.Println("match on",key, "extracting value")
					d, t, err := readToken(dec)
					cb(i, d, t, err)
				}
				break
			}
		}
		if !matchConsumed {
			if matchFound {
				// fmt.Println("match on",key, "going in...")
				d, _, err := readToken(dec)
				if err != nil {
					return err
				}
				newBasePath := append(basePath, key)
				if d == e_start_array {
					// visit each elm
					for dec.More() {
						err = expectDelimitToken(dec, e_start_struct)
						if err != nil {
							return err
						}
						err = pe.extractPaths(dec, newBasePath, cb)
						if err != nil {
							return err
						}
					}
					err := expectDelimitToken(dec, e_end_array)
					if err != nil {
						return err
					}
				} else {
					err = pe.extractPaths(dec, newBasePath, cb)
					if err != nil {
						return err
					}
				}
			} else {
				// fmt.Println("No match on",key,"skipping token/entity")
				err = skipEntity(dec)
				if err != nil {
					return err
				}
			}
		}
	}
	err := expectDelimitToken(dec, e_end_struct)
	if err != nil {
		return err
	}
	return nil
}

// Data structure to hold the extracted information
type Record struct {
	billingCode string
	providerGroups []ProviderGroup
}
type ProviderGroup struct {
	npi []string
	tinType string
	tinValue string
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
	err = expectDelimitToken(dec, e_start_struct)
	if err != nil {
			return err
	}
	// Allocated the data structure to hold the extracted data
	records := make([]Record, 0)
	record := Record{
		providerGroups: make([]ProviderGroup, 0),
	}
	providerGroup := ProviderGroup{
	}
	// The paths of interest
	pe := PathExtractor{
		paths: []Path{
			{components: []string{"in_network", "billing_code"}},
			{components: []string{"in_network", "negotiated_rates", "provider_groups", "npi"}},
			{components: []string{"in_network", "negotiated_rates", "provider_groups", "tin", "type"}},
			{components: []string{"in_network", "negotiated_rates", "provider_groups", "tin", "value"}},
		},
	}
	pe.extractPaths(dec, []string{}, func (path_index int, token_type int, token json.Token, err error) {
		switch path_index {
		case 0:
			if token_type != e_string {
				fmt.Println("error, expecting string for billing_code, got", eTok[token_type])
			}
			str, err := ToString(token_type, token, "%.f")
			if err != nil {
				fmt.Println("Error while ToString on billing_code:",err)
			} else {
				// fmt.Println("Got billing_code:",str)
				if len(record.providerGroups) > 0 {
					records = append(records, record)
					record = Record{
						providerGroups: make([]ProviderGroup, 0),
					}
				}
				record.billingCode = str
			}
		case 1:
			if token_type != e_start_array {
				fmt.Println("error, expecting array for npi, got", eTok[token_type])
			}
			values, err := ToArray(dec, "%.f")
			if err != nil {
				fmt.Println("Error while ToArray on npi:",err)
			} else {
				// fmt.Println("Got npi:",values)
				if(len(providerGroup.npi) > 0) {
					record.providerGroups = append(record.providerGroups, providerGroup)
					providerGroup = ProviderGroup{}
				} 
				providerGroup.npi = values
			}
		case 2:
			if token_type != e_string {
				fmt.Println("error, expecting string for tin.type, got", eTok[token_type])
			}
			str, err := ToString(token_type, token, "%.f")
			if err != nil {
				fmt.Println("Error while ToString on tin.type:",err)
			} else {
				// fmt.Println("Got tin_type:",str)
				providerGroup.tinType = str
			}
		case 3:
			if token_type != e_string {
				fmt.Println("error, expecting string for tin.value, got", eTok[token_type])
			}
			str, err := ToString(token_type, token, "%.f")
			if err != nil {
				fmt.Println("Error while ToString on tin.value:",err)
			} else {
				// fmt.Println("Got tin_value:",str)
				providerGroup.tinValue = str
			}
		}
	})
	fmt.Println("That's it!")
	for i := range records {
		fmt.Println(records[i])
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