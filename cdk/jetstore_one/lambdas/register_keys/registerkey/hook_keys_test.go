package registerkey

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/datatable"
)

// A key a hook adds that RegisterFileKeys does not read is dropped there without
// a word, so a misspelt key is a registration that silently lacks what the site
// meant to add (JM-096, JM-040). doFileKey now names it in a warning. Keys the
// file key's path put there are not the hook's and are not named.
func TestDoFileKeyWarnsOfAHookKeyNothingReads(t *testing.T) {
	calls := withCapturedRegistration(t)
	withHook(t, &stubHook{added: map[string]any{
		"schema_provider_jsn":  `{"schema_name":"a"}`,
		"schema_provider_json": `{"schema_name":"a"}`,
		"file_size":            int64(10),
	}})
	var buf bytes.Buffer
	saved := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(saved) })

	fileKey := "client=C1/vendor=V1/object_type=ot/processing_ticket=T1/data.csv"
	if err := doFileKey(context.Background(), nil, &datatable.DataTableContext{}, fileKey, 1, "", eventResume{}); err != nil {
		t.Fatalf("doFileKey returned %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d registrations, want 1: a warning must not stop the registration", len(*calls))
	}
	out := buf.String()
	if !strings.Contains(out, "schema_provider_jsn") {
		t.Errorf("no warning names the misspelt key; log:\n%s", out)
	}
	for _, k := range []string{"schema_provider_json", "file_size", "processing_ticket"} {
		if strings.Contains(out, `"`+k+`"`) {
			t.Errorf("the warning names %q, which is read or is not the hook's; log:\n%s", k, out)
		}
	}
}
