package datatable

// PutSchemaEventToS3, jetstore_maintenance_02 D01 and task AD.1, 2026-10-01.
//
// Until that date a request without a file_key returned 200 and wrote nothing
// (that project's F3), which is what the React *Put Schema Event* dialog sends by
// design. These tests need no database and no S3: the upload is observed through
// putSchemaEventUpload, and the prefix is set on the package variable the
// function reads, which is what JETS_s3_SCHEMA_TRIGGERS populates at start-up.

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

type recordedUpload struct {
	bucket, key, body string
}

// withRecordedUploads swaps the upload for a recorder and sets the prefix, and
// restores both when the test ends.
func withRecordedUploads(t *testing.T, prefix string, fail error) *[]recordedUpload {
	t.Helper()
	var got []recordedUpload
	savedUpload, savedPrefix := putSchemaEventUpload, jetsS3SchemaTriggers
	t.Cleanup(func() { putSchemaEventUpload, jetsS3SchemaTriggers = savedUpload, savedPrefix })
	jetsS3SchemaTriggers = prefix
	putSchemaEventUpload = func(bucket, key string, buf []byte) error {
		got = append(got, recordedUpload{bucket, key, string(buf)})
		return fail
	}
	return &got
}

const testSchemaTriggers = "jetstore/schema_triggers"
const testEvent = `{"client":"CGT","object_type":"Eligibility","file_key":"client=CGT/x"}`

func putSchemaEvent(rows ...map[string]any) (int, error) {
	ctx := &DataTableContext{}
	_, code, err := ctx.PutSchemaEventToS3(&RegisterFileKeyAction{
		Action: "put_schema_event_to_s3",
		Data:   rows,
	}, "")
	return code, err
}

// The criterion-9 case: no file_key writes the default name under the prefix.
func TestPutSchemaEventWithoutFileKeyWritesTheDefaultName(t *testing.T) {
	for name, row := range map[string]map[string]any{
		"absent": {"event": testEvent},
		"null":   {"event": testEvent, "file_key": nil},
		"empty":  {"event": testEvent, "file_key": ""},
	} {
		t.Run(name, func(t *testing.T) {
			got := withRecordedUploads(t, testSchemaTriggers, nil)
			code, err := putSchemaEvent(row)
			if err != nil || code != http.StatusOK {
				t.Fatalf("got %d, %v; want 200, nil", code, err)
			}
			want := recordedUpload{"", testSchemaTriggers + "/jetstore_ui_event.json", testEvent}
			if len(*got) != 1 || (*got)[0] != want {
				t.Fatalf("uploads = %+v, want exactly %+v", *got, want)
			}
		})
	}
}

// A file_key the caller names is still honoured: the default is a default.
func TestPutSchemaEventKeepsANamedFileKey(t *testing.T) {
	got := withRecordedUploads(t, testSchemaTriggers, nil)
	code, err := putSchemaEvent(map[string]any{"event": testEvent, "file_key": "mine.json"})
	if err != nil || code != http.StatusOK {
		t.Fatalf("got %d, %v; want 200, nil", code, err)
	}
	if len(*got) != 1 || (*got)[0].key != testSchemaTriggers+"/mine.json" {
		t.Fatalf("uploads = %+v, want one at %s/mine.json", *got, testSchemaTriggers)
	}
}

// The other criterion-9 case: an empty event is a 400 and nothing is written.
func TestPutSchemaEventRefusesAnEmptyEvent(t *testing.T) {
	for name, rows := range map[string][]map[string]any{
		"no rows":          {},
		"event absent":     {{"file_key": "x.json"}},
		"event null":       {{"event": nil}},
		"event empty":      {{"event": ""}},
		"event whitespace": {{"event": "  \n\t"}},
		"event not text":   {{"event": map[string]any{"client": "CGT"}}},
		// A bad second row refuses the request before the good first one is
		// written, so a 400 never follows a partial write.
		"second row empty": {{"event": testEvent}, {"event": ""}},
	} {
		t.Run(name, func(t *testing.T) {
			got := withRecordedUploads(t, testSchemaTriggers, nil)
			code, err := putSchemaEvent(rows...)
			if code != http.StatusBadRequest || err == nil {
				t.Fatalf("got %d, %v; want 400 and an error", code, err)
			}
			if len(*got) != 0 {
				t.Fatalf("uploads = %+v, want none", *got)
			}
		})
	}
}

func TestPutSchemaEventRefusesANonStringFileKey(t *testing.T) {
	got := withRecordedUploads(t, testSchemaTriggers, nil)
	code, err := putSchemaEvent(map[string]any{"event": testEvent, "file_key": 42.0})
	if code != http.StatusBadRequest || err == nil || len(*got) != 0 {
		t.Fatalf("got %d, %v, uploads %+v; want 400, an error, none", code, err, *got)
	}
}

// An unset prefix would put the object at "/jetstore_ui_event.json", at the
// bucket root, where no notification is watching: a 200 that triggers nothing.
func TestPutSchemaEventRefusesAnUnsetPrefix(t *testing.T) {
	got := withRecordedUploads(t, "", nil)
	code, err := putSchemaEvent(map[string]any{"event": testEvent})
	if code != http.StatusInternalServerError || err == nil ||
		!strings.Contains(err.Error(), "JETS_s3_SCHEMA_TRIGGERS") || len(*got) != 0 {
		t.Fatalf("got %d, %v, uploads %+v; want 500 naming the variable, none", code, err, *got)
	}
}

func TestPutSchemaEventReportsAnUploadFailure(t *testing.T) {
	withRecordedUploads(t, testSchemaTriggers, errors.New("no credentials"))
	code, err := putSchemaEvent(map[string]any{"event": testEvent})
	if code != http.StatusInternalServerError || err == nil {
		t.Fatalf("got %d, %v; want 500 and an error", code, err)
	}
}
