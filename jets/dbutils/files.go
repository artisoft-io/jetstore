package dbutils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"

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

// FileDbObject.Status mapping
const (
	FO_Open    string = "open"
	// FO_Merged  string = "merged"
	// FO_Deleted string = "deleted"
)

// Query workspace_changes table for rows by workspace_name, status, and optionally content_type (if not empty)
func QueryFileObject(dbpool *pgxpool.Pool, workspaceName, status, contentType string) ([]*FileDbObject, error) {
	var stmt string
	var rows pgx.Rows
	var err error
	if len(contentType) > 0 {
		stmt = `SELECT key, oid, file_name, content_type, user_email	FROM jetsapi.workspace_changes 
		WHERE workspace_name = $1 AND status = $2 AND content_type = $3`
		rows, err = dbpool.Query(context.Background(), stmt, workspaceName, status, contentType)
		} else {
		stmt = `SELECT key, oid, file_name, content_type, user_email	FROM jetsapi.workspace_changes 
		WHERE workspace_name = $1 AND status = $2`
		rows, err = dbpool.Query(context.Background(), stmt, workspaceName, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fileObjects := make([]*FileDbObject,0)
	for rows.Next() {
		fo := FileDbObject{
			WorkspaceName: workspaceName,
			Status: status,
		}
		if err := rows.Scan(&fo.Key, &fo.Oid, &fo.FileName, &fo.ContentType, &fo.UserEmail); err != nil {
			return nil, err
		}
		fileObjects = append(fileObjects, &fo)
	}
	return fileObjects, nil
}

// // Workspace Changes - keeping track of assets changed
// "workspace_changes": {
// 	Stmt: `INSERT INTO jetsapi.workspace_changes 
// 		(workspace_name, oid, file_name, content_type, status, user_email) 
// 		VALUES ($1, $2, $3, $4, $5, $6)
// 		ON CONFLICT ON CONSTRAINT workspace_changes_unique_cstraint
// 		DO UPDATE SET (oid, status, user_email, last_update) = 
// 		(EXCLUDED.oid, EXCLUDED.status, EXCLUDED.user_email, DEFAULT)
// 		RETURNING key`,
// 	ColumnKeys: []string{"workspace_name", "oid", "file_name", "content_type", "status", "user_email"},
// },

func (fo *FileDbObject) UpdateFileObject(txWrite pgx.Tx, ctx context.Context) error {
	// Update FileDbObject metadata
	// Workspace Changes - keeping track of assets changed
	stmt := `INSERT INTO jetsapi.workspace_changes 
		(workspace_name, oid, file_name, content_type, status, user_email) 
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT ON CONSTRAINT workspace_changes_unique_cstraint
		DO UPDATE SET (oid, status, user_email, last_update) = 
		(EXCLUDED.oid, EXCLUDED.status, EXCLUDED.user_email, DEFAULT)
		RETURNING key`
	err := txWrite.QueryRow(ctx, stmt, 
		fo.WorkspaceName, 
		fo.Oid,
		fo.FileName,
		fo.ContentType,
		fo.Status,
		fo.UserEmail,
	).Scan(&fo.Key)
	if err != nil {
		return fmt.Errorf("while updating workspace_changes table: %v", err)
	}
	return nil
}

// Insert the content of fd into database with metadata specified by fo
// Expect to have fo.workspace_name and fo.file_name available
// Will populate fo.Oid and fo.Key
func (fo *FileDbObject) WriteObject(dbpool *pgxpool.Pool, fd *os.File) (int64, error) {
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

	err = fo.UpdateFileObject(txWrite, ctx)
	if err != nil {
		log.Println(err)
		return 0, err
	}

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

// Case fo.Oid == 0:
//   Expect to have fo.WorkspaceName and fo.FileName available
//   (alternatively could use fo.Key in future)
//   Returns fo with Oid, ContentType, Status, and UserEmail populated
//   and write the object in fd
// Case fo.Oid != 0:
//   Write the object in fd. Does not change fo.
func (fo *FileDbObject)ReadObject(dbpool *pgxpool.Pool, fd *os.File) (int64, error) {
	ctx := context.Background()
	txRead, err := dbpool.Begin(ctx)
	if err != nil {
			return 0, err
	}
	defer txRead.Rollback(ctx)

	if fo.Oid == 0 {
		// Read FileDbObject metadata
		stmt := `SELECT oid, content_type, status, user_email 
			FROM jetsapi.workspace_changes WHERE workspace_name = $1 AND file_name = $2`
		err = txRead.QueryRow(ctx, stmt, fo.WorkspaceName, fo.FileName).Scan(
			&fo.Oid, &fo.ContentType, &fo.Status, &fo.UserEmail)
		if err != nil {
			return 0, fmt.Errorf("while reading from workspace_changes table: %v", err)
		}
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