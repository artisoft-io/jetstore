package compute_pipes

import (
	"bytes"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/date_utils"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Parse Date Match Function

// ParseDateMatchFunction is a match function to validate dates.
// ParseDateMatchFunction implements FunctionCount interface
type ParseDateMatchFunction struct {
	parseDateConfig   *ParseDateSpec
	nullDates         []time.Time
	nbrSamplesSeen    int
	nbrValidDatesSeen int
	formatMatch       map[string]int
	otherFormatMatch  map[string]int
	tokenMatches      map[string]int
	minMaxDateFormat  string
	minMax            *minMaxDateValue
	seenCache         map[string]*pdCache
}
type minMaxDateValue struct {
	minValue time.Time
	maxValue time.Time
	count    int
}

type pdCache struct {
	tm  time.Time
	otm time.Time
	fmt []string
}

// Match implements the match function, returns true when match.
func (pd ParseDateFTSpec) CheckYearRange(tm time.Time) bool {
	if pd.YearLessThan > 0 && tm.Year() >= pd.YearLessThan {
		// fmt.Printf("*** Year %d not less than %d\n", tm.Year(), pd.YearLessThan)
		return false
	}
	if pd.YearGreaterThan > 0 && tm.Year() < pd.YearGreaterThan {
		// fmt.Printf("*** Year %d not greater than %d\n", tm.Year(), pd.YearGreaterThan)
		return false
	}
	// fmt.Printf("*** Year %d out of range\n", tm.Year())
	return true
}

// ParseDateDateFormat returns the all the matches of the first subgroup of [value] amongs the [dateFormats]
func ParseDateDateFormat(dateFormats [][]string, value string) (time.Time, []string) {
	var tm time.Time
	var fmt []string
	for _, fmts := range dateFormats {
		for _, f := range fmts {
			tm2, err := date_utils.ParseDateTime(f, value)
			if err == nil {
				tm = tm2
				fmt = append(fmt, f)
			}
		}
		if len(fmt) > 0 {
			// fmt.Printf("*** Got match for value %s with format %s\n", value, f)
			return tm, fmt
		}
	}
	return time.Time{}, nil
}

// Qualify as a date:
//   - 6 < len < 30
//   - contains digits, letters, space, comma, dash, slash, column, apostrophe
//
// Example of longest date to expect:
// 23 November 2025 13:10 AM
func DoesQualifyAsDate(value string) bool {
	if len(value) > 29 || len(value) < 6 {
		// fmt.Printf("*** DoesQualifyAsDate: sample too long or too short\n")
		return false
	}
	hasDigits := false
	for _, c := range value {
		switch {
		case unicode.IsDigit(c):
			hasDigits = true
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
	return hasDigits
}

// ParseDateMatchFunction implements FunctionCount interface
func (p *ParseDateMatchFunction) NewValue(value string) bool {
	// fmt.Printf("*** ParseDate NewValue: %s\n", value)
	if p.parseDateConfig.DateSamplingMaxCount > 0 &&
		p.nbrSamplesSeen >= p.parseDateConfig.DateSamplingMaxCount {
		// do nothing
		// fmt.Printf("*** Max samples reached @ %d samples, new value: %s\n", p.nbrSamplesSeen, value)
		return false
	}
	p.nbrSamplesSeen++
	if !DoesQualifyAsDate(value) {
		// fmt.Printf("*** ParseDate: %s does not qualify as a date\n", value)
		return false
	}
	var tm, otm time.Time
	var dateFmt []string
	cachedValue := p.seenCache[value]
	switch {
	case cachedValue != nil:
		dateFmt = cachedValue.fmt
		tm = cachedValue.tm
		if !tm.IsZero() {
			for _, fmt := range dateFmt {
				if len(fmt) > 0 {
					p.formatMatch[fmt] += 1
				}
			}
			// fmt.Printf("*** Got tm from cache w/ fmt: %s for value %s\n", dateFmt, value)
			goto parse_date_arguments
		}
		otm = cachedValue.otm
		if !otm.IsZero() {
			for _, fmt := range dateFmt {
				if len(fmt) > 0 {
					p.otherFormatMatch[fmt] += 1
				}
			}
			// fmt.Printf("*** Got otm from cache w/ fmt: %s for value %s\n", dateFmt, value)
		}
		return false

	case p.parseDateConfig.UseJetstoreParser:
		// Use jetstore date parser
		d, _ := rdf.ParseDateStrict(value)
		if d != nil {
			tm = *d
		}
		if tm.IsZero() {
			return false
		}
		// fmt.Printf("*** Got tm match w/ jetstore date parser\n")
		p.seenCache[value] = &pdCache{tm: tm}

	default:
		// Check if any DateFormats match the value
		tm, dateFmt = ParseDateDateFormat(p.parseDateConfig.DateFormats, value)
		if !tm.IsZero() {
			// fmt.Printf("*** Got Match w/ fmt: %v for value %s\n", dateFmt, value)
			for _, fmt := range dateFmt {
				if len(fmt) > 0 {
					p.formatMatch[fmt] += 1
				}
			}
			p.seenCache[value] = &pdCache{tm: tm, fmt: dateFmt}
		}
	}

	if tm.IsZero() && len(p.parseDateConfig.OtherDateFormats) > 0 {
		// Check Other Date Format
		otm, dateFmt = ParseDateDateFormat(p.parseDateConfig.OtherDateFormats, value)
		if !otm.IsZero() && otm.Year() >= 1920 && otm.Year() <= 2100 {
			for _, fmt := range dateFmt {
				if len(fmt) > 0 {
					p.otherFormatMatch[fmt] += 1
				}
			}
			p.seenCache[value] = &pdCache{otm: otm, fmt: dateFmt}
			// fmt.Printf("*** Got otm Match w/ fmt: %s for value %s\n", dateFmt, value)
		}
	}

parse_date_arguments:
	if tm.IsZero() {
		// It's not a date
		return false
	}

	// Check if the date is in null dates list
	if slices.ContainsFunc(p.nullDates, tm.Equal) {
		// fmt.Printf("*** Date %v is in null dates list\n", tm)
		p.nbrSamplesSeen-- // do not count null dates in samples seen
		return true
	}

	// Set min/max values
	if p.minMax.minValue.IsZero() || tm.Before(p.minMax.minValue) {
		// fmt.Printf("*** Set minValue: %v, was %v\n", tm, p.minMax.minValue )
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
	// fmt.Printf("*** That was a valid date ***\n")
	p.nbrValidDatesSeen++
	return false
}

func (p *ParseDateMatchFunction) GetMinMaxValues() *MinMaxValue {
	if p == nil || p.minMax == nil {
		return nil
	}
	// fmt.Printf("*** GetMinMaxValues HitRatio: %v/%v = %v\n", p.minMax.count, p.nbrSamplesSeen, float64(p.minMax.count)/float64(p.nbrSamplesSeen))
	if p.minMax.minValue.IsZero() || p.minMax.maxValue.IsZero() {
		return nil
	}

	return &MinMaxValue{
		MinValue:   p.minMax.minValue.Format(p.minMaxDateFormat),
		MaxValue:   p.minMax.maxValue.Format(p.minMaxDateFormat),
		MinMaxType: "date",
		HitRatio:   float64(p.minMax.count) / float64(p.nbrSamplesSeen),
		NbrSamples: p.nbrSamplesSeen,
	}
}

type matchCount struct {
	token string
	count int
}

func (m matchCount) String() string {
	return fmt.Sprintf("(%s: %d)", m.token, m.count)
}

func (p *ParseDateMatchFunction) Done(ctx *AnalyzeTransformationPipe, outputRow []any) error {
	if p.minMax != nil {
		ipos, ok := (*ctx.outputCh.Columns)["min_date"]
		if ok {
			outputRow[ipos] = p.minMax.minValue
		}
		ipos, ok = (*ctx.outputCh.Columns)["max_date"]
		if ok {
			outputRow[ipos] = p.minMax.maxValue
		}
	}
	var ratioFactor float64
	if p.nbrSamplesSeen > 0 {
		ratioFactor = 100 / float64(p.nbrSamplesSeen)
	}
	for token, count := range p.tokenMatches {
		ipos, ok := (*ctx.outputCh.Columns)[token]
		if ok {
			if ratioFactor > 0 {
				outputRow[ipos] = float64(count) * ratioFactor
			} else {
				outputRow[ipos] = -1.0
			}
		}
	}

	var matches []matchCount
	for token, count := range p.formatMatch {
		if count > 0 {
			matches = append(matches, matchCount{token: token, count: count})
		}
	}
	ml := len(matches)
	// fmt.Printf("*** parseDateMatchFunction DONE: Got %d matches for formatMatches for %s\n", ml, outputRow[(*ctx.outputCh.Columns)["column_name"]])
	if ml > 0 {
		ipos, ok := (*ctx.outputCh.Columns)[p.parseDateConfig.DateFormatToken]
		if ok {
			// Sort by count decreasing, switching lhs and rhs in a and b assignment
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
			// Take top matches, up to 3. The first match must account for 75% of total valid dates seen,
			// fmt.Printf("*** Got matches: %v, 75%% 0f %d valid dates seen is %d\n", matches, p.nbrValidDatesSeen, int(0.75*float64(p.nbrValidDatesSeen)))
			var formats []string
			ct := int(0.75 * float64(p.nbrValidDatesSeen))
			if matches[0].count >= ct {
				for i := range matches {
					if i == 3 {
						break
					}
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
				// Set the token (e.g. dobRe, dateRe, etc to 0)
				for token := range p.tokenMatches {
					ipos, ok := (*ctx.outputCh.Columns)[token]
					if ok {
						outputRow[ipos] = 0.0
					}
				}
				// fmt.Printf("*** Top Formats:\n")
			}
		}
	}
	// Other formats -- looking if any one is more than 98% of total accepted samples
	matches = nil
	for token, count := range p.otherFormatMatch {
		if count > 0 {
			matches = append(matches, matchCount{token: token, count: count})
		}
	}
	ml = len(matches)
	// fmt.Printf("*** Got %d matches for otherFormatMatch\n", ml)
	ipos, ok := (*ctx.outputCh.Columns)[p.parseDateConfig.OtherDateFormatToken]
	if ok {
		if ml > 0 {
			// Take matches
			var formats []string
			ct := int(0.98 * float64(p.nbrSamplesSeen))
			for i := range matches {
				if matches[i].count >= ct {
					formats = append(formats, matches[i].token)
				}
			}
			// save the formats count
			outputRow[ipos] = len(formats)
			// fmt.Printf("*** Nbr Other Formats: %d\n", outputRow[ipos])
		} else {
			outputRow[ipos] = 0
		}
	}

	return nil
}

func (ctx *BuilderContext) NewParseDateMatchFunction(columnPos int, fspec *FunctionTokenNode, sp SchemaProvider) (*ParseDateMatchFunction, error) {
	parseDateConfig := fspec.ParseDateConfig
	if parseDateConfig == nil {
		return nil, fmt.Errorf("configuration error: analyze parse_date function is missing parse_date_config element")
	}

	// Check if using lookup to determine date format
	if parseDateConfig.DateFormatLookup != nil {
		// Check if it's a date column by looking up a the date format
		dateLookupTbl := ctx.lookupTableManager.LookupTableMap[parseDateConfig.DateFormatLookup.LookupName]
		if dateLookupTbl == nil {
			return nil, fmt.Errorf("error: anonymize date format lookup table %s not found", parseDateConfig.DateFormatLookup.LookupName)
		}
		columnPosStr := strconv.Itoa(columnPos)
		metaRow, err := dateLookupTbl.Lookup(&columnPosStr)
		if err != nil {
			return nil, fmt.Errorf("while getting the date format metadata row for column pos %d: %v", columnPos, err)
		}
		if metaRow == nil {
			// Not a date
			return nil, nil
		}
		// Check that the column is classified as 'date'
		classificationValuePos := dateLookupTbl.ColumnMap()[parseDateConfig.DateFormatLookup.DataClassificationColumn]
		classificationValueI := (*metaRow)[classificationValuePos]
		if classificationValueI == nil {
			return nil, nil
		}
		classificationValue, ok := classificationValueI.(string)
		if !ok {
			return nil, fmt.Errorf("error: expecting string for data classification, got %v", classificationValueI)
		}
		switch classificationValue {
		case "date", "dob", "dod":
		default:
			// Not a date
			return nil, nil
		}
		lookupValuePos := dateLookupTbl.ColumnMap()[parseDateConfig.DateFormatLookup.DateFormatColumn]
		dateFormatI := (*metaRow)[lookupValuePos]
		if dateFormatI == nil {
			return nil, nil
		}
		dateLayoutsCsv, ok := dateFormatI.(string)
		if !ok {
			return nil, fmt.Errorf("error: expecting string for anonymize type (e.g. text, date), got %v", dateFormatI)
		}
		if len(dateLayoutsCsv) == 0 {
			// Not a date
			return nil, nil
		}
		r := csv.NewReader(bytes.NewReader([]byte(dateLayoutsCsv)))
		dateLayouts, err := r.Read()
		// fmt.Println("*** Got date layouts:", dateLayouts)
		if err != nil {
			return nil, fmt.Errorf("while decoding date formats from csv:%v", err)
		}
		for _, dateFormat := range dateLayouts {
			parseDateConfig.DateFormats = append(parseDateConfig.DateFormats, []string{dateFormat})
		}
	}

	// Determine the date format to use if not provided in DateFormats
	switch {
	case !parseDateConfig.UseJetstoreParser && len(parseDateConfig.DateFormats) == 0:
		var spLayout string
		if sp != nil {
			spLayout = sp.ReadDateLayout()
		}
		if len(spLayout) > 0 {
			parseDateConfig.DateFormats = append(parseDateConfig.DateFormats, []string{spLayout})
		} else {
			log.Println("WARNING: analyze parse_date function has no date format configured, using jetstore internal date parser")
			parseDateConfig.UseJetstoreParser = true
		}
	case len(parseDateConfig.DateFormats) > 0:
		// Make date format in golang format in case they are in java format
		// fmt.Printf("*** Date Formats: \"%s\"\n", strings.Join(parseDateConfig.DateFormats, "\", \""))
		goDateFormats := make([][]string, 0, 2*len(parseDateConfig.DateFormats))
		for i := range parseDateConfig.DateFormats {
			goReadFmt := make([]string, 0, len(parseDateConfig.DateFormats[i]))
			goWriteFmt := make([]string, 0, len(parseDateConfig.DateFormats[i]))
			for _, fmt := range parseDateConfig.DateFormats[i] {
				writeFormat := date_utils.FromJavaDateFormat(fmt, false)
				goWriteFmt = append(goWriteFmt, writeFormat)
				readFormat := date_utils.FromJavaDateFormat(fmt, true)
				if readFormat != writeFormat {
					goReadFmt = append(goReadFmt, readFormat)
				}
			}
			if len(goWriteFmt) > 0 {
				goDateFormats = append(goDateFormats, goWriteFmt)
			}
			if len(goReadFmt) > 0 {
				goDateFormats = append(goDateFormats, goReadFmt)
			}
		}
		parseDateConfig.DateFormats = goDateFormats
		// fmt.Printf("*** GO Date Formats: \"%s\"\n", strings.Join(parseDateConfig.DateFormats, "\", \""))

	default:
		parseDateConfig.UseJetstoreParser = true
	}

	if len(parseDateConfig.OtherDateFormats) > 0 {
		// Make date format in golang format in case they are in java format
		// fmt.Printf("*** Other Date Formats: \"%s\"\n", strings.Join(parseDateConfig.OtherDateFormats, "\", \""))
		for i := range parseDateConfig.OtherDateFormats {
			goFmt := make([]string, 0, len(parseDateConfig.OtherDateFormats[i]))
			for _, fmt := range parseDateConfig.OtherDateFormats[i] {
				writeFormat := date_utils.FromJavaDateFormat(fmt, false)
				goFmt = append(goFmt, writeFormat)
			}
			parseDateConfig.OtherDateFormats[i] = goFmt
		}
		// fmt.Printf("*** GO Other Date Formats: \"%s\"\n", strings.Join(parseDateConfig.OtherDateFormats, "\", \""))
	}

	format := "2006-01-02"
	if len(parseDateConfig.MinMaxDateFormat) > 0 {
		format = parseDateConfig.MinMaxDateFormat
	}
	tokenMatches := make(map[string]int)
	for _, args := range parseDateConfig.ParseDateArguments {
		tokenMatches[args.Token] = 0
	}
	// Convert null date values to time.Time for comparison
	var nullDates []time.Time
	for _, nd := range parseDateConfig.NullDates {
		if t, err := time.Parse(format, nd); err == nil {
			nullDates = append(nullDates, t)
		}
	}
	return &ParseDateMatchFunction{
		parseDateConfig:  parseDateConfig,
		minMaxDateFormat: format,
		minMax:           &minMaxDateValue{},
		tokenMatches:     tokenMatches,
		nullDates:        nullDates,
		formatMatch:      make(map[string]int),
		otherFormatMatch: make(map[string]int),
		seenCache:        make(map[string]*pdCache),
	}, nil
}
