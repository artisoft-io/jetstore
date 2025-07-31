package dbutils

import (
	"context"
	"fmt"
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

// Query workspace_changes table for rows by workspace_name, status, and optionally content_type (if not empty)
func QueryFileObject(dbpool *pgxpool.Pool, workspaceName, contentType string) ([]*FileDbObject, error) {
	var stmt string
	var rows pgx.Rows
	var err error
	if len(contentType) > 0 {
		stmt = `SELECT key, file_name, content_type, user_email	FROM jetsapi.workspace_changes 
		WHERE workspace_name = $1 AND content_type = $2`
		rows, err = dbpool.Query(context.Background(), stmt, workspaceName, contentType)
	} else {
		stmt = `SELECT key, file_name, content_type, user_email	FROM jetsapi.workspace_changes 
		WHERE workspace_name = $1`
		rows, err = dbpool.Query(context.Background(), stmt, workspaceName)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fileObjects := make([]*FileDbObject, 0)
	for rows.Next() {
		fo := FileDbObject{
			WorkspaceName: workspaceName,
		}
		if err := rows.Scan(&fo.Key, &fo.FileName, &fo.ContentType, &fo.UserEmail); err != nil {
			return nil, err
		}
		fileObjects = append(fileObjects, &fo)
	}
	return fileObjects, nil
}

// Write Db Object, identified by fo.Oid to local file system
func (fo *FileDbObject) WriteDbObject2LocalFile(dbpool *pgxpool.Pool, localFileName string) error {
	fileHd, err := os.Create(localFileName)
	if err != nil {
		return fmt.Errorf("failed to os.Create on local workspace file %s for write: %v", fo.FileName, err)
	}
	defer fileHd.Close()
	n, err := fo.ReadObject(dbpool, fileHd)
	if err != nil {
		return fmt.Errorf("failed to read file object %s from database for write: %v", fo.FileName, err)
	}
	log.Println("Updated local file", fo.FileName, "size", n)
	return nil
}

// Insert the content of fd into database with metadata specified by fo
// Expect to have fo.workspace_name and fo.file_name available
// Will populate fo.Oid and fo.Key
func (fo *FileDbObject) WriteObject(dbpool *pgxpool.Pool, data []byte) (int64, error) {

	// Update FileDbObject metadata
	// Workspace Changes - keeping track of assets changed
	stmt := `INSERT INTO jetsapi.workspace_changes 
		(workspace_name, oid, data, file_name, content_type, status, user_email) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT ON CONSTRAINT workspace_changes_unique_cstraint
		DO UPDATE SET (oid, data, status, user_email, last_update) = 
		(EXCLUDED.oid, EXCLUDED.data, EXCLUDED.status, EXCLUDED.user_email, DEFAULT)
		RETURNING key`
	err := dbpool.QueryRow(context.TODO(), stmt,
		fo.WorkspaceName,
		0,
		data,
		fo.FileName,
		fo.ContentType,
		fo.Status,
		fo.UserEmail,
	).Scan(&fo.Key)
	if err != nil {
		err = fmt.Errorf("while updating workspace_changes table: %v", err)
		log.Println(err)
		return 0, err
	}
	return int64(len(data)), err
}

//	Read from workspace_changes table, update fo, write data to fd
func (fo *FileDbObject) ReadObject(dbpool *pgxpool.Pool, fd *os.File) (int64, error) {
	var data []byte
	stmt := `SELECT data, content_type, user_email 
			FROM jetsapi.workspace_changes WHERE workspace_name = $1 AND file_name = $2`
	err := dbpool.QueryRow(context.TODO(), stmt, fo.WorkspaceName, fo.FileName).Scan(
		&data, &fo.ContentType, &fo.UserEmail)
	if err != nil {
		return 0, fmt.Errorf("while reading from workspace_changes table: %v", err)
	}
	n, err := fd.Write(data)
	return int64(n), err
}
