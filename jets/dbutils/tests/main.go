package main

import (
	"context"
	"fmt"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v4/pgxpool"
)

func main() {

	// Get the dsn from the aws secret
	dsn, err := awsi.GetDsnFromSecret(os.Getenv("JETS_DSN_SECRET"), true, 10)
	if err != nil {
		panic(err)
	}
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		panic(err)
	}
	defer dbpool.Close()


	fo := dbutils.FileDbObject{
		WorkspaceName: "walrus_ws",
		FileName: "test file2",
		ContentType: "rules",
		Status: "open",
		UserEmail: "michel@artisoft.io",
	}
	fileContent := "this is the file content"
	n, err := fo.WriteObject(dbpool, []byte(fileContent))
	fmt.Println("WriteObject done of size",n,"error is",err)

	n, err = fo.ReadObject(dbpool, os.Stdout)
	fmt.Println("ReadObject done of size",n,"error is",err)

}