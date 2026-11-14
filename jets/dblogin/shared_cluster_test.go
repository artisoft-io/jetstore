package dblogin_test

// Several JetStore databases on one PostgreSQL cluster. The group roles are
// cluster-wide and Ensure runs per database, so two databases share them: two
// Ensure calls may create them at the same moment, and a database whose
// apiserver user did not create them may not be able to grant them. Skipped
// unless JETS_TEST_DSN is set; see jets/dblogin/dblogintest.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/dblogin"
	"github.com/artisoft-io/jetstore/jets/dblogin/dblogintest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Two databases of one cluster, one apiserver user, set up at the same moment:
// both succeed, and each opens both of its logins.
func TestEnsureTwoDatabasesAtOnce(t *testing.T) {
	ctx := context.Background()
	pools := []*pgxpool.Pool{
		dblogintest.Apiserver(t, "dbl_shared", "dbl_shared_1"),
		dblogintest.Apiserver(t, "dbl_shared", "dbl_shared_2"),
	}
	errs := make([]error, len(pools))
	var wg sync.WaitGroup
	for i, pool := range pools {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = dblogin.Ensure(ctx, pool)
		}()
	}
	wg.Wait()
	for i, pool := range pools {
		if errs[i] != nil {
			t.Fatalf("Ensure on dbl_shared_%d: %v", i+1, errs[i])
		}
		for _, group := range []string{dblogin.Reader, dblogin.IDE} {
			login, err := dblogin.Open(ctx, pool, group, 1)
			if err != nil {
				t.Fatalf("Open %s on dbl_shared_%d: %v", group, i+1, err)
			}
			login.Close()
		}
	}
}

// The race itself, staged: a group role created by one database's setup and
// not yet committed, while another database's setup reaches the same CREATE
// ROLE. The second blocks on the first, and must carry on once the first
// commits rather than fail on the duplicate. Staged on a group of its own,
// because Reader and IDE exist as soon as any test has run.
func TestEnsureLoginToleratesAConcurrentCreate(t *testing.T) {
	ctx := context.Background()
	db1 := dblogintest.Apiserver(t, "dbl_shared", "dbl_shared_1")
	db2 := dblogintest.Apiserver(t, "dbl_shared", "dbl_shared_2")
	for _, pool := range []*pgxpool.Pool{db1, db2} {
		if err := dblogin.Ensure(ctx, pool); err != nil { // the login table
			t.Fatal(err)
		}
	}
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	group := "dbl_race_" + hex.EncodeToString(suffix)
	login1, login2 := group+"_1", group+"_2"
	t.Cleanup(func() {
		for _, role := range []string{login1, login2, group} {
			if _, err := db1.Exec(ctx, "DROP ROLE IF EXISTS "+pgx.Identifier{role}.Sanitize()); err != nil {
				t.Errorf("dropping %s: %v", role, err)
			}
		}
	})

	tx1, err := db1.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback(ctx)
	if err = dblogin.EnsureLogin(ctx, tx1, group, login1); err != nil {
		t.Fatal(err)
	}

	second := make(chan error, 1)
	go func() {
		tx2, err := db2.Begin(ctx)
		if err != nil {
			second <- err
			return
		}
		defer tx2.Rollback(ctx)
		if err = dblogin.EnsureLogin(ctx, tx2, group, login2); err != nil {
			second <- err
			return
		}
		second <- tx2.Commit(ctx)
	}()

	// Wait until the second is blocked on the first's uncommitted role.
	waiting := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var n int
		if err = db1.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = 'dbl_shared_2' AND wait_event_type = 'Lock'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if waiting = n > 0; waiting {
			break
		}
	}
	if !waiting {
		t.Fatal("the second setup never blocked on the first's CREATE ROLE")
	}
	if err = tx1.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-second:
		if err != nil {
			t.Fatalf("the second database's setup failed on a role the first created: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the second setup did not finish")
	}
	var members int
	if err = db1.QueryRow(ctx, `SELECT count(*) FROM pg_auth_members m JOIN pg_roles g ON g.oid = m.roleid
		JOIN pg_roles r ON r.oid = m.member WHERE g.rolname = $1 AND r.rolname = ANY($2)`,
		group, []string{login1, login2}).Scan(&members); err != nil {
		t.Fatal(err)
	}
	if members != 2 {
		t.Errorf("%d of the two logins are members of %s", members, group)
	}
}

// A second database whose apiserver user holds CREATEROLE, did not create the
// group roles and holds no ADMIN OPTION on them. From PostgreSQL 16 it may not
// grant them, and Ensure says which role, for which user, and the GRANT that
// fixes it; before 16, CREATEROLE may grant any role that is not a superuser,
// and Ensure succeeds.
func TestEnsureWithoutAdminOptionOnTheGroups(t *testing.T) {
	const user, database = "dbl_noadmin", "dbl_noadmin"
	ctx := context.Background()
	admin := superuser(t)
	// The group roles exist, created by another database's apiserver: here, by
	// the cluster's superuser.
	for _, group := range []string{dblogin.Reader, dblogin.IDE} {
		exec(t, admin, `DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '`+group+`') THEN
				CREATE ROLE `+group+` NOLOGIN;
			END IF;
		END $$`)
	}
	pool := dblogintest.Apiserver(t, user, database)
	// Apiserver gives its user ADMIN OPTION on the groups, which is what this
	// test is about not having, and makes it a superuser when asked to.
	// CASCADE: an earlier run's grants made with that option go too.
	exec(t, admin,
		"ALTER ROLE "+user+" NOSUPERUSER CREATEROLE",
		"REVOKE "+dblogin.Reader+", "+dblogin.IDE+" FROM "+user+" CASCADE")
	// And its logins are new, as they are on a database set up for the first
	// time: one an earlier run left would already be granted.
	var logins []string
	for _, group := range []string{dblogin.Reader, dblogin.IDE} {
		login, err := dblogin.LoginName(ctx, pool, group)
		if err != nil {
			t.Fatal(err)
		}
		logins = append(logins, login)
		exec(t, admin, "DROP ROLE IF EXISTS "+login)
	}
	var version int
	if err := admin.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil {
		t.Fatal(err)
	}

	err := dblogin.Ensure(ctx, pool)
	if version < 160000 {
		if err != nil {
			t.Fatalf("PostgreSQL %d: Ensure without ADMIN OPTION on the groups: %v", version, err)
		}
		return
	}
	var missing *dblogin.MissingAdminError
	if !errors.As(err, &missing) {
		t.Fatalf("PostgreSQL %d: Ensure = %v, want a MissingAdminError", version, err)
	}
	if missing.Role != dblogin.Reader || missing.User != user {
		t.Errorf("MissingAdminError{Role: %q, User: %q}, want %q and %q", missing.Role, missing.User, dblogin.Reader, user)
	}
	want := `GRANT "` + dblogin.Reader + `", "` + dblogin.IDE + `" TO "` + user + `" WITH ADMIN OPTION`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("the error does not give the GRANT to run (%s):\n%v", want, err)
	}
	// Nothing was left half done: the database's logins were not created.
	var left int
	if err = admin.QueryRow(ctx, "SELECT count(*) FROM pg_roles WHERE rolname = ANY($1)", logins).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d login(s) of %s were left behind", left, database)
	}

	// The GRANT the error gives is the whole fix.
	exec(t, admin, want)
	if err = dblogin.Ensure(ctx, pool); err != nil {
		t.Fatalf("Ensure after the GRANT: %v", err)
	}
}

// superuser is a pool on JETS_TEST_DSN, whose user is a superuser.
func superuser(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of jets/dblogin/dblogintest")
	}
	if _, err := url.Parse(dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func exec(t *testing.T, pool *pgxpool.Pool, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if _, err := pool.Exec(context.Background(), s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
}
