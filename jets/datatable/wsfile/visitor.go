package wsfile

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"strings"
)

// This file contains function to visit the workspace directory to collect file information

// This struct correspond to MenuEntry for the ui
type WorkspaceStructure struct {
	Key           string            `json:"key"`
	WorkspaceName string            `json:"workspace_name"`
	ResultType    string            `json:"result_type"`
	ResultData    *[]*WorkspaceNode `json:"result_data"`
}
type WorkspaceNode struct {
	Key          string            `json:"key"`
	PageMatchKey string            `json:"pageMatchKey"`
	Type         string            `json:"type"`
	Size         int64             `json:"size"`
	Label        string            `json:"label"`
	RoutePath    string            `json:"route_path"`
	RouteParams  map[string]string `json:"route_params"`
	Children     *[]*WorkspaceNode `json:"children"`
}


func VisitDirWrapper(root, dir, dirLabel string, filters *[]string, workspaceName string) (*WorkspaceNode, error) {
	var children *[]*WorkspaceNode
	var err error
	children, err = visitDir(root, dir, dir, filters, workspaceName)
	if err != nil {
		return nil, err
	}

	for _, c := range *children {
		if c.Type == "dir" {
			c.Children, err = visitChildren(root+"/"+dir, dir+"/"+c.Label, c.Label, filters, workspaceName)
			if err != nil {
				return nil, err
			}
		}
	}

	results := &WorkspaceNode{
		Key:          dir,
		Type:         "section",
		PageMatchKey: dir,
		Label:        dirLabel,
		RoutePath:    "/workspace/:workspace_name/home",
		RouteParams: map[string]string{
			"workspace_name": workspaceName,
			"label":          dirLabel,
		},
		Children: children,
	}

	return results, nil
}

func visitChildren(root, relativeRoot, dir string, filters *[]string, workspaceName string) (*[]*WorkspaceNode, error) {
	var children *[]*WorkspaceNode
	var err error
	children, err = visitDir(root, relativeRoot, dir, filters, workspaceName)
	if err != nil {
		return nil, err
	}

	for _, c := range *children {
		if c.Type == "dir" {
			c.Children, err = visitChildren(root+"/"+dir, relativeRoot+"/"+c.Label, c.Label, filters, workspaceName)
			if err != nil {
				return nil, err
			}
		}
	}

	return children, nil
}

// Function that visit a directory path to collect the file structure
// This function returns the direct children of the directory
// root is workspace root path (full path)
// relativeRoot is file relative root with respect to workspace root (file path within workspace)
// relativeRoot includes dir as the last component of it
// Note: This function cannot be called recursively, otherwise it will interrupt WalDir
//
// A section directory that does not exist yields an empty section rather than an
// error, and that is a deliberate change of failure mode (task F.0a, 2026-08-23).
//
// The sections are hard-coded in workspace_data_table_action.go, one
// VisitDirWrapper call each, and before this change a missing directory made
// WalkDir invoke its callback with a stat error — which the callback returned,
// which failed the whole workspace_file_structure request. **One absent folder
// therefore emptied the IDE's entire file tree, not its own heading.** The
// process_sequence removal earlier the same day had to be sequenced around that
// blast radius, and its comment says so.
//
// It was not hypothetical by then: S.3 added the `user_flows` section, and no
// workspace in the corpus has a `user_flows/` directory — cedargate_ws, jets_ws,
// usi_ws and walrus_ws all lack one at the commits this branch pins. The tree was
// failing for every workspace, and it read as a server error rather than as a
// missing folder.
//
// Only fs.ErrNotExist is swallowed. A directory that exists and cannot be read is
// still an error, because that one is a real fault rather than a section nobody
// has authored into this workspace yet.
func visitDir(root, relativeRoot, dir string, filters *[]string, workspaceName string) (*[]*WorkspaceNode, error) {

	// fmt.Println("*visitDir called for:",fmt.Sprintf("%s/%s", root, dir))
	dirPath := fmt.Sprintf("%s/%s", root, dir)
	children := make([]*WorkspaceNode, 0)
	if _, statErr := os.Stat(dirPath); errors.Is(statErr, fs.ErrNotExist) {
		log.Printf("workspace directory %q does not exist, reporting an empty section", dirPath)
		return &children, nil
	}
	fileSystem := os.DirFS(dirPath)

	err := fs.WalkDir(fileSystem, ".", func(path string, info fs.DirEntry, err error) error {
		// fmt.Println("*** WalkDir @",path, "err is",err)
		if err != nil {
			log.Printf("ERROR while walking workspace directory %q: %v", path, err)
			return err
		}

		if info.Name() == "." {
			return nil
		}

		if info.IsDir() {

			subdir := info.Name()
			// fmt.Println("visiting directory:", subdir)
			children = append(children, &WorkspaceNode{
				Key:          path,
				Type:         "dir",
				PageMatchKey: path,
				Label:        subdir,
				RouteParams: map[string]string{
					"workspace_name": workspaceName,
					"label":          subdir,
				},
			})
			return fs.SkipDir

		} else {

			filename := info.Name()
			keepEntry := false
			for i := range *filters {
				if strings.HasSuffix(filename, (*filters)[i]) {
					keepEntry = true
				}
			}
			if keepEntry {
				// fmt.Println("visiting file:", filename)
				fileInfo, err := info.Info()
				var size int64
				if err != nil {
					log.Println("while trying to get the file size:", err)
				} else {
					size = fileInfo.Size()
				}
				relativeFileName := url.QueryEscape(fmt.Sprintf("%s/%s", relativeRoot, filename))
				children = append(children, &WorkspaceNode{
					Key:          path,
					Type:         "file",
					PageMatchKey: relativeFileName,
					Label:        filename,
					Size:         size,
					RoutePath:    "/workspace/:workspace_name/home",
					RouteParams: map[string]string{
						"workspace_name": workspaceName,
						"file_name":      relativeFileName,
						"label":          filename,
					},
				})
			}
		}
		return nil
	})

	if err != nil {
		log.Println("while walking workspace dir:", err)
		return nil, err
	}
	return &children, nil
}
