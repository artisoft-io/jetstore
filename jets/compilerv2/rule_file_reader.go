package main

import (
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/stack"
)

// This file has utility functions for reading rule files
// taking into consideration the import statements and other
// compiler directives.
// It must also resolve the global line number to the local
// line number in each file.

type readFileFunc func(filePath string) (string, error)

type RuleFileReader struct {
	basePath            string
	mainFileName        string
	globalLineNum       int
	combinedContent     strings.Builder
	importedFileNames   map[string]bool
	importedFileInfo    []*ImportedFileInfo
	inProgressFileStack stack.Stack[ImportedFileInfo]
	readFile            readFileFunc
}

func NewRuleFileReader(basePath string, mainFileName string, readFile readFileFunc) *RuleFileReader {
	return &RuleFileReader{
		basePath:            basePath,
		mainFileName:        mainFileName,
		globalLineNum:       0,
		importedFileNames:   make(map[string]bool),
		importedFileInfo:    make([]*ImportedFileInfo, 0),
		inProgressFileStack: *stack.NewStack[ImportedFileInfo](5),
		readFile:            readFile,
	}
}

// From python version:
// keep track of the imports for error reporting
//   'StartLine' is the first line of the rule file (incl)
//   'EndLine' is the last line of the rule file (excl), ie. +1

type ImportedFileInfo struct {
	FileName   string
	StartLine  int
	EndLine    int
	LineOffset int // global line number - local line number
}

func (i *ImportedFileInfo) String() string {
	return fmt.Sprintf("ImportedFileInfo{FileName: %s, StartLine: %d, EndLine: %d, LineOffset: %d}",
		i.FileName, i.StartLine, i.EndLine, i.LineOffset)
}
func NewImportedFileInfo(fileName string, startLine, lineOffset int) *ImportedFileInfo {
	return &ImportedFileInfo{
		FileName:   fileName,
		StartLine:  startLine,
		EndLine:    0,
		LineOffset: lineOffset,
	}
}

// Read the main rule file and all its imports recursively
// Return the combined content as a single string
func (r *RuleFileReader) ReadAll() (string, error) {
	err := r.readFileRecursive(r.mainFileName)
	if err != nil {
		return "", err
	}
	return r.combinedContent.String(), nil
}

// Return the local file name and line position of the given global line number
// The global line number is 1-based as well as the local line number
func (r *RuleFileReader) GetLocalFileAndLine(globalLineNum int) (string, int, error) {
	// Use a zero-base global line number for easier comparison
	globalLineNum--
	if globalLineNum < 1 {
		return "", 0, fmt.Errorf("global line number must be > 1 or this reference the first compiler directive line")
	}
	// use the imported file info to find the correct file and line number
	for _, fileInfo := range r.importedFileInfo {
		if globalLineNum >= fileInfo.StartLine && (fileInfo.EndLine == 0 || globalLineNum < fileInfo.EndLine) {
			localLineNum := globalLineNum - fileInfo.StartLine + fileInfo.LineOffset
			return fileInfo.FileName, localLineNum + 1, nil
		}
	}
	return "", 0, fmt.Errorf("global line number %d not found in any imported files", globalLineNum)
}

func (r *RuleFileReader) PrintImportedFiles() {
	fmt.Println("Imported Files:")
	for _, fileInfo := range r.importedFileInfo {
		fmt.Println(fileInfo)
	}
}

// readFileRecursive reads a rule file, processes its import statements recursively,
// and appends the content to combinedContent.
// It also updates the global line number and tracks imported files to avoid circular imports.
func (r *RuleFileReader) readFileRecursive(fileName string) error {
	if r.importedFileNames[fileName] {
		// Already imported, skip to avoid circular import
		return nil
	}

	// Put jet compiler directive to mark the file, this also replace the import statement
	// so the imported file starts at the next line
	r.combinedContent.WriteString(fmt.Sprintf("@JetCompilerDirective source_file = \"%s\";\n", fileName))
	r.globalLineNum++

	// Put the file ImportFileInfo on the stack
	fileInfo := NewImportedFileInfo(fileName, r.globalLineNum, 0)
	r.importedFileInfo = append(r.importedFileInfo, fileInfo)
	r.inProgressFileStack.Push(fileInfo)
	r.importedFileNames[fileName] = true

	filePath := fmt.Sprintf("%s/%s", r.basePath, fileName)
	content, err := r.readFile(filePath)
	if err != nil {
		return err
	}

	for iLine, line := range splitLines(content) {
		line = strings.TrimSpace(line)
		// Check for import statement
		// If found, pause the current file, read the imported file recursively,
		// then resume the current file
		if isImportStatement(line) {
			importFileName := extractImportFileName(line)
			if importFileName != "" {

				// Pause the current file
				currentFileInfo, ok := r.inProgressFileStack.Peek()
				if !ok {
					return fmt.Errorf("failed to peek in progress file stack")
				}
				currentFileInfo.EndLine = r.globalLineNum
				// lineOffset := r.globalLineNum - currentFileInfo.StartLine + currentFileInfo.LineOffset

				// Read the imported file
				err := r.readFileRecursive(importFileName)
				if err != nil {
					return err
				}

				// Resume the current file
				fileInfo := NewImportedFileInfo(fileName, r.globalLineNum, iLine)
				r.importedFileInfo = append(r.importedFileInfo, fileInfo)
				r.inProgressFileStack.Push(fileInfo)
			}
		} else {
			r.combinedContent.WriteString(line + "\n")
			r.globalLineNum++
		}
	}

	return nil
}

func splitLines(content string) []string {
	return strings.Split(content, "\n")
}

func isImportStatement(line string) bool {
	return strings.HasPrefix(line, "import ")
}

func extractImportFileName(line string) string {
	matches := reImportPattern.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
