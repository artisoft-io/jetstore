package rdf

import (
	// "fmt"
	"log"
	"reflect"
	"strconv"
	"time"

	"github.com/google/uuid"
)

func (lhs *Node) AgeMonthsAsOf(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	var lhsTime, rhsTime *time.Time
	switch vv := lhs.Value.(type) {
	case LDate:
		lhsTime = vv.Date
	case LDatetime:
		lhsTime = vv.Datetime
	default:
		return nil
	}

	switch vv := rhs.Value.(type) {
	case LDate:
		rhsTime = vv.Date
	case LDatetime:
		rhsTime = vv.Datetime
	default:
		return nil
	}

	years := rhsTime.Year() - lhsTime.Year()
	// fmt.Printf("*Got %d years, year day:(%d, %d)\n", years, lhsTime.YearDay(), rhsTime.YearDay())
	var months int

	// Add the number of months in the last year
	if rhsTime.YearDay() <= lhsTime.YearDay() {
		years -= 1
		months += int(12 - lhsTime.Month())
		months += int(rhsTime.Month())
	} else {
		months += int(rhsTime.Month() - lhsTime.Month())
	}
	// fmt.Printf("Got %d years, %d months\n", years, months)
	return I(years*12 + months)
}

func (lhs *Node) AgeAsOf(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	var lhsTime, rhsTime *time.Time
	switch vv := lhs.Value.(type) {
	case LDate:
		lhsTime = vv.Date
	case LDatetime:
		lhsTime = vv.Datetime
	default:
		return nil
	}

	switch vv := rhs.Value.(type) {
	case LDate:
		rhsTime = vv.Date
	case LDatetime:
		rhsTime = vv.Datetime
	default:
		return nil
	}

	age := rhsTime.Year() - lhsTime.Year()
	// fmt.Printf("*Got %d years, year day:(%d, %d)\n", age, lhsTime.YearDay(), rhsTime.YearDay())
	if rhsTime.YearDay() < lhsTime.YearDay() {
		age -= 1
	}
	// fmt.Printf("==Got %d years, year day:(%d, %d)\n", age, lhsTime.YearDay(), rhsTime.YearDay())
	return I(age)
}

// unary operator
func (rhs *Node) CreateEntity(rdfSession *RdfSession) *Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case int:
		if rhsv == 0 {
			return createEntity(rdfSession, "")
		}
		return createEntity(rdfSession, strconv.Itoa(rhsv))
	case string:
		return createEntity(rdfSession, rhsv)
	case float64:
		if NearlyEqual(rhsv, 0) {
			return createEntity(rdfSession, "")
		}
		return createEntity(rdfSession, strconv.FormatFloat(rhsv, 'G', 15, 64))
	default:
		return nil
	}
}

func createEntity(rdfSession *RdfSession, name string) *Node {
	if name == "" {
		name = uuid.NewString()
	}
	rm := rdfSession.ResourceMgr
	entity := rm.NewResource(name)
	_, err := rdfSession.InsertInferred(entity, rm.JetsResources.Jets__key, rm.NewTextLiteral(name))
	if err != nil {
		log.Panicf("while calling InsertInferred (createEntity operator): %v", err)
	}
	return entity
}

// unary operator
func (rhs *Node) CreateLiteral(rdfSession *RdfSession) *Node {
	if rhs == nil {
		return nil
	}

	switch reflect.TypeOf(rhs.Value).Kind() {
	case reflect.Int:
		return rhs
	case reflect.Float64:
		return rhs
	case reflect.String:
		return rhs
	default:
		log.Printf("Argment is not a literal (create_literal): %v", rhs.Value)
		return nil
	}
}

func (rhs *Node) CreateResource(rdfSession *RdfSession) *Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case int:
		return R(strconv.Itoa(rhsv))
	case float64:
		return R(strconv.FormatFloat(rhsv, 'G', 15, 64))
	case string:
		return R(rhsv)
	default:
		log.Printf("Argment is not a literal (create_resource): %v", rhsv)
		return nil
	}
}

func (rhs *Node) DayPeriodOf() *Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case string:
		t, err := ParseDatetime(rhsv)
		if err != nil {
			return nil
		}
		return I(dayPeriodOf(t))
	case LDate:
		return I(dayPeriodOf(rhsv.Date))
	case LDatetime:
		return I(dayPeriodOf(rhsv.Datetime))
	default:
		return nil
	}
}

func dayPeriodOf(t *time.Time) int {
	// secPerDay = 24 * 60 * 60 = 84400
	// dayPeriod = int(unixTime/secPerDay + 1)
	return int(t.Unix()/84400 + 1)
}

func (rhs *Node) MonthPeriodOf() *Node {
	if rhs == nil {
		return nil
	}
	switch rhsv := rhs.Value.(type) {
	case string:
		t, err := ParseDatetime(rhsv)
		if err != nil {
			return nil
		}
		return I(monthPeriodOf(t))
	case LDate:
		return I(monthPeriodOf(rhsv.Date))
	case LDatetime:
		return I(monthPeriodOf(rhsv.Datetime))
	default:
		return nil
	}
}

func monthPeriodOf(t *time.Time) int {
	// monthPeriod = (year-1970)*12 + month
	return (t.Year()-1970)*12 + int(t.Month())
}

func (rhs *Node) WeekPeriodOf() *Node {
	if rhs == nil {
		return nil
	}
	switch rhsv := rhs.Value.(type) {
	case string:
		t, err := ParseDatetime(rhsv)
		if err != nil {
			return nil
		}
		return I(weekPeriodOf(t))
	case LDate:
		return I(weekPeriodOf(rhsv.Date))
	case LDatetime:
		return I(weekPeriodOf(rhsv.Datetime))
	default:
		return nil
	}
}

func weekPeriodOf(t *time.Time) int {
	// secPerDay = 24 * 60 * 60 = 84400
	// secPerWeek = 7 * secPerDay = 604800
	// weekPeriod = int(unixTime/secPerWeek + 1)
	return int(t.Unix()/604800 + 1)
}

func (lhs *Node) Exist(rdfSession *RdfSession, rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	obj := rdfSession.GetObject(lhs, rhs)
	var result *Node
	if obj == nil {
		result = FALSE()
	} else {
		result = TRUE()
	}
	return result
}

func (lhs *Node) ExistNot(rdfSession *RdfSession, rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	obj := rdfSession.GetObject(lhs, rhs)
	var result *Node
	if obj == nil {
		result = TRUE()
	} else {
		result = FALSE()
	}
	return result
}
