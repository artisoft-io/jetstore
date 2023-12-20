package main

import (
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

// Utilities to dowmload & upload to s3

func downloadS3Files() (string, error) {
	var inFilePath string
	var err error
	if isPartFiles == 1 {
		log.Printf("Downloading multi-part file from s3 folder: %s", *inFile)
		s3Objects, err := awsi.ListS3Objects(inFile, *awsBucket, *awsRegion)
		if err != nil || s3Objects == nil || len(s3Objects) == 0 {
			return "", fmt.Errorf("failed to download list of files from s3 (or folder is empty): %v", err)
		}

		// Create a local temp directory to hold the files
		inFilePath, err = os.MkdirTemp("", "jetstore")
		if err != nil {
			return "", fmt.Errorf("failed to create local temp directory: %v", err)
		}
		for i := range s3Objects {
			_, err = downloadS3Object(s3Objects[i].Key, inFilePath, 1000) 
			if err != nil {
				return "", fmt.Errorf("failed to download s3 file %s: %v", s3Objects[i].Key, err)
			}
		}
	} else {
		// Download single file using a download manager to a temp file (fileHd)
		inFilePath, err = downloadS3Object(*inFile, "", 0) 
		if err != nil {
			return "", fmt.Errorf("failed to download input file: %v", err)
		}
	}
	return inFilePath, nil
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
	log.Println("downloaded", nsz, "bytes from s3")	
	if minSize > 0 && nsz < minSize {
		log.Printf("Ignoring sentinel file %s", s3Key)
		fileHd.Close()
		os.Remove(inFilePath)
	}
	return inFilePath, nil
}
