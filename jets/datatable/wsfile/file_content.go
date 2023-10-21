package wsfile

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains function to get and save file content & execute command in local workspace

// Run command in workspace
func RunCommand(buf *strings.Builder, command string, args *[]string, workspaceName string) error {
	var cmd *exec.Cmd
	if args != nil {
		cmd = exec.Command(command, (*args)...)
	} else {
		cmd = exec.Command(command)
	}
	if workspaceName != "" {
		cmd.Dir = fmt.Sprintf("%s/%s", os.Getenv("WORKSPACES_HOME"), workspaceName)
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("WORKSPACE=%s", workspaceName),
		)
	}
	cmd.Stdout = buf
	cmd.Stderr = buf
	buf.WriteString(fmt.Sprintf("Executing command %s in workspace %s\n", command, workspaceName))
	err := cmd.Run()
	if err != nil {
		msg := fmt.Sprintf("while executing command '%v': %v\n", command, err)
		log.Print(msg)
		buf.WriteString(msg)
	}
	buf.WriteString("Done executing command\n")
	return err
}

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

// content type (dir) that are saved using archives
var archiveContentType = map[string]bool {"reports": true}
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
	// Check if file is part of a dir that is archived
	if archiveContentType[contentType] {
		path = fmt.Sprintf("%s/%s/%s", os.Getenv("WORKSPACES_HOME"), workspaceName, "reports.tgz")
		// Archive dir contentType
		var buf strings.Builder
		command := "tar"
		args := []string{"cfvz", "reports.tgz", "reports/"} 
		buf.WriteString("\nArchiving the reports\n")
		err = RunCommand(&buf, command, &args, workspaceName)
		defer os.Remove(path)
		cmdLog := buf.String()
		if err != nil {
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Println(cmdLog)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			return fmt.Errorf("while archiving the reports folder : %v", err)
		}
		log.Println(cmdLog)
	}

	fileHd, err = os.Open(path)
	if err != nil {
		return fmt.Errorf("(2) failed to open local workspace file %s: %v", fileName, err)
	}
	defer fileHd.Close()

	n, err := fo.WriteObject(dbpool, fileHd)
	if err != nil {
		return fmt.Errorf("failed to save local workspace file %s in database: %v", fileName, err)
	}
	fmt.Println("uploaded", fo.FileName, "size", n, "bytes to database")
	return nil
}
