package dbutils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Functions to read and write files to postgres as blob

// Env variables dependency:
// ---

type FileDbObject struct {
	Key           int    `json:"key"`
	WorkspaceName string `json:"workspace_name"`
	Oid           uint32 `json:"oid"`
	FileName      string `json:"file_name"`
	ContentType   string `json:"content_type"`
	Status        string `json:"status"`
	UserEmail     string `json:"user_email"`
}

// Insert the content of fd into database with metadata specified by fo
// Expect to have fo.workspace_name and fo.file_name available
// Will populate fo.Oid and fo.Key
func (fo *FileDbObject)WriteObject(dbpool *pgxpool.Pool, fd *os.File, token *string) (int64, error) {
	// data, err := os.ReadFile("/tmp/dat")
	// Read bytes from the file to be imported as large object
	// b, err := ioutil.ReadFile(pathToLargeObjectFile)
	ctx := context.Background()
	txWrite, err := dbpool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer txWrite.Rollback(ctx)

	// Query oid associated with workspace/file if entry exist
	// Read FileDbObject metadata
	stmt := `SELECT oid	FROM jetsapi.workspace_changes WHERE workspace_name = $1 AND file_name = $2`
	err = txWrite.QueryRow(ctx, stmt, fo.WorkspaceName, fo.FileName).Scan(&fo.Oid)
	if err != nil {
		if err.Error() == "no rows in result set" {
			fo.Oid = 0
		} else {
			return 0, fmt.Errorf("while reading from workspace_changes table: %v", err)
		}
	}

	loWrite := txWrite.LargeObjects()
	if fo.Oid == 0 {
		fo.Oid, err = loWrite.Create(ctx, 0)
		if err != nil {
			return 0, err
		}	
	}

	// insert the metadata - transform it into a data row (map)
	data := []map[string]interface{}{}
	err = AppendDataRow(fo, &data)
	if err != nil {
		return 0, err
	}
	// insert in jetstore db using api
	//* TODO Move core db api (datatable.DataTableAction) to dbutils, avoid datatable.Context in core func
	//       Keep the token as argument for future use, so to be able to check user has role/capability
	//* TODO Make this insert be part of txWrite transaction ***
	dataTableAction := &datatable.DataTableAction{
		Action:      "insert_rows",
		FromClauses: []datatable.FromClause{{Schema: "jetsapi", Table: "workspace_changes"}},
		Data:        data,
	}
	datatableCtx := &datatable.Context{
		Dbpool: dbpool,
	}
	results, _, err := datatableCtx.InsertRows(dataTableAction, *token)
	if err != nil {
		log.Printf("while calling InsertRows:%v\n", err)
		return 0, err
	}
	if results == nil {
		err = fmt.Errorf("unexpected error: results of InsertRows is null")
		log.Println(err)
		return 0, err
	}
	keys := (*results)["returned_keys"].(*[]int)
	fo.Key = (*keys)[0]

	// open blob with ID
	obj, err := loWrite.Open(ctx, fo.Oid, pgx.LargeObjectModeWrite)
	if err != nil {
		return 0, err
	}

	reader := bufio.NewReader(fd)
	n, err := io.Copy(obj, reader)
	if err != nil {
		return 0, err
	}
	err = txWrite.Commit(ctx)
	return n, err
}

// Expect to have fo.WorkspaceName and fo.FileName available
// (alternatively could use fo.Key in future)
// Returns fo with Oid, ContentType, Status, and UserEmail populated
// and write the object in fd
func (fo *FileDbObject)ReadObject(dbpool *pgxpool.Pool, fd *os.File) (int64, error) {
	ctx := context.Background()
	txRead, err := dbpool.Begin(ctx)
	if err != nil {
			return 0, err
	}
	defer txRead.Rollback(ctx)

	// Read FileDbObject metadata
	stmt := `SELECT oid, content_type, status, user_email 
		FROM jetsapi.workspace_changes WHERE workspace_name = $1 AND file_name = $2`
	err = txRead.QueryRow(ctx, stmt, fo.WorkspaceName, fo.FileName).Scan(
		&fo.Oid, &fo.ContentType, &fo.Status, &fo.UserEmail)
	if err != nil {
		return 0, fmt.Errorf("while reading from workspace_changes table: %v", err)
	}

	loRead := txRead.LargeObjects()
	obj, err := loRead.Open(ctx, fo.Oid, pgx.LargeObjectModeRead)
	if err != nil {
			return 0, err
	}

	writer := bufio.NewWriter(fd)
	n, err := io.Copy(writer, obj)
	if err != nil {
		return 0, err
	}
	err = txRead.Commit(ctx)
	return n, err
}