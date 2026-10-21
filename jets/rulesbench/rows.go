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
// extraction and do not cover it. Its Write is an Options.OnExtractedRow.
//
// One file per class of each session, <dir>/<RunDir>/<engine>/<session>/<class>.rows,
// one line per row in the order the extraction emitted it, the cells separated by
// tabs. A cell is rendered with its Go type, so an int and a double that print
// alike do not compare equal, and a multi-valued column keeps its values' order:
// two extractions are the same when diff -r finds nothing.
type RowWriter struct {
	dir   string
	mu    sync.Mutex
	files map[string]*rowFile
	err   error
}

type rowFile struct {
	f *os.File
	w *bufio.Writer
}

// NewRowWriter writes the row tree under dir.
func NewRowWriter(dir string) *RowWriter {
	return &RowWriter{dir: dir, files: map[string]*rowFile{}}
}

// Write appends row to its class's file. It is called on the class's drain
// goroutine, so concurrently for different classes. The first error stops the
// writing and is Close's.
func (w *RowWriter) Write(p DumpPoint, className string, row []any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return
	}
	path := filepath.Join(w.dir, p.RunDir(), p.Engine, safeName(p.Session), safeName(className)+".rows")
	rf := w.files[path]
	if rf == nil {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			w.err = err
			return
		}
		f, err := os.Create(path)
		if err != nil {
			w.err = err
			return
		}
		rf = &rowFile{f: f, w: bufio.NewWriter(f)}
		w.files[path] = rf
	}
	for i, cell := range row {
		if i > 0 {
			rf.w.WriteByte('\t')
		}
		rf.w.WriteString(renderCell(cell))
	}
	if _, err := rf.w.WriteString("\n"); err != nil {
		w.err = fmt.Errorf("%s: %w", path, err)
	}
}

// Close flushes and closes every file, and returns the first error.
func (w *RowWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for path, rf := range w.files {
		if err := rf.w.Flush(); err != nil && w.err == nil {
			w.err = fmt.Errorf("%s: %w", path, err)
		}
		if err := rf.f.Close(); err != nil && w.err == nil {
			w.err = err
		}
	}
	w.files = map[string]*rowFile{}
	return w.err
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
