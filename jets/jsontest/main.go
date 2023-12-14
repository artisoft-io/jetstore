package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	data := make(map[string]interface{})
	data["columns"] = []string{
		"wrs.Member_SSN",
		"wrs.Walrus_Eligible",
		"wrs.Generated_ID",
		"jets:key",
		"rdf:type",
	}
	data["query"] = `
			SELECT
			e."wrs.Member_SSN",
			e."wrs.Walrus_Eligible",
			e."wrs.Generated_ID",
			e."jets:key",
			'wrs:PreviousEligibility' AS "rdf:type",
			($CURRENT_SOURCE_PERIOD - sr."month_period") AS "jets:source_period_sequence"
		FROM
			"wrs:Eligibility" e,
			jetsapi.session_registry sr
		WHERE
			e.session_id = sr.session_id
			AND sr."month_period" >= ($CURRENT_SOURCE_PERIOD -1)
			AND sr."month_period" < $CURRENT_SOURCE_PERIOD
			AND e."Eligibility:shard_id" = $SHARD_ID
		ORDER BY
			e."Eligibility:domain_key" ASC`
		
  data_json, _ := json.Marshal(data)
  fmt.Println(string(data_json))

	// doing the reverse
	fmt.Println("...Doing the revers now...")
	err := json.Unmarshal(data_json, &data)
	if err != nil {
		panic(err)
	}
	columns := make([]string, 0)
	for _,iface := range data["columns"].([]interface{}) {
		columns = append(columns, iface.(string))
	}
	fmt.Println("Columns are:", columns)
	query := data["query"].(string)
	fmt.Println("The query is:", query)

	doOther1()
}

func doOther1() {
	if(os.Getenv("JETS_STACK_TAGS_JSON") != "") {
		var tags map[string]string
		err := json.Unmarshal([]byte(os.Getenv("JETS_STACK_TAGS_JSON")), &tags)
		if err != nil {
			fmt.Println("** Invalid JSON in JETS_STACK_TAGS_JSON:", err)
			os.Exit(1)
		}
		fmt.Println("Got JETS_STACK_TAGS_JSON:")
		for k, v := range tags {
			fmt.Println("  ",k,"=",v)
		}
	}
}