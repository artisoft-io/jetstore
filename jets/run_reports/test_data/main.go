package main

import (
	"log"

	"github.com/xitongsys/parquet-go/reader"
)

func main() {
	var err error

	///read
	fr, err := NewLocalFileReader("csv.parquet")
	if err != nil {
		log.Println("Can't open file")
		return
	}

	pr, err := reader.NewParquetReader(fr, nil, 4)
	if err != nil {
		log.Println("Can't create parquet reader", err)
		return
	}

	num := int(pr.GetNumRows())
	res, err := pr.ReadByNumber(num)
	if err != nil {
		log.Println("Can't read", err)
		return
	}

	log.Println(res)

	pr.ReadStop()
	fr.Close()

}