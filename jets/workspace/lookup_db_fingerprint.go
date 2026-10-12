package workspace

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Skipping the lookup.db rebuild when nothing it is built from has changed.
//
// **Packaging lookup.db is most of a workspace compile**: measured 2026-10-11,
// about 2.1 s of jets_ws's 2.2 s and 1.6 s of usi_ws's 5.6 s. Since a compile
// and a rules unit test run exclude each other, that is time a user waits.
//
// **What decides lookup.db's bytes, and therefore what the fingerprint covers:**
//
//  1. the lookup table definitions the compiler collected from the rule
//     files, deduplicated by name with the first one kept, in that order —
//     each hashed as its whole JSON encoding, so a field added to
//     rete.LookupTableNode participates without anybody remembering to add it;
//  2. the bytes of every CSV file those definitions name;
//  3. the packager's code — this file, lookup_tables.go, and what they call
//     (jets/csv, rdf.ParseBool). **Code cannot hash itself, so this one is
//     a convention: bump lookupDbFormatVersion whenever a change to that code
//     would change lookup.db for the same inputs**;
//  4. the SQLite library version, which is written into the file header, so a
//     driver upgrade rebuilds instead of leaving a file a rebuild would not
//     reproduce.
//
// **File modification times are deliberately not used**: a git checkout, a
// Docker COPY and the database sync all rewrite them.
//
// The fingerprint is stored inside lookup.db itself, in lookupDbMetaTable,
// written last. Anything other than a readable, complete file carrying the
// same fingerprint rebuilds — so a fresh container, which has no lookup.db,
// always builds.

// lookupDbFormatVersion versions the packager's output. Bump it when a change
// to the packager would produce a different lookup.db from the same inputs.
// It is a var only so a test can bump it.
//
// 2: rdf.ParseBool reads "yes" as true, so a YES in a bool column packages as 1.
// 3: and "y", so a Y does too.
// 4: an empty bool cell packages as NULL, served as rdf null, not 0; a value
// outside the accepted spellings stops the compile (I-45).
var lookupDbFormatVersion = 4

// lookupDbMetaTable holds the fingerprint. The double underscores keep it out
// of the namespace of lookup names, which are rule-file resources.
const lookupDbMetaTable = "__jets_lookup_db_meta__"

// uniqueLookupTables keeps the first definition of each lookup name, in order.
// A lookup imported by several main rule files is collected once per file, and
// the packager has always used the first.
func uniqueLookupTables(lookupTables []*rete.LookupTableNode) []*rete.LookupTableNode {
	seen := make(map[string]bool, len(lookupTables))
	unique := make([]*rete.LookupTableNode, 0, len(lookupTables))
	for _, tbl := range lookupTables {
		if seen[tbl.Name] {
			continue
		}
		seen[tbl.Name] = true
		unique = append(unique, tbl)
	}
	return unique
}

// sqliteLibraryVersion is the version of the SQLite the packager links.
func sqliteLibraryVersion() (string, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return "", err
	}
	defer db.Close()
	var v string
	err = db.QueryRow("SELECT sqlite_version()").Scan(&v)
	return v, err
}

// lookupDbFingerprint hashes everything lookup.db is built from (see above).
// tables must already be deduplicated; wsDir is the workspace root the CSV
// paths are relative to. Every component is length-prefixed or a fixed-size
// digest, so no two different inputs frame to the same byte stream.
func lookupDbFingerprint(tables []*rete.LookupTableNode, wsDir, sqliteVersion string) (string, error) {
	h := sha256.New()
	writeFramed(h, []byte("jetstore lookup.db"))
	writeFramed(h, []byte(strconv.Itoa(lookupDbFormatVersion)))
	writeFramed(h, []byte(sqliteVersion))
	for _, tbl := range tables {
		def, err := json.Marshal(tbl)
		if err != nil {
			return "", fmt.Errorf("while encoding lookup table %s: %w", tbl.Name, err)
		}
		writeFramed(h, def)
		if len(tbl.CsvFile) == 0 {
			// The packager rejects this; nothing to hash.
			continue
		}
		digest, err := fileSha256(filepath.Join(wsDir, tbl.CsvFile))
		if err != nil {
			return "", fmt.Errorf("while hashing csv file of lookup table %s: %w", tbl.Name, err)
		}
		h.Write(digest)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeFramed(h hash.Hash, b []byte) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(b)))
	h.Write(n[:])
	h.Write(b)
}

func fileSha256(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// storedLookupDbFingerprint reads the fingerprint from an existing lookup.db.
// It returns fs.ErrNotExist when there is no file, and an error when the file
// cannot be read or carries no fingerprint.
func storedLookupDbFingerprint(dbPath string) (fingerprint string, formatVersion string, err error) {
	if _, err := os.Stat(dbPath); err != nil {
		return "", "", err
	}
	// mode=ro: a check must not create or modify the file it is checking.
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return "", "", err
	}
	defer db.Close()
	// A file shorter than its header says is refused by SQLite itself
	// ("database disk image is malformed"), so a truncated copy fails here.
	rows, err := db.Query(fmt.Sprintf("SELECT name, value FROM %q", lookupDbMetaTable))
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return "", "", errors.New("no input fingerprint, it was built before one was recorded")
		}
		return "", "", err
	}
	defer rows.Close()
	for rows.Next() {
		var name, value string
		if err = rows.Scan(&name, &value); err != nil {
			return "", "", err
		}
		switch name {
		case "inputs_sha256":
			fingerprint = value
		case "format_version":
			formatVersion = value
		}
	}
	if err = rows.Err(); err != nil {
		return "", "", err
	}
	if fingerprint == "" {
		return "", "", errors.New("no input fingerprint")
	}
	return fingerprint, formatVersion, nil
}

// writeLookupDbFingerprint records the fingerprint. Nothing time-dependent is
// stored: a skipped lookup.db must be byte for byte what a rebuild produces.
func writeLookupDbFingerprint(db *sql.DB, fingerprint, sqliteVersion string) error {
	stmt := fmt.Sprintf(`CREATE TABLE %q (name TEXT PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO %q (name, value) VALUES ('inputs_sha256', '%s'), ('format_version', '%d'), ('sqlite_version', '%s');`,
		lookupDbMetaTable, lookupDbMetaTable, fingerprint, lookupDbFormatVersion, sqliteVersion)
	_, err := db.Exec(stmt)
	return err
}

// LookupDbResult says what EnsureLookupDb did and why, for the compile log.
type LookupDbResult struct {
	Rebuilt     bool
	Reason      string
	Fingerprint string
	Elapsed     time.Duration
}

func (r LookupDbResult) String() string {
	short := r.Fingerprint
	if len(short) > 12 {
		short = short[:12]
	}
	if r.Rebuilt {
		return fmt.Sprintf("Lookup tables packaged into lookup.db in %v (%s; inputs sha256 %s)\n",
			r.Elapsed.Round(time.Millisecond), r.Reason, short)
	}
	return fmt.Sprintf("lookup.db is up to date, packaging skipped in %v (inputs sha256 %s unchanged)\n",
		r.Elapsed.Round(time.Millisecond), short)
}

// EnsureLookupDb makes the workspace's lookup.db current: it rebuilds it from
// lookupTables unless the existing file carries the fingerprint of exactly the
// inputs it would be rebuilt from.
func EnsureLookupDb(lookupTables []*rete.LookupTableNode) (LookupDbResult, error) {
	wsDir := fmt.Sprintf("%s/%s", workspaceHome, wprefix)
	return ensureLookupDb(lookupTables, wsDir, filepath.Join(wsDir, "lookup.db"))
}

func ensureLookupDb(lookupTables []*rete.LookupTableNode, wsDir, dbPath string) (LookupDbResult, error) {
	start := time.Now()
	tables := uniqueLookupTables(lookupTables)
	var res LookupDbResult
	sqliteVersion, err := sqliteLibraryVersion()
	if err != nil {
		return res, fmt.Errorf("while reading the sqlite library version: %w", err)
	}
	fingerprint, err := lookupDbFingerprint(tables, wsDir, sqliteVersion)
	if err != nil {
		// Let the packager report the missing or unreadable file in its own words.
		res.Reason = fmt.Sprintf("could not fingerprint the inputs: %v", err)
	} else {
		res.Fingerprint = fingerprint
		stored, storedVersion, err := storedLookupDbFingerprint(dbPath)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			res.Reason = "no lookup.db"
		case err != nil:
			res.Reason = fmt.Sprintf("existing lookup.db not reusable: %v", err)
		case storedVersion != strconv.Itoa(lookupDbFormatVersion):
			res.Reason = fmt.Sprintf("packager format version changed from %s to %d", storedVersion, lookupDbFormatVersion)
		case stored != fingerprint:
			res.Reason = "lookup definitions, csv files or sqlite version changed"
		default:
			res.Elapsed = time.Since(start)
			return res, nil
		}
	}
	res.Rebuilt = true
	if err = packageLookupTables(tables, wsDir, dbPath, fingerprint, sqliteVersion); err != nil {
		return res, err
	}
	res.Elapsed = time.Since(start)
	return res, nil
}
