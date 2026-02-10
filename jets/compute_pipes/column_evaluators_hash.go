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

func (ctx *hashColumnEval) InitializeCurrentValue(currentValue *[]any) {}
func (ctx *hashColumnEval) Update(currentValue *[]any, input *[]any) error {
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
func (ctx *hashColumnEval) Done(currentValue *[]any) error {
	return nil
}

// The Hash operator example (dw_rawfilename is string):
//
//		Case hash function:
//	 =========================================
//			{
//				"name": "jets_partition",
//				"type": "hash",
//				"hash_expr": {
//					"expr": "dw_rawfilename",
//					"composite_expr": ["partion", "dw_rawfilename"],
//					"nbr_jets_partitions": 3,
//					"alternate_composite_expr": ["name", "gender", "format_date(dob)"],
//				}
//
// jets_partition will be of type uint64
//
//		Case compute domain key:
//	 =========================================
//			{
//				"name": "Claim:domain_key",
//				"type": "hash",
//				"hash_expr": {
//					"domain_key": "Claim",
//					"compute_domain_key": true
//				}
//
// Claim:domain_key will be of type string
func (ctx *BuilderContext) BuildHashTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.HashExpr == nil {
		return nil, fmt.Errorf("error: Type 'hash' must have field 'hash_expr' != nil")
	}
	outputPos, ok := (*outCh.Columns)[spec.Name]
	if !ok {
		return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.Name)
	}
	hashEvaluator, err := ctx.NewHashEvaluator(source, spec.HashExpr)

	return &hashColumnEval{
		hashEvaluator: hashEvaluator,
		outputPos:     outputPos,
	}, err
}

// Hashing Algo supported
type HashingAlgoEnum int

const (
	HashingAlgo_None HashingAlgoEnum = iota
	HashingAlgo_SHA1
	HashingAlgo_MD5
)

func (e HashingAlgoEnum) String() string {
	return [...]string{"none", "sha1", "md5"}[e]
}

// HashEvaluator is a type to compute a hash key based on an input record.
// The hashing algo can be sha1, md5 or none. This is used to compute domain key ONLY.
// For regular hash-based partitioning, the hashing algo is always FNV-1a 64bit, and the hash key is always uint64.
type HashEvaluator struct {
	inputPos          int
	compositeInputKey []PreprocessingFunction
	partitions        uint64
	altInputKey       []PreprocessingFunction
	computeDomainKey  bool
	hashingAlgo       HashingAlgoEnum
	delimit           string
	// debugCount int
}

func (ctx *HashEvaluator) String() string {
	var b strings.Builder
	b.WriteString("HashEvaluator(")
	if ctx.inputPos > -1 {
		fmt.Fprintf(&b, "inputPos=%d, ", ctx.inputPos)
	}
	if len(ctx.compositeInputKey) > 0 {
		b.WriteString("compositeInputKey=[")
		for i, pf := range ctx.compositeInputKey {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(pf.String())
		}
		b.WriteString("], ")
	}
	if ctx.computeDomainKey {
		b.WriteString("computeDomainKey=true, ")
		fmt.Fprintf(&b, "hashingAlgo=%s, ", ctx.hashingAlgo.String())
	}
	if ctx.partitions > 0 {
		fmt.Fprintf(&b, "partitions=%d, ", ctx.partitions)
	}
	if len(ctx.altInputKey) > 0 {
		b.WriteString("altInputKey=[")
		for i, pf := range ctx.altInputKey {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(pf.String())
		}
		b.WriteString("], ")
	}
	fmt.Fprintf(&b, "delimit='%s')", ctx.delimit)
	return b.String()
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

	// perform some validation
	switch {
	case exprLen > 0 && (compositeLen > 0 || domainKeyLen > 0):
		return nil, fmt.Errorf("error: HashExpression cannot have both Expr and CompositeExpr/DomainKey")
	case compositeLen > 0 && domainKeyLen > 0:
		return nil, fmt.Errorf("error: HashExpression cannot have both CompositeExpr and DomainKey")
	case exprLen == 0 && compositeLen == 0 && domainKeyLen == 0:
		return nil, fmt.Errorf("error: HashExpression must have one of Expr, CompositeExpr or DomainKey")
	}

	inputPos := -1
	var compositeInputKey []PreprocessingFunction
	var ok bool
	var domainKeyInfo *DomainKeyInfo
	hashingAlgo := HashingAlgo
	hashingEnum := HashingAlgo_None

	if exprLen > 0 {
		inputPos, ok = (*source.Columns)[spec.Expr]
		if !ok {
			// assuming it's using a pre-processing function (will fail later if not)
			inputPos = -1
			spec.CompositeExpr = []string{spec.Expr}
			compositeLen = 1
			exprLen = 0
		}
	}
	if compositeLen > 0 || domainKeyLen > 0 {
		var keys []string
		if domainKeyLen > 0 {
			dk := source.DomainKeySpec
			if dk == nil {
				return nil, fmt.Errorf(
					"error: hash operator is configured with domain key but no domain key spec available on source '%s'", source.Config.Name)
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
			if len(source.DomainKeySpec.HashingOverride) > 0 {
				if source.DomainKeySpec.HashingOverride == "none" {
					// This is the case of domain_table or performing a merge with ordered sources.
					hashingAlgo = "none"
					toUpper = len(keys) > 1
				} else {
					hashingAlgo = source.DomainKeySpec.HashingOverride
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
					"error: unknown hashing algo '%s' for computing domain key, expecting sha1, md5 or none, check JETS_DOMAIN_KEY_HASH_ALGO",
					hashingAlgo)
			}
		}

		compositeInputKey, err = ParsePreprocessingExpressions(keys, toUpper, source.Columns)
		if err != nil {
			return nil, fmt.Errorf("while calling ParsePreprocessingExpressions (input channel name %s): %v", source.Name, err)
		}
	}

	var partitions uint64
	if !spec.NoPartitions {
		if spec.NbrJetsPartitionsAny != nil {
			partitions = spec.NbrJetsPartitions()
		} else {
			partitions = uint64(ctx.cpConfig.ClusterConfig.NbrPartitions(spec.MultiStepShardingMode))
		}
	}
	var altInputKey []PreprocessingFunction
	if len(spec.AlternateCompositeExpr) > 0 {
		altInputKey, err = ParsePreprocessingExpressions(spec.AlternateCompositeExpr, true, source.Columns)
		if err != nil {
			return nil, fmt.Errorf("%v in source name %s", err, source.Name)
		}
	}
	// var debugCount int
	// if ctx.cpConfig.ClusterConfig.IsDebugMode {
	// 	debugCount = 100
	// }

	return &HashEvaluator{
		inputPos:          inputPos,
		compositeInputKey: compositeInputKey,
		partitions:        partitions,
		altInputKey:       altInputKey,
		computeDomainKey:  spec.ComputeDomainKey,
		hashingAlgo:       hashingEnum,
		delimit:           DomainKeyDelimit,
		// debugCount: debugCount,
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
		hashedValue = Hash(fmt.Appendf(nil, "%v", vv), partitions)
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

	// compute the hash of value @ inputPos or using compositeInputKey, if it's nil use the alternate (composite) key
	var inputVal, hashedValue any
	switch {
	case ctx.inputPos > -1:
		if ctx.inputPos < len(input) {
			inputVal = input[ctx.inputPos]
		} else {
			return nil, fmt.Errorf("error: HashEvaluator.ComputeHash called with invalid read key position of %d for len of %d", ctx.inputPos, len(input))
		}
	case len(ctx.compositeInputKey) > 0:
		var buf bytes.Buffer
		err = makeAlternateKey(&buf, &ctx.compositeInputKey, "", &input)
		if err != nil {
			return nil, fmt.Errorf("while making composite key for HashEvaluator.ComputeHash: %v", err)
		}
		inputVal = buf.Bytes()
	}

	// If the input value is null and we have an alternate key, use the alternate key to compute the hash
	if inputVal == nil && len(ctx.altInputKey) > 0 {
		// Make the alternate key to hash
		var buf bytes.Buffer
		err = makeAlternateKey(&buf, &ctx.altInputKey, "", &input)
		// fmt.Printf("##### # makeAlternateKey: %v\n", inputVal)
		if err != nil {
			return nil, err
		}
		inputVal = buf.Bytes()
	}

	h := EvalHash(inputVal, ctx.partitions)
	if h != nil {
		hashedValue = *h
		// 	fmt.Printf("##### # EvalHash k: %v, nbr partitions: %d => %v\n", inputVal, ctx.partitions, hashedValue)
		// } else {
		// 	fmt.Printf("##### # EvalHash k: %v, nbr partitions: %d => NULL\n", inputVal, ctx.partitions)
	}
	// if ctx.debugCount > 0 {
	// 	log.Printf("HashEvaluator.ComputeHash debug %d: input=%v => hash=%v\n", ctx.debugCount, inputVal, hashedValue)
	// 	ctx.debugCount--
	// }
	return hashedValue, nil
}

func (ctx *HashEvaluator) ComputeDomainKey(input []any) (any, error) {
	var err error
	var data []byte
	if ctx.inputPos > -1 {
		if ctx.inputPos < len(input) {
			inputVal, ok := input[ctx.inputPos].(string)
			if !ok {
				return nil, fmt.Errorf("error: ComputeDomainKey expected string value at position %d but got %T: %v", ctx.inputPos, input[ctx.inputPos], input[ctx.inputPos])
			}
			if ctx.hashingAlgo == HashingAlgo_None {
				return inputVal, nil
			}
			data = []byte(inputVal)
		} else {
			return nil, fmt.Errorf("error: ComputeDomainKey called with invalid read key position of %d for len of %d", ctx.inputPos, len(input))
		}
	} else {
		// Use the composite key
		var buf bytes.Buffer
		err = makeAlternateKey(&buf, &ctx.compositeInputKey, ctx.delimit, &input)
		if err != nil {
			return nil, err
		}
		if ctx.hashingAlgo == HashingAlgo_None {
			return buf.String(), nil
		}
		data = buf.Bytes()
	}

	switch ctx.hashingAlgo {
	case HashingAlgo_SHA1:
		return uuid.NewSHA1(HashingSeed, data).String(), nil
	case HashingAlgo_MD5:
		return uuid.NewMD5(HashingSeed, data).String(), nil
	default:
		// not expected rto come here since we already check for HashingAlgo_None
		log.Printf("warning: ComputeDomainKey with unknown hashing algo '%s', returning unhashed composite key", ctx.hashingAlgo.String())
		return string(data), nil
	}
}

func makeAlternateKey(buf *bytes.Buffer, compositeKey *[]PreprocessingFunction, delimit string, input *[]any) error {
	var err error
	sz := len(delimit)
	for i, pf := range *compositeKey {
		if i > 0 && sz > 0 {
			buf.WriteString(delimit)
		}
		err = pf.ApplyPF(buf, input)
		if err != nil {
			return err
		}
	}
	return nil
}
