package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// dsn := "postgresql://postgres:<PWD>@<IP>:5432/postgres?sslmode=disable"

func main() {
	dsn := "postgresql://postgres:ArtiSoft001@172.17.0.2:5432/postgres?sslmode=disable"
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	// writing to db
	fmt.Println("Writing to database")
	in_rows := [][]interface{}{
		{"012345678901", "12345", "KEY12345", int32(36), []string{"s1", "s2"}},
		{"012345678902", "12346", "KEY12346", int32(29), []string{"s3", "s4"}},
	}

	copyCount, err := dbpool.CopyFrom(
		context.Background(),
		pgx.Identifier{"hc:Claim"},
		[]string{"hc:claim_number", "hc:member_number", "jets:key", "shard_id", "rdf:type"},
		pgx.CopyFromRows(in_rows),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CopyFrom failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("The result is:", copyCount)

	fmt.Println("Reading from Database. . .")
	// reading from db
	sqlstmt := `SELECT "hc:claim_number", "hc:member_number", "jets:key", "shard_id", "rdf:type" FROM "hc:Claim"`
	rows, err := dbpool.Query(context.Background(), sqlstmt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()
	rowCount := 0

	// full data structure
	dataGrps := make([][]interface{}, 0)
	
	for rows.Next() {
		// single row structure
		dataRow := make([]interface{}, 0) 
		dataRow = append(dataRow, &sql.NullString{}) // "hc:claim_number"
		dataRow = append(dataRow, &sql.NullString{}) // "hc:member_number"
		dataRow = append(dataRow, &sql.NullString{})	// "jets:key"
		dataRow = append(dataRow, &sql.NullInt32{}) 	// "shard_id"
		dataRow = append(dataRow, &[]string{})	     	// "rdf:type"
		// scan the row
		if err := rows.Scan(dataRow...); err != nil {
			fmt.Fprintf(os.Stderr, "Scan failed: %v\n", err)
			os.Exit(1)
		}
		dataGrps = append(dataGrps, dataRow)

		// // single row structure
		// v0 := sql.NullString{}
		// v1 := sql.NullString{}
		// v2 := sql.NullString{}
		// v3 := sql.NullString{}
		// v4 := []string{}
		// // scan the row
		// if err := rows.Scan(&v0, &v1, &v2, &v3, &v4); err != nil {
		// 	fmt.Fprintf(os.Stderr, "Scan failed: %v\n", err)
		// 	os.Exit(1)
		// }
		// fmt.Println(" Got Row ",v0," ",v1," ",v2," ",v3," ",v4)

		rowCount += 1
	}

	fmt.Println("We have:")
	for i := range dataGrps {
		rr := dataGrps[i]
		fmt.Print("    ")
		v0 := rr[0].(*sql.NullString)
		v1 := rr[1].(*sql.NullString)
		v2 := rr[2].(*sql.NullString)
		v3 := rr[3].(*sql.NullInt32)
		v4 := rr[4].(*[]string)
		fmt.Println(" ", v0.String," ", v1.String," ", v2.String," ", v3.Int32," ", *v4)
	}
	fmt.Println()
}

