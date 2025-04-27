package compute_pipes

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

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

// The Hash operator example (dw_rawfilename is string):
//
//	 case hash function:
//		{
//			"name": "jets_partition",
//			"type": "hash",
//			"hash_expr": {
//				"expr": "dw_rawfilename",
//				"composite_expr": ["partion", "dw_rawfilename"],
//				"nbr_jets_partitions": 3,
//				"alternate_composite_expr": ["name", "gender", "format_date(dob)"],
//			}
//
// jets_partition will be of type uint64
//
//	 case compute domain key:
//		{
//			"name": "Claim:domain_key",
//			"type": "hash",
//			"hash_expr": {
//				"domain_key": "Claim",
//				"compute_domain_key": true
//			}
//
// Claim:domain_key will be of type string
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

// Hashing Algo for computing Domain Key
type HashingAlgoEnum int

const (
	HashingAlgo_None HashingAlgoEnum = iota
	HashingAlgo_SHA1
	HashingAlgo_MD5
)

// HashEvaluator is a type to compute a hask key based on an input record.
type HashEvaluator struct {
	inputPos          int
	compositeInputKey []PreprocessingFunction
	partitions        uint64
	altInputKey       []PreprocessingFunction
	computeDomainKey  bool
	hashingAlgo       HashingAlgoEnum
	delimit           string
}

var HashingSeed uuid.UUID
var HashingAlgo string = strings.ToLower(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO"))
var DomainKeyDelimit string = os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")
func init() {
	var err error
	seed := os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")
	if len(seed) > 0 {
		HashingSeed, err = uuid.Parse(seed)
		if err != nil {
			log.Panicf("while initializing HashingSeed (uuid) from JETS_DOMAIN_KEY_HASH_SEED: %v", err)
		}
	}
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
	if spec.ComputeDomainKey {
		if domainKeyLen == 0 {
			return nil, fmt.Errorf("error: domain_key in hash operator not set while compute_domain_key is true")
		}
		if exprLen > 0 || compositeLen > 0 {
			return nil, fmt.Errorf("error: compute domain key in hash operator with exprLen > 0 || compositeLen > 0")
		}
	}
	inputPos := -1
	var compositeInputKey []PreprocessingFunction
	var ok bool
	var domainKeyInfo *DomainKeyInfo
	hashingAlgo := HashingAlgo
	hashingEnum := HashingAlgo_None

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
				return nil, fmt.Errorf(
					"error: hash operator is configured with domain key but no domain key spec available on source '%s'", source.config.Name)
			}
			domainKeyInfo, ok = dk.DomainKeys[spec.DomainKey]
			if ok {
				keys = domainKeyInfo.KeyExpr
			}
		} else {
			keys = spec.CompositeExpr
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("error: hash operator configured as domain key or composite key has no columns")
		}
		toUpper := true
		if spec.ComputeDomainKey {
			toUpper = len(keys) > 1
			if len(source.domainKeySpec.HashingOverride) > 0 {
				if source.domainKeySpec.HashingOverride == "none" {
					// This is the case of domain_table
					hashingAlgo = "none"
				} else {
					hashingAlgo = source.domainKeySpec.HashingOverride
				}
			}
			switch hashingAlgo {
			case "sha1":
				hashingEnum = HashingAlgo_SHA1
			case "md5":
				hashingEnum = HashingAlgo_MD5
			case "none":
				hashingEnum = HashingAlgo_None
			default:
				return nil, fmt.Errorf(
					"error: unknown hasing also '%s' for computing domain key, expecting sha1, md5 or none, check JETS_DOMAIN_KEY_HASH_ALGO",
					hashingAlgo)
			}
		}
	
		compositeInputKey, err = ParsePreprocessingExpressions(keys, toUpper, source.columns)
		if err != nil {
			return nil, fmt.Errorf("while calling ParsePreprocessingExpressions (input channel name %s): %v", source.name, err)
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
		altInputKey, err = ParsePreprocessingExpressions(spec.AlternateCompositeExpr, true, source.columns)
		if err != nil {
			return nil, fmt.Errorf("%v in source name %s", err, source.name)
		}
	}

	return &HashEvaluator{
		inputPos:          inputPos,
		compositeInputKey: compositeInputKey,
		partitions:        partitions,
		altInputKey:       altInputKey,
		computeDomainKey:  spec.ComputeDomainKey,
		hashingAlgo:       hashingEnum,
		delimit:           DomainKeyDelimit,
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
	if ctx.computeDomainKey {
		return ctx.ComputeDomainKey(input)
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

func (ctx *HashEvaluator) ComputeDomainKey(input []any) (any, error) {
	var buf bytes.Buffer
	var err error
	sz := len(ctx.delimit)
	for i, pf := range ctx.compositeInputKey {
		if i > 0 && sz > 0 {
			buf.WriteString(ctx.delimit)
		}
		err = pf.ApplyPF(&buf, &input)
		if err != nil {
			return nil, err
		}
	}
	switch ctx.hashingAlgo {
	case HashingAlgo_SHA1:
		return uuid.NewSHA1(HashingSeed, buf.Bytes()).String(), nil
	case HashingAlgo_MD5:
		return uuid.NewMD5(HashingSeed, buf.Bytes()).String(), nil
	default:
		return buf.String(), nil
	}
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
