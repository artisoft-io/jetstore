package compute_pipes

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Utility function and components for Analyze operator

type DistinctCount struct {
	Value string
	Count int
}

type RegexCount struct {
	Rexpr *regexp.Regexp
	Count int
}

func NewRegexCount(re *regexp.Regexp) *RegexCount {
	return &RegexCount{Rexpr: re}
}

type LookupCount struct {
	Name  string
	Count int
}

func NewLookupCount(name string) *LookupCount {
	return &LookupCount{
		Name: name,
	}
}

type KeywordCount struct {
	Name     string
	Keywords []string
	Count    int
}

func NewKeywordCount(name string, keywords []string) *KeywordCount {
	return &KeywordCount{
		Name:     name,
		Keywords: keywords,
	}
}

type MatchFunction interface {
	Match(value string) bool
}

type FunctionCount struct {
	Function MatchFunction
	Count    int
}

func NewFunctionCount(fspec *FunctionTokenNode, sp SchemaProvider) (*FunctionCount, error) {
	var fnc MatchFunction
	var err error
	switch fspec.FunctionName {
	case "parse_date":
		fnc, err = NewParseDateMatchFunction(fspec, sp)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("error: unknown function_name '%s' in Analyze operator", fspec.FunctionName)
	}
	return &FunctionCount{
		Function: fnc,
	}, nil
}

// Analyze data TransformationSpec implementing PipeTransformationEvaluator interface
type AnalyzeState struct {
	ColumnName         string
	ColumnPos          int
	DistinctValues     map[string]*DistinctCount
	NullCount          int
	LenWelford         *WelfordAlgo
	ValueToDouble      func(d any) (float64, error)
	ValueAsDoubleCount int
	ValueWelford       *WelfordAlgo
	RegexMatch         map[string]*RegexCount
	LookupState        []*LookupTokensState
	KeywordMatch       map[string]*KeywordCount
	FunctionMatch      map[string]*FunctionCount
	TotalRowCount      int
	Spec               *TransformationSpec
}

type LookupTokensState struct {
	LookupTbl   LookupTable
	KeyRe       *regexp.Regexp
	LookupMatch map[string]*LookupCount
}

func NewLookupTokensState(lookupTbl LookupTable, keyRe string, tokens []string) (*LookupTokensState, error) {
	var err error
	lookupMatch := make(map[string]*LookupCount)
	for _, token := range tokens {
		lookupMatch[token] = NewLookupCount(token)
	}
	var re *regexp.Regexp
	if len(keyRe) > 0 {
		re, err = regexp.Compile(keyRe)
		if err != nil {
			return nil, fmt.Errorf("while compiling regex %s: %v", keyRe, err)
		}
	}
	return &LookupTokensState{
		LookupTbl:   lookupTbl,
		KeyRe:       re,
		LookupMatch: lookupMatch,
	}, nil
}

func (ctx *BuilderContext) NewAnalyzeState(columnName string, columnPos int, inputColumns *map[string]int, spec *TransformationSpec) (*AnalyzeState, error) {

	if spec == nil || spec.AnalyzeConfig == nil || inputColumns == nil {
		return nil, fmt.Errorf("error: analyse Pipe Transformation spec is missing analyze_config section or input columns map is nil")
	}
	config := spec.AnalyzeConfig
	sp := ctx.schemaManager.schemaProviders[config.SchemaProvider]
	// Create analyze actions
	regexMatch := make(map[string]*RegexCount)
	for i := range config.RegexTokens {
		conf := &config.RegexTokens[i]
		rexp, err := regexp.Compile(conf.Rexpr)
		if err != nil {
			return nil, fmt.Errorf("while compiling regex %s: %v", conf.Name, err)
		}
		regexMatch[conf.Name] = NewRegexCount(rexp)
	}
	lookupState := make([]*LookupTokensState, 0)
	if len(config.LookupTokens) > 0 && ctx.lookupTableManager != nil {
		for i := range config.LookupTokens {
			lookupNode := &config.LookupTokens[i]
			lookupTable := ctx.lookupTableManager.LookupTableMap[lookupNode.Name]
			if lookupTable == nil {
				return nil, fmt.Errorf("error: lookup table %s not found (NewAlalyzeState)", lookupNode.Name)
			}
			state, err := NewLookupTokensState(lookupTable, lookupNode.KeyRe, lookupNode.Tokens)
			if err != nil {
				return nil, err
			}
			lookupState = append(lookupState, state)
		}
	}
	keywordMatch := make(map[string]*KeywordCount)
	for i := range config.KeywordTokens {
		kw := &config.KeywordTokens[i]
		keywordMatch[kw.Name] = NewKeywordCount(kw.Name, kw.Keywords)
	}
	functionMatch := make(map[string]*FunctionCount)
	for i := range config.FunctionTokens {
		conf := &config.FunctionTokens[i]
		f, err := NewFunctionCount(conf, sp)
		if err != nil {
			return nil, err
		}
		functionMatch[conf.Name] = f
	}

	// Determine which Wellford algo we need
	var lenWelford, valueWelford *WelfordAlgo
	cmap := *inputColumns

	// Column length
	_, ok := cmap["avr_length"]
	if !ok {
		_, ok = cmap["length_var"]
	}
	if ok {
		lenWelford = NewWelfordAlgo()
	}

	// Column value
	_, ok = cmap["avr_value"]
	if !ok {
		_, ok = cmap["value_var"]
	}
	if ok {
		valueWelford = NewWelfordAlgo()
	}

	var valueToDouble func(d any) (float64, error)
	_, ok1 := cmap["is_value_numeric_count"]
	_, ok2 := cmap["is_value_numeric_count_pct"]
	if ok1 || ok2 || valueWelford != nil {
		valueToDouble = ToDouble
	}

	return &AnalyzeState{
		ColumnName:     columnName,
		ColumnPos:      columnPos,
		DistinctValues: make(map[string]*DistinctCount),
		LenWelford:     lenWelford,
		ValueToDouble:  valueToDouble,
		ValueWelford:   valueWelford,
		RegexMatch:     regexMatch,
		LookupState:    lookupState,
		KeywordMatch:   keywordMatch,
		FunctionMatch:  functionMatch,
		Spec:           spec,
	}, nil
}

func (state *AnalyzeState) NewValue(value interface{}) error {
	state.TotalRowCount += 1
	if value == nil {
		state.NullCount += 1
		return nil
	}
	switch vv := value.(type) {
	case string:
		if strings.ToUpper(vv) == "NULL" {
			state.NullCount += 1
			return nil
		}
		return state.NewToken(vv)
	default:
		return state.NewToken(fmt.Sprintf("%v", value))
	}
}

func (state *AnalyzeState) NewToken(value string) error {
	// work on the upper case value of the token
	value = strings.ToUpper(value)
	
	// Distinct Values
	dv := state.DistinctValues[value]
	if dv == nil {
		dv = &DistinctCount{
			Value: value,
		}
		state.DistinctValues[value] = dv
	}
	dv.Count += 1

	// length Welford's Algo
	if state.LenWelford != nil {
		state.LenWelford.Update(float64(len(value)))
	}

	// Numeric Value / value Welford
	if state.ValueToDouble != nil {
		vv, err2 := state.ValueToDouble(value)
		if err2 == nil {
			state.ValueAsDoubleCount += 1
			if state.ValueWelford != nil {
				state.ValueWelford.Update(vv)
			}
		}
	}
	
	// Regex matches
	for _, reCount := range state.RegexMatch {
		if reCount.Rexpr.MatchString(value) {
			reCount.Count += 1
		}
	}
	
	// Lookup matches
	var row *[]interface{}
	var err error
	for _, lookupState := range state.LookupState {
		if lookupState.KeyRe != nil {
			key := lookupState.KeyRe.FindStringSubmatch(value)
			if len(key) > 1 {
				row, err = lookupState.LookupTbl.Lookup(&key[1])
			}
		} else {
			row, err = lookupState.LookupTbl.Lookup(&value)
		}
		if err != nil {
			return fmt.Errorf("while calling lookup, with key %s: %v", value, err)
		}
		// The first and only column returned is called tokens and is an array of string
		if row != nil {
			tokens, ok := (*row)[0].([]string)
			if !ok {
				return fmt.Errorf("error: lookup row first elm is not []string in AnalyzeState.NewToken")
			}
			for _, token := range tokens {
				lkCount := lookupState.LookupMatch[token]
				if lkCount != nil {
					lkCount.Count += 1
				}
			}
		}
	}

	// Keyword set matches
	for _, kwm := range state.KeywordMatch {
		for _, kw := range kwm.Keywords {
			if strings.Contains(value, kw) {
				kwm.Count += 1
				break
			}
		}
	}

	// Function matches
	for _, fm := range state.FunctionMatch {
		if fm.Function.Match(value) {
			fm.Count += 1
		}
	}

	return nil
}

// ParseDateMatchFunction is a match function to vaidate a date
type ParseDateMatchFunction struct {
	dateLayout      string
	yearLessThan    int
	yearGreaterThan int
}

// Match implements MatchFunction.
func (p *ParseDateMatchFunction) Match(value string) bool {
	if p == nil {
		return false
	}
	d, err := time.Parse(p.dateLayout, value)
	if err != nil {
		return false
	}
	if p.yearLessThan > 0 && d.Year() >= p.yearLessThan {
		return false
	}
	if p.yearGreaterThan > 0 && d.Year() < p.yearGreaterThan {
		return false
	}
	return true
}

func NewParseDateMatchFunction(fspec *FunctionTokenNode, sp SchemaProvider) (MatchFunction, error) {
	var ok bool
	layout := sp.ReadDateLayout()
	if layout == "" && fspec.Arguments != nil {
		layout, ok = fspec.Arguments["default_date_format"].(string)
		if !ok {
			return nil, fmt.Errorf("error: no date format available in NewParseDateMatchFunction")
		}
	}
	yearLT := 0
	yearGT := 0
	if fspec.Arguments != nil {
		flt, ok := fspec.Arguments["year_less_than"].(float64)
		if !ok {
			yearLT = 0
		} else {
			yearLT = int(flt)
		}
		fgt, ok := fspec.Arguments["year_greater_than"].(float64)
		if !ok {
			yearGT = 0
		} else {
			yearGT = int(fgt)
		}
	}
	return &ParseDateMatchFunction{
		dateLayout:      layout,
		yearLessThan:    yearLT,
		yearGreaterThan: yearGT,
	}, nil
}

// Welford's online algorithm
// https://en.wikipedia.org/wiki/Algorithms_for_calculating_variance#Online
// An example Python implementation for Welford's algorithm is given below.
//
// # For a new value new_value, compute the new count, new mean, the new M2.
// # mean accumulates the mean of the entire dataset
// # M2 aggregates the squared distance from the mean
// # count aggregates the number of samples seen so far
// def update(existing_aggregate, new_value):
//     (count, mean, M2) = existing_aggregate
//     count += 1
//     delta = new_value - mean
//     mean += delta / count
//     delta2 = new_value - mean
//     M2 += delta * delta2
//     return (count, mean, M2)

// # Retrieve the mean, variance and sample variance from an aggregate
// def finalize(existing_aggregate):
//     (count, mean, M2) = existing_aggregate
//     if count < 2:
//         return float("nan")
//     else:
//         (mean, variance, sample_variance) = (mean, M2 / count, M2 / (count - 1))
//         return (mean, variance, sample_variance)

type WelfordAlgo struct {
	Mean  float64
	M2    float64
	Count int
}

func NewWelfordAlgo() *WelfordAlgo {
	return &WelfordAlgo{}
}

func (w *WelfordAlgo) Update(value float64) {
	w.Count += 1
	delta := value - w.Mean
	w.Mean += delta / float64(w.Count)
	delta2 := value - w.Mean
	w.M2 += delta * delta2
}

func (w *WelfordAlgo) Finalize() (mean, variance float64) {
	return w.Mean, w.M2 / float64(w.Count)
}
