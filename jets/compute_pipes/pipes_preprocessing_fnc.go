package compute_pipes

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This file contains the construct for pre-processing functions used in
// domain keys and hashing operator

var preprocessingFncRe *regexp.Regexp

func init() {
	preprocessingFncRe = regexp.MustCompile(`^(.*?)\((.*?)\)$`)
}

type PreprocessingFunction interface {
	ApplyPF(buf *bytes.Buffer, input *[]any) error
	String() string
}

func ParsePreprocessingExpressions(inputExprs []string, toUpper bool, columns *map[string]int) ([]PreprocessingFunction, error) {
	preprocessingFncs := make([]PreprocessingFunction, len(inputExprs))
	for i := range inputExprs {
		// Get the processing function, if any, and the column name
		v := preprocessingFncRe.FindStringSubmatch(inputExprs[i])
		if len(v) < 3 {
			pos, ok := (*columns)[inputExprs[i]]
			if !ok {
				return nil, fmt.Errorf("error in ParsePreprocessingExpressions: column %s not found", inputExprs[i])
			}
			preprocessingFncs[i] = &DefaultPF{
				inputPos: pos,
				toUpper:  toUpper,
			}
		} else {
			pos, ok := (*columns)[v[2]]
			if !ok {
				return nil,
					fmt.Errorf("error in ParsePreprocessingExpressions: column %s not found, taken from %s", v[2], inputExprs[i])
			}
			switch v[1] {
			case "format_date":
				preprocessingFncs[i] = &FormatDatePF{inputPos: pos}
			case "remove_mi":
				preprocessingFncs[i] = &RemoveMiPF{
					inputPos: pos,
					toUpper:  toUpper,
				}
			default:
				return nil,
					fmt.Errorf("error in ParsePreprocessingExpressions: key definition has an unknown preprocessing function %s",
						inputExprs[i])
			}
		}
	}
	return preprocessingFncs, nil
}

// DefaultPF is when there is no preprocessing function, simply add the value to the byte buffer
type DefaultPF struct {
	inputPos int
	toUpper  bool
}

func (pf *DefaultPF) ApplyPF(buf *bytes.Buffer, input *[]any) error {
	switch vv := (*input)[pf.inputPos].(type) {
	case string:
		if pf.toUpper {
			buf.WriteString(strings.ToUpper(vv))
		} else {
			buf.WriteString(vv)
		}
	case []byte:
		buf.Write(vv)
	case nil:
		// do nothing
	case time.Time:
		buf.WriteString(strconv.FormatInt(vv.Unix(), 10))
	default:
		fmt.Fprintf(buf, "%v", vv)
	}
	return nil
}
func (pf *DefaultPF) String() string {
	return fmt.Sprintf("DefaultPF(inputPos=%d,toUpper=%v)", pf.inputPos, pf.toUpper)
}

// FormatDatePF is writing a date field using YYYYMMDD format
// This assume the date in the input is a valid date as string
// Returns no error if date is empty or not valid
type FormatDatePF struct {
	inputPos int
}

func (pf *FormatDatePF) ApplyPF(buf *bytes.Buffer, input *[]any) error {
	switch vv := (*input)[pf.inputPos].(type) {
	case string:
		y, m, d, err := rdf.ParseDateComponents(vv)
		if err != nil {
			// return fmt.Errorf("error: in FormatDatePF the input date is not a valid date: %v", err)
			return nil
		}
		fmt.Fprintf(buf, "%d%02d%02d", y, m, d)
	case []byte:
		buf.Write(vv)
	case nil:
		// do nothing
	case time.Time:
		fmt.Fprintf(buf, "%d%02d%02d", vv.Year(), vv.Month(), vv.Day())
	default:
		fmt.Fprintf(buf, "%v", vv)
	}
	return nil
}
func (pf *FormatDatePF) String() string {
	return fmt.Sprintf("FormatDatePF(inputPos=%d)", pf.inputPos)
}

// RemoveMiPF remove last 2 char if last-1 is a space, e.g. "michel f" becomes "michel"
type RemoveMiPF struct {
	inputPos int
	toUpper  bool
}

var spc byte

func init() {
	s := []byte(" ")
	spc = s[0]
}

func (pf *RemoveMiPF) ApplyPF(buf *bytes.Buffer, input *[]any) error {
	v := (*input)[pf.inputPos]
	if v == nil {
		return nil
	}
	value, ok := v.(string)
	if !ok {
		// return fmt.Errorf("error: in FormatDatePF the input date is not a string: %v", v)
		return nil
	}
	if pf.toUpper {
		value = strings.ToUpper(value)
	}
	l := len(value)
	if l > 2 {
		v := value[l-2]
		if v == spc {
			value = value[:l-2]
		}
	}
	buf.WriteString(value)
	return nil
}
func (pf *RemoveMiPF) String() string {
	return fmt.Sprintf("RemoveMiPF(inputPos=%d,toUpper=%v)", pf.inputPos, pf.toUpper)
}
