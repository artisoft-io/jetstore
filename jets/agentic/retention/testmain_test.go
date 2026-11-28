package retention

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/dbtest"
)

// TestMain gives the package's database tests a database of their own on the
// server JETS_TEST_DSN names; see jets/dbtest.
func TestMain(m *testing.M) { dbtest.Main(m) }
