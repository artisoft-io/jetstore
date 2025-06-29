package main

import (
	"encoding/json"
	"fmt"
)

type A struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Format string `json:"format,omitempty"`
}
type B struct {
	A
	Key string `json:"key"`
}

func main() {

	data := B{
		A: A{
			Type:   "type1",
			Name:   "name1",
			Format: "format1",
		},
		Key: "key1",
	}

	data_json, _ := json.MarshalIndent(data, "", " ")
	fmt.Println(string(data_json))

	// doing the reverse
	fmt.Println("...Doing the revers now...")
	err := json.Unmarshal(data_json, &data)
	if err != nil {
		panic(err)
	}
	fmt.Println("Got Type:", data.Type)
	fmt.Println("Got Key:", data.Key)
}
