package runbook

import (
	"errors"
	"strings"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// Finding codes. Every finding of this package carries one of them.
const (
	CodeJSON          = "runbookJson"          // not JSON, or a field of the wrong JSON type
	CodeUnknownField  = "runbookUnknownField"  // a field the format or the step type does not have
	CodeFormat        = "runbookFormat"        // `format` missing or not 1
	CodeName          = "runbookName"          // `runbook` malformed, or not the file name
	CodeID            = "runbookStepId"        // a step id malformed, reserved or repeated
	CodeType          = "runbookType"          // an unknown step type or value type
	CodeRequired      = "runbookRequired"      // a required field is missing
	CodeLimit         = "runbookLimit"         // a duration, count or cap out of bounds
	CodeConfirm       = "runbookConfirm"       // a ddl that turns off its confirmation
	CodeRef           = "runbookRef"           // a ref that does not resolve, or resolves later
	CodeCondition     = "runbookCondition"     // a condition outside the grammar
	CodeNesting       = "runbookNesting"       // a repeat inside a repeat
	CodeValue         = "runbookValue"         // a literal that does not fit its type
	CodeSQLFile       = "runbookSqlFile"       // a SQL path malformed, or the file missing
	CodeSQLStatements = "runbookSqlStatements" // the wrong number of statements for the step type
	CodeSQLBind       = "runbookSqlBind"       // a `:name` that resolves to nothing, or an unused bind
	CodeSQLSubst      = "runbookSqlSubstitution"
)

// findings accumulates findings; its zero value is ready to use.
type findings []wsvalidate.Finding

func (fs *findings) errorf(code, path, msg string) {
	*fs = append(*fs, wsvalidate.Finding{Severity: wsvalidate.Error, Code: code, Message: msg, Path: path})
}

func (fs *findings) warnf(code, path, msg string) {
	*fs = append(*fs, wsvalidate.Finding{Severity: wsvalidate.Warning, Code: code, Message: msg, Path: path})
}

func (fs *findings) add(f wsvalidate.Finding) { *fs = append(*fs, f) }

// err folds the errors into one Go error, for the UnmarshalJSON methods.
func (fs findings) err() error {
	var msgs []string
	for _, f := range wsvalidate.ErrorsOnly(fs) {
		if f.Path != "" {
			msgs = append(msgs, f.Path+": "+f.Message)
		} else {
			msgs = append(msgs, f.Message)
		}
	}
	if len(msgs) == 0 {
		return nil
	}
	return errors.New(strings.Join(msgs, "; "))
}

// pointer appends one segment to a JSON Pointer, escaped per RFC 6901.
func pointer(base, segment string) string {
	segment = strings.ReplaceAll(segment, "~", "~0")
	segment = strings.ReplaceAll(segment, "/", "~1")
	return base + "/" + segment
}
