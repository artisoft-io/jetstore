package compute_pipes

import (
	"testing"
)

// This file contains test cases for Analyze operator
func TestCombineColumnNameToken01(t *testing.T) {
	env := map[string]any{
		"${COLUMN_NAME_TOKEN_JSON}": `{"name":"classification","lookup":[{"name":"id","column_names":["fname","lname"],"column_name_fragments":["name_"]}]}`,
	}
	configToken := &ColumnNameTokenNode{
		Name: "classification",
		Lookup: []*ColumnNameLookupNode{
			{Name: "id", ColumnNames: []string{"myId"}},
		},
	}
	result, err := combineColumnNameToken(configToken, env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Lookup == nil {
		t.Fatal("result.Lookup should not be nil")
	}
	if len(result.Lookup) != 2 {
		t.Fatalf("expecting 2 lookup entries, got %d", len(result.Lookup))
	}
	if result.Lookup[0].Name != "id" || result.Lookup[0].ColumnNames[0] != "myId" {
		t.Error("first lookup entry is not correct")
	}
	if result.Lookup[1].Name != "id" || result.Lookup[1].ColumnNames[0] != "fname" || result.Lookup[1].ColumnNames[1] != "lname" {
		t.Error("second lookup entry is not correct")
	}

	colName2Token, colFragment2Token := prepareColName2TokenMap(result)
	if len(colName2Token) != 3 {
		t.Error("expecting 3 entries in colName2Token, got ", len(colName2Token))
	}
	if colName2Token["FNAME"] != "id" {
		t.Error("colName2Token for 'FNAME' is not correct: got " + colName2Token["FNAME"])
	}
	if colName2Token["LNAME"] != "id" {
		t.Error("colName2Token for 'LNAME' is not correct: got " + colName2Token["LNAME"])
	}
	if colName2Token["MYID"] != "id" {
		t.Error("colName2Token for 'MYID' is not correct: got " + colName2Token["MYID"])
	}

	if len(colFragment2Token) != 1 {
		t.Error("expecting 1 entry in colFragment2Token, got ", len(colFragment2Token))
	}
	if colFragment2Token["NAME_"] != "id" {
		t.Error("colFragment2Token for 'NAME_' is not correct: got " + colFragment2Token["NAME_"])
	}
}
func TestCombineColumnNameToken11(t *testing.T) {
	colName2Pos := map[string]int{
		"fname": 0,
		"lname": 1,
		"myId": 2,
	}
	env := map[string]any{
		"${COLUMN_NAME_TOKEN_JSON}": `{"name":"classification","lookup":[{"name":"id","column_pos":[0, 1],"column_name_fragments":["name_"]}]}`,
	}
	configToken := &ColumnNameTokenNode{
		Name: "classification",
		Lookup: []*ColumnNameLookupNode{
			{Name: "id", ColumnNames: []string{"myId"}},
		},
	}
	result, err := combineColumnNameToken(configToken, env, colName2Pos)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Lookup == nil {
		t.Fatal("result.Lookup should not be nil")
	}
	if len(result.Lookup) != 2 {
		t.Fatalf("expecting 2 lookup entries, got %d", len(result.Lookup))
	}
	if result.Lookup[0].Name != "id" || result.Lookup[0].ColumnNames[0] != "myId" {
		t.Error("first lookup entry is not correct")
	}
	if result.Lookup[1].Name != "id" || result.Lookup[1].ColumnNames[0] != "fname" || result.Lookup[1].ColumnNames[1] != "lname" {
		t.Error("second lookup entry is not correct")
	}

	colName2Token, colFragment2Token := prepareColName2TokenMap(result)
	if len(colName2Token) != 3 {
		t.Error("expecting 3 entries in colName2Token, got ", len(colName2Token))
	}
	if colName2Token["FNAME"] != "id" {
		t.Error("colName2Token for 'FNAME' is not correct: got " + colName2Token["FNAME"])
	}
	if colName2Token["LNAME"] != "id" {
		t.Error("colName2Token for 'LNAME' is not correct: got " + colName2Token["LNAME"])
	}
	if colName2Token["MYID"] != "id" {
		t.Error("colName2Token for 'MYID' is not correct: got " + colName2Token["MYID"])
	}

	if len(colFragment2Token) != 1 {
		t.Error("expecting 1 entry in colFragment2Token, got ", len(colFragment2Token))
	}
	if colFragment2Token["NAME_"] != "id" {
		t.Error("colFragment2Token for 'NAME_' is not correct: got " + colFragment2Token["NAME_"])
	}
}
func TestCombineColumnNameToken111(t *testing.T) {
	colName2Pos := map[string]int{
		"fname": 0,
		"lname": 1,
		"myId": 2,
	}
	env := map[string]any{
		"${COLUMN_NAME_TOKEN_JSON}": `{"name":"classification","lookup":[{"name":"id","column_pos":[0, 1],"column_name_fragments":["name_"]}]}`,
	}
	configToken := &ColumnNameTokenNode{
		Name: "classification",
		Lookup: []*ColumnNameLookupNode{
			{Name: "id", ColumnPos: []int{2}},
		},
	}
	result, err := combineColumnNameToken(configToken, env, colName2Pos)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Lookup == nil {
		t.Fatal("result.Lookup should not be nil")
	}
	if len(result.Lookup) != 2 {
		t.Fatalf("expecting 2 lookup entries, got %d", len(result.Lookup))
	}
	if result.Lookup[0].Name != "id" || result.Lookup[0].ColumnNames[0] != "myId" {
		t.Error("first lookup entry is not correct")
	}
	if result.Lookup[1].Name != "id" || result.Lookup[1].ColumnNames[0] != "fname" || result.Lookup[1].ColumnNames[1] != "lname" {
		t.Error("second lookup entry is not correct")
	}

	colName2Token, colFragment2Token := prepareColName2TokenMap(result)
	if len(colName2Token) != 3 {
		t.Error("expecting 3 entries in colName2Token, got ", len(colName2Token))
	}
	if colName2Token["FNAME"] != "id" {
		t.Error("colName2Token for 'FNAME' is not correct: got " + colName2Token["FNAME"])
	}
	if colName2Token["LNAME"] != "id" {
		t.Error("colName2Token for 'LNAME' is not correct: got " + colName2Token["LNAME"])
	}
	if colName2Token["MYID"] != "id" {
		t.Error("colName2Token for 'MYID' is not correct: got " + colName2Token["MYID"])
	}

	if len(colFragment2Token) != 1 {
		t.Error("expecting 1 entry in colFragment2Token, got ", len(colFragment2Token))
	}
	if colFragment2Token["NAME_"] != "id" {
		t.Error("colFragment2Token for 'NAME_' is not correct: got " + colFragment2Token["NAME_"])
	}
}

func TestCombineColumnNameToken02(t *testing.T) {
	env := map[string]any{
		"${COLUMN_NAME_TOKEN_JSON}": `{"name":"classification","lookup":[{"name":"id","column_names":["fname","lname"]}]}`,
		"${OVERRIDE_COLUMN_NAME_TOKEN}": "1",
	}
	configToken := &ColumnNameTokenNode{
		Name: "classification",
		Lookup: []*ColumnNameLookupNode{
			{Name: "id", ColumnNames: []string{"myId"}},
		},
	}
	result, err := combineColumnNameToken(configToken, env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Lookup == nil {
		t.Fatal("result.Lookup should not be nil")
	}
	if len(result.Lookup) != 1 {
		t.Fatalf("expecting 1 lookup entry, got %d", len(result.Lookup))
	}
	if result.Lookup[0].Name != "id" || result.Lookup[0].ColumnNames[0] != "fname" || result.Lookup[0].ColumnNames[1] != "lname" {
		t.Error("lookup entry is not correct")
	}

	colName2Token, colFragment2Token := prepareColName2TokenMap(result)
	if len(colName2Token) != 2 {
		t.Error("expecting 2 entries in colName2Token, got ", len(colName2Token))
	}
	if colName2Token["FNAME"] != "id" {
		t.Error("colName2Token for 'FNAME' is not correct: got " + colName2Token["FNAME"])
	}
	if colName2Token["LNAME"] != "id" {
		t.Error("colName2Token for 'LNAME' is not correct: got " + colName2Token["LNAME"])
	}

	if len(colFragment2Token) != 0 {
		t.Error("colFragment2Token should be empty")
	}
}

func TestCombineColumnNameToken12(t *testing.T) {
	colName2Pos := map[string]int{
		"fname": 0,
		"lname": 1,
		"myId": 2,
	}
	env := map[string]any{
		"${COLUMN_NAME_TOKEN_JSON}": `{"name":"classification","lookup":[{"name":"id","column_pos":[0, 1]}]}`,
		"${OVERRIDE_COLUMN_NAME_TOKEN}": "1",
	}
	configToken := &ColumnNameTokenNode{
		Name: "classification",
		Lookup: []*ColumnNameLookupNode{
			{Name: "id", ColumnNames: []string{"myId"}},
		},
	}
	result, err := combineColumnNameToken(configToken, env, colName2Pos)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Lookup == nil {
		t.Fatal("result.Lookup should not be nil")
	}
	if len(result.Lookup) != 1 {
		t.Fatalf("expecting 1 lookup entry, got %d", len(result.Lookup))
	}
	if result.Lookup[0].Name != "id" || result.Lookup[0].ColumnNames[0] != "fname" || result.Lookup[0].ColumnNames[1] != "lname" {
		t.Error("lookup entry is not correct")
	}

	colName2Token, colFragment2Token := prepareColName2TokenMap(result)
	if len(colName2Token) != 2 {
		t.Error("expecting 2 entries in colName2Token, got ", len(colName2Token))
	}
	if colName2Token["FNAME"] != "id" {
		t.Error("colName2Token for 'FNAME' is not correct: got " + colName2Token["FNAME"])
	}
	if colName2Token["LNAME"] != "id" {
		t.Error("colName2Token for 'LNAME' is not correct: got " + colName2Token["LNAME"])
	}

	if len(colFragment2Token) != 0 {
		t.Error("colFragment2Token should be empty")
	}
}

// Testing when the env var ${COLUMN_NAME_TOKEN_JSON} is overriding a config column name token with the same name, but the override env var ${OVERRIDE_COLUMN_NAME_TOKEN} is not set to 1, so the two column name tokens should be merged by appending the lookup entries from the env var to the config document.
func TestCombineColumnNameToken03(t *testing.T) {
	env := map[string]any{
		"${COLUMN_NAME_TOKEN_JSON}": `{"name":"classification","lookup":[{"name":"id","column_names":["fname","lname"]}]}`,
	}
	configToken := &ColumnNameTokenNode{
		Name: "classification",
		Lookup: []*ColumnNameLookupNode{
			{Name: "unclassified", ColumnNames: []string{"fname","addr"}},
		},
	}
	result, err := combineColumnNameToken(configToken, env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Lookup == nil {
		t.Fatal("result.Lookup should not be nil")
	}
	if len(result.Lookup) != 2 {
		t.Fatalf("expecting 2 lookup entries, got %d", len(result.Lookup))
	}
	if result.Lookup[0].Name != "unclassified" || result.Lookup[0].ColumnNames[0] != "fname" || result.Lookup[0].ColumnNames[1] != "addr" {
		t.Error("first lookup entry is not correct")
	}
	if result.Lookup[1].Name != "id" || result.Lookup[1].ColumnNames[0] != "fname" || result.Lookup[1].ColumnNames[1] != "lname" {
		t.Error("second lookup entry is not correct")
	}

		colName2Token, colFragment2Token := prepareColName2TokenMap(result)
	if len(colName2Token) != 3 {
		t.Error("expecting 3 entries in colName2Token, got ", len(colName2Token))
	}
	if colName2Token["FNAME"] != "id" {
		t.Error("colName2Token for 'FNAME' is not correct: got " + colName2Token["FNAME"])
	}
	if colName2Token["LNAME"] != "id" {
		t.Error("colName2Token for 'LNAME' is not correct: got " + colName2Token["LNAME"])
	}
	if colName2Token["ADDR"] != "unclassified" {
		t.Error("colName2Token for 'ADDR' is not correct: got " + colName2Token["ADDR"])
	}

	if len(colFragment2Token) != 0 {
		t.Error("colFragment2Token should be empty")
	}
}
func TestCombineColumnNameToken13(t *testing.T) {
	colName2Pos := map[string]int{
		"fname": 0,
		"lname": 1,
		"myId": 2,
		"addr": 3,
	}
	env := map[string]any{
		"${COLUMN_NAME_TOKEN_JSON}": `{"name":"classification","lookup":[{"name":"id","column_pos":[0,1]}]}`,
	}
	configToken := &ColumnNameTokenNode{
		Name: "classification",
		Lookup: []*ColumnNameLookupNode{
			{Name: "unclassified", ColumnNames: []string{"fname","addr"}},
		},
	}
	result, err := combineColumnNameToken(configToken, env, colName2Pos)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Lookup == nil {
		t.Fatal("result.Lookup should not be nil")
	}
	if len(result.Lookup) != 2 {
		t.Fatalf("expecting 2 lookup entries, got %d", len(result.Lookup))
	}
	if result.Lookup[0].Name != "unclassified" || result.Lookup[0].ColumnNames[0] != "fname" || result.Lookup[0].ColumnNames[1] != "addr" {
		t.Error("first lookup entry is not correct")
	}
	if result.Lookup[1].Name != "id" || result.Lookup[1].ColumnNames[0] != "fname" || result.Lookup[1].ColumnNames[1] != "lname" {
		t.Error("second lookup entry is not correct")
	}

		colName2Token, colFragment2Token := prepareColName2TokenMap(result)
	if len(colName2Token) != 3 {
		t.Error("expecting 3 entries in colName2Token, got ", len(colName2Token))
	}
	if colName2Token["FNAME"] != "id" {
		t.Error("colName2Token for 'FNAME' is not correct: got " + colName2Token["FNAME"])
	}
	if colName2Token["LNAME"] != "id" {
		t.Error("colName2Token for 'LNAME' is not correct: got " + colName2Token["LNAME"])
	}
	if colName2Token["ADDR"] != "unclassified" {
		t.Error("colName2Token for 'ADDR' is not correct: got " + colName2Token["ADDR"])
	}

	if len(colFragment2Token) != 0 {
		t.Error("colFragment2Token should be empty")
	}
}
