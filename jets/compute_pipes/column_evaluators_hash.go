package compute_pipes

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

var preprocessingFncRe *regexp.Regexp

func init() {
	preprocessingFncRe = regexp.MustCompile(`^(.*?)\((.*?)\)$`)
}

// TransformationColumnSpec Type hash
// This construct is split in two parts:
//   - HashEvaluator: performing the hash based on input records (this is re-used in partition_writer)
//   - hashColumnEval: performing the TransformationColumn operation, delegating to HashEvaluator
type hashColumnEval struct {
	hashEvaluator *HashEvaluator
	outputPos     int
}

func (ctx *hashColumnEval) InitializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *hashColumnEval) Update(currentValue *[]interface{}, input *[]interface{}) error {
	var hashedValue any
	var err error
	if currentValue == nil || input == nil {
		return fmt.Errorf("error hashColumnEval.update cannot have nil currentValue or input")
	}

	if ctx.outputPos < len(*currentValue) {
		hashedValue, err = ctx.hashEvaluator.ComputeHash(*input)
		if err == nil {
			(*currentValue)[ctx.outputPos] = hashedValue
		}
	} else {
		err = fmt.Errorf("error: EvalHash called with invalid write key position of %d for len of %d", ctx.outputPos, len(*currentValue))
	}
	return err
}
func (ctx *hashColumnEval) Done(currentValue *[]interface{}) error {
	return nil
}

// The Hash operator full example (dw_rawfilename is string):
//
//	{
//		"name": "jets_partition",
//		"type": "hash",
//		"hash_expr": {
//			"expr": "dw_rawfilename",
//			"composite_expr": ["partion", "dw_rawfilename"],
//			"nbr_jets_partitions": 3,
//			"alternate_composite_expr": ["name", "gender", "format_date(dob)"],
//		}
//
// jets_partition will be of type uint64
func (ctx *BuilderContext) BuildHashTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.HashExpr == nil {
		return nil, fmt.Errorf("error: Type hash must have HashExpr != nil")
	}
	outputPos, ok := (*outCh.columns)[spec.Name]
	if !ok {
		return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.name)
	}
	hashEvaluator, err := ctx.NewHashEvaluator(source, spec.HashExpr)

	return &hashColumnEval{
		hashEvaluator: hashEvaluator,
		outputPos:     outputPos,
	}, err
}

// HashEvaluator is a type to compute a hask key based on an input record.
type HashEvaluator struct {
	inputPos          int
	compositeInputKey []PreprocessingFunction
	partitions        uint64
	altInputKey       []PreprocessingFunction
}

// Build the HashEvaluator, see BuildHashTCEvaluator
func (ctx *BuilderContext) NewHashEvaluator(source *InputChannel,
	spec *HashExpression) (*HashEvaluator, error) {

	var err error
	if spec == nil {
		return nil, fmt.Errorf("error: HashEvaluator must have HashExpr != nil")
	}
	// Do validation
	exprLen := len(spec.Expr)
	compositeLen := len(spec.CompositeExpr)
	domainKeyLen := len(spec.DomainKey)
	if exprLen == 0 && compositeLen == 0 && domainKeyLen == 0 {
		return nil, fmt.Errorf("error: must specify one of expr, composite_expr, or domain_key in hash operator")
	}
	inputPos := -1
	var compositeInputKey []PreprocessingFunction
	var ok bool
	switch {
	case exprLen > 0:
		inputPos, ok = (*source.columns)[spec.Expr]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in input source %s", spec.Expr, source.name)
		}
	case compositeLen > 0 || domainKeyLen > 0:
		var keys []string
		if domainKeyLen > 0 {
			dk := source.domainKeySpec
			if dk == nil {
				return nil, fmt.Errorf("error: hash operator is configured with domain key but no domain key spec available")
			}
			info, ok := dk.DomainKeys[spec.DomainKey]
			if ok {
				keys = info.KeyExpr
			}
		} else {
			keys = spec.CompositeExpr
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("error: hash operator configured as domain key or composite key has no columns")
		}
		compositeInputKey, err = ParseAltKeyDefinition(keys, source.columns)
		if err != nil {
			return nil, fmt.Errorf("while calling ParseAltKeyDefinition (input channel name %s): %v", source.name, err)
		}
	}
	var partitions uint64
	if !spec.NoPartitions {
		if spec.NbrJetsPartitions != nil {
			partitions = *spec.NbrJetsPartitions
		} else {
			partitions = uint64(ctx.cpConfig.ClusterConfig.NbrPartitions(spec.MultiStepShardingMode))
		}
	}
	var altInputKey []PreprocessingFunction
	if len(spec.AlternateCompositeExpr) > 0 {
		altInputKey, err = ParseAltKeyDefinition(spec.AlternateCompositeExpr, source.columns)
		if err != nil {
			return nil, fmt.Errorf("%v in source name %s", err, source.name)
		}
	}
	return &HashEvaluator{
		inputPos:          inputPos,
		compositeInputKey: compositeInputKey,
		partitions:        partitions,
		altInputKey:       altInputKey,
	}, nil
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

func EvalHash(key any, partitions uint64) *uint64 {
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
	case time.Time:
		hashedValue = partition(uint64(vv.Unix()), partitions)

	default:
		hashedValue = Hash([]byte(fmt.Sprintf("%v", vv)), partitions)
	}
	return &hashedValue
}

func (ctx *HashEvaluator) ComputeHash(input []any) (any, error) {
	var err error
	if input == nil {
		return nil, fmt.Errorf("error HashEvaluator.ComputeHash cannot have nil input")
	}
	// compute the hash of value @ inputPos, if it's nil use the alternate (composite) key
	var inputVal, hashedValue any
	if ctx.inputPos > -1 {
		if ctx.inputPos < len(input) {
			inputVal = input[ctx.inputPos]
		} else {
			return nil, fmt.Errorf("error: hash operator called with invalid read key position of %d for len of %d", ctx.inputPos, len(input))
		}
	} else {
		// Use the composite key
		inputVal, err = makeAlternateKey(&ctx.compositeInputKey, &input)
		// fmt.Printf("##### # makeCompositeKey: %v\n", inputVal)
		if err != nil {
			return nil, err
		}
	}
	// fmt.Printf("##### # inputVal: %v\n", inputVal)
	if inputVal == nil && ctx.altInputKey != nil {
		// Make the alternate key to hash
		inputVal, err = makeAlternateKey(&ctx.altInputKey, &input)
		// fmt.Printf("##### # makeAlternateKey: %v\n", inputVal)
		if err != nil {
			return nil, err
		}
	}

	h := EvalHash(inputVal, ctx.partitions)
	if h != nil {
		hashedValue = *h
	// 	fmt.Printf("##### # EvalHash k: %v, nbr partitions: %d => %v\n", inputVal, ctx.partitions, hashedValue)
	// } else {
	// 	fmt.Printf("##### # EvalHash k: %v, nbr partitions: %d => NULL\n", inputVal, ctx.partitions)
	}
	return hashedValue, nil
}

func ParseAltKeyDefinition(altExpr []string, columns *map[string]int) ([]PreprocessingFunction, error) {
	altInputKey := make([]PreprocessingFunction, len(altExpr))
	for i := range altExpr {
		// Get the processing function, if any, and the column name
		v := preprocessingFncRe.FindStringSubmatch(altExpr[i])
		if len(v) < 3 {
			pos, ok := (*columns)[altExpr[i]]
			if !ok {
				return nil, fmt.Errorf("error: alt column %s not found", altExpr[i])
			}
			altInputKey[i] = &DefaultPF{inputPos: pos}
		} else {
			pos, ok := (*columns)[v[2]]
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
	case time.Time:
		buf.WriteString(strconv.FormatInt(vv.Unix(), 10))
	default:
		buf.WriteString(fmt.Sprintf("%v", vv))
	}
	return nil
}

// FormatDatePF is writing a date field using YYYYMMDD format
// This assume the date in the input is a valid date as string
// Returns no error if date is empty or not valid
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
		// return fmt.Errorf("error: in FormatDatePF the input date is not a string: %v", v)
		return nil
	}
	y, m, d, err := rdf.ParseDateComponents(vv)
	if err != nil {
		// return fmt.Errorf("error: in FormatDatePF the input date is not a valid date: %v", err)
		return nil
	}
	buf.WriteString(fmt.Sprintf("%d%02d%02d", y, m, d))
	return nil
}
