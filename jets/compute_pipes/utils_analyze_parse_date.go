package compute_pipes

import (
	"bytes"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/artisoft-io/jetstore/jets/csv"
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

// Qualify as a date:
//   - len < 30
//   - contains digits, letters, space, comma, dash, slash, column, apostrophe
//
// Example of longest date to expect:
// 23 November 2025 13:10 AM
func DoesQualifyAsDate(value string) bool {
	if len(value) >= 30 {
		// fmt.Printf("*** DoesQualifyAsDate: sample too long\n")
		return false
	}
	for _, c := range value {
		switch {
		case unicode.IsDigit(c):
		case unicode.IsLetter(c):
		case c == ' ':
		case c == ',':
		case c == '-':
		case c == '/':
		case c == ':':
		case c == '\'':
		case c == '.':
		default:
			// fmt.Printf("*** DoesQualifyAsDate: invalid char\n")
			return false
		}
	}
	return true
}

// ParseDateMatchFunction implements FunctionCount interface
func (p *ParseDateMatchFunction) NewValue(value string) {
	fmt.Printf("*** Sample: %s\n", value)
	if p.nbrSamplesSeen >= p.parseDateConfig.DateSamplingMaxCount {
		// do nothing
		// fmt.Printf("*** Max samples reached @ %d samples, new value: %s\n", p.nbrSamplesSeen, value)
		return
	}
	p.nbrSamplesSeen++
	if !DoesQualifyAsDate(value) {
		return
	}
	var tm, otm time.Time
	var dateFmt string
	cachedValue := p.seenCache[value]
	switch {
	case cachedValue != nil:
		dateFmt = cachedValue.fmt
		tm = cachedValue.tm
		if !tm.IsZero() {
			if len(dateFmt) > 0 {
				p.formatMatch[dateFmt] += 1
			}
			// fmt.Printf("*** Got tm from cache w/ fmt: %s\n", dateFmt)
			goto parse_date_arguments
		}
		otm = cachedValue.otm
		if !otm.IsZero() {
			p.otherFormatMatch[dateFmt] += 1
			// fmt.Printf("*** Got otm from cache w/ fmt: %s\n", dateFmt)
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
		// fmt.Printf("*** Got tm match w/ jetstore date parser\n")
		p.seenCache[value] = &pdCache{tm: tm}

	default:
		// Check if any DateFormats match the value
		tm, dateFmt = ParseDateDateFormat(p.parseDateConfig.DateFormats, value)
		if !tm.IsZero() {
			p.formatMatch[dateFmt] += 1
			p.seenCache[value] = &pdCache{tm: tm, fmt: dateFmt}
			// fmt.Printf("*** Got tm Match w/ fmt: %s\n", dateFmt)
		}
	}
	if tm.IsZero() {
		// Check Other Date Format
		otm, dateFmt = ParseDateDateFormat(p.parseDateConfig.OtherDateFormats, value)
		if otm.IsZero() {
			return
		}
		p.otherFormatMatch[dateFmt] += 1
		p.seenCache[value] = &pdCache{otm: otm, fmt: dateFmt}
		// fmt.Printf("*** Got otm Match w/ fmt: %s\n", dateFmt)
		return
	}

parse_date_arguments:

	// Set min/max values
	if p.minMax.minValue.IsZero() || tm.Before(p.minMax.minValue) {
		p.minMax.minValue = tm
	}
	if p.minMax.maxValue.IsZero() || tm.After(p.minMax.maxValue) {
		p.minMax.maxValue = tm
	}
	p.minMax.count += 1

	for _, args := range p.parseDateConfig.ParseDateArguments {
		if args.CheckYearRange(tm) {
			p.tokenMatches[args.Token] += 1
			// fmt.Printf("*** Got CheckYearRange on token: %s\n", args.Token)
		}
	}
}

func (p *ParseDateMatchFunction) GetMinMaxValues() *MinMaxValue {
	if p == nil || p.minMax == nil {
		return nil
	}
	// fmt.Printf("*** GetMinMaxValues HitCount: %v/%v = %v\n", p.minMax.count, p.nbrSamplesSeen, float64(p.minMax.count)/float64(p.nbrSamplesSeen))
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

type matchCount struct {
	token string
	count int
}
func (m matchCount)String() string {
	return fmt.Sprintf("(%s: %d)", m.token, m.count)
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
	if p.nbrSamplesSeen > 0 {
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
	if p.parseDateConfig.TopPCTFormatMatch == 0 {
		p.parseDateConfig.TopPCTFormatMatch = 51
	}
	var matches []matchCount
	var sumCount int
	for token, count := range p.formatMatch {
		if count > 0 {
			matches = append(matches, matchCount{token: token, count: count})
			sumCount += count
		}
	}
	ml := len(matches)
	if ml > 0 {
		ipos, ok := (*ctx.outputCh.columns)[p.parseDateConfig.DateFormatToken]
		if ok {
			// Sort by count decreading, switching alhs and rhs in a and b assignment
			slices.SortFunc(matches, func(lhs, rhs matchCount) int {
				a := rhs.count
				b := lhs.count
				switch {
				case a < b:
					return -1
				case a > b:
					return 1
				default:
					return 0
				}
			})
			// fmt.Printf("*** Got matches: %v\n", matches)
			// Take top matches, if less than 4
			var formats []string
			var c int
			ct := int(float64(p.parseDateConfig.TopPCTFormatMatch) * float64(sumCount) / 100)
			for i := range matches {
				c += matches[i].count
				if c <= ct {
					formats = append(formats, matches[i].token)
				}
			}
			// save the formats
			lenf := len(formats)
			switch {
			case lenf == 1:
				outputRow[ipos] = formats[0]
				// fmt.Printf("*** Top Formats: %v\n", formats[0])
			case lenf > 1:
				if lenf > 3 {
					formats = formats[0:3]
				}
				var buf bytes.Buffer
				w := csv.NewWriter(&buf)
				err := w.Write(formats)
				if err != nil {
					return fmt.Errorf("while writing formats: %v", err)
				}
				w.Flush()
				txt := strings.TrimSuffix(buf.String(), "\n")
				// fmt.Printf("*** Top Formats: %v\n", txt)
				outputRow[ipos] = txt
			default:
				outputRow[ipos] = ""
				// fmt.Printf("*** Top Formats:\n")
			}
		}
	}
	// Other formats -- looking if any one is more than p.parseDateConfig.TopPCTFormatMatch of
	// total accepted samples
	matches = nil
	for token, count := range p.otherFormatMatch {
		if count > 0 {
			matches = append(matches, matchCount{token: token, count: count})
		}
	}
	ml = len(matches)
	if ml > 0 {
		ipos, ok := (*ctx.outputCh.columns)[p.parseDateConfig.OtherDateFormatToken]
		if ok {
			// Take matches
			var formats []string
			ct := int(float64(p.parseDateConfig.TopPCTFormatMatch) * float64(p.nbrSamplesSeen) / 100)
			for i := range matches {
				if matches[i].count >= ct {
					formats = append(formats, matches[i].token)
				}
			}
			// save the formats count
			l := len(formats)
			if l > 0 {
				outputRow[ipos] = len(formats)
			}
			// fmt.Printf("*** Nbr Other Formats: %d\n", l)
		}
	}

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
