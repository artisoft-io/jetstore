package wsfile

import (
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
func visitDir(root, relativeRoot, dir string, filters *[]string, workspaceName string) (*[]*WorkspaceNode, error) {

	// fmt.Println("*visitDir called for:",fmt.Sprintf("%s/%s", root, dir))
	fileSystem := os.DirFS(fmt.Sprintf("%s/%s", root, dir))
	children := make([]*WorkspaceNode, 0)

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
