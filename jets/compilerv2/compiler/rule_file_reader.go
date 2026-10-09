package compiler

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

// This file has utility functions for reading rule files
// taking into consideration the import statements and other
// compiler directives.
// It must also resolve the global line number to the local
// line number in each file.

var reImportPattern = regexp.MustCompile(`import\s*"([a-zA-Z0-9_\/.-]*)"`)

type readFileFunc func(baseDir, fileName string) (string, error)

// RuleFileReader reads and combines rule files
// with support for import statements and tracking line numbers
// for error reporting.
// Note globalLineNum is 1-based
type RuleFileReader struct {
	basePath          string
	mainFileName      string
	globalLineNum     int
	combinedContent   strings.Builder
	importedFileNames map[string]bool
	importedFileInfo  []*ImportedFileInfo
	readFile          readFileFunc
	// The file the last source_file directive named — what the listener will
	// attribute the next declaration to.
	currentSourceFile string
}

func NewRuleFileReader(basePath string, mainFileName string, readFile readFileFunc) *RuleFileReader {
	return &RuleFileReader{
		basePath:          basePath,
		mainFileName:      mainFileName,
		globalLineNum:     1,
		importedFileNames: make(map[string]bool),
		importedFileInfo:  make([]*ImportedFileInfo, 0),
		readFile:          readFile,
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
func NewImportedFileInfo(fileName string, startLine, endLine, lineOffset int) *ImportedFileInfo {
	return &ImportedFileInfo{
		FileName:   fileName,
		StartLine:  startLine,
		EndLine:    endLine,
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
	// use the imported file info to find the correct file and line number
	for _, fileInfo := range r.importedFileInfo {
		if globalLineNum >= fileInfo.StartLine && (fileInfo.EndLine == 0 || globalLineNum < fileInfo.EndLine) {
			localLineNum := globalLineNum - fileInfo.StartLine + fileInfo.LineOffset + 1
			return fileInfo.FileName, localLineNum, nil
		}
	}
	return "", 0, fmt.Errorf("global line number %d not found in any imported files", globalLineNum)
}

func (r *RuleFileReader) PrintImportedFiles() {
	log.Println("Imported Files:")
	for _, fileInfo := range r.importedFileInfo {
		log.Println(fileInfo)
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
	r.writeSourceFileDirective(fileName)

	content, err := r.readFile(r.basePath, fileName)
	if err != nil {
		return err
	}

	lines := splitLines(content)
	nbrLines := len(lines)
	if nbrLines == 0 {
		return nil // empty file
	}

	// Put the file ImportFileInfo on the stack. A file contributes one segment
	// per uninterrupted run of its own lines, so a file with N imports produces
	// up to N+1 of these. EndLine is provisional here and is corrected the
	// moment the segment stops being filled: it is computed from the file's
	// line count, which includes the import lines that get replaced rather
	// than written.
	segment := NewImportedFileInfo(fileName, r.globalLineNum, r.globalLineNum+nbrLines, 0)
	r.importedFileInfo = append(r.importedFileInfo, segment)
	r.importedFileNames[fileName] = true

	for iLine, line := range lines {
		line = strings.TrimSpace(line)
		// Check for import statement
		// If found, pause the current file, read the imported file recursively,
		// then resume the current file
		if isImportStatement(line) {
			importFileName := extractImportFileName(line)
			if importFileName != "" {

				// Close the segment currently being filled. It must be that
				// segment and not the first one: with two imports in one file,
				// re-closing the first would stretch its range over everything
				// the imports contributed, and GetLocalFileAndLine returns the
				// first range that matches.
				segment.EndLine = r.globalLineNum
				remainingLines := nbrLines - iLine - 1

				// Read the imported file
				err := r.readFileRecursive(importFileName)
				if err != nil {
					return err
				}

				// Resume the current file, re-marking it as the source: the
				// directive is what the listener attributes declarations to
				// (currentRuleFileName), so without it everything after an
				// import in this file would be recorded as belonging to the
				// file that was imported.
				if remainingLines > 0 {
					r.writeSourceFileDirective(fileName)
				}
				segment = NewImportedFileInfo(fileName, r.globalLineNum,
					r.globalLineNum+remainingLines, iLine+1)
				r.importedFileInfo = append(r.importedFileInfo, segment)
			}
		} else {
			r.combinedContent.WriteString(line + "\n")
			r.globalLineNum++
		}
	}

	// Close the final segment at the line actually reached rather than leaving
	// the provisional estimate, which over-counts by one per import consumed
	// after this segment began.
	segment.EndLine = r.globalLineNum

	return nil
}

// writeSourceFileDirective marks the following lines as belonging to fileName.
// It occupies one global line, so the caller's line accounting sees it — and it
// is a no-op when fileName is already the file in effect, which keeps the
// resume case from emitting a directive for a file whose import was its first
// line.
func (r *RuleFileReader) writeSourceFileDirective(fileName string) {
	if r.currentSourceFile == fileName {
		return
	}
	r.combinedContent.WriteString(fmt.Sprintf("@JetCompilerDirective source_file = \"%s\";\n", fileName))
	r.currentSourceFile = fileName
	r.globalLineNum++
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
