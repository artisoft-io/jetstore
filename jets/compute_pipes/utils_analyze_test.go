package compute_pipes

// Testing analyze operator

import (
	"testing"
)

func TestParseDoubleMatchFunction1(t *testing.T) {
	fspec := &FunctionTokenNode{
		Type: "parse_double",
	}
	fcount, err := NewParseDoubleMatchFunction(fspec)
	if err != nil {
		t.Fatal(err)
	}
	fcount.NewValue("xxxx")
	fcount.NewValue("1930")
	fcount.NewValue("1970")
	fcount.NewValue("2025")
	fcount.NewValue("2030")
	fcount.NewValue("ffff")
	result := fcount.GetMinMaxValues()
	if result == nil {
		t.Fatal(err)
	}
	if result.MinMaxType != "double" {
		t.Errorf("expecting double, got %s", result.MinMaxType)
	}
	if result.MinValue != "1930" {
		t.Errorf("expecting 1930, got %s", result.MinValue)
	}
	if result.MaxValue != "2030" {
		t.Errorf("expecting 2030, got %s", result.MaxValue)
	}
	if result.HitCount != float64(4)/float64(6) {
		t.Errorf("expecting 4, got %v", result.HitCount)
	}
}

func TestParseTextMatchFunction1(t *testing.T) {
	fspec := &FunctionTokenNode{
		Type: "parse_text",
	}
	fcount, err := NewParseTextMatchFunction(fspec)
	if err != nil {
		t.Fatal(err)
	}
	fcount.NewValue("some5")
	fcount.NewValue("a2")
	fcount.NewValue("0123456789")
	fcount.NewValue("2025")
	fcount.NewValue("2030")
	fcount.NewValue("ffff")
	result := fcount.GetMinMaxValues()
	if result == nil {
		t.Fatal(err)
	}
	if result.MinMaxType != "text" {
		t.Errorf("expecting text, got %s", result.MinMaxType)
	}
	if result.MinValue != "2" {
		t.Errorf("expecting 2, got %s", result.MinValue)
	}
	if result.MaxValue != "10" {
		t.Errorf("expecting 10, got %s", result.MaxValue)
	}
	if result.HitCount != 1 {
		t.Errorf("expecting 6, got %v", result.HitCount)
	}
}

type LookupTableTest struct {
	rows map[string]*[]any
}

// Returns the lookup row associated with key
func (tbl *LookupTableTest) Lookup(key *string) (*[]interface{}, error) {
	return tbl.rows[*key], nil
}

// Returns the row's value associated with the lookup column
func (tbl *LookupTableTest) LookupValue(row *[]interface{}, columnName string) (interface{}, error) {
	return nil, nil
}

// Returns the mapping between column name to pos in the returned row
func (tbl *LookupTableTest) ColumnMap() map[string]int {
	return nil
}

// Return true if the table is empty, ColumnMap is empty as well
func (tbl *LookupTableTest) IsEmptyTable() bool {
	return false
}

func (tbl *LookupTableTest) Size() int64 {
	return int64(len(tbl.rows))
}

func TestLookupTokensState1(t *testing.T) {
	lookup := &LookupTableTest{
		rows: map[string]*[]any{
			"john":  {[]string{"first_name"}},
			"smith": {[]string{"last_name"}},
		}}
	state, err := NewLookupTokensState(lookup, &LookupTokenNode{
		Name:   "lookupName",
		Tokens: []string{"first_name", "last_name"},
		MultiTokensMatch: []MultiTokensNode{
			{
				Name: "full_name",
				NbrTokens: 2,
				Tokens: []string{"first_name", "last_name"},
			},
		},
	})
	if err != nil {
		t.Error(err)
	}
	value := "john"
	err = state.NewValue(&value)
	if err != nil {
		t.Error(err)
	}
	if state.LookupMatch["first_name"].Count != 1 {
		t.Error("expecting 1")
	}
	if state.LookupMatch["last_name"].Count != 0 {
		t.Error("expecting 0")
	}
	if state.LookupMatch["full_name"].Count != 0 {
		t.Error("expecting 0")
	}
	value = "smith"
	err = state.NewValue(&value)
	if err != nil {
		t.Error(err)
	}
	if state.LookupMatch["last_name"].Count != 1 {
		t.Error("expecting 1")
	}
	value = "john smith"
	err = state.NewValue(&value)
	if err != nil {
		t.Error(err)
	}
	if state.LookupMatch["first_name"].Count != 1 {
		t.Error("expecting 1")
	}
	if state.LookupMatch["last_name"].Count != 1 {
		t.Error("expecting 1")
	}
	if state.LookupMatch["full_name"].Count != 1 {
		t.Error("expecting 1")
	}
	value = "smith, john"
	err = state.NewValue(&value)
	if err != nil {
		t.Error(err)
	}
	if state.LookupMatch["first_name"].Count != 1 {
		t.Error("expecting 1")
	}
	if state.LookupMatch["last_name"].Count != 1 {
		t.Error("expecting 1")
	}
	if state.LookupMatch["full_name"].Count != 2 {
		t.Error("expecting 2")
	}
	value = "smith, john P"
	err = state.NewValue(&value)
	if err != nil {
		t.Error(err)
	}
	if state.LookupMatch["first_name"].Count != 1 {
		t.Error("expecting 1")
	}
	if state.LookupMatch["last_name"].Count != 1 {
		t.Error("expecting 1")
	}
	if state.LookupMatch["full_name"].Count != 3 {
		t.Error("expecting 3")
	}
	// t.Error("That's it!")

}
