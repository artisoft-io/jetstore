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

		// A divisor of 0, or one that is not finite, would make every value
		// +Inf, -Inf or NaN
		{"scale_units", "0", "divisor argument '0' must be a number other than 0"},
		{"scale_units", "0.0", "must be a number other than 0"},
		{"scale_units", "-0", "must be a number other than 0"},
		{"scale_units", "NaN", "must be a number other than 0"},
		{"scale_units", "Inf", "must be a number other than 0"},
		{"scale_units", "1e400", "is not a number"},
		{"parse_amount", "0", "must be a number other than 0"},
		{"parse_amount", "-Inf", "must be a number other than 0"},

		// format_phone's format takes the three parts of the number as strings
		{"format_phone", "%s-%s", "must take the area code"},
		{"format_phone", "%s%s%s%s", "must take the area code"},
		{"format_phone", "%d%d%d", "must take the area code"},
		{"format_phone", "%*s%s%s", "must take the area code"},
		{"format_phone", "%[4]s%s%s", "must take the area code"},
		{"format_phone", "+1%s%s%s%", "must take the area code"},

		// reformat0's format takes the number as one integer
		{"reformat0", "%09s", "must take the number as one integer"},
		{"reformat0", "%s", "gives '%!s(int=123)'"},
		{"reformat0", "%f", "must take the number as one integer"},
		{"reformat0", "%d-%d", "must take the number as one integer"},
		{"reformat0", "000", "must take the number as one integer"},
		{"reformat0", "%*d", "must take the number as one integer"},
		{"reformat0", "%[2]d", "must take the number as one integer"},
		{"reformat0", "%05d%", "must take the number as one integer"},

		// overpunch_number's decimal places
		{"overpunch_number", "x", "decimal places argument 'x' must be an integer of 0 or more"},
		{"overpunch_number", "-1", "must be an integer of 0 or more"},
		{"overpunch_number", "2.5", "must be an integer of 0 or more"},

		// The functions whose argument was checked only per record until
		// 2026-11-14: missing, and malformed
		{"concat", "", "concat requires an argument"},
		{"concat_with", "", "concat_with requires an argument"},
		{"find_and_replace", "", "find_and_replace requires an argument"},
		{"substring", "", "substring requires an argument"},
		{"concat", "first,middle", "column 'middle' in argument first,middle is not an input column"},
		{"concat_with", "\" \",middle", "column 'middle'"},
		{"concat", "\"first", "cannot be parsed as csv"},
		{"find_and_replace", "a", "cannot be parsed as csv"},
		{"find_and_replace", "\"a\",\"b\",\"c\"", "cannot be parsed as csv"},
		{"find_and_replace", "\"\",\"x\"", "has no text to find"},
		{"substring", "3", "cannot be parsed as start,end"},
		{"substring", "a,3", "cannot be parsed as start,end"},
		{"substring", "-1,3", "start must be 0 or more"},
		{"substring", "5,3", "end must be greater than start"},
	}
	for _, c := range cases {
		ctx := NewCleansingFunctionContext(&map[string]int{"first": 0, "last": 1})
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
		{"format_phone", "(%s) %s-%s", "555.234.5678", "(555) 234-5678"},
		{"format_phone", "%s%s%s 100%%", "5552345678", "5552345678 100%"},
		{"format_phone", "%%!%s%s%s", "5552345678", "%!5552345678"},
		{"format_phone", "%[3]s-%[1]s", "5552345678", "5678-555"},
		{"overpunch_number", "0", "12{", "+120"},
		{"reformat0", "%05d", "123", "00123"},
		{"reformat0", "%09d", "12-345-678", "012345678"},
		{"reformat0", "%02d", "7", "07"},
		{"reformat0", "%v", "0042", "42"},
		{"reformat0", "%x", "255", "ff"},
		{"reformat0", "N%04d 100%%", "12", "N0012 100%"},
		{"reformat0", "%%!%03d", "5", "%!005"},
		{"reformat0", "%[1]d-%[1]d", "5", "5-5"},
		{"overpunch_number", "2", "12{", "+1.20"},
		{"apply_regex", "[0-9]+", "ab123cd", "123"},
		{"scale_units", "1", "42", "42"},
		{"scale_units", "10", "42", "5.000000"},
		{"parse_amount", "", "$1,234.50", "1234.50"},
		{"parse_amount", "100", "250", "2.500000"},
		{"parse_amount", "-100", "250", "-2.500000"},
		{"scale_units", "0.5", "3", "6.000000"},
		{"concat", "last", "Ann", "AnnLee"},
		{"concat_with", "\" \",last", "Ann", "Ann Lee"},
		{"concat_with", "\" \",last,first", "Ann", "Ann Lee Ann"},
		{"concat_with", "-", "Ann", "Ann"},
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
		got, errMsg := ctx.ApplyCleasingFunction(c.fn, c.arg, c.input, &row)
		if errMsg != "" {
			t.Errorf("%s(%q) on %q: %s", c.fn, c.arg, c.input, errMsg)
			continue
		}
		if !sameValue(got, c.want) {
			t.Errorf("%s(%q) on %q = %#v, want %#v", c.fn, c.arg, c.input, got, c.want)
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
		{"scale_units", "0"},
		{"parse_amount", "0"},
		{"format_phone", "%s-%s"},
		{"reformat0", "%s"},
		{"overpunch_number", "x"},
		{"concat", ""},
		{"concat", "no_such_column"},
		{"concat_with", ""},
		{"find_and_replace", ""},
		{"find_and_replace", "a"},
		{"substring", ""},
		{"substring", "5,3"},
	}
	row := []any{"5552345678"}
	for _, c := range cases {
		ctx := NewCleansingFunctionContext(&map[string]int{})
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s(%q) panicked: %v", c.fn, c.arg, r)
				}
			}()
			obj, errMsg := ctx.ApplyCleasingFunction(c.fn, c.arg, "5552345678", &row)
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
	if err := ctx.CheckFunction("reformat0", "%09d"); err != nil {
		t.Fatal(err)
	}
	if ctx.parsedFunctionArguments["reformat0(%09d)"] == nil {
		t.Error("the reformat0 format is not cached")
	}
	ctx = NewCleansingFunctionContext(&map[string]int{"a": 0})
	for _, c := range []struct{ fn, arg string }{
		{"substring", "0,2"}, {"find_and_replace", "\"a\",\"b\""}, {"concat", "a"}, {"concat_with", "a"},
	} {
		if err := ctx.CheckFunction(c.fn, c.arg); err != nil {
			t.Fatal(err)
		}
		if ctx.parsedFunctionArguments[c.fn+"("+c.arg+")"] == nil {
			t.Errorf("the %s argument is not cached", c.fn)
		}
	}
	// One context, one raw argument, two functions: each has its own entry,
	// so concat and concat_with do not read each other's parsed argument
	if a := ctx.parsedFunctionArguments["concat(a)"].(*ConcatFunctionArg); len(a.ColumnPositions) != 1 {
		t.Errorf("concat(a) has %d columns, want 1", len(a.ColumnPositions))
	}
	if a := ctx.parsedFunctionArguments["concat_with(a)"].(*ConcatFunctionArg); a.Delimit != "a" || len(a.ColumnPositions) != 0 {
		t.Errorf("concat_with(a) is %+v, want the delimiter a and no column", a)
	}
}

// The start check and the record path refuse the same divisor, the same
// format_phone or reformat0 format and the same decimal places: a map column
// never reaches the record path with one CheckFunction refused, and a caller
// that skips CheckFunction gets an error on the value rather than +Inf, NaN or
// fmt's %!s(MISSING).
func TestCheckFunctionAndRecordPathAgree(t *testing.T) {
	cases := []struct{ fn, arg, input string }{
		{"scale_units", "0", "42"},
		{"scale_units", "-0.0", "42"},
		{"parse_amount", "0", "42"},
		{"parse_amount", "NaN", "42"},
		{"format_phone", "%s-%s", "5552345678"},
		{"format_phone", "%d", "5552345678"},
		{"reformat0", "%s", "123"},
		{"reformat0", "%d%d", "123"},
		{"reformat0", "%s", ""},
		{"overpunch_number", "-1", "12{"},
		{"overpunch_number", "two", "12{"},
	}
	for _, c := range cases {
		ctx := NewCleansingFunctionContext(&map[string]int{})
		checkErr := ctx.CheckFunction(c.fn, c.arg)
		if checkErr == nil {
			t.Errorf("CheckFunction(%q, %q): expected an error", c.fn, c.arg)
		}
		obj, errMsg := ctx.ApplyCleasingFunction(c.fn, c.arg, c.input, nil)
		if errMsg == "" || obj != nil {
			t.Errorf("%s(%q) on %q = %#v, %q: want an error and no value", c.fn, c.arg, c.input, obj, errMsg)
			continue
		}
		if checkErr != nil && checkErr.Error() != errMsg {
			t.Errorf("%s(%q): the start check says %q and the record path %q", c.fn, c.arg, checkErr, errMsg)
		}
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
