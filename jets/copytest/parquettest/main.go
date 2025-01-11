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
		`message test {
			required int64 id;
			required binary city (STRING);
			optional int64 population;
		}`)
	if err != nil {
		log.Fatalf("Parsing schema definition failed: %v", err)
	}

	fw := goparquet.NewFileWriter(f,
		goparquet.WithCompressionCodec(parquet.CompressionCodec_SNAPPY),
		goparquet.WithSchemaDefinition(schemaDef),
		goparquet.WithCreator("write-lowlevel"),
	)

	inputData := []struct {
		ID   int
		City string
		Pop  int
	}{
		{ID: 1, City: "Berlin", Pop: 3520031},
		{ID: 2, City: "Hamburg", Pop: 1787408},
		{ID: 3, City: "Munich", Pop: 1450381},
		{ID: 4, City: "Cologne", Pop: 1060582},
		{ID: 5, City: "Frankfurt", Pop: 732688},
	}

	for _, input := range inputData {
		if err := fw.AddData(map[string]interface{}{
			"id":         int64(input.ID),
			"city":       []byte(input.City),
			"population": int64(input.Pop),
		}); err != nil {
			log.Fatalf("Failed to add input %v to parquet file: %v", input, err)
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
