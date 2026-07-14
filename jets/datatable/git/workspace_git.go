package git

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// This package execute git command in the workspace directory

// Environment needed:
// WORKSPACE Workspace currently in use
// WORKSPACES_HOME Home dir of workspaces

type WorkspaceGit struct {
	WorkspaceName         string
	WorkspaceUri          string
	WorkspaceBranch       string
	FeatureBranch         string
	WorkspacesHome        string
	ActiveWorkspace       string
	ActiveWorkspaceBranch string
}

func NewWorkspaceGit(workspaceName, workspaceUri, workspaceBranch, featureBranch string) *WorkspaceGit {
	return &WorkspaceGit{
		WorkspaceName:         workspaceName,
		WorkspaceUri:          workspaceUri,
		WorkspaceBranch:       workspaceBranch,
		FeatureBranch:         featureBranch,
		WorkspacesHome:        os.Getenv("WORKSPACES_HOME"),
		ActiveWorkspace:       os.Getenv("WORKSPACE"),
		ActiveWorkspaceBranch: os.Getenv("WORKSPACE_BRANCH"),
	}
}

func InitWorkspaceGit(workspaceGit *WorkspaceGit) *WorkspaceGit {
	workspaceGit.WorkspacesHome = os.Getenv("WORKSPACES_HOME")
	workspaceGit.ActiveWorkspace = os.Getenv("WORKSPACE")
	workspaceGit.ActiveWorkspaceBranch = os.Getenv("WORKSPACE_BRANCH")
	return workspaceGit
}

// credURLPattern matches credentials embedded in an https remote URL
// (e.g. https://user:token@host) so they can be redacted from logs and output.
var credURLPattern = regexp.MustCompile(`(https://)[^@\s/]+(@)`)

// redactURLCreds masks credentials embedded in https remote URLs.
func redactURLCreds(s string) string {
	return credURLPattern.ReplaceAllString(s, "${1}***${2}")
}

// authRemoteURL builds an authenticated https remote URL with the credentials
// properly percent-encoded so that special characters cannot alter the URL.
func authRemoteURL(gitUser, gitToken, gitRepo string) string {
	return fmt.Sprintf("https://%s@%s", url.UserPassword(gitUser, gitToken).String(), gitRepo)
}

// validateGitRef ensures a branch/ref name cannot be interpreted as a command-line
// option and does not contain characters that are invalid in git references.
func validateGitRef(ref string) error {
	if ref == "" {
		return fmt.Errorf("error, git reference (branch) is empty")
	}
	if strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, " \t\r\n\\:?*[~^\x00") {
		return fmt.Errorf("invalid git reference: %q", ref)
	}
	return nil
}

// runGit executes a git command with the given arguments in dir. Arguments are passed
// directly to git without a shell, which prevents command/argument injection. Any
// credentials embedded in remote URLs are redacted from the returned output and logs.
func runGit(dir string, args ...string) (string, error) {
	// Sanitize the arguments to prevent injection of options/flags
	args = utils.SanitizeArgs(args)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var b1 bytes.Buffer
	cmd.Stdout = &b1
	cmd.Stderr = &b1
	err := cmd.Run()
	outText := redactURLCreds(b1.String())
	safeCmd := redactURLCreds("git " + strings.Join(args, " "))
	log.Printf("Result from %s :: %s", safeCmd, outText)
	if err != nil {
		err = fmt.Errorf("error while executing '%s' command (see log)", safeCmd)
		log.Printf("%v", err)
		return outText, err
	}
	return outText, nil
}

// runGitBuf runs a git command, appending the (redacted) command line and its output to buf.
func (wg *WorkspaceGit) runGitBuf(buf *strings.Builder, dir string, args ...string) (string, error) {
	buf.WriteString("Executing command ")
	buf.WriteString(redactURLCreds("git " + strings.Join(args, " ")))
	buf.WriteString("\n")
	result, err := runGit(dir, args...)
	buf.WriteString(result)
	return result, err
}

// tokenizeGitCommand splits a single git command line into arguments, honoring single and
// double quotes, without any shell interpretation of metacharacters (;, |, &&, $(), ...).
func tokenizeGitCommand(line string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	var inSingle, inDouble, hasToken bool
	for _, r := range line {
		switch {
		case inSingle:
			if r == '\'' {
				inSingle = false
			} else {
				cur.WriteRune(r)
			}
		case inDouble:
			if r == '"' {
				inDouble = false
			} else {
				cur.WriteRune(r)
			}
		case r == '\'':
			inSingle, hasToken = true, true
		case r == '"':
			inDouble, hasToken = true, true
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			if hasToken {
				tokens = append(tokens, cur.String())
				cur.Reset()
				hasToken = false
			}
		default:
			cur.WriteRune(r)
			hasToken = true
		}
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unbalanced quotes in command")
	}
	if hasToken {
		tokens = append(tokens, cur.String())
	}
	return tokens, nil
}

// Function to delete local workspace directory
func (wg *WorkspaceGit) DeleteWorkspace() error {
	if _, err := utils.ValidateWorkspaceName(wg.WorkspaceName); err != nil {
		return err
	}
	if wg.WorkspaceName == wg.ActiveWorkspace {
		return fmt.Errorf("invaid request, cannot delete the active workspace")
	}
	log.Printf("Deleting local workspace '%s' directory", wg.WorkspaceName)
	if err := os.RemoveAll(filepath.Join(wg.WorkspacesHome, wg.WorkspaceName)); err != nil {
		return fmt.Errorf("while deleting the local workspace dir %s: %v", wg.WorkspaceName, err)
	}
	return nil
}

// Post processing for workspace_registry table to get status from file system:
//   - If workspace_registry.status == 'error', then status = 'error'
//   - If workspace_name folder does not exist: status = removed
//   - If workspace_name == os.Getenv("WORKSPACE") && workspace_branch == os.Getenv("WORKSPACE_BRANCH"):
//   - status = 'active' if local branch set to feature_branch (i.e. != workspace_branch)
//   - status = 'active, missing feature branch' if local branch == workspace_branch
//   - If git status in workspace_name folder contains 'nothing to commit, working tree clean': status = no changes
//   - else: status = modified
//
// Get the column position for workspace_name, workspace_branch, feature_branch and status
func (wg *WorkspaceGit) GetStatus() (string, error) {
	if _, err := utils.ValidateWorkspaceName(wg.WorkspaceName); err != nil {
		return "", err
	}
	workspacePath := filepath.Join(wg.WorkspacesHome, wg.WorkspaceName)

	// Check if workspace is the active workspace
	isActiveWorkspace := false
	if wg.WorkspaceName == wg.ActiveWorkspace && wg.WorkspaceBranch == wg.ActiveWorkspaceBranch {
		isActiveWorkspace = true
	}

	// First, check if workspace directory exists or not
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return "local workspace removed", nil
	}

	// Get the local branch name
	branchName, err := runGit(workspacePath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("while getting local branch name: %v", err)
	}
	branchName = strings.TrimSpace(branchName)
	log.Printf("Local branch is '%s' of local repo %s", branchName, workspacePath)
	if branchName != wg.ActiveWorkspaceBranch {
		wg.FeatureBranch = branchName
	}

	// Issue the git status command to see if workspace has modifications
	result, err := runGit(workspacePath, "status")
	if err != nil {
		return "", fmt.Errorf("error while executing 'git status' command: %v", err)
	}
	hasModif := !strings.Contains(result, "nothing to commit, working tree clean")
	log.Printf("Local branch is '%s' has modifications: %v", branchName, hasModif)

	// Determine the workspace status
	switch {
	case isActiveWorkspace && branchName == wg.ActiveWorkspaceBranch:
		return "active, feature branch missing or removed", nil
	case isActiveWorkspace && branchName != wg.ActiveWorkspaceBranch && !hasModif:
		return "active", nil
	case isActiveWorkspace && branchName != wg.ActiveWorkspaceBranch && hasModif:
		return "active, local file(s) modified", nil
	case !isActiveWorkspace && branchName == wg.ActiveWorkspaceBranch:
		return "feature branch missing or removed", nil
	case !isActiveWorkspace && branchName != wg.ActiveWorkspaceBranch && !hasModif:
		return "no changes", nil
	case !isActiveWorkspace && branchName != wg.ActiveWorkspaceBranch && hasModif:
		return "local file(s) modified", nil

	default:
		return "", fmt.Errorf("unexpected error while determining workspace status")
	}
}

func (wg *WorkspaceGit) UpdateLocalWorkspace(userName, userEmail, gitUser, gitToken, wsPreviousName string) (string, error) {
	// Insert or update workspace entry in workspace_registry table:
	//	- If folder workspace_name in workspaces root does not exists, chechout workspace_uri in workspace_name
	//  - If user is renaming workspace_name, delete the old workspace folder under workspaces root
	//    Note: UI must provide old workspace name as 'previous.workspace_name' virtual column
	if _, err := utils.ValidateWorkspaceName(wg.WorkspaceName); err != nil {
		return "", err
	}
	if err := validateGitRef(wg.WorkspaceBranch); err != nil {
		return "", err
	}
	if err := validateGitRef(wg.FeatureBranch); err != nil {
		return "", err
	}
	gitRepo := strings.TrimPrefix(wg.WorkspaceUri, "https://")
	remoteURL := authRemoteURL(gitUser, gitToken, gitRepo)
	workspacePath := filepath.Join(wg.WorkspacesHome, wg.WorkspaceName)
	var buf strings.Builder

	// First, check if workspace directory exists or not
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		buf.WriteString("\nWorkspace directory does not exist, checking out workspace from git\n")
		// git clone --quiet 'https://<user>:<token>@<repo>' <workspace_name>
		if _, err := wg.runGitBuf(&buf, wg.WorkspacesHome, "clone", "--quiet", remoteURL, wg.WorkspaceName); err != nil {
			buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
			return buf.String(), err
		}
		buf.WriteString("\n")
	} else {
		// Update repository
		// git fetch 'https://<user>:<token>@<repo>'
		if _, err := wg.runGitBuf(&buf, workspacePath, "fetch", remoteURL); err != nil {
			buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
			return buf.String(), err
		}
	}

	// Check if the feature branch exists, if so switch to it
	_, err := wg.runGitBuf(&buf, workspacePath, "show-ref", "--verify", "--quiet", "refs/heads/"+wg.FeatureBranch)
	if err != nil {
		// Feature branch does not exist, check out the WorkspaceBranch and create the FeatureBranch from it
		buf.WriteString(
			fmt.Sprintf("Feature Branch '%s' does not exist in local repo %s\nCreating it from Workspace Branch %s...\n",
				wg.FeatureBranch, workspacePath, wg.WorkspaceBranch))
		// git switch <WorkspaceBranch>
		if _, err := wg.runGitBuf(&buf, workspacePath, "switch", wg.WorkspaceBranch); err != nil {
			buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
			return buf.String(), err
		}
		// FastForward WorkspaceBranch
		// git pull --rebase=false --commit --no-edit 'https://<user>:<token>@<repo>' WorkspaceBranch
		if _, err := wg.runGitBuf(&buf, workspacePath, "pull", "--rebase=false", "--commit", "--no-edit", remoteURL, wg.WorkspaceBranch); err != nil {
			return buf.String(), err
		}
		buf.WriteString("\nChanges pulled from repository\n")
		// git switch -c <FeatureBranch> <WorkspaceBranch>
		if _, err := wg.runGitBuf(&buf, workspacePath, "switch", "-c", wg.FeatureBranch, wg.WorkspaceBranch); err != nil {
			buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
			return buf.String(), err
		}
		buf.WriteString("\n")
		// Publish the branch
		// git push 'https://<user>:<token>@<repo>'
		if _, err := wg.runGitBuf(&buf, workspacePath, "push", remoteURL); err != nil {
			buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
			return buf.String(), err
		}
	} else {
		// Switch to the feature branch
		// git checkout <FeatureBranch>
		if _, err := wg.runGitBuf(&buf, workspacePath, "checkout", wg.FeatureBranch); err != nil {
			buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
			return buf.String(), err
		}
		buf.WriteString("\n")
		// FastForward FeatureBranch
		// git pull --rebase=false --commit --no-edit 'https://<user>:<token>@<repo>' FeatureBranch
		if _, err := wg.runGitBuf(&buf, workspacePath, "pull", "--rebase=false", "--commit", "--no-edit", remoteURL, wg.FeatureBranch); err != nil {
			return buf.String(), err
		}
		buf.WriteString("\nChanges pulled from repository\n")
	}
	return buf.String(), nil
}

func (wg *WorkspaceGit) CommitLocalWorkspace(gitProfile *user.GitProfile, wsCommitMessage string) (string, error) {
	// Commit and push workspace changes, git commands to execute:
	// git add -A
	// git commit -m '<message>'
	// git push 'https://<user>:<token>@<repo>'
	if _, err := utils.ValidateWorkspaceName(wg.WorkspaceName); err != nil {
		return "", err
	}
	workspacePath := filepath.Join(wg.WorkspacesHome, wg.WorkspaceName)
	var buf strings.Builder

	// Set user info
	if _, err := wg.runGitBuf(&buf, workspacePath, "config", "user.email", gitProfile.Email); err != nil {
		buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
		return buf.String(), err
	}
	buf.WriteString("\n")
	if _, err := wg.runGitBuf(&buf, workspacePath, "config", "user.name", gitProfile.Name); err != nil {
		buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
		return buf.String(), err
	}

	// Add all changes to git index
	if _, err := wg.runGitBuf(&buf, workspacePath, "add", "-A"); err != nil {
		buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
		return buf.String(), fmt.Errorf("error while trying to (git) add file contents to the index")
	}
	buf.WriteString("\n")

	// Commit changes
	if wsCommitMessage == "" {
		wsCommitMessage = "Changes from JetStore UI"
	}
	if _, err := wg.runGitBuf(&buf, workspacePath, "commit", "-m", wsCommitMessage); err != nil {
		buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
		return buf.String(), fmt.Errorf("error while trying to (commit) record changes to the repository")
	}
	buf.WriteString("\n")

	// Push changes to repo
	gitRepo := strings.TrimPrefix(wg.WorkspaceUri, "https://")
	if _, err := wg.runGitBuf(&buf, workspacePath, "push", authRemoteURL(gitProfile.GitHandle, gitProfile.GitToken, gitRepo)); err != nil {
		buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
		return buf.String(), err
	}
	buf.WriteString("\nChanges pushed to repository\n")

	return buf.String(), nil
}

func (wg *WorkspaceGit) PushOnlyWorkspace(gitUser, gitToken string) (string, error) {
	// git push 'https://<user>:<token>@<repo>'
	if _, err := utils.ValidateWorkspaceName(wg.WorkspaceName); err != nil {
		return "", err
	}
	workspacePath := filepath.Join(wg.WorkspacesHome, wg.WorkspaceName)
	var buf strings.Builder

	gitRepo := strings.TrimPrefix(wg.WorkspaceUri, "https://")
	if _, err := wg.runGitBuf(&buf, workspacePath, "push", authRemoteURL(gitUser, gitToken, gitRepo)); err != nil {
		buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
		return buf.String(), err
	}

	return buf.String(), nil
}

func (wg *WorkspaceGit) GitCommandWorkspace(gitCommand string) (string, error) {
	// Execute git command(s), one per line. Each command is tokenized and executed
	// directly (no shell), which prevents command/argument injection. Only 'git'
	// commands are permitted.
	if _, err := utils.ValidateWorkspaceName(wg.WorkspaceName); err != nil {
		return "", err
	}
	workspacePath := filepath.Join(wg.WorkspacesHome, wg.WorkspaceName)
	var buf strings.Builder

	commands := strings.Split(gitCommand, "\n")
	for i := range commands {
		if strings.TrimSpace(commands[i]) == "" {
			continue
		}
		args, err := tokenizeGitCommand(commands[i])
		if err != nil {
			buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
			return buf.String(), err
		}
		if len(args) == 0 {
			continue
		}
		if args[0] != "git" {
			err := fmt.Errorf("only 'git' commands are allowed, got: %q", args[0])
			buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
			return buf.String(), err
		}
		if _, err := wg.runGitBuf(&buf, workspacePath, args[1:]...); err != nil {
			buf.WriteString("\n")
			buf.WriteString(fmt.Sprintf("\nGot error: %v", err))
			return buf.String(), err
		}
		buf.WriteString("\n")
	}
	buf.WriteString("\nDone Executing Command(s)\n")
	return buf.String(), nil
}

// Pull changes from orign repo by merging changes into current branch
func (wg *WorkspaceGit) PullRemoteWorkspace(gitUser, gitToken string) (string, error) {
	if _, err := utils.ValidateWorkspaceName(wg.WorkspaceName); err != nil {
		return "", err
	}
	if err := validateGitRef(wg.WorkspaceBranch); err != nil {
		return "", err
	}
	workspacePath := filepath.Join(wg.WorkspacesHome, wg.WorkspaceName)
	var buf strings.Builder

	gitRepo := strings.TrimPrefix(wg.WorkspaceUri, "https://")
	if _, err := wg.runGitBuf(&buf, workspacePath, "pull", "--rebase=false", "--commit", "--no-edit", authRemoteURL(gitUser, gitToken, gitRepo), wg.WorkspaceBranch); err != nil {
		return buf.String(), err
	}
	buf.WriteString("\nChanges pulled from repository\n")

	return buf.String(), nil
}
