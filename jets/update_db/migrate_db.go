package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/artisoft-io/jetstore/jets/sqlscript"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// execScript executes an entire init db script as a single multi-statement
// simple query, deliberately: PostgreSQL is then the only thing that decides
// where one statement ends and the next begins.
//
// This function used to read up to the next `;` byte and execute the text in
// between. A `;` is an ordinary character inside a `--` comment, a string
// literal, a quoted identifier or a dollar-quoted body, so any of those split a
// statement in two and the second fragment was submitted as SQL — which
// PostgreSQL then reported as a syntax error in text that looked nothing like
// the cause. A comment in `workspaces/jets_ws` cost a deployment on 2026-09-12,
// and two walrus client scripts (`ciseit`, `fbin`) carry twelve semicolons
// inside a multi-line JSON literal and could not be loaded at all.
//
// Two properties come with sending the whole file, and both are wanted:
//
//   - pgx uses the simple protocol whenever Exec is called with no arguments
//     (`Conn.exec`, "Always use simple protocol when there are no arguments"),
//     and PostgreSQL wraps a multi-statement simple query in a single implicit
//     transaction. The whole script therefore applies or none of it does. These
//     scripts are written as DELETE followed by INSERT against the same table,
//     so a failure part-way through used to leave configuration deleted.
//
//   - A script that opens its own transaction still behaves as its author wrote
//     it: an explicit BEGIN supersedes the implicit transaction rather than
//     conflicting with it, which an explicit transaction opened here would not.
//     No init db script does this today, but the report scripts under
//     `reports/` do, and they are read by the same kind of code.
//
// The cost is that a failure no longer names the statement. PostgreSQL reports
// a character position for the errors where that is knowable, so the position
// is turned back into a file, line and column.
func execScript(ctx context.Context, dbpool *pgxpool.Pool, sqlFile, script string) error {
	_, err := dbpool.Exec(ctx, script)
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Position > 0 {
		if line, col := sqlscript.Locate(script, int(pgErr.Position)); line > 0 {
			return fmt.Errorf("error while executing %s:%d:%d: %v", sqlFile, line, col, err)
		}
	}
	return fmt.Errorf("error while executing %s: %v", sqlFile, err)
}

func loadConfig(dbpool *pgxpool.Pool, baseDir, fileName string) error {
	sqlFile, err := utils.ConfineFilePath(baseDir, fileName)
	if err != nil {
		return err
	}
	log.Println("Initializing jetsapi db using", sqlFile)
	script, err := os.ReadFile(sqlFile)
	if err != nil {
		return fmt.Errorf("error while opening jetsapi init db file: %v", err)
	}
	return execScript(context.Background(), dbpool, sqlFile, string(script))
}

func InitializeBaseJetsapiDb(dbpool *pgxpool.Pool, jetsDbInitPath *string) error {
	// initialize jetsapi database -- base initialization only
	// jetsDbInitPath using base__workspace_init_db.sql
	if len(jetsDbInitScriptPath) > 0 {
		err := loadConfig(dbpool, filepath.Dir(jetsDbInitScriptPath), filepath.Base(jetsDbInitScriptPath))
		if err != nil {
			return err
		}
	}
	return loadConfig(dbpool, *jetsDbInitPath, "base__workspace_init_db.sql")
}

func InitializeJetsapiDb4Clients(dbpool *pgxpool.Pool, jetsDbInitPath *string, clients *string) error {
	// initialize jetsapi database for the clients
	if clients == nil {
		return fmt.Errorf("InitializeJetsapiDb4Clients: Invalid argument, clients cannot be nil")
	}
	clientList := strings.Split(*clients, ",")
	for i := range clientList {
		fileName := fmt.Sprintf("%s_workspace_init_db.sql", strings.ToLower(clientList[i]))
		err := loadConfig(dbpool, *jetsDbInitPath, fileName)
		if err != nil {
			return err
		}
	}
	return nil
}

// InitializeJetsapiDb initializes the jetsapi database using all client files
// in the directory, skipping base__workspace_init_db.sql.
//
// Each file is one transaction, not the whole run: a failure at the seventh of
// eighteen client scripts leaves the first six applied. That is deliberate. The
// scripts are written to be re-runnable — DELETE followed by INSERT ... ON
// CONFLICT DO NOTHING throughout — so the repair is to fix the script and run
// again, and one transaction spanning every client would hold locks on the
// whole of jetsapi for the length of a deployment.
func InitializeJetsapiDb(dbpool *pgxpool.Pool, jetsDbInitPath *string) error {
	fileSystem := os.DirFS(*jetsDbInitPath)
	err := fs.WalkDir(fileSystem, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("ERROR while walking workspace init db directory %q: %v", path, err)
			return err
		}
		if info.IsDir() || path == "base__workspace_init_db.sql" {
			return nil
		}
		return loadConfig(dbpool, *jetsDbInitPath, path)
	})
	if err != nil {
		return fmt.Errorf("error walking the workspace init db path %s: %v", *jetsDbInitPath, err)
	}
	return nil
}
