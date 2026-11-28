package datatable

// InsertRawRows takes a request with any number of rows, each carrying one
// raw_rows buffer, and replaces dataTableAction.Data with a buffer's parsed rows
// before inserting them. It held the request rows through a pointer to that same
// field, so the second request row was read from the first buffer's parsed rows:
// refused as missing raw_rows when the first buffer had two rows or more, and an
// index out of range (a panic) when it had one. A process mapping's table was
// read from parsed row irow rather than from the buffer's first row.
//
// Needs JETS_TEST_DSN; skipped otherwise.

import (
	"context"
	"net/http"
	"testing"

	"github.com/artisoft-io/jetstore/jets/user"
)

func TestInsertRawRowsReadsEveryRequestRow(t *testing.T) {
	pool := createTablesFromSchema(t, "users", "process_mapping")
	bg := context.Background()
	previousAdmin, previousSecret := user.AdminEmail, user.ApiSecret
	user.AdminEmail, user.ApiSecret = "admin@test", "raw-rows-multi-test"
	t.Cleanup(func() { user.AdminEmail, user.ApiSecret = previousAdmin, previousSecret })
	if _, err := pool.Exec(bg, `INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles)
		VALUES ('admin@test', 'Admin', 'hash', 1, '{}')`); err != nil {
		t.Fatal(err)
	}
	token, err := user.CreateToken("admin@test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &DataTableContext{Dbpool: pool}

	const header = "client,object_type,input_column,data_property\n"
	for _, c := range []struct {
		name string
		raw  []string
		want map[string]string // table_name -> its input columns, in order
	}{
		{
			// The first buffer has two rows: the second request row was read as
			// parsed row 1, which has no raw_rows.
			name: "first buffer of two rows",
			raw: []string{
				header + "c,A,a1,ex:a1\nc,A,a2,ex:a2\n",
				header + "c,B,b1,ex:b1\n",
			},
			want: map[string]string{"c_A": "a1,a2", "c_B": "b1"},
		},
		{
			// The first buffer has one row: the second request row was index 1
			// of a slice of one.
			name: "first buffer of one row",
			raw: []string{
				header + "c,C,c1,ex:c1\n",
				header + "c,D,d1,ex:d1\nc,D,d2,ex:d2\n",
			},
			want: map[string]string{"c_C": "c1", "c_D": "d1,d2"},
		},
		{
			// The third buffer's table is read from its first row, not from its
			// row 2, which names another object type.
			name: "table from the buffer's first row",
			raw: []string{
				header + "c,E,e1,ex:e1\n",
				header + "c,F,f1,ex:f1\n",
				header + "c,G,g1,ex:g1\nc,G,g2,ex:g2\nc,H,g3,ex:g3\n",
			},
			want: map[string]string{"c_E": "e1", "c_F": "f1", "c_G": "g1,g2,g3", "c_H": ""},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			data := make([]map[string]any, 0, len(c.raw))
			for _, raw := range c.raw {
				data = append(data, map[string]any{"raw_rows": raw, "user_email": "admin@test"})
			}
			code, err, panicked := func() (code int, err error, panicked any) {
				defer func() { panicked = recover() }()
				_, code, err = ctx.InsertRawRows(&DataTableAction{
					Action:      "insert_raw_rows",
					FromClauses: []FromClause{{Schema: "jetsapi", Table: "raw_rows/process_mapping"}},
					Data:        data,
				}, token)
				return
			}()
			if panicked != nil {
				t.Fatalf("the handler panicked: %v", panicked)
			}
			if code != http.StatusOK || err != nil {
				t.Fatalf("answered %d, %v; want 200", code, err)
			}
			for table, want := range c.want {
				var got string
				if err := pool.QueryRow(bg, `SELECT coalesce(string_agg(input_column, ',' ORDER BY input_column), '')
					FROM jetsapi.process_mapping WHERE table_name = $1`, table).Scan(&got); err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("%s maps %q, want %q", table, got, want)
				}
			}
		})
	}
}
