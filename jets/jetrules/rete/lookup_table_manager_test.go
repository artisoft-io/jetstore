package rete

import (
	"log"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

type TestLookupTableManagerContext struct {
	resourceManager *rdf.ResourceManager
	metaGraph       *rdf.RdfGraph
	reteSession     *ReteSession
}

func NewTestLookupTableManagerContext() *TestLookupTableManagerContext {
	rm := rdf.NewResourceManager(nil)
	if rm == nil {
		log.Println("error: nil returned by NewResourceManager")
		return nil
	}
	metaGraph := rdf.NewRdfGraph("META")
	if metaGraph == nil {
		log.Println("error: nil returned by NewRdfGraph")
		return nil
	}
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		log.Println("error: nil returned by NewRdfSession")
		return nil
	}
	reteSession := NewReteSession(rdfSession)
	if reteSession == nil {
		log.Println("error: nil returned by NewReteSession")
		return nil
	}
	return &TestLookupTableManagerContext{
		resourceManager: rm,
		metaGraph:       metaGraph,
		reteSession:     reteSession,
	}
}

func TestLookupTable1(t *testing.T) {
	workspaceHome = "/home/michel/projects/repos/jetstore/jets/jetrules"
	wprefix = "test_ws"
	ctx := NewTestLookupTableManagerContext()
	jetRuleModel := &JetruleModel{
		LookupTables: []LookupTableNode{
			{
				Columns: []LookupTableColumn{
					{Name:"ZIP_CITY", Type: "text"},
					{Name:"ZIP_COUNTY", Type: "text"},
					{Name:"SC_ID", Type: "text"},
					{Name:"ZIP_STATE", Type: "text"},
					{Name:"ZIP", Type: "text"},
				},
				CsvFile: "lookups/common/zipcode.csv",
				Key: []string{"ZIP"},
				Name: "common_zipcode",
				Type: "lookup",
			},
		},
	}
	rm := ctx.reteSession.RdfSession.ResourceMgr
	rm.NewResource("ZIP_CITY")
	rm.NewResource("ZIP_COUNTY")
	rm.NewResource("SC_ID")
	rm.NewResource("ZIP_STATE")
	rm.NewResource("ZIP")
	lookupMgr, err := NewLookupTableManager(rm, ctx.metaGraph, jetRuleModel)
	if err != nil {
		t.Fatalf("while calling NewLookupTableManager: %v", err)
	}
	if lookupMgr == nil {
		t.Fatalf("error: NewLookupTableManager returned nil")
	}
	if len(lookupMgr.LookupTableMap) == 0 {
		t.Fatalf("error: LookupTableManager has nil or empty LookupTableMap")
	}
	lookupTable := lookupMgr.LookupTableMap["common_zipcode"]
	if lookupTable == nil {
		t.Fatalf("error: LookupTableManager has nil lookupTable for common_zipcode")
	}
	name := "common_zipcode"
	key := "07960"
	row, err := lookupTable.Lookup(ctx.reteSession, &name, &key)
	if err != nil || row == nil {
		t.Fatalf("error: LookupTable returned nil for key 07960, err: %v", err)
	}
	if row.String() != "jets:lookup:07960" {
		t.Errorf("row key not matching")
	}
	if !ctx.reteSession.RdfSession.Contains(row, rm.GetResource("ZIP"), rm.GetLiteral("07960")) {
		t.Errorf("triple not found")
	}
	log.Println("Got row", row.String())
	itor := ctx.reteSession.RdfSession.Find()
	log.Println("The graph contains:")
	for t3 := range itor.Itor {
		log.Println(t3)
	}
	itor.Done()
}
func TestLookupTable2(t *testing.T) {
	workspaceHome = "/home/michel/projects/repos/jetstore/jets/jetrules"
	wprefix = "test_ws"
	ctx := NewTestLookupTableManagerContext()
	jetRuleModel := &JetruleModel{
		LookupTables: []LookupTableNode{
			{
				Columns: []LookupTableColumn{
					{Name:"PRIORITY", Type: "int"},
					{Name:"TYPE", Type: "text"},
				},
				CsvFile: "lookups/IM/Exclusion_Type.csv",
				Key: []string{"TYPE"},
				Name: "IM_Exclusion_Type",
				Type: "lookup",
			},
		},
	}
	rm := ctx.reteSession.RdfSession.ResourceMgr
	rm.NewResource("PRIORITY")
	rm.NewResource("TYPE")
	lookupMgr, err := NewLookupTableManager(rm, ctx.metaGraph, jetRuleModel)
	if err != nil {
		t.Fatalf("while calling NewLookupTableManager: %v", err)
	}
	if lookupMgr == nil {
		t.Fatalf("error: NewLookupTableManager returned nil")
	}
	if len(lookupMgr.LookupTableMap) == 0 {
		t.Fatalf("error: LookupTableManager has nil or empty LookupTableMap")
	}
	lookupTable := lookupMgr.LookupTableMap["IM_Exclusion_Type"]
	if lookupTable == nil {
		t.Fatalf("error: LookupTableManager has nil lookupTable for IM_Exclusion_Type")
	}
	name := "IM_Exclusion_Type"
	key := "MODALITY"
	row, err := lookupTable.Lookup(ctx.reteSession, &name, &key)
	if err != nil || row == nil {
		t.Fatalf("error: LookupTable returned nil for key MODALITY, err: %v", err)
	}
	if row.String() != "jets:lookup:MODALITY" {
		t.Errorf("row key not matching")
	}
	if !ctx.reteSession.RdfSession.Contains(row, rm.GetResource("PRIORITY"), rm.GetLiteral(2)) {
		t.Errorf("triple not found")
	}
	t.Log("Got row", row.String())
	itor := ctx.reteSession.RdfSession.Find()
	log.Println("The graph contains:")
	for t3 := range itor.Itor {
		log.Println(t3)
	}
	itor.Done()
}

func TestLookupRand1(t *testing.T) {
	workspaceHome = "/home/michel/projects/repos/jetstore/jets/jetrules"
	wprefix = "test_ws"
	ctx := NewTestLookupTableManagerContext()
	jetRuleModel := &JetruleModel{
		LookupTables: []LookupTableNode{
			{
				Columns: []LookupTableColumn{
					{Name:"PRIORITY", Type: "int"},
					{Name:"TYPE", Type: "text"},
				},
				CsvFile: "lookups/IM/Exclusion_Type.csv",
				Key: []string{"TYPE"},
				Name: "IM_Exclusion_Type",
				Type: "lookup",
			},
		},
	}
	rm := ctx.reteSession.RdfSession.ResourceMgr
	rm.NewResource("PRIORITY")
	rm.NewResource("TYPE")
	lookupMgr, err := NewLookupTableManager(rm, ctx.metaGraph, jetRuleModel)
	if err != nil {
		t.Fatalf("while calling NewLookupTableManager: %v", err)
	}
	if lookupMgr == nil {
		t.Fatalf("error: NewLookupTableManager returned nil")
	}
	if len(lookupMgr.LookupTableMap) == 0 {
		t.Fatalf("error: LookupTableManager has nil or empty LookupTableMap")
	}
	lookupTable := lookupMgr.LookupTableMap["IM_Exclusion_Type"]
	if lookupTable == nil {
		t.Fatalf("error: LookupTableManager has nil lookupTable for IM_Exclusion_Type")
	}
	name := "IM_Exclusion_Type"
	row, err := lookupTable.LookupRand(ctx.reteSession, &name)
	if err != nil || row == nil {
		t.Fatalf("error: LookupTable returned nil for key MODALITY, err: %v", err)
	}
	if !strings.HasPrefix(row.String(), "jets:lookup:rand:") {
		t.Errorf("row key not matching")
	}
	if !ctx.reteSession.RdfSession.ContainsSP(row, rm.GetResource("PRIORITY")) {
		t.Errorf("triple not found")
	}
	t.Log("Got row", row.String())
	itor := ctx.reteSession.RdfSession.Find()
	log.Println("The graph contains:")
	for t3 := range itor.Itor {
		log.Println(t3)
	}
	itor.Done()
}

func TestLookupMultiRand1(t *testing.T) {
	workspaceHome = "/home/michel/projects/repos/jetstore/jets/jetrules"
	wprefix = "test_ws"
	ctx := NewTestLookupTableManagerContext()
	jetRuleModel := &JetruleModel{
		LookupTables: []LookupTableNode{
			{
				Columns: []LookupTableColumn{
					{Name:"PRIORITY", Type: "int"},
					{Name:"TYPE", Type: "text"},
				},
				CsvFile: "lookups/IM/Exclusion_Type.csv",
				Key: []string{"TYPE"},
				Name: "IM_Exclusion_Type",
				Type: "lookup",
			},
		},
	}
	rm := ctx.reteSession.RdfSession.ResourceMgr
	rm.NewResource("PRIORITY")
	rm.NewResource("TYPE")
	lookupMgr, err := NewLookupTableManager(rm, ctx.metaGraph, jetRuleModel)
	if err != nil {
		t.Fatalf("while calling NewLookupTableManager: %v", err)
	}
	if lookupMgr == nil {
		t.Fatalf("error: NewLookupTableManager returned nil")
	}
	if len(lookupMgr.LookupTableMap) == 0 {
		t.Fatalf("error: LookupTableManager has nil or empty LookupTableMap")
	}
	lookupTable := lookupMgr.LookupTableMap["IM_Exclusion_Type"]
	if lookupTable == nil {
		t.Fatalf("error: LookupTableManager has nil lookupTable for IM_Exclusion_Type")
	}
	name := "IM_Exclusion_Type"
	row, err := lookupTable.MultiLookupRand(ctx.reteSession, &name)
	if err != nil || row == nil {
		t.Fatalf("error: LookupTable returned nil for key MODALITY, err: %v", err)
	}
	if !strings.HasPrefix(row.String(), "jets:lookup:multi:rand:") {
		t.Errorf("row key not matching")
	}
	if !ctx.reteSession.RdfSession.Contains(rm.GetResource("jets:cache:IM_Exclusion_Type"), rm.GetResource("jets:lookup_multi_rows"), row) {
		t.Errorf("triple not found")
	}
	t.Log("Got row", row.String())
	itor := ctx.reteSession.RdfSession.Find()
	log.Println("The graph contains:")
	for t3 := range itor.Itor {
		log.Println(t3)
	}
	itor.Done()
}
