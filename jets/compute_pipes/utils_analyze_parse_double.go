package compute_pipes

import "strconv"

// Parse Double Match Function

type ParseDoubleMatchFunction struct {
	nbrSamplesSeen int
	minMax         *minMaxDoubleValue
	largeValues    *largeDoubleValue
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
	p.nbrSamplesSeen++
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
		HitCount:   float64(p.minMax.count)/float64(p.nbrSamplesSeen),
	}
}

func (p *ParseDoubleMatchFunction) Done(ctx *AnalyzeTransformationPipe, outputRow []any) error {
	if p.minMax != nil {
		ipos, ok := (*ctx.outputCh.columns)["min_double"]
		if ok {
			outputRow[ipos] = p.minMax.minValue
		}
		ipos, ok = (*ctx.outputCh.columns)["max_double"]
		if ok {
			outputRow[ipos] = p.minMax.maxValue
		}
	}

	var ratioFactor float64
	if p.nbrSamplesSeen == 0 {
		ratioFactor = 100 / float64(p.nbrSamplesSeen)
	}
	if p.largeValues != nil {
		ipos, ok := (*ctx.outputCh.columns)["large_double_pct"]
		if ok {
			if ratioFactor > 0 {
				outputRow[ipos] = float64(p.largeValues.count) * ratioFactor
			} else {
				outputRow[ipos] = -1.0
			}
		}
	}
	return nil
}

func NewParseDoubleMatchFunction(fspec *FunctionTokenNode) (*ParseDoubleMatchFunction, error) {
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
