package git

import (
	"bytes"
	"fmt"
	"log"
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

// HeadCommit reports the commit the local workspace tree is checked out at, as a
// full 40-character sha, for the workspace named under workspacesHome.
//
// **This is not workspace_registry.last_git_log and the difference is the whole
// reason it exists.** That column accumulates the *transcript* of the git commands
// and the compile output (`UpdateLocalWorkspace` and `compileWorkspaceAction`,
// `jets/datatable/workspace_helper_functions.go:96`) -- free text, appended to,
// and overwritten on the next compile. It cannot be resolved to a commit by any
// consumer. This can.
//
// It is deliberately best-effort for its caller's sake: the workspace tree may be
// synced without a .git directory, in which case the honest answer is that the
// commit is not known, and that is a reason to write null rather than to fail a
// compile.
func HeadCommit(workspacesHome, workspaceName string) (string, error) {
	// Git off: the commit is not knowable, and that is an expected state rather
	// than a fault.
	//
	// **The wording of this error is the whole of the change, not the
	// short-circuit.** The caller (`UpdateWorkspaceVersionDb`,
	// `jets/workspace/compile_workspace_utils.go:493`) already treats any error
	// here as "record a null commit" and reports it with a Notice prefix, so the
	// short-circuit alone would have been invisible to it. What would not have
	// been invisible is `runGit`'s own failure log -- it logs the failing command
	// at error volume before returning -- and a deployment configured to have no
	// repository would then print that line on every compile, for the one thing
	// it was told to expect. An operator reading a log full of git failures in a
	// deployment they deliberately configured without git has no way to tell that
	// alarm from a real one, so the alarm is removed at the source rather than
	// filtered at the reader.
	//
	// This returns no sha and does not consult the environment for one. A build
	// argument carrying the commit is a separate decision with a separate
	// validator, and it lives at the caller (`AR.1`) where the null it replaces is
	// written.
	if NoGitAccess() {
		return "", fmt.Errorf(
			"the workspace commit is not known: git integration is off for this deployment (%s)",
			NoGitAccessEnvVar)
	}
	if _, err := utils.ValidateWorkspaceName(workspaceName); err != nil {
		return "", err
	}
	out, err := runGit(filepath.Join(workspacesHome, workspaceName), "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(out)
	// rev-parse prints the sha on success and an explanation on some failures that
	// still exit zero; refuse anything that is not a sha rather than storing prose.
	if !shaPattern.MatchString(commit) {
		return "", fmt.Errorf("unexpected output from git rev-parse HEAD in workspace %s", workspaceName)
	}
	return commit, nil
}

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Function to delete local workspace directory
//
// **Deliberately not gated on NoGitAccess.** It is the one workspace operation
// here the switch leaves alone: of the seven a button can reach, six run git and
// this one runs os.RemoveAll instead, so there is nothing here for the switch to
// turn off. Gating it would remove the one way a
// deployment without a repository has of reclaiming a workspace it no longer
// wants, and would do so under a notice claiming no git operation was performed
// -- which would be true and beside the point.
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
// With git off (NoGitAccess), the last four rules are replaced by two --
// 'active, no git repository' and 'no git repository' -- and no git command runs.
// The directory test is still made first; see the comment on it below.
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

	// Git off: answer from the file system alone, having already answered the one
	// question the file system can settle on its own.
	//
	// **The order of these two tests is deliberate and is not a matter of taste.**
	// A workspace whose directory has been deleted is removed whether or not this
	// deployment has a repository, and `local workspace removed` is the string the
	// registry screen gates four of its buttons on
	// (`workspaceRegistryTable.tc.json`). Testing the switch first would report a
	// directory that is not there as `no git repository`, re-enabling Push Only,
	// Pull and Export for a workspace with nothing in it.
	//
	// **Neither string below is free choice.** The screen enables Commit & Push
	// only when the status contains `modified`, disables Delete when it contains
	// `active`, and disables four more on `in progress` or `removed` -- all by
	// substring, all in that table, and nothing on either side of the seam checks
	// the pairing. So: no `modified`, because only `git status` can establish that
	// and it has not been run; `active` on the active workspace, because that is
	// what stops a user deleting the workspace this deployment is running out of;
	// and neither `in progress` nor `removed`, which would grey out buttons that
	// work perfectly well here.
	if NoGitAccess() {
		// The read path writes this field back into the result row
		// (`DoWorkspaceReadAction`, `jets/datatable/workspace_data_table_action.go`),
		// so leaving it at whatever the caller passed would put a branch name on a
		// screen in a deployment that has no branches. The empty string is the
		// honest answer and the row renders with a blank cell.
		wg.FeatureBranch = ""
		if isActiveWorkspace {
			return "active, no git repository", nil
		}
		return "no git repository", nil
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

// skipForNoGitAccess is what each of the five pure-git operations returns when
// git is off: the notice, and no error.
//
// # Why no error
//
// The handler writes the returned string into
// workspace_registry.last_git_log and, on an error, also sets the row's status to
// 'error' and reports a 400 to the browser. Nothing failed here -- the deployment
// was configured to have no repository and the operation it implies did not
// happen -- so an error would turn a documented configuration into a red screen
// on every button press. The notice reaches the *View Last Log* dialog by the
// path the transcript always took, which is the point: the UI is told the usual
// thing, and the usual thing happens to say that nothing was done.
//
// # Why one log line rather than one per command
//
// A caller is a button, and the operations it stands in for run between one and
// six git commands. Logging per skipped command would make *Update Workspace*
// six lines and *Push Only* one, for the same amount of nothing, and would
// suggest to a reader that six separate decisions were taken. One line per
// operation is one line per user action.
//
// The operation name prefixes the log line and does not enter the returned
// string. The apiserver log carries the operations of all workspaces interleaved
// and needs to say which button produced the silence; last_git_log is already
// per-workspace and per-action, so there it would be noise.
func skipForNoGitAccess(operation string) (string, error) {
	log.Printf("%s: %s", operation, NoGitAccessNotice)
	return NoGitAccessNotice, nil
}

func (wg *WorkspaceGit) UpdateLocalWorkspace(userName, userEmail, gitUser, gitToken, wsPreviousName string) (string, error) {
	// Git off: return before the validation as well as before the git commands.
	//
	// **The validation below is not a safety check for this path, it is a
	// precondition of the commands that follow it.** validateGitRef rejects an
	// empty feature branch, and a deployment with no repository has no branches to
	// name -- so a workspace registered without one would be refused here, and
	// refused for a reason that only makes sense when there is a remote to push it
	// to. The same argument holds for the other four operations, which is why the
	// check sits at the top of each rather than after the guards.
	if NoGitAccess() {
		return skipForNoGitAccess("UpdateLocalWorkspace")
	}
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
	// Git off: skip the whole sequence, including the two 'git config' calls.
	//
	// **Those two are worth naming because they are the ones that would look
	// harmless.** They write user.email and user.name into the workspace tree's
	// .git/config, which on a workstation checkout is a submodule of the
	// developer's own repository -- so the one part of this function that
	// contacts no remote is also the one that would leave a durable edit behind in
	// a tree the operator did not offer for editing.
	if NoGitAccess() {
		return skipForNoGitAccess("CommitLocalWorkspace")
	}
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
	// Git off: there is no remote to push to, and the credentials this takes are
	// the ones a deployment without a repository has no reason to hold.
	if NoGitAccess() {
		return skipForNoGitAccess("PushOnlyWorkspace")
	}
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
	// Git off: run nothing, including the commands that only read.
	//
	// **This function is the one where a read-only exception would be tempting**,
	// since the screen's *Git Status* button arrives here and 'git status' changes
	// no file. The argument against it is that the text is the user's: deciding
	// which of an arbitrary command line's effects are read-only means parsing
	// git's own surface, and 'git status' is one typo away from 'git stash'. A
	// switch that is a boolean everywhere except in the one place that takes free
	// text is not a switch a reader can hold in their head.
	if NoGitAccess() {
		return skipForNoGitAccess("GitCommandWorkspace")
	}
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
	// Git off: no merge into the working tree.
	//
	// **Only the pull is skipped, and the skip is here rather than at the button.**
	// The handler's pull_workspace action pulls and then re-stashes the workspace
	// file changes, re-applies the database overrides and optionally recompiles;
	// those three are business logic and stay useful in a deployment with no
	// repository. Returning the notice from this function is what lets the caller
	// keep the rest without knowing anything about the switch.
	if NoGitAccess() {
		return skipForNoGitAccess("PullRemoteWorkspace")
	}
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
