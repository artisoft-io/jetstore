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
	Name      string
	LookupTbl LookupTable
	Count     int
}

func NewLookupCount(name string, lookupTbl LookupTable) *LookupCount {
	return &LookupCount{
		Name:      name,
		LookupTbl: lookupTbl,
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
	LookupMatch    map[string]*LookupCount
	KeywordMatch   map[string]*KeywordCount
	RegexTokens    []string
	LookupTokens   []string
	TotalRowCount  int
	Spec           *TransformationSpec
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
	lookupMatch := make(map[string]*LookupCount)
	if spec.LookupTokens != nil {
		for i := range *spec.LookupTokens {
			lkConfig := &(*spec.LookupTokens)[i]
			for _, token := range lkConfig.Tokens {
				lookupMatch[token] = NewLookupCount(token, ctx.lookupTableManager.LookupTableMap[lkConfig.Name])
			}
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
		LookupMatch:    lookupMatch,
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
	for _, lkCount := range state.LookupMatch {
		row, err := lkCount.LookupTbl.Lookup(&value)
		if err != nil {
			return fmt.Errorf("while calling lookup %s, with key %s: %v", lkCount.Name, value, err)
		}
		// The first and only column returned is called tokens and is an array of string
		if row != nil {
			tokens, ok := (*row)[0].([]string)
			if !ok {
				return fmt.Errorf("error: lookup row first elm is not []string in AnalyzeState.NewToken")
			}
			for _, token := range tokens {
				lkCount := state.LookupMatch[token]
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
		outputRow[ctx.outputCh.columns["column_name"]] = state.ColumnName
		outputRow[ctx.outputCh.columns["column_pos"]] = state.ColumnPos
		outputRow[ctx.outputCh.columns["distinct_count"]] = len(state.DistinctValues)
		outputRow[ctx.outputCh.columns["null_count"]] = state.NullCount
		outputRow[ctx.outputCh.columns["total_count"]] = state.TotalRowCount
		avrLen, avrVar := state.Welford.Finalize()
		outputRow[ctx.outputCh.columns["avr_length"]] = avrLen
		outputRow[ctx.outputCh.columns["length_var"]] = avrVar
		// The context driven columns
		outputRow[ctx.outputCh.columns["processing_ticket"]] = ctx.env["$PROCESSING_TICKET"]
		outputRow[ctx.outputCh.columns["layout_name"]] = ctx.layoutName
		outputRow[ctx.outputCh.columns["session_id"]] = ctx.sessionId

		// The regex tokens
		for name, m := range state.RegexMatch {
			outputRow[ctx.outputCh.columns[name]] = m.Count
		}
		// The lookup tokens
		for name, m := range state.LookupMatch {
			outputRow[ctx.outputCh.columns[name]] = m.Count
		}
		// The keywords match
		for name, m := range state.KeywordMatch {
			outputRow[ctx.outputCh.columns[name]] = m.Count
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
	if spec == nil || spec.RegexTokens == nil || spec.LookupTokens == nil {
		return nil, fmt.Errorf("error: Analyze Pipe Transformation spec is missing regex aand/or lookup definition")
	}
	// Setup the output channel columns
	columns := []string{
		"column_name",
		"column_pos",
		"distinct_count",
		"null_count",
		"total_count",
		"avr_length",
		"length_var",
	}
	for i := range *spec.RegexTokens {
		node := &(*spec.RegexTokens)[i]
		columns = append(columns, node.Name)
	}
	for i := range *spec.LookupTokens {
		columns = append(columns, (*spec.LookupTokens)[i].Tokens...)
	}
	// Add the trailing columns that are context driven (if you change this, see above in done func)
	columns = append(columns, "processing_ticket", "layout_name", "session_id")
	//*
	log.Println("The Analyze output columns:")
	columnsMap := make(map[string]int)
	for i, c := range columns {
		//*
		log.Println(c)
		columnsMap[c] = i
	}
	outputCh.config.Columns = columns
	outputCh.columns = columnsMap

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
