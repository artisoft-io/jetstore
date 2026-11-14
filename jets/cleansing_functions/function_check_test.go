package cleansing_functions

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Every configuration CheckFunction refuses, with a fragment of the error.
func TestCheckFunctionRefuses(t *testing.T) {
	cases := []struct {
		fn, arg, want string
	}{
		{"no_such_function", "", "unknown cleansing function 'no_such_function'"},
		{"Trim", "", "unknown cleansing function 'Trim'"},
		{"reformat0", "", "reformat0 requires an argument"},
		{"overpunch_number", "", "overpunch_number requires an argument"},
		{"apply_regex", "", "apply_regex requires an argument"},
		{"scale_units", "", "scale_units requires an argument"},
		{"scrub_characters", "", "scrub_characters requires an argument"},
		{"split_on", "", "split_on requires an argument"},
		{"unique_split_on", "", "unique_split_on requires an argument"},
		{"slice_input", "", "slice_input requires an argument"},
		{"apply_regex", "([0-9]", "is not a valid regular expression"},
		{"scale_units", "ten", "divisor argument 'ten' is not a number"},
		{"parse_amount", "1x", "divisor argument '1x' is not a number"},
		{"slice_input", "^,-1", "must have position >= 0"},
		{"slice_input", "^,a,:", "expecting from (int)"},
		{"slice_input", "^,2,:,1", "to > from"},
	}
	for _, c := range cases {
		ctx := NewCleansingFunctionContext(&map[string]int{})
		err := ctx.CheckFunction(c.fn, c.arg)
		if err == nil {
			t.Errorf("CheckFunction(%q, %q): expected an error", c.fn, c.arg)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("CheckFunction(%q, %q) = %q, want it to contain %q", c.fn, c.arg, err, c.want)
		}
	}
}

// A valid configuration of each function is accepted, and runs on a value.
func TestCheckFunctionAccepts(t *testing.T) {
	columns := map[string]int{"first": 0, "last": 1}
	row := []any{"Ann", "Lee"}
	cases := []struct {
		fn, arg, input string
		want           any
	}{
		{"", "", "x", nil}, // no cleansing function
		{"trim", "", "  a  ", "a"},
		{"validate_date", "", "2024-01-31", "2024-01-31"},
		{"to_upper", "", "abc", "ABC"},
		{"to_zip5", "", "12345-6789", "12345"},
		{"to_zipext4_from_zip9", "", "12345-6789", "6789"},
		{"to_zipext4", "", "6789", "6789"},
		{"format_phone", "", "(555) 234-5678", "+15552345678"},
		{"reformat0", "%05d", "123", "00123"},
		{"overpunch_number", "2", "12{", "+1.20"},
		{"apply_regex", "[0-9]+", "ab123cd", "123"},
		{"scale_units", "1", "42", "42"},
		{"scale_units", "10", "42", "5.000000"},
		{"parse_amount", "", "$1,234.50", "1234.50"},
		{"parse_amount", "100", "250", "2.500000"},
		{"concat", "last", "Ann", "AnnLee"},
		{"concat_with", "\" \",last", "Ann", "Ann Lee"},
		{"find_and_replace", "\"a\",\"o\"", "banana", "bonono"},
		{"scrub_characters", "-", "12-34", "1234"},
		{"substring", "0,2", "abcd", "ab"},
		{"split_on", ",", "a,b", []string{"a", "b"}},
		{"unique_split_on", ",", "a", []string{"a-0"}},
		{"slice_input", "\",\",1,:", "a,b,c", []string{"b", "c"}},
		{"ndc10_to_11", "", "1234-5678-90", "01234567890"},
	}
	for _, c := range cases {
		ctx := NewCleansingFunctionContext(&columns)
		if err := ctx.CheckFunction(c.fn, c.arg); err != nil {
			t.Errorf("CheckFunction(%q, %q): %v", c.fn, c.arg, err)
			continue
		}
		if c.fn == "" {
			continue
		}
		got, errMsg := ctx.ApplyCleasingFunction(c.fn, c.arg, c.input, 0, &row)
		if errMsg != "" {
			t.Errorf("%s(%q) on %q: %s", c.fn, c.arg, c.input, errMsg)
			continue
		}
		if !sameValue(got, c.want) {
			t.Errorf("%s(%q) on %q = %#v, want %#v", c.fn, c.arg, c.input, got, c.want)
		}
	}
}

// The functions whose argument is checked per record still report a missing
// argument on the value, as an error, and are not refused before the step.
func TestCheckFunctionLeavesPerRecordArguments(t *testing.T) {
	row := []any{"x"}
	for _, fn := range []string{"concat", "concat_with", "find_and_replace", "substring"} {
		ctx := NewCleansingFunctionContext(&map[string]int{"c": 0})
		if err := ctx.CheckFunction(fn, ""); err != nil {
			t.Errorf("CheckFunction(%q, \"\"): %v", fn, err)
		}
		if _, errMsg := ctx.ApplyCleasingFunction(fn, "", "x", 0, &row); errMsg == "" {
			t.Errorf("%s with no argument: expected an error on the value", fn)
		}
	}
}

// A configuration CheckFunction refuses, reaching ApplyCleasingFunction anyway,
// is an error on the value and not a panic.
func TestApplyCleasingFunctionDoesNotPanic(t *testing.T) {
	cases := []struct{ fn, arg string }{
		{"no_such_function", ""},
		{"reformat0", ""},
		{"overpunch_number", ""},
		{"apply_regex", ""},
		{"apply_regex", "([0-9]"},
		{"scale_units", ""},
		{"scale_units", "ten"},
		{"parse_amount", "1x"},
		{"scrub_characters", ""},
		{"split_on", ""},
		{"unique_split_on", ""},
		{"slice_input", ""},
		{"slice_input", "^,-1"},
	}
	row := []any{"12"}
	for _, c := range cases {
		ctx := NewCleansingFunctionContext(&map[string]int{})
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s(%q) panicked: %v", c.fn, c.arg, r)
				}
			}()
			obj, errMsg := ctx.ApplyCleasingFunction(c.fn, c.arg, "12", 0, &row)
			if errMsg == "" || obj != nil {
				t.Errorf("%s(%q): expected an error and no value, got %#v, %q", c.fn, c.arg, obj, errMsg)
			}
		}()
	}
}

// The argument CheckFunction parsed is the one the records use: it is cached
// in the context.
func TestCheckFunctionCachesTheParsedArgument(t *testing.T) {
	ctx := NewCleansingFunctionContext(&map[string]int{})
	if err := ctx.CheckFunction("apply_regex", "[0-9]+"); err != nil {
		t.Fatal(err)
	}
	if err := ctx.CheckFunction("scale_units", "10"); err != nil {
		t.Fatal(err)
	}
	if err := ctx.CheckFunction("slice_input", "^,0"); err != nil {
		t.Fatal(err)
	}
	if ctx.reMap["[0-9]+"] == nil {
		t.Error("the regex is not cached")
	}
	if ctx.argdMap["10"] != 10 {
		t.Error("the divisor is not cached")
	}
	if ctx.parsedFunctionArguments["slice_input(^,0)"] == nil {
		t.Error("the slice argument is not cached")
	}
}

// cleansingFunctions names exactly the cases ApplyCleasingFunction implements,
// read from its source, so a function added to one and not the other fails here.
func TestCleansingFunctionsMatchSwitch(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cleansing_functions.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var inSwitch []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "ApplyCleasingFunction" {
			continue
		}
		for _, stmt := range fn.Body.List {
			sw, ok := stmt.(*ast.SwitchStmt)
			if !ok {
				continue
			}
			if id, ok := sw.Tag.(*ast.Ident); !ok || id.Name != "functionName" {
				continue
			}
			for _, cc := range sw.Body.List {
				for _, e := range cc.(*ast.CaseClause).List {
					lit, ok := e.(*ast.BasicLit)
					if !ok {
						continue
					}
					name, _ := strconv.Unquote(lit.Value)
					inSwitch = append(inSwitch, name)
				}
			}
		}
	}
	if len(inSwitch) == 0 {
		t.Fatal("found no case in ApplyCleasingFunction's switch on functionName")
	}
	assertSameNames(t, "ApplyCleasingFunction", inSwitch)
}

// cleansingFunctions agrees with jetsapi.mapping_function_registry, which the UI
// offers: the same names, and an argument required by the registry is one
// CheckFunction or the record path asks for.
func TestCleansingFunctionsMatchRegistry(t *testing.T) {
	data, err := os.ReadFile("../jets_init_db.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(data)
	start := strings.Index(sql, "INSERT INTO jetsapi.mapping_function_registry")
	if start < 0 {
		t.Fatal("mapping_function_registry seed not found in jets_init_db.sql")
	}
	end := strings.Index(sql[start:], ";")
	rows := regexp.MustCompile(`\('(\w+)',\s*'([01])'\)`).FindAllStringSubmatch(sql[start:start+end], -1)
	var names []string
	for _, r := range rows {
		names = append(names, r[1])
		required := r[2] == "1"
		if check, ok := cleansingFunctions[r[1]]; ok && required != (check != argOptional) {
			t.Errorf("%s: registry is_argument_required is %v, cleansingFunctions says %v", r[1], required, check)
		}
	}
	assertSameNames(t, "mapping_function_registry", names)
}

func assertSameNames(t *testing.T, where string, names []string) {
	t.Helper()
	known := make([]string, 0, len(cleansingFunctions))
	for k := range cleansingFunctions {
		known = append(known, k)
	}
	sort.Strings(known)
	sort.Strings(names)
	if strings.Join(known, ",") != strings.Join(names, ",") {
		t.Errorf("cleansingFunctions and %s differ:\n  cleansingFunctions: %v\n  %s: %v", where, known, where, names)
	}
}

func sameValue(got, want any) bool {
	switch w := want.(type) {
	case []string:
		g, ok := got.([]string)
		if !ok || len(g) != len(w) {
			return false
		}
		sort.Strings(g)
		for i := range w {
			if g[i] != w[i] {
				return false
			}
		}
		return true
	default:
		return got == want
	}
}
