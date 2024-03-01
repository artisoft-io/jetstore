package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

func main() {

	// fmt.Println("Reading file ./unit_test1.pc.json")
	file, err := os.ReadFile("./unit_test2.pc.json")
	if err != nil {
		panic(fmt.Sprintf("while reading json file:%v\n", err))
	}

	var cpConfig compute_pipes.ComputePipesConfig
	err = json.Unmarshal(file, &cpConfig)
	if err != nil {
		panic(fmt.Sprintf("while unmarshaling json file:%v\n", err))
	}
	// Echo to output
	cpJson, err := json.MarshalIndent(cpConfig, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("while marshaling back to json:%v\n", err))
	}
	fmt.Println(string(cpJson))
}
