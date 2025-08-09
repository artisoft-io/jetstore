package compute_pipes

import "strconv"

// Parse Text Match Function

type ParseTextMatchFunction struct {
	minMax         *minMaxLength
}

type minMaxLength struct {
	minValue *int
	maxValue *int
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
		HitCount:   1,
	}
}

func (p *ParseTextMatchFunction) Done(ctx *AnalyzeTransformationPipe, outputRow []any) error {
	if p.minMax != nil {
		ipos, ok := (*ctx.outputCh.columns)["min_length"]
		if ok {
			outputRow[ipos] = p.minMax.minValue
		}
		ipos, ok = (*ctx.outputCh.columns)["max_length"]
		if ok {
			outputRow[ipos] = p.minMax.maxValue
		}
	}
	return nil
}

func NewParseTextMatchFunction(fspec *FunctionTokenNode) (*ParseTextMatchFunction, error) {
	return &ParseTextMatchFunction{
		minMax: &minMaxLength{},
	}, nil
}
