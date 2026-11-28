package compute_pipes

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/dblogin/dblogintest"
	"github.com/artisoft-io/jetstore/jets/dbtest"
)

// TestMain gives the package's database tests a database of their own on the
// server JETS_TEST_DSN names, and drops the restricted logins dblogin.Ensure
// makes for each database they made; see jets/dbtest.
func TestMain(m *testing.M) { dbtest.Main(m, dbtest.RolesOf(dblogintest.Logins)) }
