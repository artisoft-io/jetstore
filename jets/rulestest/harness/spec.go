// Package harness runs one rules unit test: it builds the engine, asserts the
// input sessions, runs the rules through the pipeline's own loop, and evaluates
// the checks. It may import compute_pipes; the test definition and the result
// file it works from are in jets/rulestest, which must not (see that package).
package harness

import (
	"encoding/json"
	"fmt"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/rulestest"
)

// JetrulesSpecOf converts a test's jetrules_config into the JetrulesSpec the
// pipeline code takes. The two share their JSON names by construction, so the
// conversion goes through JSON: a field JetrulesConfig declares cannot be
// dropped on the way.
func JetrulesSpecOf(c *rulestest.JetrulesConfig) (*compute_pipes.JetrulesSpec, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("while converting jetrules_config: %w", err)
	}
	var spec compute_pipes.JetrulesSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		return nil, fmt.Errorf("while converting jetrules_config: %w", err)
	}
	return &spec, nil
}
