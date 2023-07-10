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
