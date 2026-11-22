package datatable

// Adding files to a workspace from the IDE: a new file, or files and folders
// uploaded from the user's computer.
//
// **An added file is saved as an edit is.** It is checked by the same per-suffix
// validators (checkWorkspaceFileIn) and written by the same function
// (wsfile.SaveContent), so it is an override like any saved file: a
// jetsapi.workspace_changes row that every start of the apiserver applies over
// the workspace, listed in the IDE's Workspace Changes until it is committed.
// Its revert deletes it, since the workspace as deployed has no copy of it to
// restore (wsfile.DeleteFileChange); with no stash, on a workstation in dev
// mode, the revert is refused as it is for any file there.
//
// Two actions share this file:
//
//   - `check_workspace_files` checks the files and writes nothing. It answers,
//     for each file, whether it is new, replaces a file, is refused and why,
//     or is not one the IDE lists, and the warnings a save would carry. The IDE
//     asks it before an upload, so that it can show what the upload does
//     before doing it.
//   - `add_workspace_files` adds them. **All or nothing on the checks**: one
//     refused file refuses the request and nothing is written, so a runbook
//     never arrives without its SQL. A file that is there already is replaced
//     only when its row says `replace: true`; a new file from the IDE's *New
//     file* never says so, and so never overwrites one.
//
// Each data row is `{file_name, file_content, replace}`, `file_name` escaped as
// the save's is (url.QueryUnescape).
//
// **The files of one request are checked together**: the checks that read the
// rest of the workspace (a form's tables, the reports files and their
// directives) read it with the request's files in place (addedFilesFS), so a
// folder holding a form and its table, or `reports/config.json` and the
// reports files it runs, is checked as it will be once added, whatever the
// order of its files.

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// What an added file is, or would be.
const (
	// AddedNew is a file the workspace does not have.
	AddedNew = "new"
	// AddedReplaces is a file the workspace has, which the request replaces.
	AddedReplaces = "replaces"
	// AddedExists is a file the workspace has, in an add request whose row does
	// not ask to replace it. It refuses the request.
	AddedExists = "exists"
	// AddedNotListed is a file the IDE's tree would not list (wsfile.ListedInIde).
	// It refuses the request.
	AddedNotListed = "not_listed"
	// AddedRefused is a file refused for its name or its content. It refuses the
	// request.
	AddedRefused = "refused"
)

// addedFile is one data row of a request.
type addedFile struct {
	// wire is the file_name as sent, escaped, and answered as sent.
	wire    string
	name    string
	content string
	replace bool
}

// AddedFileResult is what the actions answer for each file, in the order sent.
type AddedFileResult struct {
	// FileName is the file_name as sent, escaped.
	FileName string `json:"file_name"`
	// Path is the workspace-relative path it names.
	Path   string `json:"path"`
	Status string `json:"status"`
	// Message says why a file is refused, absent otherwise.
	Message string `json:"message,omitempty"`
	// Warnings are its validator's findings that do not refuse it, as a save
	// answers them (saveCheckedWorkspaceFile).
	Warnings []wsvalidate.Finding `json:"warnings"`
}

// readAddedFiles reads the data rows of a request. A row with no file_name, or
// whose file_name does not unescape, is an error of the request rather than a
// refused file: there is no path to report it under.
func readAddedFiles(data []map[string]any) ([]addedFile, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no file to add")
	}
	files := make([]addedFile, 0, len(data))
	for i, row := range data {
		wire, ok := row["file_name"].(string)
		if !ok || wire == "" {
			return nil, fmt.Errorf("file %d has no file_name", i+1)
		}
		name, err := url.QueryUnescape(wire)
		if err != nil {
			return nil, fmt.Errorf("file %d: %v", i+1, err)
		}
		content, ok := row["file_content"].(string)
		if !ok && row["file_content"] != nil {
			return nil, fmt.Errorf("%s: file_content is not text", name)
		}
		replace, _ := row["replace"].(bool)
		files = append(files, addedFile{wire: wire, name: name, content: content, replace: replace})
	}
	return files, nil
}

// addedNameProblem says what is wrong with a name a file is added under, or ""
// when nothing is: it is a workspace-relative path, written as the tree writes
// one (forward slashes, no `.` or `..` segment, no empty segment, no leading
// or trailing slash), with no control character. **The name is checked as
// written rather than cleaned**, so the file added is the file named; the
// confinement check after it (wsfile.ResolveWorkspacePath) is the guard
// against escaping the workspace, and this one makes the refusal say why.
func addedNameProblem(name string) string {
	switch {
	case strings.TrimSpace(name) == "":
		return "the file has no name"
	case strings.ContainsAny(name, "\\"):
		return "a path is written with forward slashes"
	case strings.HasPrefix(name, "/"):
		return "a path is relative to the workspace, and does not start with /"
	case strings.HasSuffix(name, "/"):
		return "the path names a folder, not a file"
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "the name holds a control character"
		}
	}
	for _, segment := range strings.Split(name, "/") {
		switch segment {
		case "":
			return "the path has an empty folder name"
		case ".", "..":
			return "the path escapes the workspace or names the folder it is in"
		}
		if strings.TrimSpace(segment) != segment {
			return "a folder or file name starts or ends with a space"
		}
	}
	return ""
}

// checkAddedFiles checks each file of a request against the workspace whose
// root is root, as it would be with the request's files in it, and says what
// each is. It writes nothing.
//
// A file is refused, in this order, for its name (addedNameProblem), for
// escaping the workspace, for being named twice in the request, for being a
// file the IDE does not list (wsfile.ListedInIde), for a folder in its place or
// a file in place of one of its folders, and for its content, by the checks a
// save makes (checkWorkspaceFileIn). Otherwise it is new or replaces a file.
func checkAddedFiles(workspaceName, root string, files []addedFile) []AddedFileResult {
	results := make([]AddedFileResult, len(files))
	overlay := addedFilesFS{base: os.DirFS(root), files: map[string]string{}}
	seen := map[string]int{}
	for i, f := range files {
		results[i] = AddedFileResult{FileName: f.wire, Path: f.name, Status: AddedRefused}
		if problem := addedNameProblem(f.name); problem != "" {
			results[i].Message = fmt.Sprintf("%s cannot be added: %s", f.name, problem)
			continue
		}
		if _, err := wsfile.ResolveWorkspacePath(workspaceName, f.name); err != nil {
			results[i].Message = fmt.Sprintf("%s cannot be added: %v", f.name, err)
			continue
		}
		if first, twice := seen[f.name]; twice {
			results[i].Message = fmt.Sprintf("%s cannot be added: it is named twice, as file %d and file %d",
				f.name, first+1, i+1)
			continue
		}
		seen[f.name] = i
		overlay.files[f.name] = f.content
	}
	for i, f := range files {
		if results[i].Message != "" {
			continue
		}
		if !wsfile.ListedInIde(f.name) {
			results[i].Status = AddedNotListed
			results[i].Message = fmt.Sprintf("%s cannot be added: the IDE lists no file there. "+
				"A file is added under a section of the file tree, with a file type the section lists", f.name)
			continue
		}
		exists, problem := addedPathState(root, f.name)
		if problem != "" {
			results[i].Message = fmt.Sprintf("%s cannot be added: %s", f.name, problem)
			continue
		}
		warnings, err := checkWorkspaceFileIn(f.name, f.content, overlay)
		if err != nil {
			results[i].Message = err.Error()
			continue
		}
		if warnings == nil {
			warnings = []wsvalidate.Finding{}
		}
		results[i].Warnings = warnings
		if exists {
			results[i].Status = AddedReplaces
		} else {
			results[i].Status = AddedNew
		}
	}
	return results
}

// addedPathState says whether a file is at name in the workspace whose root is
// root, or what stands in the way of writing one there: a file in place of one
// of the folders on its path, or a folder or something other than a file at
// name. **A problem names the workspace path only**, never the server's.
func addedPathState(root, name string) (exists bool, problem string) {
	var dirs []string
	for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
		dirs = append(dirs, dir)
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(dirs[i])))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return false, ""
		case err != nil:
			log.Printf("while checking %s for an added file: %v", dirs[i], err)
			return false, fmt.Sprintf("its folder %s cannot be read", dirs[i])
		case !info.IsDir():
			return false, fmt.Sprintf("%s is a file, not a folder", dirs[i])
		}
	}
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name)))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, ""
	case err != nil:
		log.Printf("while checking %s for an added file: %v", name, err)
		return false, "it cannot be read"
	case info.Mode().IsRegular():
		return true, ""
	case info.IsDir():
		return false, "a folder of that name is there"
	default:
		return false, "something other than a file is there"
	}
}

// addRefusal is the error an add request is refused with when a file is not to
// be added: one line per such file, in the order sent. nil when every file may
// be added.
func addRefusal(results []AddedFileResult) error {
	var lines []string
	for _, r := range results {
		switch r.Status {
		case AddedNew, AddedReplaces:
		case AddedExists:
			lines = append(lines, fmt.Sprintf("%s cannot be added: a file of that name is there, "+
				"and the request does not replace it", r.Path))
		default:
			lines = append(lines, r.Message)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return fmt.Errorf("nothing was added:\n%s", strings.Join(lines, "\n"))
}

// addCheckedFiles is the add action once the capability is established: check
// every file, refuse the request when any file is refused or would replace a
// file its row does not replace, and otherwise write each with write, in the
// order sent.
//
// **A write that fails part way is a 500 that says how far it got**: the files
// before it are added, as overrides, and the rest are not. The checks have
// passed by then, so a retry of the request with `replace` on the files added
// completes it.
func addCheckedFiles(workspaceName, root string, files []addedFile,
	write func(name, content string) error) (results []AddedFileResult, httpStatus int, err error) {
	results = checkAddedFiles(workspaceName, root, files)
	for i, f := range files {
		if results[i].Status == AddedReplaces && !f.replace {
			results[i].Status = AddedExists
		}
	}
	if err = addRefusal(results); err != nil {
		log.Println(err)
		return results, http.StatusBadRequest, err
	}
	for i, f := range files {
		if err = write(f.name, f.content); err != nil {
			err = fmt.Errorf("%d of %d files added, then %s could not be: %v", i, len(files), f.name, err)
			log.Println(err)
			return results, http.StatusInternalServerError, err
		}
	}
	return results, http.StatusOK, nil
}

// workspaceRoot is the root of the workspace a request names, or an error the
// request is refused with.
func workspaceRoot(dataTableAction *DataTableAction) (string, error) {
	if dataTableAction.WorkspaceName == "" {
		return "", fmt.Errorf("missing workspace_name")
	}
	return wsfile.ResolveWorkspacePath(dataTableAction.WorkspaceName, ".")
}

// CheckWorkspaceFiles is the `check_workspace_files` action: what adding the
// request's files would do, and nothing written. It wants the capability the
// save wants.
func (ctx *DataTableContext) CheckWorkspaceFiles(dataTableAction *DataTableAction, token string) (results *map[string]any, httpStatus int, err error) {
	if _, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token); err2 != nil {
		status, refusal := RefusalFor(err2)
		return nil, status, refusal
	}
	root, err := workspaceRoot(dataTableAction)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("CheckWorkspaceFiles: %v", err)
	}
	files, err := readAddedFiles(dataTableAction.Data)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("CheckWorkspaceFiles: %v", err)
	}
	return &map[string]any{"files": checkAddedFiles(dataTableAction.WorkspaceName, root, files)}, http.StatusOK, nil
}

// AddWorkspaceFiles is the `add_workspace_files` action: the request's files
// added to the workspace as overrides, all of them or none. It wants the
// capability the save wants, and each file is recorded as made by the signed-in
// user.
func (ctx *DataTableContext) AddWorkspaceFiles(dataTableAction *DataTableAction, token string) (results *map[string]any, httpStatus int, err error) {
	signedIn, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		status, refusal := RefusalFor(err2)
		return nil, status, refusal
	}
	root, err := workspaceRoot(dataTableAction)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("AddWorkspaceFiles: %v", err)
	}
	files, err := readAddedFiles(dataTableAction.Data)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("AddWorkspaceFiles: %v", err)
	}
	added, httpStatus, err := addCheckedFiles(dataTableAction.WorkspaceName, root, files,
		func(name, content string) error {
			return wsfile.SaveContent(ctx.Dbpool, dataTableAction.WorkspaceName, name, content, signedIn.Email)
		})
	if err != nil {
		return nil, httpStatus, err
	}
	log.Printf("added to workspace %s by %s: %s", dataTableAction.WorkspaceName, signedIn.Email,
		strings.Join(sortedAddedPaths(added), ", "))
	return &map[string]any{"files": added}, http.StatusOK, nil
}

// addedFilesFS is the workspace as the checks of a request read it: the files
// of the request in place of, or beside, the workspace's own. Only Open is
// needed, since the checks read a file or stat one (fs.ReadFile, fs.Stat); a
// directory is the workspace's, without the request's files in its listing.
type addedFilesFS struct {
	base  fs.FS
	files map[string]string
}

func (o addedFilesFS) Open(name string) (fs.File, error) {
	if content, ok := o.files[name]; ok {
		return &addedFileHandle{name: path.Base(name), Reader: strings.NewReader(content), size: int64(len(content))}, nil
	}
	if o.base == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return o.base.Open(name)
}

// addedFileHandle is an open file of a request, read from memory.
type addedFileHandle struct {
	*strings.Reader
	name string
	size int64
}

func (f *addedFileHandle) Stat() (fs.FileInfo, error) { return addedFileInfo{f.name, f.size}, nil }
func (f *addedFileHandle) Close() error               { return nil }

type addedFileInfo struct {
	name string
	size int64
}

func (i addedFileInfo) Name() string       { return i.name }
func (i addedFileInfo) Size() int64        { return i.size }
func (i addedFileInfo) Mode() fs.FileMode  { return 0644 }
func (i addedFileInfo) ModTime() time.Time { return time.Time{} }
func (i addedFileInfo) IsDir() bool        { return false }
func (i addedFileInfo) Sys() any           { return nil }

// sortedAddedPaths is the paths of results, sorted, for a log line.
func sortedAddedPaths(results []AddedFileResult) []string {
	paths := make([]string, 0, len(results))
	for _, r := range results {
		paths = append(paths, r.Path)
	}
	sort.Strings(paths)
	return paths
}
