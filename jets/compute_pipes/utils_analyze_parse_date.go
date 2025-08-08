package compute_pipes

import (
	"fmt"
	"log"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Parse Date Match Function

// ParseDateMatchFunction is a match function to vaidate dates.
// ParseDateMatchFunction implements FunctionCount interface
type ParseDateMatchFunction struct {
	parseDateConfig  *ParseDateSpec
	nbrSamplesSeen   int
	formatMatch      map[string]int
	otherFormatMatch map[string]int
	tokenMatches     map[string]int
	minMaxDateFormat string
	minMax           *minMaxDateValue
	seenCache        map[string]*pdCache
}
type minMaxDateValue struct {
	minValue time.Time
	maxValue time.Time
	count    int
}

type pdCache struct {
	tm  time.Time
	otm time.Time
	fmt string
}

// Match implements the match function.
func (pd ParseDateFTSpec) CheckYearRange(tm time.Time) bool {
	if pd.YearLessThan > 0 && tm.Year() >= pd.YearLessThan {
		return false
	}
	if pd.YearGreaterThan > 0 && tm.Year() < pd.YearGreaterThan {
		return false
	}
	return true
}

// ParseDateDateFormat returns the first match of [value] amongs the [dateFormats]
func ParseDateDateFormat(dateFormats []string, value string) (tm time.Time, fmt string) {
	var err error
	for _, fmt = range dateFormats {
		tm, err = time.Parse(fmt, value)
		if err == nil {
			return
		}
	}
	return time.Time{}, ""
}

// ParseDateMatchFunction implements FunctionCount interface
func (p *ParseDateMatchFunction) NewValue(value string) {
	if p.nbrSamplesSeen >= p.parseDateConfig.DateSamplingMaxCount {
		// do nothing
		return
	}
	p.nbrSamplesSeen++

	var tm, otm time.Time
	var fmt string
	cachedValue := p.seenCache[value]
	switch {
	case cachedValue != nil:
		fmt = cachedValue.fmt
		tm = cachedValue.tm
		if !tm.IsZero() && len(fmt) > 0 {
			p.formatMatch[fmt] += 1
			goto parse_date_arguments
		}
		otm = cachedValue.otm
		if !otm.IsZero() {
			p.otherFormatMatch[fmt] += 1
		}
		return

	case p.parseDateConfig.UseJetstoreParser:
		// Use jetstore date parser
		d, _ := rdf.ParseDateStrict(value)
		if d != nil {
			tm = *d
		}
		if tm.IsZero() {
			return
		}
		p.seenCache[value] = &pdCache{tm: tm}

	default:
		// Check if any DateFormats match the value
		tm, fmt = ParseDateDateFormat(p.parseDateConfig.DateFormats, value)
		if !tm.IsZero() {
			p.formatMatch[fmt] += 1
			p.seenCache[value] = &pdCache{tm: tm, fmt: fmt}
		}
	}
	if tm.IsZero() {
		// Check Other Date Format
		otm, fmt = ParseDateDateFormat(p.parseDateConfig.OtherDateFormats, value)
		if otm.IsZero() {
			return
		}
		p.otherFormatMatch[fmt] += 1
		p.seenCache[value] = &pdCache{otm: otm, fmt: fmt}
		return
	}
	// Set min/max values
	if p.minMax.minValue.IsZero() || tm.Before(p.minMax.minValue) {
		p.minMax.minValue = tm
	}
	if p.minMax.maxValue.IsZero() || tm.After(p.minMax.maxValue) {
		p.minMax.maxValue = tm
	}
	p.minMax.count += 1

parse_date_arguments:
	for _, args := range p.parseDateConfig.ParseDateArguments {
		if args.CheckYearRange(tm) {
			p.tokenMatches[args.Token] += 1
		}
	}
}

func (p *ParseDateMatchFunction) GetMinMaxValues() *MinMaxValue {
	if p == nil || p.minMax == nil {
		return nil
	}
	if p.minMax.minValue.IsZero() || p.minMax.maxValue.IsZero() {
		return nil
	}
	return &MinMaxValue{
		MinValue:   p.minMax.minValue.Format(p.minMaxDateFormat),
		MaxValue:   p.minMax.maxValue.Format(p.minMaxDateFormat),
		MinMaxType: "date",
		HitCount:   float64(p.minMax.count) / float64(p.nbrSamplesSeen),
	}
}

func (p *ParseDateMatchFunction) Done(ctx *AnalyzeTransformationPipe, outputRow []any) error {
	if p.minMax != nil {
		ipos, ok := (*ctx.outputCh.columns)["min_date"]
		if ok {
			outputRow[ipos] = p.minMax.minValue
		}
		ipos, ok = (*ctx.outputCh.columns)["max_date"]
		if ok {
			outputRow[ipos] = p.minMax.maxValue
		}
	}
	var ratioFactor float64
	if p.nbrSamplesSeen == 0 {
		ratioFactor = 100 / float64(p.nbrSamplesSeen)
	}
	for token, count := range p.tokenMatches {
		ipos, ok := (*ctx.outputCh.columns)[token]
		if ok {
			if ratioFactor > 0 {
				outputRow[ipos] = float64(count) * ratioFactor
			} else {
				outputRow[ipos] = -1.0
			}
		}
	}
	// Find the winning format / other format
	
	return nil
}

func NewParseDateMatchFunction(fspec *FunctionTokenNode, sp SchemaProvider) (*ParseDateMatchFunction, error) {
	parseDateConfig := fspec.ParseDateConfig
	if parseDateConfig == nil {
		return nil, fmt.Errorf("configuration error: analyze parse_date function is missing parse_date_config element")
	}
	// Determine the date format to use if not provided in DateFormats
	if !parseDateConfig.UseJetstoreParser && len(parseDateConfig.DateFormats) == 0 {
		var spLayout string
		if sp != nil {
			spLayout = sp.ReadDateLayout()
		}
		if len(spLayout) > 0 {
			parseDateConfig.DateFormats = append(parseDateConfig.DateFormats, spLayout)
		} else {
			log.Println("WARNING: analyze parse_date function has no date format configured, using jetstore internal date parser")
			parseDateConfig.UseJetstoreParser = true
		}
	}
	format := "2006-01-02"
	if len(parseDateConfig.MinMaxDateFormat) > 0 {
		format = parseDateConfig.MinMaxDateFormat
	}
	return &ParseDateMatchFunction{
		parseDateConfig:  parseDateConfig,
		minMaxDateFormat: format,
		minMax:           &minMaxDateValue{},
		tokenMatches:     make(map[string]int),
		formatMatch:      make(map[string]int),
		otherFormatMatch: make(map[string]int),
		seenCache:        make(map[string]*pdCache),
	}, nil
}
