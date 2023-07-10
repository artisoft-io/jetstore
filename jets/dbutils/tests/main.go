package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v4/pgxpool"
)

func main() {

	// Get the dsn from the aws secret
	dsn, err := awsi.GetDsnFromSecret(os.Getenv("JETS_DSN_SECRET"),os.Getenv("JETS_REGION"), true, 10)
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
	fd, err := os.Open("./test1.jr")
	if err != nil {
		panic(err)
	}
	n, err := fo.WriteObject(dbpool, fd)
	fmt.Println("WriteObject done of size",n,"object oid is",fo.Oid,"error is",err)

	n, err = fo.ReadObject(dbpool, os.Stdout)
	fmt.Println("ReadObject done of size",n,"object oid is",fo.Oid,"error is",err)
	fmt.Println("The FileDbObject is:")
	b, _ := json.MarshalIndent(fo, "", " ")
	fmt.Println(string(b))

	// Updating File Object content with "updated_test1.jr"
	fmt.Println("\n-------------------")
	fmt.Println()
	fmt.Println("Updating FileDbObject content")
	fd, err = os.Open("./updated_test1.jr")
	if err != nil {
		panic(err)
	}
	n, err = fo.WriteObject(dbpool, fd)
	fmt.Println("UPDATED WriteObject done of size",n,"object oid is",fo.Oid,"error is",err)
	n, err = fo.ReadObject(dbpool, os.Stdout)
	fmt.Println("UPDATED ReadObject done of size",n,"object oid is",fo.Oid,"error is",err)
	fmt.Println("The UPDATED FileDbObject is:")
	b, _ = json.MarshalIndent(fo, "", " ")
	fmt.Println(string(b))

}