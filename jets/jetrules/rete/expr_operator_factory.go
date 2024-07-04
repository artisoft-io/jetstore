package rete

// Factory for creating Expression operators

func CreateBinaryOperator(op string) BinaryOperator {
	switch op {
		// Logical Operators
	case ">":  return NewGtOp()
  // if(op == "and")               return create_expr_binary_operator<AndVisitor>(key, lhs, rhs);
  // if(op == "or")                return create_expr_binary_operator<OrVisitor>(key, lhs, rhs);
  // if(op == "==")                return create_expr_binary_operator<EqVisitor>(key, lhs, rhs);
  // if(op == "!=")                return create_expr_binary_operator<NeVisitor>(key, lhs, rhs);
  // if(op == "<")                 return create_expr_binary_operator<LtVisitor>(key, lhs, rhs);
  // if(op == "<=")                return create_expr_binary_operator<LeVisitor>(key, lhs, rhs);
  // if(op == ">=")                return create_expr_binary_operator<GeVisitor>(key, lhs, rhs);

		// Arithmetic operators
	case "+":  return NewAddOp()
	case "-":  return NewSubOp()
	case "/":  return NewDivOp()
	case "*":  return NewMultOp()
	case "min_of":  return NewMinMaxOp(true, false)
	case "max_of":  return NewMinMaxOp(false, false)
  // if(op == "sorted_head")       return create_expr_binary_operator<SortedHeadVisitor>(key, lhs, rhs);
  // if(op == "sum_values")        return create_expr_binary_operator<SumValuesVisitor>(key, lhs, rhs);

  // String operators
  // if(op == "literal_regex")     return create_expr_binary_operator<RegexVisitor>(key, lhs, rhs);
  // if(op == "apply_format")      return create_expr_binary_operator<ApplyFormatVisitor>(key, lhs, rhs);
  // if(op == "contains")          return create_expr_binary_operator<ContainsVisitor>(key, lhs, rhs);
  // if(op == "starts_with")       return create_expr_binary_operator<StartsWithVisitor>(key, lhs, rhs);
  // if(op == "substring_of")      return create_expr_binary_operator<SubstringOfVisitor>(key, lhs, rhs);
  // if(op == "char_at")           return create_expr_binary_operator<CharAtVisitor>(key, lhs, rhs);
  // if(op == "replace_char_of")   return create_expr_binary_operator<ReplaceCharOfVisitor>(key, lhs, rhs);

  // "Iterator" operator
  // if(op == "range")             return create_expr_binary_operator<RangeVisitor>(key, lhs, rhs);

  // Resource operators
  // if(op == "size_of")           return create_expr_binary_operator<SizeOfVisitor>(key, lhs, rhs);
  // if(op == "exist")             return create_expr_binary_operator<ExistVisitor>(key, lhs, rhs);
  // if(op == "exist_not")         return create_expr_binary_operator<ExistNotVisitor>(key, lhs, rhs);

  // Lookup operators (in expr_op_others.h)
  // if(op == "lookup")            return create_expr_binary_operator<LookupVisitor>(key, lhs, rhs);
  // if(op == "multi_lookup")      return create_expr_binary_operator<MultiLookupVisitor>(key, lhs, rhs);

  // Utility operators (in expr_op_others.h)
  // if(op == "age_as_of")            return create_expr_binary_operator<AgeAsOfVisitor>(key, lhs, rhs);
  // if(op == "age_in_months_as_of")  return create_expr_binary_operator<AgeInMonthsAsOfVisitor>(key, lhs, rhs);
		
	}
	return nil
}

func CreateUnaryOperator(op string) UnaryOperator {
	switch op {
	  // Arithmetic operators
	case "abs":  return NewAbsOp()
  // if(op == "to_int")            return create_expr_unary_operator<ToIntVisitor>(key, arg);
  // if(op == "to_double")         return create_expr_unary_operator<ToDoubleVisitor>(key, arg);
  
  // Date/Datetime operators
  // if(op == "to_timestamp")      return create_expr_unary_operator<ToTimestampVisitor>(key, arg);
  // if(op == "month_period_of")   return create_expr_unary_operator<MonthPeriodVisitor>(key, arg);
  // if(op == "week_period_of")    return create_expr_unary_operator<WeekPeriodVisitor>(key, arg);
  // if(op == "day_period_of")     return create_expr_unary_operator<DayPeriodVisitor>(key, arg);
  
  // Logical operators
  // if(op == "not")               return create_expr_unary_operator<NotVisitor>(key, arg);

  // String operators
  // if(op == "to_upper")           return create_expr_unary_operator<To_upperVisitor>(key, arg);
  // if(op == "to_lower")           return create_expr_unary_operator<To_lowerVisitor>(key, arg);
  // if(op == "trim")               return create_expr_unary_operator<TrimVisitor>(key, arg);
  // if(op == "length_of")          return create_expr_unary_operator<LengthOfVisitor>(key, arg);
  // if(op == "parse_usd_currency") return create_expr_unary_operator<ParseUsdCurrencyVisitor>(key, arg);
  // if(op == "uuid_md5")           return create_expr_unary_operator<CreateNamedMd5UUIDVisitor>(key, arg);
  // if(op == "uuid_sha1")          return create_expr_unary_operator<CreateNamedSha1UUIDVisitor>(key, arg);

  // Resource operators
  // if(op == "create_entity")     return create_expr_unary_operator<CreateEntityVisitor>(key, arg);
  // if(op == "create_literal")    return create_expr_unary_operator<CreateLiteralVisitor>(key, arg);
  // if(op == "create_resource")   return create_expr_unary_operator<CreateResourceVisitor>(key, arg);
  // if(op == "create_uuid_resource") return create_expr_unary_operator<CreateUUIDResourceVisitor>(key, arg);
  // if(op == "is_literal")        return create_expr_unary_operator<IsLiteralVisitor>(key, arg);
  // if(op == "is_resource")       return create_expr_unary_operator<IsResourceVisitor>(key, arg);
  // if(op == "raise_exception")   return create_expr_unary_operator<RaiseExceptionVisitor>(key, arg);

  // Lookup operators (in expr_op_others.h)
  // if(op == "lookup_rand")       return create_expr_unary_operator<LookupRandVisitor>(key, arg);
  // if(op == "multi_lookup_rand") return create_expr_unary_operator<MultiLookupRandVisitor>(key, arg);
	}
	return nil
}
