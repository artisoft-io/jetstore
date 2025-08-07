package compute_pipes

import (
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

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

func (p *ParseDateMatchFunction) GetLargeValue() *LargeValue {
	return nil
}

func NewParseDateMatchFunction(fspec *FunctionTokenNode, sp SchemaProvider) (FunctionCount, error) {
	// var spLayout string
	// if sp != nil {
	// 	spLayout = sp.ReadDateLayout()
	// }
	// matches := make(map[string]int)
	// matchers := make([]*ParseDateMatcher, 0, len(fspec.ParseDateArguments))
	// for i := range fspec.ParseDateArguments {
	// 	config := &fspec.ParseDateArguments[i]
	// 	var layout string

	// 	switch {
	// 	case config.UseJetstoreParser:
	// 	case len(config.DateFormat) > 0:
	// 		layout = config.DateFormat
	// 	case len(spLayout) > 0:
	// 		layout = spLayout
	// 	case len(config.DefaultDateFormat) > 0:
	// 		layout = config.DefaultDateFormat
	// 	}
	// 	matches[config.Token] = 0
	// 	matchers = append(matchers, &ParseDateMatcher{
	// 		token:           config.Token,
	// 		dateLayout:      layout,
	// 		yearLessThan:    config.YearLessThan,
	// 		yearGreaterThan: config.YearGreaterThan,
	// 	})
	// }
	// format := "2006-01-02"
	// if len(fspec.MinMaxDateFormat) > 0 {
	// 	format = fspec.MinMaxDateFormat
	// }
	return &ParseDateMatchFunction{
		// matchers:         matchers,
		// matches:          matches,
		// minMaxDateFormat: format,
		minMax:           &minMaxDateValue{},
	}, nil
}

