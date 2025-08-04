package compute_pipes

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
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
	Rexpr            *regexp.Regexp
	UseScrubbedValue bool
	Count            int
}

func NewRegexCount(re *regexp.Regexp, useScrubbedValue bool) *RegexCount {
	return &RegexCount{
		Rexpr:            re,
		UseScrubbedValue: useScrubbedValue,
	}
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

// MinValue and MaxValue are expressed as string but represent one of:
//   - min/max date when MinMaxType == "date"
//   - min/max double when MinMaxType == "double"
//   - min/max length when MinMaxType == "text"
type MinMaxValue struct {
	MinValue   string
	MaxValue   string
	MinMaxType string
	HitCount   int
}

// LargeValue is expressed as string but represent one of:
//   - min/max date when MinMaxType == "date"
//   - min/max double when MinMaxType == "double"
//   - min/max length when MinMaxType == "text"
//
// Note: currently only MinMaxType == "double" is implementing LargeValue check
type LargeValue struct {
	Value     string
	ValueType string
	HitCount  float64
}
type FunctionCount interface {
	NewValue(value string)
	GetMatchToken() map[string]int
	GetMinMaxValues() *MinMaxValue
	GetLargeValue() *LargeValue
}

func NewFunctionCount(fspec *FunctionTokenNode, sp SchemaProvider) (FunctionCount, error) {
	var fnc FunctionCount
	var err error
	switch fspec.Type {
	case "parse_date":
		fnc, err = NewParseDateMatchFunction(fspec, sp)

	case "parse_double":
		fnc, err = NewParseDoubleMatchFunction(fspec)

	case "parse_text":
		fnc, err = NewParseTextMatchFunction(fspec)

	default:
		return nil, fmt.Errorf("error: unknown function_name '%s' in Analyze operator", fspec.Type)
	}
	return fnc, err
}

// Analyze data TransformationSpec implementing PipeTransformationEvaluator interface
type AnalyzeState struct {
	ColumnName     string
	ColumnPos      int
	DistinctValues map[string]*DistinctCount
	NullCount      int
	LenWelford     *WelfordAlgo
	CharToScrub    map[rune]bool
	RegexMatch     map[string]*RegexCount
	LookupState    []*LookupTokensState
	KeywordMatch   map[string]*KeywordCount
	FunctionMatch  []FunctionCount
	TotalRowCount  int
	Spec           *TransformationSpec
}

type LookupTokensState struct {
	LookupTbl        LookupTable
	KeyRe            *regexp.Regexp
	LookupMatch      map[string]*LookupCount
	MultiTokensMatch []MultiTokensNode
}

func (state *LookupTokensState) LookupValue(value *string) ([]string, error) {
	row, err := state.LookupTbl.Lookup(value)
	if err != nil {
		return nil, fmt.Errorf("while calling lookup, with key %s: %v", *value, err)
	}
	if row == nil {
		return nil, nil
	}
	// The first and only column returned is called tokens and is an array of string
	tokens, ok := (*row)[0].([]string)
	if !ok {
		return nil, fmt.Errorf("error: lookup row first elm is not []string in LookupTokensState.LookupValue (AnalyzeState)")
	}
	return tokens, nil
}
func (state *LookupTokensState) NewValue(value *string) error {
	var tokens []string
	var err error
	if state.KeyRe != nil {
		key := state.KeyRe.FindStringSubmatch(*value)
		if len(key) > 1 {
			tokens, err = state.LookupValue(&key[1])
		}
	} else {
		tokens, err = state.LookupValue(value)
	}
	if err != nil {
		return err
	}
	for _, token := range tokens {
		lkCount := state.LookupMatch[token]
		if lkCount != nil {
			lkCount.Count += 1
		}
	}
	// fmt.Printf("*** Entering multi token matches\n")
	// Look for multi token matches
	if len(state.MultiTokensMatch) == 0 {
		return nil
	}
	splitValues := strings.Fields(*value)
	if len(splitValues) < 2 {
		return nil
	}
	// Sort the slice by string length in descending order
	sort.Slice(splitValues, func(i, j int) bool {
		return len(splitValues[i]) > len(splitValues[j])
	})
	// remove single letter words and ',' suffixes
	for i := range splitValues {
		if len(splitValues[i]) < 2 {
			splitValues = splitValues[0:i]
			break
		}
		splitValues[i] = strings.TrimSuffix(splitValues[i], ",")
	}
	// check if this match any multi token config
	n := len(splitValues)
	// fmt.Printf("*** Got %d split values: %v\n", len(splitValues), splitValues)
multiTokensLoop:
	for i := range state.MultiTokensMatch {
		if state.MultiTokensMatch[i].NbrTokens == n {
			// fmt.Printf("*** Got MultiTokensMatch %s with split values %v\n", state.MultiTokensMatch[i].Name, splitValues)

		splitValueLoop:
			for _, value := range splitValues {
				tokens, _ = state.LookupValue(&value)
				// fmt.Printf("*** Split value: %s, got tokens %v\n", value, tokens)
				// fmt.Printf("*** MultiTokensMatch tokenMap: %v\n", state.MultiTokensMatch[i].tokenMap)
				for _, token := range tokens {
					if state.MultiTokensMatch[i].tokenMap[token] {
						continue splitValueLoop
					}
				}
				// Did not match
				continue multiTokensLoop
			}
			// Got multi token to all match
			// fmt.Printf("*** Got multi token match %s\n", state.MultiTokensMatch[i].Name)
			lkCount := state.LookupMatch[state.MultiTokensMatch[i].Name]
			if lkCount != nil {
				lkCount.Count += 1
			}
		}
	}
	return nil
}

func NewLookupTokensState(lookupTbl LookupTable, lookupNode *LookupTokenNode) (*LookupTokensState, error) {
	var err error
	lookupMatch := make(map[string]*LookupCount)
	for _, token := range lookupNode.Tokens {
		lookupMatch[token] = NewLookupCount(token)
	}
	for i := range lookupNode.MultiTokensMatch {
		name := lookupNode.MultiTokensMatch[i].Name
		lookupMatch[name] = NewLookupCount(name)
		lookupNode.MultiTokensMatch[i].tokenMap = make(map[string]bool,
			len(lookupNode.MultiTokensMatch[i].Tokens))
		for _, token := range lookupNode.MultiTokensMatch[i].Tokens {
			lookupNode.MultiTokensMatch[i].tokenMap[token] = true
		}
	}
	var re *regexp.Regexp
	if len(lookupNode.KeyRe) > 0 {
		re, err = regexp.Compile(lookupNode.KeyRe)
		if err != nil {
			return nil, fmt.Errorf("while compiling regex %s: %v", lookupNode.KeyRe, err)
		}
	}
	return &LookupTokensState{
		LookupTbl:        lookupTbl,
		KeyRe:            re,
		LookupMatch:      lookupMatch,
		MultiTokensMatch: lookupNode.MultiTokensMatch,
	}, nil
}

func (ctx *BuilderContext) NewAnalyzeState(columnName string, columnPos int, inputColumns *map[string]int, spec *TransformationSpec) (*AnalyzeState, error) {

	if spec == nil || spec.AnalyzeConfig == nil || inputColumns == nil {
		return nil, fmt.Errorf("error: analyze Pipe Transformation spec is missing analyze_config section or input columns map is nil")
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
		regexMatch[conf.Name] = NewRegexCount(rexp, conf.UseScrubbedValue)
	}

	lookupState := make([]*LookupTokensState, 0)
	if len(config.LookupTokens) > 0 && ctx.lookupTableManager != nil {
		for i := range config.LookupTokens {
			lookupNode := &config.LookupTokens[i]
			lookupTable := ctx.lookupTableManager.LookupTableMap[lookupNode.Name]
			if lookupTable == nil {
				return nil, fmt.Errorf("error: lookup table %s not found (NewAlalyzeState)", lookupNode.Name)
			}
			state, err := NewLookupTokensState(lookupTable, lookupNode)
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
	// make a map of rune to scrub
	toScrub := make(map[rune]bool)
	for _, r := range config.ScrubChars {
		toScrub[r] = true
	}

	return &AnalyzeState{
		ColumnName:     columnName,
		ColumnPos:      columnPos,
		DistinctValues: make(map[string]*DistinctCount),
		CharToScrub:    toScrub,
		LenWelford:     lenWelford,
		RegexMatch:     regexMatch,
		LookupState:    lookupState,
		KeywordMatch:   keywordMatch,
		FunctionMatch:  functionMatch,
		Spec:           spec,
	}, nil
}

func (state *AnalyzeState) NewValue(value any) error {
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
	// Scrub some unmeaningful chars
	// Note: assuming ascii, so working with bytes
	// Note: CharToScrub may contain the space char so
	// the operator MultiTokensNode will use the original value
	// Note: Some operators use value and other use scrubbedValue
	scrubbedValue := value
	if len(state.CharToScrub) > 0 {
		scrubbed := make([]rune, 0, len(value))
		for _, r := range value {
			if !state.CharToScrub[r] {
				scrubbed = append(scrubbed, r)
			}
		}
		scrubbedValue = string(scrubbed)
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
		if reCount.UseScrubbedValue {
			if reCount.Rexpr.MatchString(scrubbedValue) {
				reCount.Count += 1
			}
		} else {
			if reCount.Rexpr.MatchString(value) {
				reCount.Count += 1
			}
		}
	}

	// Lookup matches
	var err error
	for _, lookupState := range state.LookupState {
		err = lookupState.NewValue(&value)
		if err != nil {
			return nil
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

// Parse Date Match Function

// ParseDateMatchFunction is a match function to vaidate dates.
// ParseDateMatchFunction implements FunctionCount interface
type ParseDateMatchFunction struct {
	matchers         []*ParseDateMatcher
	matches          map[string]int
	minMaxDateFormat string
	minMax           *minMaxDateValue
}

type ParseDateMatcher struct {
	token           string
	dateLayout      string
	yearLessThan    int
	yearGreaterThan int
}
type minMaxDateValue struct {
	minValue *time.Time
	maxValue *time.Time
	count    int
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
			d, err = rdf.ParseDateStrict(value)
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
	gotMatch := false
	parsedDate := make(map[string]*time.Time)
	for _, pd := range p.matchers {
		if pd.Match(value, parsedDate) {
			p.matches[pd.token] += 1
			gotMatch = true
		}
	}
	if gotMatch {
		for _, d := range parsedDate {
			if d != nil {
				if p.minMax.minValue == nil || d.Before(*p.minMax.minValue) {
					p.minMax.minValue = d
				}
				if p.minMax.maxValue == nil || d.After(*p.minMax.maxValue) {
					p.minMax.maxValue = d
				}
				p.minMax.count += 1
				break
			}
		}
	}
}

func (p *ParseDateMatchFunction) GetMatchToken() map[string]int {
	if p == nil {
		return nil
	}
	return p.matches
}

func (p *ParseDateMatchFunction) GetMinMaxValues() *MinMaxValue {
	if p == nil || p.minMax == nil {
		return nil
	}
	if p.minMax.minValue == nil || p.minMax.maxValue == nil {
		return nil
	}
	return &MinMaxValue{
		MinValue:   p.minMax.minValue.Format(p.minMaxDateFormat),
		MaxValue:   p.minMax.maxValue.Format(p.minMaxDateFormat),
		MinMaxType: "date",
		HitCount:   p.minMax.count,
	}
}

// *TODO ParseDateMatchFunction.GetLargeValue()
func (p *ParseDateMatchFunction) GetLargeValue() *LargeValue {
	return nil
}

func NewParseDateMatchFunction(fspec *FunctionTokenNode, sp SchemaProvider) (FunctionCount, error) {
	var spLayout string
	if sp != nil {
		spLayout = sp.ReadDateLayout()
	}
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
	format := "2006-01-02"
	if len(fspec.MinMaxDateFormat) > 0 {
		format = fspec.MinMaxDateFormat
	}
	return &ParseDateMatchFunction{
		matchers:         matchers,
		matches:          matches,
		minMaxDateFormat: format,
		minMax:           &minMaxDateValue{},
	}, nil
}

// Parse Double Match Function

type ParseDoubleMatchFunction struct {
	minMax      *minMaxDoubleValue
	largeValues *largeDoubleValue
}

type minMaxDoubleValue struct {
	minValue *float64
	maxValue *float64
	count    int
}

type largeDoubleValue struct {
	largeValue float64
	count      int
}

// ParseDoubleMatchFunction implements FunctionCount interface
func (p *ParseDoubleMatchFunction) NewValue(value string) {
	fvalue, err := strconv.ParseFloat(value, 64)
	if err == nil {
		if p.minMax.minValue == nil || fvalue < *p.minMax.minValue {
			p.minMax.minValue = &fvalue
		}
		if p.minMax.maxValue == nil || fvalue > *p.minMax.maxValue {
			p.minMax.maxValue = &fvalue
		}
		p.minMax.count += 1
		if p.largeValues != nil && fvalue >= p.largeValues.largeValue {
			p.largeValues.count++
		}
	}
}

func (p *ParseDoubleMatchFunction) GetMatchToken() map[string]int {
	return nil
}

func (p *ParseDoubleMatchFunction) GetLargeValue() *LargeValue {
	if p.largeValues == nil {
		return nil
	}
	return &LargeValue{
		Value:     strconv.FormatFloat(p.largeValues.largeValue, 'f', -1, 64),
		ValueType: "double",
		HitCount:  float64(p.largeValues.count),
	}
}

func (p *ParseDoubleMatchFunction) GetMinMaxValues() *MinMaxValue {
	if p == nil || p.minMax == nil {
		return nil
	}
	if p.minMax.minValue == nil || p.minMax.maxValue == nil {
		return nil
	}
	return &MinMaxValue{
		MinValue:   strconv.FormatFloat(*p.minMax.minValue, 'f', -1, 64),
		MaxValue:   strconv.FormatFloat(*p.minMax.maxValue, 'f', -1, 64),
		MinMaxType: "double",
		HitCount:   p.minMax.count,
	}
}

func NewParseDoubleMatchFunction(fspec *FunctionTokenNode) (FunctionCount, error) {
	var largeValue largeDoubleValue
	result := &ParseDoubleMatchFunction{
		minMax: &minMaxDoubleValue{},
	}
	if fspec.LargeDouble != nil {
		largeValue.largeValue = *fspec.LargeDouble
		result.largeValues = &largeValue
	}
	return result, nil
}

// Parse Text Match Function

type ParseTextMatchFunction struct {
	minMax *minMaxLength
}

type minMaxLength struct {
	minValue *int
	maxValue *int
	count    int
}

// ParseTextMatchFunction implements FunctionCount interface
func (p *ParseTextMatchFunction) NewValue(value string) {
	length := len(value)
	if p.minMax.minValue == nil || length < *p.minMax.minValue {
		p.minMax.minValue = &length
	}
	if p.minMax.maxValue == nil || length > *p.minMax.maxValue {
		p.minMax.maxValue = &length
	}
	p.minMax.count += 1
}

func (p *ParseTextMatchFunction) GetMatchToken() map[string]int {
	return nil
}

// *TODO ParseTextMatchFunction.GetLargeValue()
func (p *ParseTextMatchFunction) GetLargeValue() *LargeValue {
	return nil
}

func (p *ParseTextMatchFunction) GetMinMaxValues() *MinMaxValue {
	if p == nil || p.minMax == nil {
		return nil
	}
	if p.minMax.minValue == nil || p.minMax.maxValue == nil {
		return nil
	}
	return &MinMaxValue{
		MinValue:   strconv.FormatInt(int64(*p.minMax.minValue), 10),
		MaxValue:   strconv.FormatInt(int64(*p.minMax.maxValue), 10),
		MinMaxType: "text",
		HitCount:   p.minMax.count,
	}
}

func NewParseTextMatchFunction(fspec *FunctionTokenNode) (FunctionCount, error) {
	return &ParseTextMatchFunction{
		minMax: &minMaxLength{},
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
