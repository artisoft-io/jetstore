package wsfile

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
)

// This file contains functions to stash workspace in the jetstore ui container

// StashFiles --------------------------------------------------------------------------
// Function to copy all workspace files to a stash location
// The stash is used when deleting workspace changes to restore the file to original content
func StashFiles(workspaceName string) error {
	workspacePath := fmt.Sprintf("%s/%s", os.Getenv("WORKSPACES_HOME"), workspaceName)
	stashPath := StashDir()
	log.Printf("Stashing workspace files from %s to %s", workspacePath, stashPath)

	// make sure the stash directory exists
	var err error
	if err2 := os.Mkdir(stashPath, 0755); os.IsExist(err2) {
		log.Println("Workspace stash", stashPath, "exists")		
	} else {
		log.Println("Workspace stash directory ", stashPath, "created")
	}

	// copy all files if targetDir does not exists
	if _, err2 := os.Stat(fmt.Sprintf("%s/%s", stashPath, workspaceName)); err2 != nil {
		log.Println("Stashing workspace files")
		targetDir := fmt.Sprintf("--target-directory=%s", stashPath)
		cmd := exec.Command("cp", "--recursive", "--no-dereference", targetDir, workspacePath)
		var b bytes.Buffer
		cmd.Stdout = &b
		cmd.Stderr = &b
		err = cmd.Run()
		if err != nil {
			log.Printf("while executing cp to stash of the workspace files: %v", err)
		} else {
			log.Println("cp workspace files to stash output:")
		}
		b.WriteTo(os.Stdout)
		log.Println("============================")

		// Removing files that we don't want to be restaured
		targetDir = fmt.Sprintf("%s/%s", stashPath, workspaceName)
		exec.Command("rm", "--recursive", "--force", fmt.Sprintf("%s/.git", targetDir)).Run()
		exec.Command("rm", "--recursive", "--force", fmt.Sprintf("%s/.github", targetDir)).Run()
		exec.Command("rm", "--recursive", "--force", fmt.Sprintf("%s/.gitignore", targetDir)).Run()
		exec.Command("rm", "--recursive", "--force", fmt.Sprintf("%s/lookup.db", targetDir)).Run()
		exec.Command("rm", "--recursive", "--force", fmt.Sprintf("%s/workspace.db", targetDir)).Run()
	} else {
		log.Println("Workspace files already stashed, not overriting them")
	}

	return err
}

// Function to remove the stash
func ClearStash(workspaceName string) error {
	log.Printf("Clearing workspace '%s' stash", workspaceName)
	return exec.Command("rm", "--recursive", "--force", fmt.Sprintf("%s/%s", StashDir(), workspaceName)).Run()
}

func StashDir() string {
	return fmt.Sprintf("%s/ws:stash", os.Getenv("WORKSPACES_HOME"))
}

// Function to restore file from stash, it copy src file to dst
func CopyFiles(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

// Restaure (copy dir recursively) srcDir to dstDir
func RestaureFiles(srcDir, dstDir string) error {
	targetDir := fmt.Sprintf("--target-directory=%s", dstDir)
	cmd := exec.Command("cp", "--recursive", "--no-dereference", targetDir, srcDir)
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		log.Printf("while executing restaure from stash all the workspace files: %v", err)
	} else {
		log.Println("restaure all workspace files from stash output:")
	}
	b.WriteTo(os.Stdout)
	log.Println("============================")
	return err
}
