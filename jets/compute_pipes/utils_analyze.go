package compute_pipes

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
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
	HitCount   float64
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
	ParseDate      *ParseDateMatchFunction
	ParseDouble    *ParseDoubleMatchFunction
	ParseText      *ParseTextMatchFunction
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

	var pdate *ParseDateMatchFunction
	var pdouble *ParseDoubleMatchFunction
	var ptext *ParseTextMatchFunction
	var err error
	for i := range config.FunctionTokens {
		conf := &config.FunctionTokens[i]
		switch conf.Type {
		case "parse_date":
			pdate, err = NewParseDateMatchFunction(conf, sp)

		case "parse_double":
			pdouble, err = NewParseDoubleMatchFunction(conf)

		case "parse_text":
			ptext, err = NewParseTextMatchFunction(conf)

		default:
			return nil, fmt.Errorf("error: unknown function_name '%s' in Analyze operator", conf.Type)
		}
		if err != nil {
			return nil, err
		}
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
		ParseDate:      pdate,
		ParseDouble:    pdouble,
		ParseText:      ptext,
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
	if state.ParseDate != nil {
		state.ParseDate.NewValue(value)
	}
	if state.ParseDouble != nil {
		state.ParseDouble.NewValue(value)
	}
	if state.ParseText != nil {
		state.ParseText.NewValue(value)
	}

	return nil
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
