package compute_pipes_test

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/rulestest"
)

// A pipeline and a rules unit test refuse the retired Go engine with the same words.
// Each package holds its own copy, so that compute_pipes does not import rulestest.
func TestGoEngineRetiredIsOneMessage(t *testing.T) {
	if compute_pipes.GoEngineRetired != rulestest.GoEngineRetired {
		t.Errorf("compute_pipes says %q, rulestest %q", compute_pipes.GoEngineRetired, rulestest.GoEngineRetired)
	}
}
