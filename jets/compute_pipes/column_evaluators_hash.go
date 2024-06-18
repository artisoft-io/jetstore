package compute_pipes

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"math/rand"
	"regexp"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)
var preprocessingFncRe *regexp.Regexp
func init() {
	preprocessingFncRe = regexp.MustCompile(`^(.*?)\((.*?)\)$`)
}

// TransformationColumnSpec Type hash
type hashColumnEval struct {
	inputPos    int
	outputPos   int
	partitions  uint64
	altInputKey []PreprocessingFunction
}

func Hash(key []byte, partitions uint64) uint64 {
	h := fnv.New64a()
	h.Write(key)
	keyHash := h.Sum64()
	if partitions > 0 {
		keyHash = keyHash % partitions
	}
	return keyHash
}
func partition(key, partitions uint64) uint64 {
	if partitions > 0 {
		key = key % partitions
	}
	return key
}

func EvalHash(key interface{}, partitions uint64) *uint64 {
	var hashedValue uint64
	if key == nil {
		if partitions > 0 {
			hashedValue = uint64(rand.Int63n(int64(partitions)))
		}
		return &hashedValue
	}
	switch vv := key.(type) {
	case string:
		hashedValue = Hash([]byte(vv), partitions)
	case int:
		hashedValue = partition(uint64(vv), partitions)
	case uint:
		hashedValue = partition(uint64(vv), partitions)
	case int32:
		hashedValue = partition(uint64(vv), partitions)
	case uint32:
		hashedValue = partition(uint64(vv), partitions)
	case int64:
		hashedValue = partition(uint64(vv), partitions)
	case uint64:
		hashedValue = partition(uint64(vv), partitions)
	case []byte:
		hashedValue = Hash(vv, partitions)
	case bool:
		if vv {
			hashedValue = uint64(1)
		} else {
			hashedValue = uint64(0)
		}
	default:
		hashedValue = Hash([]byte(fmt.Sprintf("%v", vv)), partitions)
	}
	return &hashedValue
}

func (ctx *hashColumnEval) initializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *hashColumnEval) update(currentValue *[]interface{}, input *[]interface{}) error {
	var err error
	if currentValue == nil || input == nil {
		return fmt.Errorf("error hashColumnEval.update cannot have nil currentValue or input")
	}
	// compute the hash of value @ inputPos, if it's nil use the alternate (composite) key
	var hashedValue interface{}
	inputVal := (*input)[ctx.inputPos]
	// fmt.Printf("##### # inputVal: %v\n", inputVal)
	if inputVal == nil && ctx.altInputKey != nil {
		// Make the alternate key to hash
		inputVal, err = makeAlternateKey(&ctx.altInputKey, input)
		// fmt.Printf("##### # makeAlternateKey: %v\n", inputVal)
		if err != nil {
			return err
		}
	}

	h := EvalHash(inputVal, ctx.partitions)
	if h != nil {
		hashedValue = *h
		// fmt.Printf("##### # EvalHash k: %v, nbr partitions: %d => %v\n", inputVal, ctx.partitions, hashedValue)
	// } else {
	// 	fmt.Printf("##### # EvalHash k: %v, nbr partitions: %d => NULL\n", inputVal, ctx.partitions)
	}

	(*currentValue)[ctx.outputPos] = hashedValue
	return err
}
func (ctx *hashColumnEval) done(currentValue *[]interface{}) error {
	return nil
}

// The Hash operator full example (dw_rawfilename is string):
//
//	{
//		"name": "jets_partition",
//		"type": "hash",
//		"hash_expr": {
//			"expr": "dw_rawfilename",
//			"nbr_jets_partitions": 3,
//			"alternate_composite_expr": ["name", "gender", "format_date(dob)"],
//		}
//
// jets_partition will be of type uint64
func (ctx *BuilderContext) buildHashEvaluator(source *InputChannel, outCh *OutputChannel, spec *TransformationColumnSpec) (*hashColumnEval, error) {
	var err error
	if spec == nil || spec.HashExpr == nil {
		return nil, fmt.Errorf("error: Type map must have HashExpr != nil")
	}
	inputPos, ok := source.columns[spec.HashExpr.Expr]
	if !ok {
		return nil, fmt.Errorf("error column %s not found in input source %s", *spec.Expr, source.config.Name)
	}
	outputPos, ok := outCh.columns[spec.Name]
	if !ok {
		return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.config.Name)
	}
	var partitions uint64
	if spec.HashExpr.NbrJetsPartitions != nil {
		partitions = *spec.HashExpr.NbrJetsPartitions
	} else {
		partitions = ctx.cpConfig.ClusterConfig.NbrJetsPartitions
	}
	var altInputKey []PreprocessingFunction
	if spec.HashExpr.AlternateCompositeExpr != nil {
		altExpr := *spec.HashExpr.AlternateCompositeExpr
		altInputKey, err = ParseAltKeyDefinition(altExpr, source.columns)
		if err != nil {
			return nil, fmt.Errorf("%v in source name %s", err, source.config.Name)
		}
	}
	return &hashColumnEval{
		inputPos:    inputPos,
		outputPos:   outputPos,
		partitions:  partitions,
		altInputKey: altInputKey,
	}, nil
}

func ParseAltKeyDefinition(altExpr []string, columns map[string]int) ([]PreprocessingFunction, error) {
	altInputKey := make([]PreprocessingFunction, len(altExpr))
	for i := range altExpr {
		// Get the processing function, if any, and the column name
		v := preprocessingFncRe.FindStringSubmatch(altExpr[i])
		if len(v) < 3 {
			pos, ok := columns[altExpr[i]]
			if !ok {
				return nil, fmt.Errorf("error: alt column %s not found", altExpr[i])
			}	
			altInputKey[i] = &DefaultPF{inputPos: pos}
		} else {
			pos, ok := columns[v[2]]
			if !ok {
				return nil, fmt.Errorf("error: alt column %s not found, taken from %s", v[2], altExpr[i])
			}
			switch v[1] {
			case "format_date":
				altInputKey[i] = &FormatDatePF{inputPos: pos}
			default:
				return nil, fmt.Errorf("error: alt key definition has an unknown preprocessing function %s", altExpr[i])
			}
		}
	}
	return altInputKey, nil
}

func makeAlternateKey(altInputKey *[]PreprocessingFunction, input *[]interface{}) (interface{}, error) {
	var buf bytes.Buffer
	var err error
	for _, pf := range *altInputKey {
		err = pf.ApplyPF(&buf, input)
		if err != nil {
			return nil, err
		}
	}
	return buf.String(), nil
}

type PreprocessingFunction interface {
	ApplyPF(buf *bytes.Buffer, input *[]interface{}) error
}

// DefaultPF is when there is no preprocessing function, simply add the value to the byte buffer
type DefaultPF struct {
	inputPos int
}

func (pf *DefaultPF) ApplyPF(buf *bytes.Buffer, input *[]interface{}) error {
	switch vv := (*input)[pf.inputPos].(type) {
	case string:
		buf.WriteString(strings.ToUpper(vv))
	case []byte:
		buf.Write(vv)
	case nil:
		// do nothing
	default:
		buf.WriteString(fmt.Sprintf("%v", vv))
	}
	return nil
}

// FormatDatePF is writing a date field using YYYMMDD format
// This assume the date in the input is a valid date as string
type FormatDatePF struct {
	inputPos int
}

func (pf *FormatDatePF) ApplyPF(buf *bytes.Buffer, input *[]interface{}) error {
	v := (*input)[pf.inputPos]
	if v == nil {
		return nil
	}
	vv, ok := v.(string)
	if !ok {
		return fmt.Errorf("error: in FormatDatePF the input date is not a string: %v", v)
	}
	y, m, d, err := rdf.ParseDateComponents(vv)
	if err != nil {
		return fmt.Errorf("error: in FormatDatePF the input date is not a valid date: %v", err)
	}
	buf.WriteString(fmt.Sprintf("%d%02d%02d", y, m, d))
	return nil
}
