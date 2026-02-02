package compute_pipes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/utils"
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

// Get the file_key(s) from compute_pipes_shard_registry assigned to nodeId -- these are the input multipart files.
// This is used during the sharding mode.
// Returned [0][]*FileKeyInfo are the main input files, [i][]*FileKeyInfo, i=1:n are the merge channel files.
func GetFileKeys(ctx context.Context, dbpool *pgxpool.Pool, sessionId string, nodeId int) ([][]*FileKeyInfo, error) {
	var stmt string
	// Get isFile query in case the list of file_key is empty
	var maxChanPos int
	fileKeys := make([][]*FileKeyInfo, 20)
	var rows pgx.Rows
	var channelPos sql.NullInt64
	var err error
	stmt = "SELECT file_key, file_size, shard_start, shard_end, channel_pos FROM jetsapi.compute_pipes_shard_registry WHERE session_id = $1 AND shard_id = $2"
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
			&channelPos,
		); err != nil {
			return nil, err
		}
		chanPos := int(channelPos.Int64)
		if chanPos > maxChanPos {
			maxChanPos = chanPos
		}
		fileKeys[chanPos] = append(fileKeys[chanPos], &fileKeyInfo)
	}
	// Resize the slice to the actual number of channels found
	fileKeys = fileKeys[:maxChanPos+1]
	return fileKeys, nil
}

func GetS3Objects4LookbackPeriod(bucket, fileKey, lookbackPeriod string, env map[string]any) ([]*awsi.S3Object, error) {
	var s3Objects []*awsi.S3Object
	var err error
	// Lookback case, need to get the list of file keys for each lookback periods
	firstPeriodId, numPeriods, err := utils.ParseLookbackPeriod(lookbackPeriod, env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lookback_period %s: %v", lookbackPeriod, err)
	}
	log.Printf("Downloading file keys from s3 stage folder: %s, lookback first period_id: %d, num periods: %d",
		fileKey, firstPeriodId, numPeriods)
	// Make a copy of envSessings to avoid modifying the original
	lookbackEnv := make(map[string]any)
	maps.Copy(lookbackEnv, env)
	// Last period in the lookback is firstPeriodId - numPeriods
	for p := range numPeriods + 1 {
		lookbackEnv["$PERIOD_ID"] = firstPeriodId - p
		fileKeyPrefix := utils.ReplaceEnvVars(fileKey, lookbackEnv)
		log.Printf("  Lookback period_id: %d, downloading file keys from s3 stage folder: %s",
			lookbackEnv["$PERIOD_ID"], fileKeyPrefix)
		periodS3Objects, err := awsi.ListS3Objects(bucket, &fileKeyPrefix)
		if err != nil {
			return nil, fmt.Errorf("failed to download list of files from s3 for lookback period_id %d: %v",
				lookbackEnv["$PERIOD_ID"], err)
		}
		s3Objects = append(s3Objects, periodS3Objects...)
	}
	return s3Objects, nil
}

// Get the file_key(s) from s3 for the given process/session/step/partition.
// This is used during the reducing mode.
func GetS3FileKeys(processName, sessionId, mainInputStepId, jetsPartitionLabel string,
	inputChannelConfig *InputChannelConfig, envSettings map[string]any) ([][]*FileKeyInfo, error) {

	var allS3Objects [][]*awsi.S3Object
	l := 1 + len(inputChannelConfig.MergeChannels)
	allS3Objects = make([][]*awsi.S3Object, l)
	var s3BaseFolder string
	var s3Objects []*awsi.S3Object
	var err error

	// Main input source
	if len(inputChannelConfig.FileKey) == 0 {
		s3BaseFolder = fmt.Sprintf("%s/process_name=%s/session_id=%s/step_id=%s/jets_partition=%s",
			jetsS3StagePrefix, processName, sessionId, mainInputStepId, jetsPartitionLabel)
		s3Objects, err = awsi.ListS3Objects("", &s3BaseFolder)
		if err != nil || s3Objects == nil {
			return nil, fmt.Errorf("failed to download list of files from s3: %v", err)
		}

	} else {
		s3Objects, err = GetS3Objects4LookbackPeriod(inputChannelConfig.Bucket, inputChannelConfig.FileKey,
			inputChannelConfig.LookbackPeriods, envSettings)
		if err != nil {
			return nil, fmt.Errorf("failed to download list of files from s3 for main input: %v", err)
		}
	}
	allS3Objects[0] = s3Objects

	// Merge channels sources
	for i := range inputChannelConfig.MergeChannels {
		var mergeS3BaseFolder string
		mergeChannelConfig := &inputChannelConfig.MergeChannels[i]
		sid := sessionId
		if len(mergeChannelConfig.ReadSessionId) > 0 {
			sid = utils.ReplaceEnvVars(mergeChannelConfig.ReadSessionId, envSettings)
		}
		if len(mergeChannelConfig.FileKey) == 0 {
			mergeS3BaseFolder = fmt.Sprintf("%s/process_name=%s/session_id=%s/step_id=%s/jets_partition=%s",
				jetsS3StagePrefix, processName, sid, mergeChannelConfig.ReadStepId, jetsPartitionLabel)
			s3Objects, err = awsi.ListS3Objects("", &mergeS3BaseFolder)
			if err != nil || s3Objects == nil {
				return nil, fmt.Errorf("failed to download list of files from s3 for merge channel %d: %v", i, err)
			}
		} else {
			s3Objects, err = GetS3Objects4LookbackPeriod(mergeChannelConfig.Bucket, mergeChannelConfig.FileKey,
				mergeChannelConfig.LookbackPeriods, envSettings)
			if err != nil {
				return nil, fmt.Errorf("failed to download list of files from s3 for merge channel %d: %v", i, err)
			}
		}
		allS3Objects[i+1] = s3Objects
	}

	// Convert to FileKeyInfo
	fileKeys := make([][]*FileKeyInfo, l)
	for i := range allS3Objects {
		fileKeys[i] = make([]*FileKeyInfo, 0, len(allS3Objects[i]))
		for j := range allS3Objects[i] {
			if allS3Objects[i][j].Size > 0 {
				fileKeys[i] = append(fileKeys[i],
					&FileKeyInfo{key: allS3Objects[i][j].Key, size: int(allS3Objects[i][j].Size)})
			}
		}
	}
	return fileKeys, nil
}

func (cpCtx *ComputePipesContext) DownloadS3Files(inFolderPath []string, externalBucket string, fileKeys [][]*FileKeyInfo) error {
	// Check if we need to download the files or not, do prior to goroutine to avoid modifying cpCtx in multiple goroutines
	doDownloadFiles := cpCtx.startDownloadFiles()
	if !doDownloadFiles {
		for i := range cpCtx.FileNamesCh {
			close(cpCtx.FileNamesCh[i])
		}
		close(cpCtx.DownloadS3ResultCh)
		return nil
	}
	// perform a check
	l := len(inFolderPath)
	if l != len(fileKeys) || l != len(cpCtx.FileNamesCh) {
		return fmt.Errorf("internal error: mismatch in number of input folders (%d), file keys (%d) and channels (%d)",
			l, len(fileKeys), len(cpCtx.FileNamesCh))
	}

	// Start a goroutine to download the s3 files
	go func() {
		defer close(cpCtx.DownloadS3ResultCh)

		var waitForDone sync.WaitGroup
		for i := range l {
			waitForDone.Go(func() {
				cpCtx.downloadS3Files(inFolderPath[i], externalBucket, fileKeys[i], cpCtx.FileNamesCh[i])
			})
		}
		waitForDone.Wait()
	}()

	return nil
}

func (cpCtx *ComputePipesContext) downloadS3Files(inFolderPath, externalBucket string, fileKeys []*FileKeyInfo,
	fileNamesCh chan FileName) {
	defer close(fileNamesCh)
	var inFilePath string
	var fileSize, totalFilesSize int64
	var err error
	var fullDownload bool
	inputFormat := cpCtx.CpConfig.PipesConfig[0].InputChannel.Format
	if strings.HasPrefix(inputFormat, "parquet") {
		fullDownload = true
	}
	if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
		log.Printf("%s node %d %s Start downloading %d files from s3",
			cpCtx.SessionId, cpCtx.NodeId, cpCtx.MainInputStepId, len(fileKeys))
	}
	for i := range fileKeys {
		fileKeys[i].fullDownload = fullDownload
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Printf("%s node %d %s Downloading (full download? %v) file from s3: %s",
				cpCtx.SessionId, cpCtx.NodeId, cpCtx.MainInputStepId, fullDownload, fileKeys[i].key)
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
			case fileNamesCh <- FileName{LocalFileName: inFilePath, InFileKeyInfo: *fileKeys[i]}:
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
	if !s3Key.fullDownload && s3Key.end > 0 {
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
