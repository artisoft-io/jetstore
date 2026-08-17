package workspace

import (
	"reflect"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// The shape the defect had: nh:NimbleBase is declared once, in
// data_model/nh_base_model.jr, and imported by two main rule files. Each
// compiles it separately and each sees one of its two subclasses.
func nimbleBaseFrom(subClass string) *rete.ClassNode {
	return &rete.ClassNode{
		Type:           "class",
		Name:           "nh:NimbleBase",
		BaseClasses:    []string{"owl:Thing"},
		SubClasses:     []string{subClass},
		SourceFileName: "data_model/nh_base_model.jr",
		DataProperties: []rete.PropertyNode{
			{Name: "nh:Member_Id", ClassName: "nh:NimbleBase", Type: "text"},
		},
	}
}

func foldClasses(classes []*rete.ClassNode) map[string]*rete.ClassNode {
	out := make(map[string]*rete.ClassNode)
	for _, cls := range classes {
		if existing, ok := out[cls.Name]; ok {
			mergeClassNode(existing, cls)
		} else {
			out[cls.Name] = cloneClassNode(cls)
		}
	}
	return out
}

func TestMergeUnionsSubClassesAndDoesNotDependOnOrder(t *testing.T) {
	fromEligibility := nimbleBaseFrom("nh:Eligibility")
	fromAuthorization := nimbleBaseFrom("nh:AuthorizationBase")

	want := []string{"nh:AuthorizationBase", "nh:Eligibility"}
	for _, order := range [][]*rete.ClassNode{
		{fromEligibility, fromAuthorization},
		{fromAuthorization, fromEligibility},
	} {
		got := foldClasses(order)["nh:NimbleBase"]
		if !reflect.DeepEqual(got.SubClasses, want) {
			t.Errorf("sub_classes = %v, want %v (order %s first)",
				got.SubClasses, want, order[0].SubClasses[0])
		}
	}
}

// A single node still comes out sorted, so the view does not depend on the
// order the declarations happened to be visited in inside one unit either.
func TestSingleNodeIsSortedAndCopied(t *testing.T) {
	src := nimbleBaseFrom("nh:Eligibility")
	src.SubClasses = []string{"nh:Zebra", "nh:Alpha"}
	got := foldClasses([]*rete.ClassNode{src})["nh:NimbleBase"]
	if !reflect.DeepEqual(got.SubClasses, []string{"nh:Alpha", "nh:Zebra"}) {
		t.Errorf("sub_classes = %v, want them sorted", got.SubClasses)
	}
	if !reflect.DeepEqual(src.SubClasses, []string{"nh:Zebra", "nh:Alpha"}) {
		t.Errorf("the per-file model was mutated: %v", src.SubClasses)
	}
}

// The merge must not write through into the per-main-file models, which have
// already been written to <main_rule_file>.model.json and are still held by the
// caller.
func TestMergeDoesNotMutateTheSourceNodes(t *testing.T) {
	a := nimbleBaseFrom("nh:Eligibility")
	b := nimbleBaseFrom("nh:AuthorizationBase")
	b.DataProperties = append(b.DataProperties,
		rete.PropertyNode{Name: "nh:Upsert_Action", ClassName: "nh:NimbleBase", Type: "text"})

	merged := foldClasses([]*rete.ClassNode{a, b})["nh:NimbleBase"]

	if len(a.SubClasses) != 1 || a.SubClasses[0] != "nh:Eligibility" {
		t.Errorf("first node's sub_classes mutated: %v", a.SubClasses)
	}
	if len(a.DataProperties) != 1 {
		t.Errorf("first node's properties mutated: %v", a.DataProperties)
	}
	if len(merged.DataProperties) != 2 {
		t.Errorf("merged properties = %v, want the union of both nodes", merged.DataProperties)
	}
}

// Fields that come from the declaration are the same in every unit; the merge
// keeps them rather than dropping one node's answer for the other's.
func TestMergeKeepsDeclarationFields(t *testing.T) {
	a := nimbleBaseFrom("nh:Eligibility")
	a.AsTable = false
	a.SourceFileName = ""
	b := nimbleBaseFrom("nh:AuthorizationBase")
	b.AsTable = true
	b.BaseClasses = []string{"owl:Thing", "jets:Entity"}

	merged := foldClasses([]*rete.ClassNode{a, b})["nh:NimbleBase"]
	if !merged.AsTable {
		t.Error("as_table was lost")
	}
	if merged.SourceFileName != "data_model/nh_base_model.jr" {
		t.Errorf("source_file_name = %q, want the node that had one", merged.SourceFileName)
	}
	if !reflect.DeepEqual(merged.BaseClasses, []string{"owl:Thing", "jets:Entity"}) {
		t.Errorf("base_classes = %v, want declaration order with the missing one appended",
			merged.BaseClasses)
	}
}
