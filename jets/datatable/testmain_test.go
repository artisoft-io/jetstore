package datatable

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/dblogin"
	"github.com/artisoft-io/jetstore/jets/dblogin/dblogintest"
	"github.com/artisoft-io/jetstore/jets/dbtest"
)

// TestMain gives the package's database tests a database of their own on the
// server JETS_TEST_DSN names, and drops the restricted logins dblogin.Ensure
// makes for each database they made; see jets/dbtest. The restricted group
// roles are marked shared when the package ends, since several of its tests
// run EnsureRestrictedLogins on a database of their own rather than through
// dblogintest.Apiserver, which marks them: run alone, such a test would leave
// them on the server.
func TestMain(m *testing.M) {
	dbtest.Main(m, dbtest.RolesOf(dblogintest.Logins), dbtest.Shared(dblogin.Reader, dblogin.IDE))
}
