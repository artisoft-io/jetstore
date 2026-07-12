package awsi

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Functions to sync or copy s3 files to database (and locally) as large objects

// confineToBase ensures that candidate resolves to a path located within baseDir.
// It returns the cleaned absolute path or an error if the candidate escapes baseDir
// (e.g. via ".." segments in an untrusted S3 object key). This blocks path traversal
// (CWE-22).
func confineToBase(baseDir, candidate string) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("while resolving base directory %q: %w", baseDir, err)
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("while resolving path %q: %w", candidate, err)
	}
	if absCandidate != absBase && !strings.HasPrefix(absCandidate, absBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid path %q: escapes base directory %q", candidate, baseDir)
	}
	return absCandidate, nil
}

func SyncS3Files(dbpool *pgxpool.Pool, workspaceName, keyPrefix, trimPrefix, contentType string) error {
	wh := os.Getenv("WORKSPACES_HOME")
	// sync workspace files from s3 to locally
	log.Println("Synching overriten workspace file from s3 using keyPrefix", keyPrefix)
	keys, err := ListS3Objects(jetstoreOwnBucket, &keyPrefix)
	if err != nil {
		return err
	}
	for _, s3Obj := range keys {
		// s3Obj.Key is externally controlled; confine the derived local path within
		// WORKSPACES_HOME to prevent path traversal (CWE-22).
		localFileName, err := confineToBase(wh, strings.Replace(s3Obj.Key, "jetstore/workspaces", wh, 1))
		if err != nil {
			return err
		}
		fileDir := filepath.Dir(localFileName)
		if err = os.MkdirAll(fileDir, 0750); err != nil {
			return fmt.Errorf("while creating file directory structure: %v", err)
		}

		// #nosec G304 -- localFileName is confined to WORKSPACES_HOME by confineToBase above.
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
		if nsz > 1024*1024*1024 {
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
