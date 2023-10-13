package wsfile

import (
	"fmt"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains function to get and save file content

// GetWorkspaceFileContent --------------------------------------------------------------------------
// Function to get the workspace file content based on relative file name
// Read the file from the workspace on file system since it's already in sync with database
func GetContent(workspaceName,  fileName string) (string, error) {

	// Read file from local workspace
	var content []byte
	var err error
	content, err = os.ReadFile(fmt.Sprintf("%s/%s/%s", os.Getenv("WORKSPACES_HOME"), workspaceName, fileName))
	if err != nil {
		return "", fmt.Errorf("failed to read local workspace file %s: %v", fileName, err)
	}
	return string(content), nil
}

// SaveWorkspaceFileContent --------------------------------------------------------------------------
// Function to save the workspace file content in local workspace file system and in database
func SaveContent(dbpool *pgxpool.Pool, workspaceName, fileName, fileContent string) error {

	// Write file to local workspace
	data := []byte(fileContent)
	path := fmt.Sprintf("%s/%s/%s", os.Getenv("WORKSPACES_HOME"), workspaceName, fileName)
	err := os.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write local workspace file %s: %v", fileName, err)
	}

	// Write file and metadata to database
	var fileHd *os.File
	fileHd, err = os.Open(path)
	if err != nil {
		return fmt.Errorf("(2) failed to open local workspace file %s: %v", fileName, err)
	}
	defer fileHd.Close()
	p := strings.Index(fileName, "/")
	var contentType string
	if p > 0 {
		contentType = fileName[0:p]
	}
	if contentType == "" {
		return fmt.Errorf("failed to find contentType in %s", fileName)
	}
	fo := dbutils.FileDbObject{
		WorkspaceName: workspaceName,
		FileName:      fileName,
		ContentType:   contentType,
		Status:        dbutils.FO_Open,
		UserEmail:     "system",
	}
	n, err := fo.WriteObject(dbpool, fileHd)
	if err != nil {
		return fmt.Errorf("failed to save local workspace file %s in database: %v", fileName, err)
	}
	fmt.Println("uploaded", fo.FileName, "size", n, "bytes to database")
	return nil
}
