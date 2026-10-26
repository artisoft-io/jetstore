package rulesbench

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RowWriter writes every row a run extracts, for comparing an extraction before
// and after a change to it (rules_benchmark G11): graph dumps are made before
// extraction and do not cover it. Its Write is an Options.OnExtractedRow and its
// SessionDone an Options.AfterExtraction.
//
// One file per class of each session, <dir>/<RunDir>/<engine>/<session>/<class>.rows,
// one line per row in the order the extraction emitted it, the cells separated by
// tabs. A cell is rendered with its Go type, so an int and a double that print
// alike do not compare equal, and a multi-valued column keeps its values' order.
// A session's files are closed once it is extracted, so a run holds open only
// the files of the sessions being extracted, and a session's write error is that
// session's.
type RowWriter struct {
	dir string
	mu  sync.Mutex
	// open are the open files by session directory, then by path
	open map[string]map[string]*rowFile
	// errs are the first error of each session not yet done, by session directory
	errs map[string]error
}

type rowFile struct {
	f *os.File
	w *bufio.Writer
}

// NewRowWriter writes the row tree under dir.
func NewRowWriter(dir string) *RowWriter {
	return &RowWriter{dir: dir, open: map[string]map[string]*rowFile{}, errs: map[string]error{}}
}

func (w *RowWriter) sessionDir(p DumpPoint) string {
	return filepath.Join(w.dir, p.RunDir(), p.Engine, safeName(p.Session))
}

// Write appends row to its class's file. It is called on the class's drain
// goroutine, so concurrently for different classes and, in a pool, sessions. An
// error stops the session's writing and is returned by its SessionDone.
func (w *RowWriter) Write(p DumpPoint, className string, row []any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	dir := w.sessionDir(p)
	if w.errs[dir] != nil {
		return
	}
	path := filepath.Join(dir, safeName(className)+".rows")
	files := w.open[dir]
	if files == nil {
		files = map[string]*rowFile{}
		w.open[dir] = files
	}
	rf := files[path]
	if rf == nil {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			w.errs[dir] = err
			return
		}
		f, err := os.Create(path)
		if err != nil {
			w.errs[dir] = err
			return
		}
		rf = &rowFile{f: f, w: bufio.NewWriter(f)}
		files[path] = rf
	}
	for i, cell := range row {
		if i > 0 {
			rf.w.WriteByte('\t')
		}
		rf.w.WriteString(renderCell(cell))
	}
	if _, err := rf.w.WriteString("\n"); err != nil {
		w.errs[dir] = fmt.Errorf("%s: %w", path, err)
	}
}

// SessionDone closes the files of p's session, which Extract has finished, and
// returns the session's first error.
func (w *RowWriter) SessionDone(p DumpPoint) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	dir := w.sessionDir(p)
	err := w.errs[dir]
	if cerr := closeFiles(w.open[dir]); err == nil {
		err = cerr
	}
	delete(w.open, dir)
	delete(w.errs, dir)
	return err
}

// Close closes the files of any session that ended without SessionDone, and
// returns the first error not yet returned.
func (w *RowWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var first error
	for dir, files := range w.open {
		if err := closeFiles(files); err != nil && first == nil {
			first = err
		}
		delete(w.open, dir)
	}
	for dir, err := range w.errs {
		if first == nil {
			first = err
		}
		delete(w.errs, dir)
	}
	return first
}

// closeFiles flushes and closes files, and returns the first error.
func closeFiles(files map[string]*rowFile) error {
	var first error
	for path, rf := range files {
		err := rf.w.Flush()
		if cerr := rf.f.Close(); err == nil {
			err = cerr
		}
		if err != nil && first == nil {
			first = fmt.Errorf("%s: %w", path, err)
		}
	}
	return first
}

// renderCell renders one cell of an extracted row with its type.
func renderCell(v any) string {
	switch c := v.(type) {
	case nil:
		return "null"
	case []any:
		parts := make([]string, len(c))
		for i := range c {
			parts[i] = renderCell(c[i])
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case string:
		return "string:" + strconv.Quote(c)
	case float64:
		return "float64:" + strconv.FormatFloat(c, 'g', -1, 64)
	case time.Time:
		return "time:" + c.Format(time.RFC3339Nano) + " " + c.Location().String()
	default:
		return strconv.Quote(fmt.Sprintf("%T:%v", v, v))
	}
}
