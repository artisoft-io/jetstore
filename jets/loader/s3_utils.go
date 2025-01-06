package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

// Utilities to dowmload from s3

type DownloadS3Result struct {
	InputFilesCount int
	err             error
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
func downloadS3Files(done <-chan struct{}) (<-chan string, <-chan string, <-chan DownloadS3Result, string, error) {
	var inFolderPath string
	var err error
	headersFileCh := make(chan string, 1)
	fileNamesCh := make(chan string, 2)
	downloadS3ResultCh := make(chan DownloadS3Result, 1)

	// Create a local temp directory to hold the file(s)
	inFolderPath, err = os.MkdirTemp("", "jetstore")
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("failed to create local temp directory: %v", err)
	}

	go func() {
		defer close(headersFileCh)
		defer close(fileNamesCh)
		var inFilePath string
		var err error
		if isPartFiles == 1 {
			var fileKeys []string
			log.Printf("Getting file keys from s3 folder: %s", *inFile)
			s3Objects, err := awsi.ListS3Objects("", inFile)
			if err != nil || s3Objects == nil {
				downloadS3ResultCh <- DownloadS3Result{
					err: fmt.Errorf("failed to download list of files from s3: %v", err),
				}
				return
			}
			if len(s3Objects) == 0 {
				downloadS3ResultCh <- DownloadS3Result{}
				return
			}
			fileKeys = make([]string, 0)
			for i := range s3Objects {
				if s3Objects[i].Size > 0 {
					fileKeys = append(fileKeys, s3Objects[i].Key)
				}
			}
			log.Printf("Downloading multi-part file from s3 folder: %s", *inFile)
			gotHeaders := false
			for i := range fileKeys {
				retry := 0
			do_retry:
				inFilePath, err = downloadS3Object(fileKeys[i], inFolderPath, 1)
				if retry < 6 {
					time.Sleep(500 * time.Millisecond)
					retry++
					goto do_retry
				}
				if err != nil {
					downloadS3ResultCh <- DownloadS3Result{
						InputFilesCount: i,
						err:             fmt.Errorf("failed to download s3 file %s: %v", fileKeys[i], err),
					}
					return
				}
				if len(inFilePath) > 0 {
					if !gotHeaders {
						headersFileCh <- inFilePath
						gotHeaders = true
					}
					select {
					case fileNamesCh <- inFilePath:
					case <-done:
						downloadS3ResultCh <- DownloadS3Result{InputFilesCount: i + 1}
						return
					}
				}
			}
			downloadS3ResultCh <- DownloadS3Result{InputFilesCount: len(fileKeys)}
		} else {
			// Download single file using a download manager to a temp file (fileHd)
			inFilePath, err = downloadS3Object(*inFile, inFolderPath, 1)
			switch {
			case err != nil:
				downloadS3ResultCh <- DownloadS3Result{err: fmt.Errorf("while downloading single file from s3: %v", err)}
				return
			case inFilePath == "":
				downloadS3ResultCh <- DownloadS3Result{err: fmt.Errorf("error: got sentinel file from s3: %s", *inFile)}
				return
			}
			if inFilePath == "" {
				downloadS3ResultCh <- DownloadS3Result{err: fmt.Errorf("while downloading single file from s3: %v", err)}
				return
			}
			headersFileCh <- inFilePath
			select {
			case fileNamesCh <- inFilePath:
			case <-done:
				downloadS3ResultCh <- DownloadS3Result{}
				return
			}
			downloadS3ResultCh <- DownloadS3Result{InputFilesCount: 1}
		}
	}()

	return headersFileCh, fileNamesCh, downloadS3ResultCh, inFolderPath, nil
}

func downloadS3Object(s3Key, localDir string, minSize int64) (string, error) {
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
	log.Printf("S3Key: %s, Temp file name: %s", s3Key, inFilePath)

	// Download the object
	nsz, err := awsi.DownloadFromS3(*awsBucket, *awsRegion, s3Key, fileHd)
	if err != nil {
		return "", fmt.Errorf("failed to download input file: %v", err)
	}
	log.Println("downloaded", nsz, "bytes for key", s3Key)
	if minSize > 0 && nsz < minSize {
		log.Printf("Ignoring sentinel file %s", s3Key)
		fileHd.Close()
		os.Remove(inFilePath)
		return "", nil
	}
	return inFilePath, nil
}
