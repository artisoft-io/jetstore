// The pending override and the last compile's reports.tgz are read from the rows
// the report's sync and the compile write.
//
// Needs JETS_TEST_DSN; skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5466:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5466/postgres go test ./jets/run_reports/delegate/ -run PendingOverride
package delegate

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/optstats"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestThePendingOverrideIsTheRowTheSyncWrites(t *testing.T) {
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	b, err := os.ReadFile("../../jets_schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var defs []schema.TableDefinition
	if err := json.Unmarshal(b, &defs); err != nil {
		t.Fatal(err)
	}
	for i := range defs {
		if defs[i].TableName != "workspace_changes" {
			continue
		}
		def := defs[i]
		if err := def.UpdateTableSchema(pool, true); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = def.DropTable(pool) })
	}

	if data, err := optimizerStatsCurrent.override(pool, "jets_ws"); err != nil || data != nil {
		t.Fatalf("no row: got (%q, %v), want (nil, nil)", data, err)
	}
	content := statsFile(t, map[string][]int{"jet_rules/acme_main.jr": {2}})
	// As awsi.SyncS3Files writes it after the merge.
	fo := dbutils.FileDbObject{WorkspaceName: "jets_ws", FileName: optstats.FileName, ContentType: "jet_rules", UserEmail: "system"}
	if _, err := fo.WriteObject(pool, content); err != nil {
		t.Fatal(err)
	}
	data, err := optimizerStatsCurrent.override(pool, "jets_ws")
	if err != nil || !bytes.Equal(data, content) {
		t.Fatalf("got (%q, %v), want the row's content", data, err)
	}
	if data, err := optimizerStatsCurrent.override(pool, "other_ws"); err != nil || data != nil {
		t.Fatalf("another workspace: got (%q, %v), want (nil, nil)", data, err)
	}

	// The last compile's archive, as workspace.UploadWorkspaceAssets writes it.
	archive := tgz(t, map[string][]byte{optstats.FileName: content})
	ar := dbutils.FileDbObject{WorkspaceName: "jets_ws", FileName: "reports.tgz", ContentType: "reports.tgz", UserEmail: "system"}
	if _, err := ar.WriteObject(pool, archive); err != nil {
		t.Fatal(err)
	}
	got, err := optimizerStatsCurrent.reportsArchive(pool, "jets_ws")
	if err != nil || !bytes.Equal(got, archive) {
		t.Fatalf("reports.tgz: got %d bytes, %v; want the row's content", len(got), err)
	}
}
