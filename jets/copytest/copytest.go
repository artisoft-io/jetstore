package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// dsn := "postgresql://postgres:<PWD>@<IP>:5432/postgres?sslmode=disable"

// Env variables:
// JETS_BUCKET
// JETS_DSN_SECRET
// JETS_REGION
func main() {
	// Get the dsn from the aws secret
	dsn, err := awsi.GetDsnFromSecret(os.Getenv("JETS_DSN_SECRET"), true, 3)
	if err != nil {
		log.Panicln(fmt.Errorf("while getting dsn from aws secret: %v", err))
	}
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		panic(err)
	}
	defer dbpool.Close()

	// writing to db
	fmt.Println("Writing to database")
	in_rows := [][]any{
		{"012345678901", "12345", "KEY\\12345", int32(36), []any{"s1", "s2"}},
		{"012345678902", "12346", "KEY\"12346", int32(29), []any{"s3", "s4"}},
	}

	copyCount, err := dbpool.CopyFrom(
		context.Background(),
		pgx.Identifier{"test_table"},
		[]string{"hc:claim_number", "hc:member_number", "jets:key", "shard_id", "rdf:type"},
		pgx.CopyFromRows(in_rows),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CopyFrom failed: %v\n", err)
		panic(err)
	}

	fmt.Println("The result is:", copyCount)

	fmt.Println("Reading from Database. . .")
	// full data structure
	dataGrps := make([][]any, 0)
	// row structure to scan the data
	scans := make([]any, 0) 
	scans = append(scans, &sql.NullString{}) // "hc:claim_number"
	scans = append(scans, &sql.NullString{}) // "hc:member_number"
	scans = append(scans, &sql.NullString{})	// "jets:key"
	scans = append(scans, &sql.NullInt32{}) 	// "shard_id"
	scans = append(scans, &[]string{})	     	// "rdf:type"

	// reading from db
	sqlstmt := `SELECT "hc:claim_number", "hc:member_number", "jets:key", "shard_id", "rdf:type" FROM "test_table"`
	tag, err := dbpool.QueryFunc(context.Background(), sqlstmt, nil, scans, func(qfr pgx.QueryFuncRow) error {
		// type or unwrap the data as needed
		row := make([]any, 0) 
		row = append(row, scans[0].(*sql.NullString).String)
		row = append(row, scans[1].(*sql.NullString).String)
		row = append(row, scans[2].(*sql.NullString).String)
		row = append(row, scans[3].(*sql.NullInt32).Int32)
		vv := make([]any, 0)
		for _, v := range *scans[4].(*[]string) {
			vv = append(vv, v)
		}
		row = append(row, vv)
		dataGrps = append(dataGrps, row)
		return nil
	})

	if err != nil {
		log.Panicln("QueryFunc failed:", err)
	}
	log.Println("QueryFunc Result:", tag)
	log.Println("We have:")
	for i := range dataGrps {
		log.Println(dataGrps[i])
	}
	log.Println()
}

