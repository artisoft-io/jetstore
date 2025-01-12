package main

import (
	"fmt"
	"log"

	"github.com/fraugster/parquet-go/parquet"
	"github.com/fraugster/parquet-go/parquetschema"
)

func main() {

	schemaDef, err := parquetschema.ParseSchemaDefinition(
		`message example1 {
			optional binary aco (UTF8);
			optional int32 start_date (DATE);
			optional double amount;
			optional int32 status;
			optional int64 count;
			optional binary notes;
			optional binary name (STRING);
		}`)
	if err != nil {
		log.Fatalf("Parsing schema definition failed: %v", err)
	}

	for i, colDef := range schemaDef.RootColumn.Children {
		se := colDef.SchemaElement
		fmt.Printf("Column %d: %s", i, se.Name)
		if se.Type != nil {
			p := *se.Type
			var txt string
			switch p {
			case parquet.Type_BOOLEAN:
				txt = "BOOLEAN"
			case parquet.Type_INT32:
				txt = "INT32"
			case parquet.Type_INT64:
				txt = "INT64"
			case parquet.Type_INT96:
				txt = "INT96"
			case parquet.Type_FLOAT:
				txt = "FLOAT"
			case parquet.Type_DOUBLE:
				txt = "DOUBLE"
			case parquet.Type_BYTE_ARRAY:
				txt = "BYTE_ARRAY"
			case parquet.Type_FIXED_LEN_BYTE_ARRAY:
				txt = "FIXED_LEN_BYTE_ARRAY"
			default:
				txt = "<UNSET>"
			}
			fmt.Printf(", type: %s", txt)

		} else {
			log.Fatalf("column type is nil")
		}
		if se.ConvertedType != nil {
			p := se.ConvertedType
			var txt string
			switch *p {
			case parquet.ConvertedType_UTF8:
				txt = "UTF8"
			case parquet.ConvertedType_MAP:
				txt = "MAP"
			case parquet.ConvertedType_MAP_KEY_VALUE:
				txt = "MAP_KEY_VALUE"
			case parquet.ConvertedType_LIST:
				txt = "LIST"
			case parquet.ConvertedType_ENUM:
				txt = "ENUM"
			case parquet.ConvertedType_DECIMAL:
				txt = "DECIMAL"
			case parquet.ConvertedType_DATE:
				txt = "DATE"
			case parquet.ConvertedType_TIME_MILLIS:
				txt = "TIME_MILLIS"
			case parquet.ConvertedType_TIME_MICROS:
				txt = "TIME_MICROS"
			case parquet.ConvertedType_TIMESTAMP_MILLIS:
				txt = "TIMESTAMP_MILLIS"
			case parquet.ConvertedType_TIMESTAMP_MICROS:
				txt = "TIMESTAMP_MICROS"
			case parquet.ConvertedType_UINT_8:
				txt = "UINT_8"
			case parquet.ConvertedType_UINT_16:
				txt = "UINT_16"
			case parquet.ConvertedType_UINT_32:
				txt = "UINT_32"
			case parquet.ConvertedType_UINT_64:
				txt = "UINT_64"
			case parquet.ConvertedType_INT_8:
				txt = "INT_8"
			case parquet.ConvertedType_INT_16:
				txt = "INT_16"
			case parquet.ConvertedType_INT_32:
				txt = "INT_32"
			case parquet.ConvertedType_INT_64:
				txt = "INT_64"
			case parquet.ConvertedType_JSON:
				txt = "JSON"
			case parquet.ConvertedType_BSON:
				txt = "BSON"
			case parquet.ConvertedType_INTERVAL:
				txt = "INTERVAL"
			}
			fmt.Printf(", converted type: %s", txt)
		}
		if se.LogicalType != nil {
			fmt.Printf(", logical type: ")
			p := se.LogicalType
			if p.IsSetSTRING() {
				fmt.Printf("string, ")
			}
			if p.IsSetMAP() {
				fmt.Printf("map, ")
			}
			if p.IsSetLIST() {
				fmt.Printf("list, ")
			}
			if p.IsSetENUM() {
				fmt.Printf("enum, ")
			}
			if p.IsSetDECIMAL() {
				fmt.Printf("decimal, ")
			}
			if p.IsSetDATE() {
				fmt.Printf("date, ")
			}
			if p.IsSetTIME() {
				fmt.Printf("time, ")
			}
			if p.IsSetTIMESTAMP() {
				fmt.Printf("timestamp, ")
			}
			if p.IsSetINTEGER() {
				fmt.Printf("integer, ")
			}
			if p.IsSetUNKNOWN() {
				fmt.Printf("unknown, ")
			}
			if p.IsSetJSON() {
				fmt.Printf("json, ")
			}
			if p.IsSetBSON() {
				fmt.Printf("bson, ")
			}
			if p.IsSetUUID() {
				fmt.Printf("uuid, ")
			}

		}
		fmt.Println()

		// log.Printf("Column %d: %s, type: %s, converted type: %s,  logical type: %s, repetition type: %s\n", i,
		// 	se.Name, se.Type, se.ConvertedType.String(), se.LogicalType.String(), se.RepetitionType.String())
	}

}
