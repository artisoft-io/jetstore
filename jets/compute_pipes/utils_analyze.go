package compute_pipes

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
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

type FunctionCount interface {
	NewValue(value string)
	GetMatchToken() map[string]int
}

func NewFunctionCount(fspec *FunctionTokenNode, sp SchemaProvider) (FunctionCount, error) {
	var fnc FunctionCount
	var err error
	switch fspec.Type {
	case "parse_date":
		fnc, err = NewParseDateMatchFunction(fspec, sp)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("error: unknown function_name '%s' in Analyze operator", fspec.Type)
	}
	return fnc, nil
}

// Analyze data TransformationSpec implementing PipeTransformationEvaluator interface
type AnalyzeState struct {
	ColumnName     string
	ColumnPos      int
	DistinctValues map[string]*DistinctCount
	NullCount      int
	LenWelford     *WelfordAlgo
	RegexMatch     map[string]*RegexCount
	LookupState    []*LookupTokensState
	KeywordMatch   map[string]*KeywordCount
	FunctionMatch  []FunctionCount
	TotalRowCount  int
	Spec           *TransformationSpec
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
	functionMatch := make([]FunctionCount, 0, len(config.FunctionTokens))
	for i := range config.FunctionTokens {
		conf := &config.FunctionTokens[i]
		f, err := NewFunctionCount(conf, sp)
		if err != nil {
			return nil, err
		}
		functionMatch = append(functionMatch, f)
	}

	// Determine which Wellford algo we need
	var lenWelford *WelfordAlgo
	cmap := *inputColumns

	// Column length
	_, ok := cmap["avr_length"]
	if !ok {
		_, ok = cmap["length_var"]
	}
	if ok {
		lenWelford = NewWelfordAlgo()
	}

	return &AnalyzeState{
		ColumnName:     columnName,
		ColumnPos:      columnPos,
		DistinctValues: make(map[string]*DistinctCount),
		LenWelford:     lenWelford,
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
		return state.NewToken(vv)
	default:
		return state.NewToken(fmt.Sprintf("%v", value))
	}
}

func (state *AnalyzeState) NewToken(value string) error {
	// work on the upper case value of the token
	value = strings.ToUpper(value)
	if value == "NULL" {
		state.NullCount += 1
		return nil
	}
	// Remove leading 0 when there is 4 or more of them
	if strings.HasPrefix(value, "0000") {
		value = strings.TrimLeft(value, "0")
		if len(value) == 0 {
			value = "0"
		}
	}

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
		fm.NewValue(value)
	}

	return nil
}

// ParseDateMatchFunction is a match function to vaidate dates.
// ParseDateMatchFunction implements FunctionCount interface
type ParseDateMatchFunction struct {
	matchers []*ParseDateMatcher
	matches  map[string]int
}

type ParseDateMatcher struct {
	token           string
	dateLayout      string
	yearLessThan    int
	yearGreaterThan int
}

// Match implements the match function.
func (pd *ParseDateMatcher) Match(value string, parsedDate map[string]*time.Time) bool {
	if pd == nil {
		return false
	}
	var err error
	d, ok := parsedDate[pd.dateLayout]
	if !ok {
		if len(pd.dateLayout) == 0 {
			// Use jetstore date parser
			d, err = rdf.ParseDate(value)
			if err != nil {
				d = nil
			}
		} else {
			v, err := time.Parse(pd.dateLayout, value)
			if err != nil {
				d = nil
			} else {
				d = &v
			}
		}
		parsedDate[pd.dateLayout] = d
	}
	if d == nil {
		return false
	}
	if pd.yearLessThan > 0 && d.Year() >= pd.yearLessThan {
		return false
	}
	if pd.yearGreaterThan > 0 && d.Year() < pd.yearGreaterThan {
		return false
	}
	return true
}

// ParseDateMatchFunction implements FunctionCount interface
func (p *ParseDateMatchFunction) NewValue(value string) {
	parsedDate := make(map[string]*time.Time)
	for _, pd := range p.matchers {
		if pd.Match(value, parsedDate) {
			p.matches[pd.token] += 1
		}
	}
}

func (p *ParseDateMatchFunction) GetMatchToken() map[string]int {
	if p == nil {
		return nil
	}
	return p.matches
}

func NewParseDateMatchFunction(fspec *FunctionTokenNode, sp SchemaProvider) (FunctionCount, error) {
	spLayout := sp.ReadDateLayout()
	matches := make(map[string]int)
	matchers := make([]*ParseDateMatcher, 0, len(fspec.ParseDateArguments))
	for i := range fspec.ParseDateArguments {
		config := &fspec.ParseDateArguments[i]
		var layout string

		switch {
		case config.UseJetstoreParser:
		case len(config.DateFormat) > 0:
			layout = config.DateFormat
		case len(spLayout) > 0:
			layout = spLayout
		case len(config.DefaultDateFormat) > 0:
			layout = config.DefaultDateFormat
		}
		matches[config.Token] = 0
		matchers = append(matchers, &ParseDateMatcher{
			token:           config.Token,
			dateLayout:      layout,
			yearLessThan:    config.YearLessThan,
			yearGreaterThan: config.YearGreaterThan,
		})
	}
	return &ParseDateMatchFunction{
		matchers: matchers,
		matches:  matches,
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
