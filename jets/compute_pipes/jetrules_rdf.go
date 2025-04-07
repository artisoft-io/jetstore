package compute_pipes

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This file contains function to cast input data into rdf type based on domain classes
type CastToRdfFnc = func(v any) (any, error)
type CastToRdfTxtFnc = func(v string) (any, error)

func BuildCastToRdfFunctions(domainClass string, properties []string) ([]CastToRdfFnc, error) {
	result := make([]CastToRdfFnc, len(properties))
	// var doNothing CastToRdfFnc = func(v any) (any, error) {
	// 	return v, nil
	// }
	// // initialize the result
	// for i := range result {
	// 	result[i] = doNothing
	// }
	if len(domainClass) == 0 {
		return result, nil
	}
	dpMap, err := GetWorkspaceDataProperties()
	if err != nil {
		return nil, fmt.Errorf("while GetWorkspaceDataProperties: %v", err)
	}
	for i, property := range properties {
		dp := dpMap[property]
		if dp != nil {
			result[i] = func(v any) (any, error) {
				return castToRdfType(v, dp.Type, dp.AsArray)
			}
		}
	}
	return result, nil
}

func BuildCastToRdfTxtFunctions(domainClass string, properties []string) ([]CastToRdfTxtFnc, error) {
	result := make([]CastToRdfTxtFnc, len(properties))
	if len(domainClass) == 0 {
		return result, nil
	}
	dpMap, err := GetWorkspaceDataProperties()
	if err != nil {
		return nil, fmt.Errorf("while GetWorkspaceDataProperties: %v", err)
	}
	for i, property := range properties {
		dp := dpMap[property]
		if dp != nil {
			result[i] = func(v string) (any, error) {
				return castToRdfTypeFromTxt(v, dp.Type, dp.AsArray)
			}
		}
	}
	return result, nil
}

// Function to cast [inValue] which is typically a string into the specified [rdfType]
// The returned value will be a slice []any if [isArray] is true.
// If [inValue] is a slice and [isArray] is false, the first element of [inValue] is casted
// to [rdfType] and returned.
// If [inValue] is an empty slice, empty string, or nil value, a nil value is returned.
func castToRdfType(inValue any, rdfType string, isArray bool) (any, error) {
	if inValue == nil {
		return nil, nil
	}

	switch vv := inValue.(type) {
	case string:
		// Delegate to string version
		return castToRdfTypeFromTxt(vv, rdfType, isArray)

	case []any:
		if len(vv) == 0 {
			return nil, nil
		}
		if isArray {
			result := make([]any, 0, len(vv))
			for i := range vv {
				if vv[i] != nil {
					elm, err := castToRdfType(vv[i], rdfType, false)
					if err != nil {
						return nil, fmt.Errorf("while casting an array elm: %v", err)
					}
					result = append(result, elm)
				}
			}
			return result, nil
		}
		// Not expecting an array, returning the first non nil elm
		for i := range vv {
			if vv[i] != nil {
				return castToRdfType(vv[i], rdfType, false)
			}
		}
		return nil, nil

	case int:
		return castToRdfTypeFromInt(vv, rdfType, isArray)

	case uint:
		return castToRdfTypeFromUInt(vv, rdfType, isArray)

	case time.Time:
		return castToRdfTypeFromTime(vv, rdfType, isArray)

	case float64:
		return castToRdfTypeFromDouble(vv, rdfType, isArray)
	case float32:
		return castToRdfTypeFromDouble(float64(vv), rdfType, isArray)

	case int32:
		return castToRdfTypeFromInt(int(vv), rdfType, isArray)
	case int64:
		return castToRdfTypeFromInt(int(vv), rdfType, isArray)
	case uint32:
		return castToRdfTypeFromUInt(uint(vv), rdfType, isArray)
	case uint64:
		return castToRdfTypeFromUInt(uint(vv), rdfType, isArray)
	}
	return nil, fmt.Errorf("error: unknown type %T for casting as rdfType %s", inValue, rdfType)
}

func castToRdfTypeFromTxt(inValue string, rdfType string, isArray bool) (any, error) {
	if len(inValue) == 0 {
		return nil, nil
	}
	if isArray {
		// expecting a slice
		if strings.HasPrefix(inValue, "{") && strings.HasSuffix(inValue, "}") {
			r := csv.NewReader(strings.NewReader(inValue[1 : len(inValue)-1]))
			r.LazyQuotes = true
			values, err := r.Read()
			if err != nil {
				return nil, fmt.Errorf("while parsing array: %v", err)
			}
			results := make([]any, 0, len(values))
			for i := range values {
				v, err := castToRdfTypeFromTxt(values[i], rdfType, false)
				if err != nil {
					return nil, fmt.Errorf("while casting array value: %v", err)
				}
				if v != nil {
					results = append(results, v)
				}
			}
			return results, nil
		}
	}

	switch rdfType {
	case "text", "string", "resource":
		return inValue, nil
	case "date":
		dt, err := rdf.ParseDate(inValue)
		if err != nil {
			return nil, err
		}
		return *dt, nil
	case "double":
		return strconv.ParseFloat(strings.TrimSpace(inValue), 64)
	case "int", "integer", "long":
		return strconv.Atoi(strings.TrimSpace(inValue))
	case "uint", "ulong":
		v, err := strconv.ParseUint(inValue, 10, 64)
		return uint(v), err
	case "bool":
		return rdf.ParseBool(inValue), nil
	case "datetime":
		dt, err := rdf.ParseDatetime(inValue)
		if err != nil {
			return nil, err
		}
		return *dt, nil
	}
	return nil, fmt.Errorf("error: unknown rdfTyoe %s for conversion from string", rdfType)
}

func castToRdfTypeFromInt(inValue int, rdfType string, isArray bool) (any, error) {
	switch rdfType {
	case "text", "string":
		if isArray {
			return []any{strconv.Itoa(inValue)}, nil
		}
		return strconv.Itoa(inValue), nil
	case "double":
		if isArray {
			return []any{float64(inValue)}, nil
		}
		return float64(inValue), nil
	case "int", "integer", "long":
		if isArray {
			return []any{inValue}, nil
		}
		return inValue, nil
	case "uint", "ulong":
		if isArray {
			return []any{uint(inValue)}, nil
		}
		return uint(inValue), nil
	case "bool":
		if inValue > 0 {
			if isArray {
				return []any{1}, nil
			}
			return 1, nil
		}
		if isArray {
			return []any{0}, nil
		}
		return 0, nil
	}
	return nil, fmt.Errorf("error: invalid rdfType (%s) for input value of type int", rdfType)
}

func castToRdfTypeFromUInt(inValue uint, rdfType string, isArray bool) (any, error) {
	switch rdfType {
	case "text", "string":
		if isArray {
			return []any{strconv.FormatUint(uint64(inValue), 10)}, nil
		}
		return strconv.FormatUint(uint64(inValue), 10), nil
	case "double":
		if isArray {
			return []any{float64(inValue)}, nil
		}
		return float64(inValue), nil
	case "int", "integer", "long":
		if isArray {
			return []any{int(inValue)}, nil
		}
		return int(inValue), nil
	case "uint", "ulong":
		if isArray {
			return []any{inValue}, nil
		}
		return inValue, nil
	case "bool":
		if inValue > 0 {
			if isArray {
				return []any{1}, nil
			}
			return 1, nil
		}
		if isArray {
			return []any{0}, nil
		}
		return 0, nil
	}
	return nil, fmt.Errorf("error: invalid rdfType (%s) for input value of type uint", rdfType)
}

func castToRdfTypeFromTime(inValue time.Time, rdfType string, isArray bool) (any, error) {
	switch rdfType {
	case "text", "string":
		if isArray {
			return []any{inValue.Format("2006-01-02T15:04:05")}, nil
		}
		return inValue.Format("2006-01-02T15:04:05"), nil
	case "int", "integer", "long":
		if isArray {
			return []any{int(inValue.UnixMilli())}, nil
		}
		return int(inValue.UnixMilli()), nil
	case "uint", "ulong":
		if isArray {
			return []any{uint(inValue.UnixMilli())}, nil
		}
		return uint(inValue.UnixMilli()), nil
	case "bool":
		if inValue.Unix() > 0 {
			if isArray {
				return []any{1}, nil
			}
			return 1, nil
		}
		if isArray {
			return []any{0}, nil
		}
		return 0, nil
	}
	return nil, fmt.Errorf("error: invalid rdfType (%s) for input value of type time.Time", rdfType)
}

func castToRdfTypeFromDouble(inValue float64, rdfType string, isArray bool) (any, error) {
	switch rdfType {
	case "text", "string":
		if isArray {
			return []any{strconv.FormatFloat(inValue, 'f', -1, 64)}, nil
		}
		return strconv.FormatFloat(inValue, 'f', -1, 64), nil
	case "double":
		if isArray {
			return []any{inValue}, nil
		}
		return inValue, nil
	case "int", "integer", "long":
		if isArray {
			return []any{int(inValue)}, nil
		}
		return int(inValue), nil
	case "uint", "ulong":
		if isArray {
			return []any{uint(inValue)}, nil
		}
		return uint(inValue), nil
	case "bool":
		if inValue > 0 {
			if isArray {
				return []any{1}, nil
			}
			return 1, nil
		}
		if isArray {
			return []any{0}, nil
		}
		return 0, nil
	}
	return nil, fmt.Errorf("error: invalid rdfType (%s) for input value of type double", rdfType)
}

func NewRdfNode(inValue any, rm *rdf.ResourceManager) (*rdf.Node, error) {
	switch vv := inValue.(type) {
	case string:
		return rm.NewTextLiteral(vv), nil
	case int:
		return rm.NewIntLiteral(vv), nil
	case uint:
		return rm.NewUIntLiteral(vv), nil
	case float64:
		return rm.NewDoubleLiteral(vv), nil
	case time.Time:
		return rm.NewDateLiteral(rdf.LDate{Date: &vv}), nil
	case rdf.LDate:
		return rm.NewDateLiteral(vv), nil
	case rdf.LDatetime:
		return rm.NewDatetimeLiteral(vv), nil
	case int64:
		return rm.NewIntLiteral(int(vv)), nil
	case uint64:
		return rm.NewUIntLiteral(uint(vv)), nil
	case int32:
		return rm.NewIntLiteral(int(vv)), nil
	case uint32:
		return rm.NewUIntLiteral(uint(vv)), nil
	case float32:
		return rm.NewDoubleLiteral(float64(vv)), nil
	default:
		return nil, fmt.Errorf("error: unknown type %T for NewRdfNode", vv)
	}
}