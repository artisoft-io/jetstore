package rete

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

func SimpleInitializer() *BetaRowInitializer {
	return NewBetaRowInitializer(
		[]int{
			0 | brcTriple,
			1 | brcTriple,
			2 | brcTriple,
		},
		[]string{"s", "p", "o"})
}

// This file contains test cases for the BetaRow

func TestAlphaNode(t *testing.T) {
	metaMgr := rdf.NewResourceManager(nil)
	metaS := metaMgr.NewResource("metaS")
	metaP := metaMgr.NewResource("metaP")
	metaO := metaMgr.NewResource("metaO")

	// Configure the first row: the initializer and the node_vertex
	initializer1 := SimpleInitializer()
	nvertex0 := NewNodeVertex(0, nil, false, 100, nil, "root vertex 0", nil, initializer1)
	nvertex1 := NewNodeVertex(1, nil, false, 100, nil, "test node vertex 1", nil, initializer1)
	isAntecedent := true
	alphaNode1 := NewAlphaNode(&FBinded{pos: 0}, &FBinded{pos: 1}, &FVariable{variable: "?o"}, nvertex1, isAntecedent, "first node")

	// Create the session
	metaGraph := rdf.NewRdfGraph("META")
	b, err := metaGraph.Insert(metaS, metaP, metaO)
	if err != nil || !b {
		t.Error("error expecting inserting triple in meta graph")
	}
	rdfSession := rdf.NewRdfSession(metaMgr, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm := rdfSession.ResourceMgr
	reteSession := NewReteSession(rdfSession)
	parentBetaRelation := NewBetaRelation(nvertex0)
	alphaNode1.InitializeIndexes(parentBetaRelation)
	if len(parentBetaRelation.rowIndexes1) != 0 {
		t.Error("expecting having len(rowIndexes1) == 0")
	}
	if len(parentBetaRelation.rowIndexes2) != 1 {
		t.Error("expecting having len(rowIndexes2) == 1")
	}
	if len(parentBetaRelation.rowIndexes3) != 0 {
		t.Error("expecting having len(rowIndexes3) == 0")
	}
	// add row1 to AlphaNode 1
	br := NewBetaRow(nvertex1, len(initializer1.InitData))
	t3 := rdf.T3(rm.NewResource("s1"), rm.NewResource("p1"), rm.NewResource("metaO"))
	if t3[0] == nil || t3[1] == nil || t3[2] == nil {
		t.Fatal("error while creating resources")
	}
	err = br.Initialize(initializer1, nil, &t3)
	if err != nil {
		t.Error("while initializing beta row 1:", err)
	}
	alphaNode1.AddIndex4BetaRow(parentBetaRelation, br)
	rows := alphaNode1.FindMatchingRows(parentBetaRelation, t3[0], t3[1], metaO)
	if len(rows) != 1 {
		t.Error("expecting to have 1 row indexed")
	}
	for row, valid := range rows {
		if !valid {
			continue
		}
		if row.Data[0] != t3[0] || row.Data[1] != t3[1] || row.Data[2] != t3[2] {
			t.Error("did not get the expected row")
		}
	}
	// add row2 to AlphaNode 1 with same s, p
	br = NewBetaRow(nvertex1, len(initializer1.InitData))
	t3 = rdf.T3(rm.NewResource("s1"), rm.NewResource("p1"), rm.NewResource("o1"))
	if t3[0] == nil || t3[1] == nil || t3[2] == nil {
		t.Fatal("error while creating resources")
	}
	err = br.Initialize(initializer1, nil, &t3)
	if err != nil {
		t.Error("while initializing beta row 2:", err)
	}
	alphaNode1.AddIndex4BetaRow(parentBetaRelation, br)
	rows = alphaNode1.FindMatchingRows(parentBetaRelation, t3[0], t3[1], rm.NewResource("o2"))
	if len(rows) != 2 {
		t.Error("expecting to have 2 row indexed")
	}
	for row, valid := range rows {
		if !valid {
			continue
		}
		if row.Data[0] != t3[0] || row.Data[1] != t3[1] {
			t.Error("did not get the expected row")
		}
	}
	// add row3 to AlphaNode 1 with same s, different p
	br = NewBetaRow(nvertex1, len(initializer1.InitData))
	t3 = rdf.T3(rm.NewResource("s1"), rm.NewResource("p2"), rm.NewResource("o1"))
	if t3[0] == nil || t3[1] == nil || t3[2] == nil {
		t.Fatal("error while creating resources")
	}
	err = br.Initialize(initializer1, nil, &t3)
	if err != nil {
		t.Error("while initializing beta row 3:", err)
	}
	alphaNode1.AddIndex4BetaRow(parentBetaRelation, br)
	rows = alphaNode1.FindMatchingRows(parentBetaRelation, t3[0], t3[1], rm.NewResource("o3"))
	if len(rows) != 1 {
		t.Error("expecting to have 1 row indexed")
	}
	for row, valid := range rows {
		if !valid {
			continue
		}
		if row.Data[0] != t3[0] || row.Data[1] != t3[1] {
			t.Error("did not get the expected row")
		}
	}
	// add alphaNode2 as antecedent to parentBetaRelation
	nvertex2 := NewNodeVertex(2, nil, false, 100, nil, "test node vertex 0", nil, initializer1)
	isAntecedent = true
	alphaNode2 := NewAlphaNode(&FConstant{node: rm.NewResource("s1")}, &FBinded{pos: 1}, &FBinded{pos: 2}, nvertex2, isAntecedent, "second node")
	alphaNode2.InitializeIndexes(parentBetaRelation)
	if len(parentBetaRelation.rowIndexes2) != 2 {
		t.Error("expecting having len(rowIndexes2) == 2")
	}
	// add row1 to AlphaNode 2
	br = NewBetaRow(nvertex2, len(initializer1.InitData))
	t3 = rdf.T3(rm.NewResource("s1"), rm.NewResource("p21"), rm.NewResource("o21"))
	if t3[0] == nil || t3[1] == nil || t3[2] == nil {
		t.Fatal("error while creating resources")
	}
	err = br.Initialize(initializer1, nil, &t3)
	if err != nil {
		t.Error("while initializing beta row 1:", err)
	}
	alphaNode2.AddIndex4BetaRow(parentBetaRelation, br)
	rows = alphaNode2.FindMatchingRows(parentBetaRelation, rm.NewResource("xx"), rm.NewResource("p21"), rm.NewResource("o21"))
	if len(rows) != 1 {
		t.Error("expecting to have 1 row indexed")
	}
	for row, valid := range rows {
		if !valid {
			continue
		}
		if row.Data[1] != t3[1] || row.Data[2] != t3[2] {
			t.Error("did not get the expected row")
		}
	}
	// add row2 to AlphaNode 2 with same p, o
	br = NewBetaRow(nvertex2, len(initializer1.InitData))
	t3 = rdf.T3(rm.NewResource("yy"), rm.NewResource("p21"), rm.NewResource("o21"))
	if t3[0] == nil || t3[1] == nil || t3[2] == nil {
		t.Fatal("error while creating resources")
	}
	err = br.Initialize(initializer1, nil, &t3)
	if err != nil {
		t.Error("while initializing beta row 2:", err)
	}
	alphaNode2.AddIndex4BetaRow(parentBetaRelation, br)
	rows = alphaNode2.FindMatchingRows(parentBetaRelation, rm.NewResource("zz"), rm.NewResource("p21"), rm.NewResource("o21"))
	if len(rows) != 2 {
		t.Error("expecting to have 2 row indexed")
	}
	for row, valid := range rows {
		if !valid {
			continue
		}
		if row.Data[1] != t3[1] || row.Data[2] != t3[2] {
			t.Error("did not get the expected row")
		}
	}
	// Check that indexes for alphaNode1 is unaffected
	rows = alphaNode1.FindMatchingRows(parentBetaRelation, rm.NewResource("s1"), rm.NewResource("p1"), rm.NewResource("xx"))
	if len(rows) != 2 {
		t.Error("expecting to have 2 row indexed")
	}
	for row, valid := range rows {
		if !valid {
			continue
		}
		if row.Data[0] != rm.NewResource("s1") || row.Data[1] != rm.NewResource("p1") {
			t.Error("did not get the expected row")
		}
	}

	// add alphaNode3 as antecedent to parentBetaRelation
	nvertex3 := NewNodeVertex(3, nil, false, 100, nil, "test node vertex 3", nil, initializer1)
	isAntecedent = true
	alphaNode3 := NewAlphaNode(&FVariable{variable: "?s"}, &FBinded{pos: 1}, &FConstant{node: rm.NewResource("o30")}, nvertex3, isAntecedent, "third node")
	alphaNode3.InitializeIndexes(parentBetaRelation)
	if len(parentBetaRelation.rowIndexes1) != 1 {
		t.Error("expecting having len(rowIndexes1) == 1")
	}
	// add row1 to AlphaNode 3
	br = NewBetaRow(nvertex3, len(initializer1.InitData))
	t3 = rdf.T3(rm.NewResource("xx"), rm.NewResource("p31"), rm.NewResource("o30"))
	if t3[0] == nil || t3[1] == nil || t3[2] == nil {
		t.Fatal("error while creating resources")
	}
	err = br.Initialize(initializer1, nil, &t3)
	if err != nil {
		t.Error("while initializing beta row 1 on node 3:", err)
	}
	alphaNode3.AddIndex4BetaRow(parentBetaRelation, br)
	rows = alphaNode3.FindMatchingRows(parentBetaRelation, rm.NewResource("zz"), rm.NewResource("p31"), rm.NewResource("o30"))
	if len(rows) != 1 {
		t.Error("expecting to have 1 row indexed")
	}
	for row, valid := range rows {
		if !valid {
			continue
		}
		if row.Data[1] != t3[1] || row.Data[2] != t3[2] {
			t.Error("did not get the expected row")
		}
	}

	// add alphaNode4 as consequent to parentBetaRelation
	nvertex4 := NewNodeVertex(4, nil, false, 100, nil, "test node vertex 4", nil, initializer1)
	isAntecedent = false
	currentLen := len(parentBetaRelation.rowIndexes2)
	alphaNode4 := NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: rm.NewResource("p40")}, &FBinded{pos: 2}, nvertex4, isAntecedent, "third node")
	alphaNode4.InitializeIndexes(parentBetaRelation)
	if len(parentBetaRelation.rowIndexes2) != currentLen {
		t.Error("NOT expecting having len(rowIndexes2) changed")
	}
	// add row1 to AlphaNode 4
	br = NewBetaRow(nvertex4, len(initializer1.InitData))
	t3 = rdf.T3(rm.NewResource("s41"), rm.NewResource("xx"), rm.NewResource("o41"))
	err = br.Initialize(initializer1, nil, &t3)
	if err != nil {
		t.Error("while initializing beta row 1 on node 4:", err)
	}
	triple := alphaNode4.ComputeConsequentTriple(reteSession, br)
	if triple == nil {
		t.Fatal("expecting to have a triple back")
	}
	t3 = *triple
	if t3[0] != rm.NewResource("s41") || t3[1] != rm.NewResource("p40") || t3[2] != rm.NewResource("o41") {
		t.Error("did not get the expected row")
	}

	// add alphaNode5 as antecedent to parentBetaRelation
	nvertex5 := NewNodeVertex(5, nil, false, 100, nil, "test node vertex 5", nil, initializer1)
	isAntecedent = true
	alphaNode5 := NewAlphaNode(&FBinded{pos: 0}, &FBinded{pos: 1}, &FBinded{pos: 2}, nvertex5, isAntecedent, "fifth node")
	alphaNode5.InitializeIndexes(parentBetaRelation)
	if len(parentBetaRelation.rowIndexes3) != 1 {
		t.Error("expecting having len(rowIndexes3) == 1")
	}
	// add row1 to AlphaNode 5
	br = NewBetaRow(nvertex5, len(initializer1.InitData))
	t3 = rdf.T3(rm.NewResource("s51"), rm.NewResource("p51"), rm.NewResource("o51"))
	err = br.Initialize(initializer1, nil, &t3)
	if err != nil {
		t.Error("while initializing beta row 1 on node 5:", err)
	}
	alphaNode5.AddIndex4BetaRow(parentBetaRelation, br)
	rows = alphaNode5.FindMatchingRows(parentBetaRelation, rm.NewResource("s51"), rm.NewResource("yy"), rm.NewResource("o51"))
	if len(rows) != 0 {
		t.Error("expecting to have 0 row matching index")
	}
	rows = alphaNode5.FindMatchingRows(parentBetaRelation, rm.NewResource("s51"), rm.NewResource("p51"), rm.NewResource("o51"))
	if len(rows) != 1 {
		t.Error("expecting to have 1 row indexed")
	}
	for row, valid := range rows {
		if !valid {
			continue
		}
		if row.Data[0] != t3[0] || row.Data[1] != t3[1] || row.Data[2] != t3[2] {
			t.Error("did not get the expected row")
		}
	}
}

func TestAlphaNodePanic1(t *testing.T) {
	metaMgr := rdf.NewResourceManager(nil)
	initializer1 := SimpleInitializer()
	nvertex0 := NewNodeVertex(0, nil, false, 100, nil, "root vertex 0", nil, initializer1)
	nvertex1 := NewNodeVertex(1, nil, false, 100, nil, "test node vertex 1", nil, initializer1)
	isAntecedent := false
	alphaNode1 := NewAlphaNode(&FBinded{pos: 0}, &FBinded{pos: 1}, &FConstant{node: metaMgr.GetLiteral("some string")}, nvertex1, isAntecedent, "first node")

	// Create the session
	metaGraph := rdf.NewRdfGraph("META")
	rdfSession := rdf.NewRdfSession(metaMgr, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm := rdfSession.ResourceMgr
	parentBetaRelation := NewBetaRelation(nvertex0)
	alphaNode1.InitializeIndexes(parentBetaRelation)
	if len(parentBetaRelation.rowIndexes1) != 0 {
		t.Error("expecting having len(rowIndexes1) == 0")
	}
	if len(parentBetaRelation.rowIndexes2) != 0 {
		t.Error("expecting having len(rowIndexes2) == 0")
	}
	if len(parentBetaRelation.rowIndexes3) != 0 {
		t.Error("expecting having len(rowIndexes3) == 0")
	}
	// add row1 to AlphaNode 1
	br := NewBetaRow(nvertex1, len(initializer1.InitData))
	t3 := rdf.T3(rm.NewResource("s1"), rm.NewResource("p1"), rm.NewResource("o1"))
	if t3[0] == nil || t3[1] == nil || t3[2] == nil {
		t.Fatal("error while creating resources")
	}
	err := br.Initialize(initializer1, nil, &t3)
	if err != nil {
		t.Error("while initializing beta row 1:", err)
	}
	alphaNode1.AddIndex4BetaRow(parentBetaRelation, br)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("FindMatchingRows did not panic")
		}
	}()
	// This should panic
	alphaNode1.FindMatchingRows(parentBetaRelation, t3[0], t3[1], t3[2])
}

func TestAlphaNodePanic2(t *testing.T) {
	metaMgr := rdf.NewResourceManager(nil)
	initializer1 := SimpleInitializer()
	nvertex0 := NewNodeVertex(0, nil, false, 100, nil, "root vertex 0", nil, initializer1)
	nvertex1 := NewNodeVertex(1, nil, false, 100, nil, "test node vertex 1", nil, initializer1)
	isAntecedent := true
	alphaNode1 := NewAlphaNode(&FBinded{pos: 0}, &FBinded{pos: 1}, &FConstant{node: metaMgr.GetLiteral("some string")}, nvertex1, isAntecedent, "first node")

	// Create the session
	metaGraph := rdf.NewRdfGraph("META")
	rdfSession := rdf.NewRdfSession(metaMgr, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm := rdfSession.ResourceMgr
	reteSession := NewReteSession(rdfSession)
	parentBetaRelation := NewBetaRelation(nvertex0)
	alphaNode1.InitializeIndexes(parentBetaRelation)
	if len(parentBetaRelation.rowIndexes1) != 0 {
		t.Error("expecting having len(rowIndexes1) == 0")
	}
	if len(parentBetaRelation.rowIndexes2) != 1 {
		t.Error("expecting having len(rowIndexes2) == 1")
	}
	if len(parentBetaRelation.rowIndexes3) != 0 {
		t.Error("expecting having len(rowIndexes3) == 0")
	}
	// add row1 to AlphaNode 1
	br := NewBetaRow(nvertex1, len(initializer1.InitData))
	t3 := rdf.T3(rm.NewResource("s1"), rm.NewResource("p1"), rm.NewResource("o1"))
	if t3[0] == nil || t3[1] == nil || t3[2] == nil {
		t.Fatal("error while creating resources")
	}
	err := br.Initialize(initializer1, nil, &t3)
	if err != nil {
		t.Error("while initializing beta row 1:", err)
	}
	alphaNode1.AddIndex4BetaRow(parentBetaRelation, br)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("ComputeConsequentTriple did not panic")
		}
	}()
	// This should panic
	alphaNode1.ComputeConsequentTriple(reteSession, br)
}
