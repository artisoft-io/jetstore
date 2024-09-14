package compute_pipes

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

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

// Analyze data TransformationSpec implementing PipeTransformationEvaluator interface
type AnalyzeState struct {
	ColumnName     string
	ColumnPos      int
	DistinctValues map[string]*DistinctCount
	NullCount      int
	Welford        *WelfordAlgo
	RegexMatch     map[string]*RegexCount
	LookupState    []*LookupTokensState
	KeywordMatch   map[string]*KeywordCount
	RegexTokens    []string
	LookupTokens   []string
	TotalRowCount  int
	Spec           *TransformationSpec
}

type LookupTokensState struct {
	LookupTbl   LookupTable
	LookupMatch map[string]*LookupCount
}

func NewLookupTokensState(lookupTbl LookupTable, tokens []string) *LookupTokensState {
	lookupMatch := make(map[string]*LookupCount)
	for _, token := range tokens {
		lookupMatch[token] = NewLookupCount(token)
	}
	return &LookupTokensState{
		LookupTbl:   lookupTbl,
		LookupMatch: lookupMatch,
	}
}

func (ctx *BuilderContext) NewAnalyzeState(columnName string, columnPos int, spec *TransformationSpec) (*AnalyzeState, error) {
	regexMatch := make(map[string]*RegexCount)
	if spec.RegexTokens != nil {
		for i := range *spec.RegexTokens {
			conf := &(*spec.RegexTokens)[i]
			rexp, err := regexp.Compile(conf.Rexpr)
			if err != nil {
				return nil, fmt.Errorf("while compiling regex %s: %v", conf.Name, err)
			}
			regexMatch[conf.Name] = NewRegexCount(rexp)
		}
	}
	lookupState := make([]*LookupTokensState, 0)
	if spec.LookupTokens != nil && ctx.lookupTableManager != nil {
		for i := range *spec.LookupTokens {
			lookupNode := &(*spec.LookupTokens)[i]
			lookupTable := ctx.lookupTableManager.LookupTableMap[lookupNode.Name]
			if lookupTable == nil {
				return nil, fmt.Errorf("error: lookup table %s not found (NewAlalyzeState)", lookupNode.Name)
			}
			lookupState = append(lookupState, NewLookupTokensState(lookupTable, lookupNode.Tokens))
		}
	}
	keywordMatch := make(map[string]*KeywordCount)
	if spec.KeywordTokens != nil {
		for i := range *spec.KeywordTokens {
			kw := &(*spec.KeywordTokens)[i]
			keywordMatch[kw.Name] = NewKeywordCount(kw.Name, kw.Keywords)
		}
	}

	return &AnalyzeState{
		ColumnName:     columnName,
		ColumnPos:      columnPos,
		DistinctValues: make(map[string]*DistinctCount),
		Welford:        NewWelfordAlgo(),
		RegexMatch:     regexMatch,
		LookupState:    lookupState,
		KeywordMatch:   keywordMatch,
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
	// Distinct Values
	dv := state.DistinctValues[value]
	if dv == nil {
		dv = &DistinctCount{
			Value: value,
		}
		state.DistinctValues[value] = dv
	}
	dv.Count += 1
	// Welford's Algo
	if state.Welford != nil {
		state.Welford.Update(float64(len(value)))
	}
	// Regex matches
	for _, reCount := range state.RegexMatch {
		if reCount.Rexpr.MatchString(value) {
			reCount.Count += 1
		}
	}
	// Lookup matches
	for _, lookupState := range state.LookupState {
		row, err := lookupState.LookupTbl.Lookup(&value)
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

	return nil
}

type AnalyzeTransformationPipe struct {
	cpConfig     *ComputePipesConfig
	source       *InputChannel
	outputCh     *OutputChannel
	analyzeState []interface{}
	layoutName   string
	spec         *TransformationSpec
	env          map[string]interface{}
	sessionId    string
	doneCh       chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *AnalyzeTransformationPipe) apply(input *[]interface{}) error {
	var err error
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in AnalyzeTransformationPipe")
	}
	if len(ctx.layoutName) == 0 {
		pos, ok := ctx.source.columns["layout_name"]
		if ok {
			name, ok := (*input)[pos].(string)
			if ok {
				ctx.layoutName = name
			}
		}
	}
	for i := range *input {
		analyzeState := ctx.analyzeState[i].(*AnalyzeState)
		err = analyzeState.NewValue((*input)[i])
		if err != nil {
			return fmt.Errorf("while calling NewValue on AnalyzeState: %v", err)
		}
	}
	return nil
}

// Analysis complete, now send out the results to ctx.outputCh.
// A row is produced for each column state in ctx.analyzeState.

func (ctx *AnalyzeTransformationPipe) done() error {
	// For each column state in ctx.analyzeState, send out a row to ctx.outputCh
	for i := range ctx.analyzeState {
		state, ok := ctx.analyzeState[i].(*AnalyzeState)
		if !ok {
			return fmt.Errorf("error: expecting type *AnalyzeState in AnalyzeTransformationPipe.done()")
		}
		var n int
		for i := range *ctx.spec.LookupTokens {
			n += len((*ctx.spec.LookupTokens)[i].Tokens)
		}
		outputRow := make([]interface{}, len(ctx.outputCh.columns))

		// The first base columns
		var ipos int
		ipos, ok = ctx.outputCh.columns["column_name"]
		if ok {
			outputRow[ipos] = state.ColumnName
		}
		ipos, ok = ctx.outputCh.columns["column_pos"]
		if ok {
			outputRow[ipos] = state.ColumnPos
		}
		ipos, ok = ctx.outputCh.columns["distinct_count"]
		if ok {
			outputRow[ipos] = len(state.DistinctValues)
		}
		ipos, ok = ctx.outputCh.columns["null_count"]
		if ok {
			outputRow[ipos] = state.NullCount
		}
		ipos, ok = ctx.outputCh.columns["total_count"]
		if ok {
			outputRow[ipos] = state.TotalRowCount
		}

		avrLen, avrVar := state.Welford.Finalize()
		ipos, ok = ctx.outputCh.columns["avr_length"]
		if ok {
			outputRow[ipos] = avrLen
		}
		ipos, ok = ctx.outputCh.columns["length_var"]
		if ok {
			outputRow[ipos] = avrVar
		}
		// The context driven columns
		ipos, ok = ctx.outputCh.columns["processing_ticket"]
		if ok {
			outputRow[ipos] = ctx.env["$PROCESSING_TICKET"]
		}
		ipos, ok = ctx.outputCh.columns["layout_name"]
		if ok {
			outputRow[ipos] = ctx.layoutName
		}
		ipos, ok = ctx.outputCh.columns["session_id"]
		if ok {
			outputRow[ipos] = ctx.sessionId
		}

		// The regex tokens
		for name, m := range state.RegexMatch {
			ipos, ok = ctx.outputCh.columns[name]
			if ok {
				outputRow[ipos] = m.Count
			}
		}

		//***
		log.Printf("Column: %s lookup tokens:", state.ColumnName)
		for token,count := range state.LookupState[0].LookupMatch {
			log.Printf("     token: %s, count: %d", token, count.Count)
		}

		// The lookup tokens
		for _, lookupState := range state.LookupState {
			for name, m := range lookupState.LookupMatch {
				ipos, ok := ctx.outputCh.columns[name]
				if ok {
					outputRow[ipos] = m.Count
				}
			}
		}

		// The keywords match
		for name, m := range state.KeywordMatch {
			ipos, ok = ctx.outputCh.columns[name]
			if ok {
				outputRow[ipos] = m.Count
			}
		}
		// Send the column result to output
		// fmt.Println("**!@@ ** Send AGGREGATE Result to", ctx.outputCh.config.Name)
		select {
		case ctx.outputCh.channel <- outputRow:
		case <-ctx.doneCh:
			log.Println("AnalyzeTransform interrupted")
		}
	}

	fmt.Println("**!@@ ** Send ANALYZE Result to", ctx.outputCh.config.Name, "DONE")
	return nil
}

func (ctx *AnalyzeTransformationPipe) finally() {}

func (ctx *BuilderContext) NewAnalyzeTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*AnalyzeTransformationPipe, error) {
	var err error
	if spec == nil || spec.RegexTokens == nil || spec.LookupTokens == nil || spec.KeywordTokens == nil {
		return nil, fmt.Errorf("error: Analyze Pipe Transformation spec is missing regex, lookup,  and/or keywords definition")
	}

	// Set up the AnalyzeState for each input column
	analyzeState := make([]interface{}, len(source.config.Columns))
	for i := range analyzeState {
		analyzeState[i], err = ctx.NewAnalyzeState(source.config.Columns[i], i, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling NewAnalyzeState for column %s: %v",
				source.config.Columns[i], err)
		}
	}
	return &AnalyzeTransformationPipe{
		cpConfig:     ctx.cpConfig,
		source:       source,
		outputCh:     outputCh,
		analyzeState: analyzeState,
		spec:         spec,
		env:          ctx.env,
		sessionId:    ctx.sessionId,
		doneCh:       ctx.done,
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
