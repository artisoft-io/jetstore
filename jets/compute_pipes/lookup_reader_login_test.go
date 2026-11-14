package compute_pipes

// Which login a sql_lookup's query runs on (lookup_reader_login.go).
//
// The unit tests need nothing. TestLookupQueriesRunAsTheReader needs a scratch
// PostgreSQL named by JETS_TEST_DSN, as a superuser: it creates a login that is
// not one (cpl_apiserver) and a database it owns (cpl_lookup), as a
// deployment's apiserver user is, and runs everything else as that login.
// Skipped when JETS_TEST_DSN is not set. Locally:
//
//	docker run -d --rm --name cpl-pg -e POSTGRES_PASSWORD=pw -p 5492:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5492/postgres \
//	  go test -count=1 -run 'LookupQueries|LookupReader' ./jets/compute_pipes/

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/dblogin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A sql_lookup with no database pool is an error, not a panic, and a pipeline
// with no sql_lookup opens no login.
func TestLookupReaderNeedsAPool(t *testing.T) {
	mgr := NewLookupTableManager([]*LookupSpec{{Key: "t", Type: "sql_lookup", Query: "SELECT 1"}}, nil, false)
	if err := mgr.PrepareLookupTables(nil); !errors.Is(err, errNoLookupPool) {
		t.Fatalf("PrepareLookupTables(nil) with a sql_lookup: got %v, want %v", err, errNoLookupPool)
	}
	if _, err := NewLookupTableSql(nil, &LookupSpec{Key: "t", Type: "sql_lookup"}, nil, false); !errors.Is(err, errNoLookupPool) {
		t.Fatalf("NewLookupTableSql(nil): got %v, want %v", err, errNoLookupPool)
	}
	if err := NewLookupTableManager(nil, nil, false).PrepareLookupTables(nil); err != nil {
		t.Fatalf("PrepareLookupTables(nil) with no lookup: got %v, want nil", err)
	}
}

func TestWithReaderHint(t *testing.T) {
	for _, code := range []string{"42501", "25006"} {
		denied := &pgconn.PgError{Code: code, Message: "refused"}
		got := withReaderHint(denied)
		if !errors.Is(got, denied) || !strings.Contains(got.Error(), dblogin.Reader) {
			t.Fatalf("withReaderHint(%s) = %v; want the error, wrapped, naming %s", code, got, dblogin.Reader)
		}
	}
	other := &pgconn.PgError{Code: "42P01", Message: "relation does not exist"}
	if got := withReaderHint(other); got != error(other) {
		t.Fatalf("withReaderHint(42P01) = %v; want it unchanged", got)
	}
}

// cplDB is a database owned by a login that is not a superuser, as a
// deployment's apiserver user is, and a pool connected as that login.
func cplDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of this file")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer admin.Close()
	for _, s := range []string{
		"DROP DATABASE IF EXISTS cpl_lookup WITH (FORCE)",
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cpl_apiserver') THEN
				CREATE ROLE cpl_apiserver LOGIN PASSWORD 'cpl' CREATEROLE;
			END IF;
		END $$`,
		"CREATE DATABASE cpl_lookup OWNER cpl_apiserver",
	} {
		if _, err = admin.Exec(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword("cpl_apiserver", "cpl")
	u.Path = "/cpl_lookup"
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestLookupQueriesRunAsTheReader(t *testing.T) {
	db := cplDB(t)
	ctx := context.Background()
	for _, s := range []string{
		"CREATE SCHEMA jetsapi",
		"CREATE TABLE jetsapi.users (user_email TEXT PRIMARY KEY, name TEXT, password TEXT, git_token TEXT, encrypted_roles TEXT[])",
		"INSERT INTO jetsapi.users VALUES ('admin@example.com', 'Admin', 'secret', NULL, '{admin}')",
		"CREATE TABLE public.cpl_thresholds (field_id TEXT, upper_failure DOUBLE PRECISION)",
		"INSERT INTO public.cpl_thresholds VALUES ('f1', 0.5), ('f2', 0.75)",
	} {
		if _, err := db.Exec(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	// The apiserver sets the logins up at its start; here, no dblogin.Ensure
	// has run, so the first lookup load sets them up, as on a database the
	// apiserver never reached.
	readerLogin, err := dblogin.LoginName(ctx, db, dblogin.Reader)
	if err != nil {
		t.Fatal(err)
	}
	load := func(query string, columns []string, values []string) (*LookupTableManager, error) {
		spec := &LookupSpec{Key: "lk", Type: "sql_lookup", Query: query, LookupKey: []string{columns[0]}, LookupValues: values}
		for _, c := range columns {
			spec.Columns = append(spec.Columns, TableColumnSpec{Name: c, RdfType: "text"})
		}
		mgr := NewLookupTableManager([]*LookupSpec{spec}, map[string]any{"$REPORT": "f1"}, false)
		return mgr, mgr.PrepareLookupTables(db)
	}

	t.Run("a lookup query runs as the reader login", func(t *testing.T) {
		mgr, err := load("SELECT 'k' AS key, session_user::text AS who", []string{"key", "who"}, []string{"who"})
		if err != nil {
			t.Fatal(err)
		}
		k := "k"
		row, err := mgr.LookupTableMap["lk"].Lookup(&k)
		if err != nil || row == nil {
			t.Fatalf("Lookup(k): %v, %v", row, err)
		}
		if got := (*row)[0]; got != readerLogin {
			t.Fatalf("session_user = %v, want %s", got, readerLogin)
		}
	})

	t.Run("a lookup query reads a public table, with env substitution", func(t *testing.T) {
		mgr, err := load("SELECT field_id, upper_failure::text AS upper FROM public.cpl_thresholds WHERE field_id = '$REPORT'",
			[]string{"field_id", "upper"}, []string{"upper"})
		if err != nil {
			t.Fatal(err)
		}
		tbl := mgr.LookupTableMap["lk"]
		if tbl.Size() != 1 {
			t.Fatalf("size = %d, want 1", tbl.Size())
		}
		k := "f1"
		row, _ := tbl.Lookup(&k)
		if row == nil || (*row)[0] != "0.5" {
			t.Fatalf("Lookup(f1) = %v, want [0.5]", row)
		}
	})

	t.Run("a data-modifying CTE in a lookup query is refused", func(t *testing.T) {
		_, err := load(`WITH x AS (UPDATE jetsapi.users SET encrypted_roles = '{admin,x}' RETURNING user_email)
			SELECT user_email AS key FROM x`, []string{"key"}, nil)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "25006" && pgErr.Code != "42501") || !strings.Contains(err.Error(), dblogin.Reader) {
			t.Fatalf("UPDATE through a CTE: got %v, want a refusal with the hint", err)
		}
		var roles []string
		if err := db.QueryRow(ctx, "SELECT encrypted_roles FROM jetsapi.users").Scan(&roles); err != nil {
			t.Fatal(err)
		}
		if len(roles) != 1 {
			t.Fatalf("encrypted_roles = %v after the refused lookup, want {admin}", roles)
		}
	})

	t.Run("the reader login refuses the write on its own", func(t *testing.T) {
		// Outside the read-only transaction: the login, not only the
		// transaction, refuses it.
		reader, err := openLookupReader(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		_, err = reader.Exec(ctx, "UPDATE jetsapi.users SET encrypted_roles = '{admin,x}'")
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("UPDATE as the reader login: got %v, want permission denied", err)
		}
		if n := reader.Stat().MaxConns(); n != lookupReaderPoolSize {
			t.Fatalf("reader pool MaxConns = %d, want %d", n, lookupReaderPoolSize)
		}
	})

	t.Run("a lookup query cannot read a secret column", func(t *testing.T) {
		_, err := load("SELECT user_email AS key, password FROM jetsapi.users", []string{"key", "password"}, []string{"password"})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("SELECT password in a lookup: got %v, want permission denied", err)
		}
	})
}
