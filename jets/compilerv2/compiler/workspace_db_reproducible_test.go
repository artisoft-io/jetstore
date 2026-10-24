package compiler

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The same sources compile to the same workspace.db, byte for byte (Q-49).
// rule_properties, jetstore_config, the rule label's extra properties and the
// workspace_control rows were written in map order, so two compiles differed.
// Twelve rule properties, a config with three keys and eight imported files,
// each with a class, make that difference all but certain over a few compiles at the base, where
// this test fails.
func TestWorkspaceDBIsReproducible(t *testing.T) {
	saved := workspaceSchema
	workspaceSchema = filepath.Join("..", "..", "workspace_schema.sql")
	t.Cleanup(func() { workspaceSchema = saved })

	var props []string
	for i := 1; i <= 12; i++ {
		props = append(props, fmt.Sprintf("p%02d=\"v%d\"", i, i))
	}
	var imports strings.Builder
	dir := t.TempDir()
	for i := 1; i <= 8; i++ {
		name := fmt.Sprintf("part%d.jr", i)
		// a class, since a file is recorded in workspace_control only when
		// something saved names it (a class, a lookup table, a rule sequence)
		write(t, dir, name, fmt.Sprintf("class ex:C%d {\n  $base_classes = [owl:Thing],\n  $data_properties = [ex:p%d as text]\n};\n", i, i))
		fmt.Fprintf(&imports, "import \"%s\"\n", name)
	}
	source := imports.String() + `
main {
  $max_rule_exec = 5000,
  $max_looping = 5,
  $input_types = [ex:Item]
};
resource rdfType = "rdf:type";
resource ex:Item = "ex:Item";
resource ex:out = "ex:out";
[R1, s=10, ` + strings.Join(props, ", ") + `]: (?x rdfType ex:Item) -> (?x ex:out 1);
[R2, ` + strings.Join(props, ", ") + `]: (?x ex:out 1) -> (?x ex:out 2);
`
	write(t, dir, "main.jr", source)
	dbPath := filepath.Join(dir, "workspace.db")

	var first []byte
	for i := 0; i < 8; i++ {
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		c := NewCompiler(dir, "main.jr", true, false, true)
		if err := c.Compile(); err != nil {
			t.Fatalf("compile %d: %v\n%s", i+1, err, c.ErrorLog().String())
		}
		got, err := os.ReadFile(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = got
			continue
		}
		if !bytes.Equal(first, got) {
			t.Fatalf("compile %d wrote a workspace.db that differs from the first (%d and %d bytes)", i+1, len(first), len(got))
		}
	}
}
