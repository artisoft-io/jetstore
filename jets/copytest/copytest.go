package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// dsn := "postgresql://postgres:<PWD>@<IP>:5432/postgres?sslmode=disable"

func main() {
	dsn := "postgresql://postgres:<PWD>@<IP>:5432/postgres?sslmode=disable"
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	rows := [][]interface{}{
		{"John", "Smith", int32(36), []string{"s1", "s2"}},
		{"Jane", "Doe", int32(29), []string{"s3", "s4"}},
	}

	copyCount, err := dbpool.CopyFrom(
		context.Background(),
		pgx.Identifier{"hc__claim"},
		[]string{"hc__claim_number", "hc__member_number", "shard_id", "rdf__type"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CopyFrom failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("The result is:", copyCount)
}
