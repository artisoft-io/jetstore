package rete

// Factory for creating Expression operators

func (ctx *ReteBuilderContext) CreateBinaryOperator(op string) BinaryOperator {
	switch op {
	// Logical Operators
	case ">":
		return NewGtOp()
	case ">=":
		return NewGeOp()
	case "<":
		return NewLtOp()
	case "<=":
		return NewLeOp()
	case "==":
		return NewEqOp()
	case "!=":
		return NewNeOp()
	case "and":
		return NewAndOp()
	case "or":
		return NewOrOp()

		// Arithmetic operators
	case "+":
		return NewAddOp()
	case "-":
		return NewSubOp()
	case "/":
		return NewDivOp()
	case "*":
		return NewMultOp()
	case "min_of":
		return NewMinMaxOp(true, false)
	case "max_of":
		return NewMinMaxOp(false, false)
	case "min_head_of":
		return NewMinMaxOp(true, true)
	case "max_head_of":
		return NewMinMaxOp(false, true)
	case "sorted_head":
		return NewSortedHeadOp()
	case "sum_values":
		return NewSumValuesOp()

		// String operators
	case "literal_regex", "apply_regex":
		return NewApplyRegexOp()
	case "apply_format":
		return NewApplyFormatOp()
	case "contains":
		return NewContainsOp()
	case "starts_with":
		return NewStartWithOp()
	case "ends_with":
		return NewEndsWithOp()
	case "substring_of":
		return NewSubstringOfOp()
	case "char_at":
		return NewCharAtOp()
	case "replace_char_of":
		return NewReplaceCharOp()

		// Resource operators
	case "range": // "Iterator" operator
		return NewRangeOp()
	case "exist":
		return NewExistOp(false)
	case "exist_not":
		return NewExistOp(true)
	case "size_of":
		return NewSizeOfOp()

		// Lookup binary operators
	case "lookup":
		return NewLookupOp()
	case "multi_lookup":
		return NewMultiLookupOp()

		// Utility operators
	case "age_as_of":
		return NewAgeAsOfOp()
	case "age_in_months_as_of":
		return NewAgeMonthsAsOfOp()
	}
	return nil
}

func (ctx *ReteBuilderContext) CreateUnaryOperator(op string) UnaryOperator {
	switch op {
	// Arithmetic operators
	case "abs":
		return NewAbsOp()
	case "to_int":
		return NewToIntOp()
	case "to_double":
		return NewToDoubleOp()
	case "to_date":
		return NewToDateOp()
	case "to_datetime":
		return NewToDatetimeOp()
	case "to_text":
		return NewToTextOp()

		// Date/Datetime operators
	case "to_timestamp":
		return NewToTimestampOp()
	case "month_period_of":
		return NewMonthPeriodOfOp()
	case "week_period_of":
		return NewWeekPeriodOfOp()
	case "day_period_of":
		return NewDayPeriodOfOp()

		// Logical operators
	case "not":
		return NewNotOp()

		// String operators
	case "to_upper":
		return NewToUpperOp()
	case "to_lower":
		return NewToLowerOp()
	case "trim":
		return NewTrimOp()
	case "length_of":
		return NewLengthOfOp()
	case "parse_usd_currency":
		return NewParseCurrencyOp()
	case "uuid_md5":
		return NewUuidMd5Op()
	case "uuid_sha1":
		return NewUuidSha1Op()

		// Resource operators
	case "create_entity":
		return NewCreateEntityOp()
	case "create_literal":
		return NewCreateLiteralOp()
	case "create_resource":
		return NewCreateResourceOp()
	case "create_uuid_resource":
		return NewCreateUuidResourceOp()
	case "is_literal":
		return NewIsLiteralOp()
	case "is_resource":
		return NewIsResourceOp()

		// Lookup rand operators
	case "lookup_rand":
		return NewLookupRandOp()
	case "multi_lookup_rand":
		return NewMultiLookupRandOp()
	}
	return nil
}
