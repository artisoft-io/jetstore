package compute_pipes

// The domain_keys_registry query (actions_start_common.go) binds the class
// names it reads as one text array. Needs a scratch PostgreSQL named by
// JETS_TEST_DSN (see lookup_reader_login_test.go); skipped otherwise.

import (
	"context"
	"slices"
	"testing"
)

func TestDomainKeysRegistryQueryBindsClassNames(t *testing.T) {
	db := cplDB(t)
	ctx := context.Background()
	for _, s := range []string{
		"CREATE SCHEMA jetsapi",
		"CREATE TABLE jetsapi.domain_keys_registry (entity_rdf_type TEXT PRIMARY KEY, object_types TEXT[] NOT NULL, domain_keys_json TEXT)",
		`INSERT INTO jetsapi.domain_keys_registry VALUES
			('ex:Claim', '{"Claim"}', '{"Claim":"ex:claim_id"}'),
			('ex:Member', '{"Member"}', NULL),
			('o''Brien', '{"Odd"}', NULL)`,
	} {
		if _, err := db.Exec(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	// A quote in a class name is a value, not SQL: the old IN list, built by
	// concatenation, failed on the first and read every row for the second.
	rows, err := db.Query(ctx, domainKeysRegistryQuery, []string{"ex:Claim", "o'Brien", "x') OR ('1'='1"})
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name string
		var objTypes []string
		var dk *string
		if err := rows.Scan(&name, &objTypes, &dk); err != nil {
			t.Fatal(err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	if want := []string{"ex:Claim", "o'Brien"}; !slices.Equal(got, want) {
		t.Fatalf("classes read = %v, want %v", got, want)
	}
}
