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
// see pgx/pgtype/pgtype.go
// https://github.com/jackc/pgx/blob/572d7fff326f1befdbf9f36a0c0a2b6661432079/pgtype/pgtype.go
func DataTypeFromOID(oid uint32) string {
	switch oid {
	case 25, 1009:	                    return "string"
	case 114, 199:	                    return "json"
	case 700,701,1021,1022,1121:        return "double" // not sure where 1121 came from
	case 1082,1182:                     return "date"
	case 1083,1183:                     return "time"
	case 1266,1270:                     return "time"      // timetz with timezone?
	case 1114,1115:                     return "timestamp" // timestamp w/o timezone?
	case 1184,1185:                     return "timestamp" // timestamp with timezone?
	case 23, 1007:                      return "int"
	case 20, 1016:                      return "long"
	}
	return "unknown"
}

func IsArrayFromOID(oid uint32) bool {
	switch oid {
	case 1009,1007,1016,1021,1022,1182, 199, 1183,1115,1185,1270, 1121:	return true
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
