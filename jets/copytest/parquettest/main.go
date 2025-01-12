package main

import (
	"fmt"
	"io"
	"log"
	"os"

	goparquet "github.com/fraugster/parquet-go"
	"github.com/fraugster/parquet-go/parquet"
	"github.com/fraugster/parquet-go/parquetschema"
)

func main() {

	writeFile("output.parquet")
	err := printFile("output.parquet")
	if err != nil {
		log.Fatalln(err)
	}
}

func writeFile(file string) {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("Opening output file failed: %v", err)
	}
	defer f.Close()

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

	fw := goparquet.NewFileWriter(f,
		goparquet.WithCompressionCodec(parquet.CompressionCodec_SNAPPY),
		goparquet.WithSchemaDefinition(schemaDef),
		goparquet.WithCreator("write-lowlevel"),
	)

	inputData := []map[string]any {
		{"aco": "aco1", "start_date": int32(22), "amount": float64(10.99), "status": int32(23), "count": int64(202), "notes": "something", "name": "some name"},
		{"aco": "aco1", "start_date": int32(22), "amount": float64(10.99), "status": int32(23), "count": int64(202), "notes": "something", "name": "some name"},
		{"aco": "aco1", "start_date": int32(22), "amount": float64(10.99), "status": int32(23), "count": int64(202), "notes": "something", "name": "some name"},
		{"aco": "aco1", "start_date": int32(22), "amount": float64(10.99), "status": int32(23), "count": int64(202), "notes": "something", "name": "some name"},
		{"aco": "aco1", "start_date": int32(22), "amount": float64(10.99), "status": int32(23), "count": int64(202), "notes": "something", "name": "some name"},
		{"aco": "aco1", "start_date": int32(22), "amount": float64(10.99), "status": int32(23), "count": int64(202), "notes": "something", "name": "some name"},
		{"aco": "aco1", "start_date": int32(22), "amount": float64(10.99), "status": int32(23), "count": int64(202), "notes": "something", "name": "some name"},
		{"aco": "aco1", "start_date": int32(22), "amount": float64(10.99), "status": int32(23), "count": int64(202), "notes": "something", "name": "some name"},
		{"aco": "aco1", "start_date": int32(22), "amount": float64(10.99), "status": int32(23), "count": int64(202), "notes": "something", "name": "some name"},
		{"aco": "aco1", "start_date": int32(22), "amount": float64(10.99), "status": int32(23), "count": int64(202), "notes": "something", "name": "some name"},
	}

	for i := range inputData {
		if err := fw.AddData(inputData[i]); err != nil {
			log.Fatalf("Failed to add input %v to parquet file: %v", inputData[i], err)
		}
	}

	if err := fw.Close(); err != nil {
		log.Fatalf("Closing parquet file writer failed: %v", err)
	}
}

func printFile(file string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	fr, err := goparquet.NewFileReader(r)
	if err != nil {
		return err
	}

	log.Printf("Printing file %s\n", file)
	log.Printf("Schema: %s\n", fr.GetSchemaDefinition())
	log.Printf("Row Group Count: %d\n", fr.RowGroupCount())
	nrows, err := fr.RowGroupNumRows()
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Nbr of rows in RowGroup: %d\n", nrows)
	err = fr.SeekToRowGroup(1)
	if err != nil {
		log.Panic(err)
	}
	rg := fr.CurrentRowGroup()
	if rg == nil {
		log.Panic("Got no row group!")
	}
	log.Printf("Compression: %s", rg.Columns[0].MetaData.Codec)

	count := 0
	for {
		row, err := fr.NextRow()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading record failed: %w", err)
		}

		log.Printf("Record %d:", count)
		for k, v := range row {
			if vv, ok := v.([]byte); ok {
				v = string(vv)
			}
			log.Printf("\t%s = %v", k, v)
		}

		count++
	}

	log.Printf("End of file %s (%d records)", file, count)
	return nil
}
