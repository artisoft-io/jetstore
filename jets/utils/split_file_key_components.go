package utils

import (
	"log"
	"strconv"
	"strings"
	"time"
)

func splitFileKey(keyMap map[string]any, fileKey *string) map[string]any {
	if fileKey != nil {
		for _, component := range strings.Split(*fileKey, "/") {
			elms := strings.Split(component, "=")
			if len(elms) == 2 {
				keyMap[elms[0]] = elms[1]
				if elms[0] == "vendor" {
					keyMap["org"] = elms[1]
				}
			}
		}
	}
	return keyMap
}

// SplitFileKeyIntoComponents splits a file key's key=value path components into
// keyMap and types year, month and day as int. A component that is absent defaults
// to 1970, 1 and 1; one that is present and not a number is 0, which is what
// strconv.Atoi returns on error, and the log line says so.
//
// The 0 is kept, not replaced by the default, because callers tell a bad date
// from a real one by it: FileKeyDate (so a job failure's period, fileKeyPeriod in
// jets/datatable, and a schema event's source period) refuses a year below 1 and
// a month or day that does not survive time.Date, and an Atoi failure read as 1970
// would pass both. A run started on such a key reads $DATE_FILE_KEY from the 0s
// (jets/compute_pipes, actions_start_common.go), and a key registered with one
// lands in a source period of year 0.
func SplitFileKeyIntoComponents(keyMap map[string]any, fileKey *string) map[string]any {
	var err error
	fileKeyObject := splitFileKey(keyMap, fileKey)
	fileKeyObject["file_key"] = *fileKey
	year := 1970
	if fileKeyObject["year"] != nil {
		year, err = strconv.Atoi(fileKeyObject["year"].(string))
		if err != nil {
			log.Printf("File Key with invalid year: %s, setting to 0", fileKeyObject["year"])
		}
	}
	month := 1
	if fileKeyObject["month"] != nil {
		month, err = strconv.Atoi(fileKeyObject["month"].(string))
		if err != nil {
			log.Printf("File Key with invalid month: %s, setting to 0", fileKeyObject["month"])
		}
	}
	day := 1
	if fileKeyObject["day"] != nil {
		day, err = strconv.Atoi(fileKeyObject["day"].(string))
		if err != nil {
			log.Printf("File Key with invalid day: %s, setting to 0", fileKeyObject["day"])
		}
	}
	// Updating object attribute with correct type
	fileKeyObject["year"] = year
	fileKeyObject["month"] = month
	fileKeyObject["day"] = day
	return fileKeyObject
}

// FileKeyDate is the date a file key carries, as year=, month= and day= components
// read by SplitFileKeyIntoComponents: ok only when the key carries a year and the
// three make a calendar date, month and day defaulting to 1. A key with no year,
// a component that is not a number, or a date that does not exist (a month of 13,
// February 30) gives ok false.
func FileKeyDate(fileKey string) (t time.Time, ok bool) {
	if fileKey == "" || !FileKeyCarries(fileKey, "year") {
		return time.Time{}, false
	}
	c := SplitFileKeyIntoComponents(map[string]any{}, &fileKey)
	year, _ := c["year"].(int)
	month, _ := c["month"].(int)
	day, _ := c["day"].(int)
	t = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	// time.Date normalises a month of 13 into the next year; a date that does
	// not survive the round trip is not the file's.
	if year < 1 || t.Year() != year || int(t.Month()) != month || t.Day() != day {
		return time.Time{}, false
	}
	return t, true
}

// FileKeyCarries reports whether the file key carries the component name as a
// name=value part of its path, as SplitFileKeyIntoComponents reads the key (a
// vendor component carries org too). It tells a year, month or day the key
// carries from one SplitFileKeyIntoComponents defaulted, which it does not
// distinguish.
func FileKeyCarries(fileKey, name string) bool {
	_, ok := splitFileKey(map[string]any{}, &fileKey)[name]
	return ok
}
