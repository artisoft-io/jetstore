package actions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
)

// Common functions and types for s3, rerwite of loader's version

type DownloadS3Result struct {
	InputFilesCount int
	Err             error
}

// Get the file_key(s) assigned to nodeId -- these are the input multipart files
func GetFileKeys(ctx context.Context, dbpool *pgxpool.Pool, sessionId string, nodeId int) ([]string, error) {
	var key, stmt string
	// Get isFile query in case the list of file_key is empty
	fileKeys := make([]string, 0)
	var rows pgx.Rows
	var err error
	stmt = "SELECT file_key	FROM jetsapi.compute_pipes_shard_registry WHERE session_id = $1 AND shard_id = $2"
	rows, err = dbpool.Query(ctx, stmt, sessionId, nodeId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Nothing to do here, no files to process for this session_id
			log.Printf("No file keys in table compute_pipes_shard_registry for session_id %s, nothing to do", sessionId)
			return []string{}, nil
		} else {
			return nil, err
		}
	}
	defer rows.Close()
	for rows.Next() {
		// scan the row
		if err = rows.Scan(&key); err != nil {
			return nil, err
		}
		fileKeys = append(fileKeys, key)
	}
	return fileKeys, nil
}


// Input arg:
// done: unbuffered channel to indicate to stop downloading file (must be an error downstream)
// Returned values:
// headersFileCh: channel having the first file name to get headers from
// fileNamesCh: channel having all file names (incl header file), one at a time
// downloadS3ResultCh: channel indicating the downoader results (nbr of files downloaded or error)
// inFolderPath: temp folder containing the downloaded files
// error when setting up the downloader
// Special case:
//
//	Case of multipart files using distribute_data operator where this shardId contains no files in
//	table compute_pipes_shard_registry:
//		- Use of global cpipesShardWithNoFileKeys == true to indicate this situation
//		- Case cpipes "reducing": inFile contains the file_key folder of another shardId to get the headers file from it.
//			This will get a file to get the headers from but the file list to process (in fileNamesCh) will be empty.
//		- Case cpipes "sharding": cpipesFileKeys will contain one file to use to obtain the headers from,
//			but the file list to process (in fileNamesCh) will be empty
//	This is needed to be able to setup the header sctucture and domain key info to be able to process records
//	obtained by peer nodes even when this node had no file assigned to it originally.
func (cpCtx *ComputePipesContext) DownloadS3Files(inFolderPath string, fileKeys []string) error {

	go func() {
		defer close(cpCtx.FileNamesCh)
		var inFilePath string
		var err error
			// switch cpipesMode {
			// case "loader", "reducing":
			// 	// Case loader mode (loaderSM) or cpipes reducing mode, get the file keys from s3
			// 	if cpipesShardWithNoFileKeys {
			// 		log.Printf("Getting first file key from s3 folder for headers only: %s", *inFile)
			// 	} else {
			// 		log.Printf("Getting file keys from s3 folder: %s", *inFile)
			// 	}
			// 	s3Objects, err := awsi.ListS3Objects(inFile)
			// 	if err != nil || s3Objects == nil {
			// 		downloadS3ResultCh <- DownloadS3Result{
			// 			err: fmt.Errorf("failed to download list of files from s3: %v", err),
			// 		}
			// 		return
			// 	}
			// 	if len(s3Objects) == 0 {
			// 		downloadS3ResultCh <- DownloadS3Result{}
			// 		return
			// 	}
			// 	fileKeys = make([]string, 0)
			// 	for i := range s3Objects {
			// 		if s3Objects[i].Size > 0 {
			// 			fileKeys = append(fileKeys, s3Objects[i].Key)
			// 		}
			// 	}

			// case "sharding":
			// 	// Process the fileKeys in the global variable cpipesFileKeys
			// 	fileKeys = cpipesFileKeys

			// default:
			// 	downloadS3ResultCh <- DownloadS3Result{
			// 		err: fmt.Errorf("error: invalid cpipesMode in downloadS3Files: %s", cpipesMode),
			// 	}
			// 	return
			// }
			log.Printf("Downloading multi-part file from s3 folder: %s", cpCtx.FileKey)
			for i := range fileKeys {
				inFilePath, err = DownloadS3Object(fileKeys[i], inFolderPath, 1)
				if err != nil {
					cpCtx.DownloadS3ResultCh <- DownloadS3Result{
						InputFilesCount: i,
						Err: fmt.Errorf("failed to download s3 file %s: %v", fileKeys[i], err),
					}
					return
				}
				if len(inFilePath) > 0 {
					select {
					case cpCtx.FileNamesCh <- FileName{LocalFileName: inFilePath,InFileKey: fileKeys[i]}:
					case <-cpCtx.Done:
						cpCtx.DownloadS3ResultCh <- DownloadS3Result{InputFilesCount: i + 1}
						return
					}
				}
			}
			cpCtx.DownloadS3ResultCh<- DownloadS3Result{InputFilesCount: len(fileKeys)}
	}()

	return nil
}

var bucket, region, kmsKeyArn string
var downloader *manager.Downloader
func init() {
	bucket = os.Getenv("JETS_BUCKET")
	region = os.Getenv("JETS_REGION")
	kmsKeyArn = os.Getenv("JETS_S3_KMS_KEY_ARN")
	var err error
	downloader, err = awsi.NewDownloader(region)
	if err != nil {
		log.Fatalf("while init s3 downloader for region %s: %v", region, err)
	}
}

func DownloadS3Object(s3Key, localDir string, minSize int64) (string, error) {
	// Download object(s) using a download manager to a temp file (fileHd)
	var inFilePath string
	var fileHd *os.File
	var err error
	fileHd, err = os.CreateTemp(localDir, "jetstore")
	if err != nil {
		return "", fmt.Errorf("failed to open temp input file: %v", err)
	}
	defer fileHd.Close()
	inFilePath = fileHd.Name()
	// log.Printf("S3Key: %s, Temp file name: %s", s3Key, inFilePath)

	// Download the object
	nsz, err := awsi.DownloadFromS3v2(downloader, bucket, s3Key, fileHd)
	if err != nil {
		return "", fmt.Errorf("failed to download input file: %v", err)
	}
	if minSize > 0 && nsz < minSize {
		log.Printf("Ignoring sentinel file %s", s3Key)
		fileHd.Close()
		os.Remove(inFilePath)
		return "", nil
	}
	log.Println("downloaded", nsz, "bytes for key", s3Key)
	return inFilePath, nil
}
