package dbutils

import (
	"encoding/json"
	"log"
)

// Basic utility functions and struct for jetstore database api

// Transform a struct into json and then back into a row (array of interface{})
// to insert into database using api
func AppendDataRow(v any, data *[]map[string]interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("while writing json:%v\n", err)
		return err
	}
	row := map[string]interface{}{}
	err = json.Unmarshal(b, &row)
	if err != nil {
		log.Printf("while reading json:%v\n", err)
		return err
	}
	*data = append(*data, row)
	return nil
}

// Utilities for converting postgres oid to data types
func DataTypeFromOID(oid uint32) string {
	switch oid {
	case 25, 1009:	                    return "string"
	case 700,701,1121:                  return "double"
	case 1082,1182:                     return "date"
	case 1083,1183:                     return "time"
	case 1114,1115:                     return "timestamp"
	case 23, 1007:                      return "int"
	case 20, 1016:                      return "long"
	}
	return "unknown"
}

func IsArrayFromOID(oid uint32) bool {
	switch oid {
	case 1009, 1007,1182, 1183, 1115, 1121, 1016:	return true
	}
	return false
}

func IsNumeric(dtype string) bool {
	switch dtype {
	case "int", "long", "uint", "ulong", "double":
		return true
	default:
		return false
	}
}
