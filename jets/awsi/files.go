package awsi

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Functions to sync or copy s3 files to database (and locally) as large objects

func SyncS3Files(dbpool *pgxpool.Pool, workspaceName, keyPrefix, trimPrefix, contentType string) error {
	bucket := os.Getenv("JETS_BUCKET")
	region := os.Getenv("JETS_REGION")
	wh := os.Getenv("WORKSPACES_HOME")
	// sync workspace files from s3 to locally
	log.Println("Synching overriten workspace file from s3 using keyPrefix",keyPrefix)
	keys, err := ListS3Objects(&keyPrefix, bucket, region)
	if err != nil {
		return err
	}
	for _, s3Obj := range keys {
		fileHd, err := os.Create(strings.Replace(s3Obj.Key, "jetstore/workspaces", wh, 1))
		if err != nil {
			return fmt.Errorf("failed to open local workspace file for write: %v", err)
		}

		// Download the object
		nsz, err := DownloadFromS3(bucket, region, s3Obj.Key, fileHd)
		if err != nil {
			fileHd.Close()
			return fmt.Errorf("failed to download input file: %v", err)
		}
		fmt.Println("downloaded", s3Obj.Key, "size", nsz, "bytes from s3")
		// Copy file to database
		fileHd.Seek(0, 0)
		fo := dbutils.FileDbObject{
			WorkspaceName: workspaceName,
			FileName:      strings.TrimPrefix(s3Obj.Key, trimPrefix),
			ContentType:   contentType,
			Status:        dbutils.FO_Open,
			UserEmail:     "system",
		}
		n, err := fo.WriteObject(dbpool, fileHd)
		fileHd.Close()
		if err != nil {
			return err
		}
		fmt.Println("uploaded", fo.FileName, "size", n, "bytes to database")
	}
	log.Println("Done synching overriten workspace file from s3")
	return nil
}
