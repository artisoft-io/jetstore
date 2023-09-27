package git

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// This package execute git command in the workspace directory

// Environment needed:
// WORKSPACE Workspace currently in use
// WORKSPACES_HOME Home dir of workspaces

type WorkspaceGit struct {
	WorkspaceName   string
	WorkspaceUri    string
	WorkspacesHome  string
	ActiveWorkspace string
}

func NewWorkspaceGit(workspaceName, workspaceUri string) *WorkspaceGit {
	return &WorkspaceGit{
		WorkspaceName:  workspaceName,
		WorkspaceUri:   workspaceUri,
		WorkspacesHome: os.Getenv("WORKSPACES_HOME"),
		ActiveWorkspace: os.Getenv("WORKSPACE"),
	}
}

func runShellCommand(dir, command string) (string, error) {
	cmd := exec.Command("/bin/bash", "-c", command)
	cmd.Dir = dir
	var b1 bytes.Buffer
	cmd.Stdout = &b1
	cmd.Stderr = &b1
	err := cmd.Run()
	if err != nil {
		err = fmt.Errorf("error while executing '%s' command: %s", command, b1.String())
		log.Printf("%v", err)
		return "", err
	}
	return b1.String(), nil
}

func (wg *WorkspaceGit) GetStatus() (string, error) {
	if wg.WorkspaceName == "" {
		return "", fmt.Errorf("error, must provide workspace_name")
	}
	workspacePath := fmt.Sprintf("%s/%s", wg.WorkspacesHome, wg.WorkspaceName)
	
	//*
	fmt.Println("*** workspacePath:",workspacePath)

	// First, check if workspace directory exists or not
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return "removed", nil
 	}

	// Check if the local branch exists
	_, err := runShellCommand(workspacePath, fmt.Sprintf("git show-ref --verify --quiet refs/heads/%s", wg.WorkspaceName))
	if err != nil {
		// Local branch does not exist, must be a newly deployed container
		log.Printf("Branch '%s' does not exist in local repo %s", wg.WorkspaceName, workspacePath)
		return "removed", nil
	}

	// Check if user info exist in local repo
	result, err := runShellCommand(workspacePath, "git config --get user.email")
	if err != nil {
		// This is not expected
		return "", fmt.Errorf("error while trying to get local user info")
	}
	if len(result) == 0 {
		// Local user info does not exist, must be a newly deployed container
		log.Printf("Branch '%s' does not have user info in local repo %s", wg.WorkspaceName, workspacePath)
		return "removed", nil
	}

	// Check if workspace is the active workspace
	isActiveWorkspace := false
	if wg.WorkspaceName == wg.ActiveWorkspace {
		isActiveWorkspace = true
	}
	
	// Issue the git status command to see if workspace has modifications
	result, err = runShellCommand(workspacePath, "git status")
	if err != nil {
		return "", fmt.Errorf("error while executing 'git status' command: %v", err)
	}
	//*
	fmt.Printf("*** Git Status for %s:\n%s",wg.WorkspaceName, result)
	if strings.Contains(result, "nothing to commit, working tree clean") {
		if isActiveWorkspace {
			return "active", nil
		}
		return "no changes", nil
	}
	if isActiveWorkspace {
		return "active modified", nil
	}
	return "modified", nil
}

func (wg *WorkspaceGit) UpdateLocalWorkspace(userName, userEmail, gitUser, gitToken, wsPreviousName string) (string, error) {
	// Insert or update workspace entry in workspace_registry table:
	//	- If folder workspace_name in workspaces root does not exists, chechout workspace_uri in workspace_name
	//  - If user is renaming workspace_name, delete the old workspace folder under workspaces root
	//    Note: UI must provide old workspace name as 'previous.workspace_name' virtual column
	if wg.WorkspaceName == "" {
		return "", fmt.Errorf("error, must provide workspace_name")
	}
	workspacePath := fmt.Sprintf("%s/%s", wg.WorkspacesHome, wg.WorkspaceName)
	
	//*
	fmt.Println("*** UpdateLocalWorkspace - workspacePath:",workspacePath)
	var buf strings.Builder

	// First, check if workspace directory exists or not
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		buf.WriteString("== Workspace directory does not exist, checking out workspace from git ==\n")
		gitRepo := strings.TrimPrefix(wg.WorkspaceUri, "https://")
		command := fmt.Sprintf("git clone 'https://%s:%s%s' %s", gitUser, gitToken, gitRepo, wg.WorkspaceName)
		_, err := runShellCommand(wg.WorkspacesHome, command)
		if err != nil {
			return "", err
		}
 	}


	// Check if user info exist in local repo
	result, err := runShellCommand(workspacePath, "git config --get user.email")
	if err != nil {
		// This is not expected
		return "", fmt.Errorf("error while trying to get local user info")
	}
	if len(result) == 0 {
		// Local user info does not exist, must be a newly deployed container
		buf.WriteString(fmt.Sprintf("Local repo '%s' does not have user info, configuring it.\n", workspacePath))
		command := fmt.Sprintf("git config user.email \"%s\"", userEmail)
		_, err := runShellCommand(workspacePath, command)
		if err != nil {
			return "", err
		}
		command = fmt.Sprintf("git config user.name \"%s\"", userName)
		_, err = runShellCommand(workspacePath, command)
		if err != nil {
			return "", err
		}
	}

	// Check if the local branch exists
	_, err = runShellCommand(workspacePath, fmt.Sprintf("git show-ref --verify --quiet refs/heads/%s", wg.WorkspaceName))
	if err != nil {
		// Local branch does not exist, must be a newly deployed container
		buf.WriteString(fmt.Sprintf("Branch '%s' does not exist in local repo %s\nCreating it...\n", wg.WorkspaceName, workspacePath))
		// git checkout -b <WorkspaceName>
		// git push -u origin HEAD
		command := fmt.Sprintf("git checkout -b %s", wg.WorkspaceName)
		result, err := runShellCommand(workspacePath, command)
		if err != nil {
			return "", err
		}
		buf.WriteString(result)
		buf.WriteString("\n")
		result, err = runShellCommand(workspacePath, "git push -u origin HEAD")
		if err != nil {
			return "", err
		}
		buf.WriteString(result)		
	}
	return buf.String(), nil
}

func (wg *WorkspaceGit) CommitLocalWorkspace(gitUser, gitToken string) (string, error) {
	// Commit and push workspace changes, git commands to execute:
	// git add -A
	// git commit -m '<message>'
	// git push 'https://<user>:<token>@<repo>' 
		if wg.WorkspaceName == "" {
		return "", fmt.Errorf("error, must provide workspace_name")
	}
	workspacePath := fmt.Sprintf("%s/%s", wg.WorkspacesHome, wg.WorkspaceName)
	
	//*
	fmt.Println("*** UpdateLocalWorkspace - workspacePath:",workspacePath)
	var buf strings.Builder

	// Add changes to git index
	result, err := runShellCommand(workspacePath, "git add -A")
	if err != nil {
		return "", fmt.Errorf("error while trying to (git) add file contents to the index")
	}
	buf.WriteString(result)
	buf.WriteString("\n")

	// Commit changes
	result, err = runShellCommand(workspacePath, "git commit -m 'Changes from JetStore UI'")
	if err != nil {
		return "", fmt.Errorf("error while trying to (commit) record changes to the repository")
	}
	buf.WriteString(result)
	buf.WriteString("\n")

	gitRepo := strings.TrimPrefix(wg.WorkspaceUri, "https://")
	command := fmt.Sprintf("git push 'https://%s:%s%s'", gitUser, gitToken, gitRepo)
	_, err = runShellCommand(wg.WorkspacesHome, command)
	if err != nil {
		return "", err
	}
	buf.WriteString("Changes pushed to repository\n")

	return buf.String(), nil
}

func (wg *WorkspaceGit) PullRemoteWorkspace(gitUser, gitToken string) (string, error) {
	// Pull workspace changes, git commands to execute:
	// git pull --rebase=false --no-commit 'https://<user>:<token>@<repo>' <WorkspaceName> 
		if wg.WorkspaceName == "" {
		return "", fmt.Errorf("error, must provide workspace_name")
	}
	workspacePath := fmt.Sprintf("%s/%s", wg.WorkspacesHome, wg.WorkspaceName)
	
	//*
	fmt.Println("*** UpdateLocalWorkspace - workspacePath:",workspacePath)
	var buf strings.Builder

	// Add changes to git index
	result, err := runShellCommand(workspacePath, "git add -A")
	if err != nil {
		return "", fmt.Errorf("error while trying to (git) add file contents to the index")
	}
	buf.WriteString(result)
	buf.WriteString("\n")

	// Commit changes
	result, err = runShellCommand(workspacePath, "git commit -m 'Changes from JetStore UI'")
	if err != nil {
		return "", fmt.Errorf("error while trying to (commit) record changes to the repository")
	}
	buf.WriteString(result)
	buf.WriteString("\n")

	gitRepo := strings.TrimPrefix(wg.WorkspaceUri, "https://")
	command := fmt.Sprintf("git push 'https://%s:%s%s'", gitUser, gitToken, gitRepo)
	_, err = runShellCommand(wg.WorkspacesHome, command)
	if err != nil {
		return "", err
	}
	buf.WriteString("Changes pushed to repository\n")

	return buf.String(), nil
}
