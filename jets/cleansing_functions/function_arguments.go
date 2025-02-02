package cleansing_functions

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
)

// Support functions and types


type ConcatFunctionArg struct {
	Delimit         string
	ColumnPositions []int
}

func ParseConcatFunctionArgument(rawArg string, functionName string, inputColumnName2Pos *map[string]int, 
	cache map[string]interface{}, input *[]interface{}) (*ConcatFunctionArg, error) {
	// rawArg is csv-encoded
	if rawArg == "" {
		return nil, fmt.Errorf("unexpected null argument to %s function", functionName)
	}
	if input == nil {
		return nil, fmt.Errorf("error: input row is required for concat and concat_with cleansing functions")
	}
	// Check if we have it cached
	v := cache[rawArg]
	if v != nil {
		// fmt.Println("*** OK Got Cached value for", rawArg)
		return v.(*ConcatFunctionArg), nil
	}
	// Parsed the raw argument into ConcatFunctionArg and put it in the cache
	rows, err := jcsv.Parse(rawArg)
	if len(rows) == 0 || len(rows[0]) == 0 || err != nil {
		// It's not csv or there's no data
		return nil, fmt.Errorf("error:no-data: argument %s cannot be parsed as csv: %v (%s function)", rawArg, err, functionName)
	}
	results := &ConcatFunctionArg{
		ColumnPositions: make([]int, 0),
	}
	for i := range rows[0] {
		if i == 0 && functionName == "concat_with" {
			results.Delimit = rows[0][i]
		} else {
			colPos, ok := (*inputColumnName2Pos)[rows[0][i]]
			// fmt.Println("*** concat:",row[i],"value @:", colPos,"ok?",ok)
			if !ok {
				// Column not found
				return nil, fmt.Errorf("error:column-not-fount: argument %s is not an input column name (%s function)", rawArg, functionName)
			}
			results.ColumnPositions = append(results.ColumnPositions, colPos)
		}
	}
	cache[rawArg] = results
	return results, nil
}

type SubStringFunctionArg struct {
	Start int
	End   int
}

func ParseSubStringFunctionArgument(rawArg string, functionName string, cache map[string]interface{}) (*SubStringFunctionArg, error) {
	// rawArg is comma separated as: start,end
	if rawArg == "" {
		return nil, fmt.Errorf("unexpected null argument to %s function", functionName)
	}
	// Check if we have it cached
	v := cache[rawArg]
	if v != nil {
		// fmt.Println("*** OK Got Cached value for", rawArg,":",v)
		return v.(*SubStringFunctionArg), nil
	}
	// Parsed the raw argument into SubStringFunctionArg and put it in the cache
	row := strings.Split(rawArg, ",")
	if len(row) != 2 {
		// The argument is not valid
		return nil, fmt.Errorf("error: argument %s cannot be parsed as start,end (%s function)", rawArg, functionName)
	}
	start, err := strconv.Atoi(strings.TrimSpace(row[0]))
	if err != nil {
		return nil, fmt.Errorf("error: argument %s cannot be parsed as start,end: %v (%s function)", rawArg, err, functionName)
	}
	end, err := strconv.Atoi(strings.TrimSpace(row[1]))
	if err != nil || (end > 0 && end <= start) {
		return nil, fmt.Errorf("error: argument %s cannot be parsed as start,end: %v (%s function)", rawArg, err, functionName)
	}
	results := &SubStringFunctionArg{
		Start: start,
		End:   end,
	}
	cache[rawArg] = results
	return results, nil
}

type FindReplaceFunctionArg struct {
	Find        string
	ReplaceWith string
}

func ParseFindReplaceFunctionArgument(rawArg string, functionName string, cache map[string]interface{}) (*FindReplaceFunctionArg, error) {
	// rawArg is csv-encoded: "text to find","text to replace with"
	if rawArg == "" {
		return nil, fmt.Errorf("unexpected null argument to %s function", functionName)
	}
	// Check if we have it cached
	v := cache[rawArg]
	if v != nil {
		// fmt.Println("*** OK Got Cached value for", rawArg,":",v)
		return v.(*FindReplaceFunctionArg), nil
	}
	// Parsed the raw argument into FindReplaceFunctionArg and put it in the cache
	rows, err := jcsv.Parse(rawArg)
	if len(rows) == 0 || len(rows[0]) != 2 || err != nil {
		// It's not csv or there's no data
		return nil, fmt.Errorf("error:no-data: argument %s cannot be parsed as csv: %v (%s function)", rawArg, err, functionName)
	}

	results := &FindReplaceFunctionArg{
		Find:        rows[0][0],
		ReplaceWith: rows[0][1],
	}
	cache[rawArg] = results
	return results, nil
}

func filterDigits(str string) string {
	// Remove non digits characters
	var buf strings.Builder
	for _, c := range str {
		if unicode.IsDigit(c) {
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

func filterDouble(str string) string {
	// clean up the amount
	var buf strings.Builder
	var c rune
	for _, c = range str {
		if c == '(' || c == '-' {
			buf.WriteRune('-')
		} else if unicode.IsDigit(c) || c == '.' {
			buf.WriteRune(c)
		}
	}
	return buf.String()
}
