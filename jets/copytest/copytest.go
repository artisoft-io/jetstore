package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v4/pgxpool"
)
// dsn := "postgresql://postgres:ArtiSoft001@172.17.0.2:5432/postgres?sslmode=disable"

func main() {
	dsn := "postgresql://postgres:ArtiSoft001@172.17.0.2:5432/postgres?sslmode=disable"
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	var greeting bool
	tblName := "hc__pharmacyclaim"
	err = dbpool.QueryRow(context.Background(), "select exists (select from pg_tables where schemaname = 'public' and tablename = $1)", tblName).Scan(&greeting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("The result is:", greeting)
}