package cleansing_functions

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// argumentCheck says how CheckFunction treats a cleansing function's argument.
type argumentCheck int

const (
	// argOptional: the function takes no argument, or an optional one.
	argOptional argumentCheck = iota
	// argRequired: the function cannot run without its argument, and
	// CheckFunction refuses the configuration when it is missing.
	argRequired
	// argPerRecord: the function needs its argument, but a missing or malformed
	// one is reported on each record, as an error on the value, rather than
	// refused by CheckFunction. These are the functions whose argument names
	// input columns or is parsed per record, and which have always reported it
	// that way.
	argPerRecord
)

// cleansingFunctions is every function ApplyCleasingFunction implements. It
// must agree with jetsapi.mapping_function_registry, seeded in
// jets/jets_init_db.sql, which is the list the UI offers: a function there and
// not here would be refused at the start of every step that maps with it.
// TestCleansingFunctionsMatchRegistry and TestCleansingFunctionsMatchSwitch hold
// the three lists together.
var cleansingFunctions = map[string]argumentCheck{
	"apply_regex":          argRequired,
	"concat":               argPerRecord,
	"concat_with":          argPerRecord,
	"find_and_replace":     argPerRecord,
	"format_phone":         argOptional,
	"ndc10_to_11":          argOptional,
	"overpunch_number":     argRequired,
	"parse_amount":         argOptional,
	"reformat0":            argRequired,
	"scale_units":          argRequired,
	"scrub_characters":     argRequired,
	"slice_input":          argRequired,
	"split_on":             argRequired,
	"substring":            argPerRecord,
	"to_upper":             argOptional,
	"to_zip5":              argOptional,
	"to_zipext4":           argOptional,
	"to_zipext4_from_zip9": argOptional,
	"trim":                 argOptional,
	"unique_split_on":      argRequired,
	"validate_date":        argOptional,
}

// CheckFunction validates a cleansing function and its argument before any
// record is read, so that a misconfigured column fails the step at its start
// instead of on its first non-blank value. It refuses:
//
//   - a function name that is not a cleansing function;
//   - a missing argument to apply_regex, overpunch_number, reformat0,
//     scale_units, scrub_characters, slice_input, split_on and unique_split_on;
//   - an apply_regex argument that is not a valid regular expression;
//   - a scale_units or parse_amount divisor that is not a number, or is 0, or
//     is not finite;
//   - a format_phone format that does not take the three parts of the number;
//   - a slice_input argument that does not parse.
//
// An empty function name means no cleansing function, and is accepted.
//
// It parses the argument with the same code ApplyCleasingFunction uses, and
// leaves the parsed form in the context's cache, so the records that follow
// reuse it. The error names the function; the caller names the column.
func (ctx *CleansingFunctionContext) CheckFunction(functionName, argument string) error {
	if functionName == "" {
		return nil
	}
	check, ok := cleansingFunctions[functionName]
	if !ok {
		return fmt.Errorf("%s", unknownFunction(functionName))
	}
	if check == argRequired && argument == "" {
		return fmt.Errorf("%s", missingArgument(functionName))
	}
	var err error
	switch functionName {
	case "apply_regex":
		_, err = ctx.regexArgument(argument)
	case "scale_units":
		if argument != "1" {
			_, err = ctx.divisorArgument(argument)
		}
	case "parse_amount":
		if argument != "" && argument != "1" {
			_, err = ctx.divisorArgument(argument)
		}
	case "format_phone":
		if argument != "" {
			err = ctx.phoneFormatArgument(argument)
		}
	case "slice_input":
		_, err = ParseSliceInputFunctionArgument(argument, functionName, ctx.parsedFunctionArguments)
	}
	if err != nil {
		return fmt.Errorf("cleansing function %s: %v", functionName, err)
	}
	return nil
}

// regexArgument compiles apply_regex's argument, once per context.
func (ctx *CleansingFunctionContext) regexArgument(argument string) (*regexp.Regexp, error) {
	if re, ok := ctx.reMap[argument]; ok {
		return re, nil
	}
	re, err := regexp.Compile(argument)
	if err != nil {
		return nil, fmt.Errorf("argument '%s' is not a valid regular expression: %v", argument, err)
	}
	ctx.reMap[argument] = re
	return re, nil
}

// divisorArgument parses the divisor of scale_units and parse_amount, once per
// context. It refuses a divisor that is not a number, and one that is 0, NaN or
// infinite, which would turn every value into +Inf, -Inf or NaN.
func (ctx *CleansingFunctionContext) divisorArgument(argument string) (float64, error) {
	if divisor, ok := ctx.argdMap[argument]; ok {
		return divisor, nil
	}
	divisor, err := strconv.ParseFloat(argument, 64)
	if err != nil {
		return 0, fmt.Errorf("divisor argument '%s' is not a number", argument)
	}
	if divisor == 0 || math.IsNaN(divisor) || math.IsInf(divisor, 0) {
		// Dividing by it would give +Inf, -Inf or NaN for every value
		return 0, fmt.Errorf("divisor argument '%s' must be a number other than 0", argument)
	}
	ctx.argdMap[argument] = divisor
	return divisor, nil
}

// The three parts of a phone number format_phone gives its format, as sample
// values for checkPhoneFormat: area code, exchange code, subscriber number.
var phoneFormatSample = []any{"555", "234", "5678"}

// phoneFormatArgument checks format_phone's argument with checkPhoneFormat,
// once per context.
func (ctx *CleansingFunctionContext) phoneFormatArgument(format string) error {
	key := "format_phone(" + format + ")"
	if ctx.parsedFunctionArguments[key] != nil {
		return nil
	}
	if err := checkPhoneFormat(format); err != nil {
		return err
	}
	ctx.parsedFunctionArguments[key] = true
	return nil
}

// checkPhoneFormat checks format_phone's argument, an fmt.Sprintf format given
// the three parts of the number as strings. It refuses a format that fmt would
// flag in its output: a verb that does not take a string (%d), fewer verbs than
// parts (%!s(MISSING)), more parts than verbs (%!(EXTRA ...)), a bad argument
// index, a '*' width or precision, or a trailing '%'. Explicit argument
// indexes are accepted, so a format may reorder the parts or leave one out.
//
// It formats the sample parts and looks for fmt's "%!" marker. A literal "%%"
// is removed first, so "%%!" in a format is not mistaken for the marker.
func checkPhoneFormat(format string) error {
	out := fmt.Sprintf(strings.ReplaceAll(format, "%%", ""), phoneFormatSample...)
	if strings.Contains(out, "%!") {
		return fmt.Errorf("format argument '%s' must take the area code, exchange code and subscriber number as three strings (%%s), it gives '%s'",
			format, fmt.Sprintf(format, phoneFormatSample...))
	}
	return nil
}

func unknownFunction(functionName string) string {
	return fmt.Sprintf("unknown cleansing function '%s'", functionName)
}

func missingArgument(functionName string) string {
	return fmt.Sprintf("cleansing function %s requires an argument", functionName)
}
