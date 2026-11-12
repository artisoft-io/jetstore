package utils

import (
	"log"
	"strconv"
	"strings"
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
// strconv.Atoi returns on error.
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
