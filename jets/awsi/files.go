package awsi

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Functions to sync or copy s3 files to database (and locally) as large objects

func SyncS3Files(dbpool *pgxpool.Pool, workspaceName, keyPrefix, trimPrefix, contentType string) error {
	wh := os.Getenv("WORKSPACES_HOME")
	// sync workspace files from s3 to locally
	log.Println("Synching overriten workspace file from s3 using keyPrefix", keyPrefix)
	keys, err := ListS3Objects(jetstoreOwnBucket, &keyPrefix)
	if err != nil {
		return err
	}
	for _, s3Obj := range keys {
		localFileName := strings.Replace(s3Obj.Key, "jetstore/workspaces", wh, 1)
		fileDir := filepath.Dir(localFileName)
		if err = os.MkdirAll(fileDir, 0770); err != nil {
			return fmt.Errorf("while creating file directory structure: %v", err)
		}

		fileHd, err := os.Create(localFileName)
		if err != nil {
			return fmt.Errorf("failed to open local workspace file for write: %v", err)
		}
		// Download the object
		nsz, err := DownloadFromS3(jetstoreOwnBucket, jetstoreOwnRegion, s3Obj.Key, fileHd)
		if err != nil {
			fileHd.Close()
			return fmt.Errorf("failed to download input file: %v", err)
		}
		fileHd.Close()
		fmt.Println("downloaded", s3Obj.Key, "size", nsz, "bytes from s3")
		// Copy file to database
		if nsz > 1024 * 1024 * 1024 {
			return fmt.Errorf("error: Jetstore does not support workspace file larger than 1 Go, the object size is: %v", nsz)
		}
		data, err := os.ReadFile(fileHd.Name())
		if err != nil {
			return err
		}
		fo := dbutils.FileDbObject{
			WorkspaceName: workspaceName,
			FileName:      strings.TrimPrefix(s3Obj.Key, trimPrefix),
			ContentType:   contentType,
			UserEmail:     "system",
		}
		n, err := fo.WriteObject(dbpool, data)
		if err != nil {
			return err
		}
		fmt.Println("uploaded", fo.FileName, "size", n, "bytes to database")
	}
	log.Println("Done synching overriten workspace file from s3")
	return nil
}
