package compute_pipes

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Common functions and types for s3, rewite of loader's version

var bucketName, regionName, kmsKeyArn string
var downloader *manager.Downloader

func init() {
	bucketName = os.Getenv("JETS_BUCKET")
	regionName = os.Getenv("JETS_REGION")
	kmsKeyArn = os.Getenv("JETS_S3_KMS_KEY_ARN")
	var err error
	downloader, err = awsi.NewDownloader(regionName)
	if err != nil {
		log.Fatalf("while init s3 downloader for region %s: %v", regionName, err)
	}
}

type DownloadS3Result struct {
	InputFilesCount int
	TotalFilesSize  int64
	Err             error
}

// Get the file_key(s) assigned to nodeId -- these are the input multipart files
func GetFileKeys(ctx context.Context, dbpool *pgxpool.Pool, sessionId string, nodeId int) ([]*FileKeyInfo, error) {
	var stmt string
	// Get isFile query in case the list of file_key is empty
	fileKeys := make([]*FileKeyInfo, 0)
	var rows pgx.Rows
	var err error
	stmt = "SELECT file_key, file_size, shard_start, shard_end FROM jetsapi.compute_pipes_shard_registry WHERE session_id = $1 AND shard_id = $2"
	rows, err = dbpool.Query(ctx, stmt, sessionId, nodeId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Nothing to do here, no files to process for this session_id
			log.Printf("No file keys in table compute_pipes_shard_registry for session_id %s, nothing to do", sessionId)
			return fileKeys, nil
		} else {
			return nil, err
		}
	}
	defer rows.Close()
	for rows.Next() {
		// scan the row
		var fileKeyInfo FileKeyInfo
		if err = rows.Scan(
			&fileKeyInfo.key, 
			&fileKeyInfo.size,
			&fileKeyInfo.start,
			&fileKeyInfo.end,
			); err != nil {
			return nil, err
		}
		fileKeys = append(fileKeys, &fileKeyInfo)
	}
	return fileKeys, nil
}

func (cpCtx *ComputePipesContext) DownloadS3Files(inFolderPath, externalBucket string, fileKeys []*FileKeyInfo) error {
	go func() {
		defer close(cpCtx.FileNamesCh)
		defer close(cpCtx.DownloadS3ResultCh)
		var inFilePath string
		var fileSize, totalFilesSize int64
		var err error
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Printf("%s node %d %s Start downloading %d files from s3",
				cpCtx.SessionId, cpCtx.NodeId, cpCtx.MainInputStepId, len(fileKeys))
		}
		for i := range fileKeys {
			if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
				log.Printf("%s node %d %s Downloading file from s3: %s",
					cpCtx.SessionId, cpCtx.NodeId, cpCtx.MainInputStepId, fileKeys[i].key)
			}
			retry := 0
		do_retry:
			inFilePath, fileSize, err = DownloadS3Object(externalBucket, fileKeys[i], inFolderPath, 1)
			if err != nil {
				if retry < 6 {
					time.Sleep(500 * time.Millisecond)
					retry++
					goto do_retry
				}
				cpCtx.DownloadS3ResultCh <- DownloadS3Result{
					InputFilesCount: i,
					TotalFilesSize:  totalFilesSize,
					Err:             fmt.Errorf("failed to download s3 file %s: %v", fileKeys[i].key, err),
				}
				return
			}
			if fileSize > 0 { // skip sentinel files
				select {
				case cpCtx.FileNamesCh <- FileName{LocalFileName: inFilePath, InFileKeyInfo: *fileKeys[i]}:
				case <-cpCtx.KillSwitch:
					cpCtx.DownloadS3ResultCh <- DownloadS3Result{InputFilesCount: i + 1, TotalFilesSize: totalFilesSize}
					return
				case <-cpCtx.Done:
					cpCtx.DownloadS3ResultCh <- DownloadS3Result{InputFilesCount: i + 1, TotalFilesSize: totalFilesSize}
					return
				}
			}
			totalFilesSize += fileSize
		}
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Printf("%s node %d %s DONE downloading %d files from s3",
				cpCtx.SessionId, cpCtx.NodeId, cpCtx.MainInputStepId, len(fileKeys))
		}
		cpCtx.DownloadS3ResultCh <- DownloadS3Result{InputFilesCount: len(fileKeys), TotalFilesSize: totalFilesSize}
	}()

	return nil
}

func DownloadS3Object(externalBucket string, s3Key *FileKeyInfo, localDir string, minSize int64) (string, int64, error) {
	// Download object(s) using a download manager to a temp file (fileHd)
	var inFilePath string
	var fileHd *os.File
	var err error
	fileHd, err = os.CreateTemp(localDir, "jetstore")
	if err != nil {
		return "", 0, fmt.Errorf("failed to open temp input file: %v", err)
	}
	if externalBucket == "" {
		externalBucket = bucketName
	}

	defer fileHd.Close()
	inFilePath = fileHd.Name()

	var byteRange *string
	if s3Key.end > 0 {
		s := fmt.Sprintf("bytes=%d-%d", s3Key.start, s3Key.end)
		byteRange = &s
	}

	// Download the object
	nsz, err := awsi.DownloadFromS3v2(downloader, externalBucket, s3Key.key, byteRange, fileHd)
	if err != nil {
		return "", 0, fmt.Errorf("failed to download input file from bucket %s: %v", externalBucket, err)
	}
	if minSize > 0 && nsz < minSize {
		log.Printf("Ignoring sentinel file %s", s3Key.key)
		fileHd.Close()
		os.Remove(inFilePath)
		return "", 0, nil
	}
	// log.Println("downloaded", nsz, "bytes for key", s3Key)
	return inFilePath, nsz, nil
}
