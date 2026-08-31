package workspace

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Object properties are graph structure and never table columns.
//
// The shape is taken from the real `cintel:Patient_Profile`, which is what
// made the rule visible: two `type: resource` object properties that were
// becoming columns nothing could ever populate, beside a `type: text` data
// property that receives the `toon` serialisation built by walking them.
func TestDomainColumnsOfDropsObjectProperties(t *testing.T) {
	tableInfo := &rete.TableNode{
		TableName: "cintel:Patient_Profile",
		ClassName: "cintel:Patient_Profile",
		Columns: []rete.TableColumnNode{
			{ColumnName: "cintel:Claim_Summary", Type: "text"},
			{ColumnName: "cintel:has_Medical_Events", Type: "resource", IsObject: true, AsArray: true},
			{ColumnName: "hc:Member_ID", Type: "text"},
			{ColumnName: "cintel:has_Pharmacy_Events", Type: "resource", IsObject: true, AsArray: true},
			{ColumnName: "jets:ruleTag", Type: "text", AsArray: true},
		},
	}
	got := make([]string, 0, len(tableInfo.Columns))
	for _, c := range DomainColumnsOf(tableInfo) {
		got = append(got, c.ColumnInfo.ColumnName)
	}
	want := []string{"cintel:Claim_Summary", "hc:Member_ID", "jets:ruleTag"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("column %d: got %q, want %q", i, got[i], want[i])
		}
	}
	// The two properties named in the failure, stated as the assertion rather
	// than left implicit in the count.
	for _, c := range DomainColumnsOf(tableInfo) {
		if c.ColumnInfo.IsObject {
			t.Errorf("%s is an object property and must not be a table column",
				c.ColumnInfo.ColumnName)
		}
	}
	// A multi-valued *data* property is unaffected: `jets:ruleTag` is AsArray and
	// stays, which is what keeps this a rule about object-ness rather than about
	// cardinality.
	if len(got) == 0 || got[len(got)-1] != "jets:ruleTag" {
		t.Errorf("a multi-valued data property must survive, got %v", got)
	}
}
