package compute_pipes

import (
	"fmt"
	"log"
	"strconv"

	"github.com/dolthub/swiss"
)

// DistinctTransformationPipe keeps the first record it receives for each value
// of the composite key named by distinct_config.distinct_on, and drops the
// records that repeat a key already seen. The record kept is sent on unchanged,
// as soon as it arrives; the key of every kept record is held in memory until
// the operator is done.
//
// The key is exact: it is the encoding built by appendDistinctKey, not a hash,
// so two records share a key only when every distinct_on column holds the same
// value in both. A hash would drop a record whose key merely collided.
type DistinctTransformationPipe struct {
	cpConfig *ComputePipesConfig
	source   *InputChannel
	outputCh *OutputChannel
	// compositeKey holds the positions in the input record of the
	// distinct_on columns, in distinct_on order.
	compositeKey []int
	distinctRows *swiss.Map[string, bool]
	// keyBuf and textBuf are reused from one record to the next, so that
	// building a key allocates only the key's string.
	keyBuf  []byte
	textBuf []byte
	spec    *TransformationSpec
	env     map[string]interface{}
	doneCh  chan struct{}
}

// Implementing interface PipeTransformationEvaluator
func (ctx *DistinctTransformationPipe) Apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in DistinctTransformationPipe")
	}
	// Skip row that are not valid
	inputLen := len(*input)
	expectedLen := len(ctx.source.Config.Columns)
	if inputLen != expectedLen {
		// Skip the row
		return nil
	}

	// make the key, from the distinct_on columns' positions
	buf := ctx.keyBuf[:0]
	for _, pos := range ctx.compositeKey {
		buf, ctx.textBuf = appendDistinctKey(buf, ctx.textBuf, (*input)[pos])
	}
	ctx.keyBuf = buf
	key := string(buf)
	_, ok := ctx.distinctRows.Get(key)
	if !ok {
		ctx.distinctRows.Put(key, true)
		select {
		case ctx.outputCh.Channel <- *input:
		case <-ctx.doneCh:
			log.Println("Distinct Transform interrupted")
		}
	}
	return nil
}

// appendDistinctKey appends one column's value to a composite key and returns
// the key and the scratch buffer, both to be reused.
//
// A nil value is the single byte '~'. Any other value is its text, prefixed by
// the text's length in decimal and a ':', so that no two sequences of values
// encode to the same key: the length says where each value ends, and a nil
// cannot be read as a length because '~' is not a digit. A value's text is
// the string itself, or for any other type what fmt's %v prints, so the
// string "1" and the number 1 are the same key, as they were before; a nil and
// the string "<nil>" are not.
func appendDistinctKey(key, scratch []byte, value any) ([]byte, []byte) {
	text := scratch[:0]
	switch vv := value.(type) {
	case nil:
		return append(key, '~'), scratch
	case string:
		key = strconv.AppendInt(key, int64(len(vv)), 10)
		key = append(key, ':')
		return append(key, vv...), scratch
	case int:
		text = strconv.AppendInt(text, int64(vv), 10)
	case int64:
		text = strconv.AppendInt(text, vv, 10)
	case int32:
		text = strconv.AppendInt(text, int64(vv), 10)
	case float64:
		text = strconv.AppendFloat(text, vv, 'g', -1, 64)
	default:
		text = fmt.Append(text, vv)
	}
	key = strconv.AppendInt(key, int64(len(text)), 10)
	key = append(key, ':')
	return append(key, text...), text
}

func (ctx *DistinctTransformationPipe) Done() error {
	return nil
}

func (ctx *DistinctTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewDistinctTransformationPipe(source *InputChannel, outputCh *OutputChannel, spec *TransformationSpec) (*DistinctTransformationPipe, error) {
	if spec == nil || spec.DistinctConfig == nil {
		return nil, fmt.Errorf("error: Distinct Pipe Transformation spec is missing distinct_config element")
	}
	if len(spec.DistinctConfig.DistinctOn) == 0 {
		// With no key every record has the same, empty, key, and the operator
		// would keep only the first record of its input. The start refuses this
		// first (validateDistinctOn); this is for a spec that did not pass there.
		return nil, fmt.Errorf("configuration error: the distinct operator reading channel '%s' has an empty "+
			"distinct_config.distinct_on: name the columns its records are distinct on", source.Name)
	}
	// Make the composite key
	compositeKey := make([]int, 0, len(spec.DistinctConfig.DistinctOn))
	for _, column := range spec.DistinctConfig.DistinctOn {
		pos, ok := (*source.Columns)[column]
		if !ok {
			return nil, fmt.Errorf("error: key column %s is not in the input channel (distinct operator)", column)
		}
		compositeKey = append(compositeKey, pos)
	}
	return &DistinctTransformationPipe{
		cpConfig:     ctx.cpConfig,
		source:       source,
		outputCh:     outputCh,
		compositeKey: compositeKey,
		distinctRows: swiss.NewMap[string, bool](1000),
		keyBuf:       make([]byte, 0, 64),
		textBuf:      make([]byte, 0, 32),
		spec:         spec,
		env:          ctx.env,
		doneCh:       ctx.done,
	}, nil
}

// validateDistinctOn refuses a distinct operator with no distinct_on, naming its
// step: with no key every record has the same, empty, key, and the operator
// would keep only the first record of its input, which no configuration means.
// ValidatePipeSpecConfig makes this check at every start, and the .pc.json save
// makes it too (configDocumentChecks), so the workspace IDE refuses the save.
//
// A conditional_config override is read with its host, as MergeTransformationSpec
// applies it: an override with a type replaces the host, and one without a type
// replaces the host's distinct_config when it sets one. So a host of type
// distinct may leave distinct_config to its overrides, since a when cannot be
// decided before the run (forEachTransformation), and is refused only when no
// override could supply it; an override that would make the step a distinct
// with no distinct_on is refused whatever its when.
func validateDistinctOn(cpConfig *ComputePipesConfig) error {
	return forEachTransformation(cpConfig, func(site transformationSite, spec *TransformationSpec) error {
		if site.override >= 0 {
			// read with its host, below
			return nil
		}
		hostIsDistinct := spec.Type == "distinct"
		suppliedByOverride := false
		for k, conditional := range spec.ConditionalConfig {
			if conditional == nil {
				continue
			}
			then := &conditional.Then
			becomesDistinct := then.Type == "distinct" || (then.Type == "" && hostIsDistinct)
			setsConfig := then.Type != "" || then.DistinctConfig != nil
			if becomesDistinct && setsConfig && !hasDistinctOn(then.DistinctConfig) {
				return emptyDistinctOnError(fmt.Sprintf("%s, conditional_config[%d]", site.where, k))
			}
			if then.Type != "" || then.DistinctConfig != nil {
				suppliedByOverride = true
			}
		}
		if hostIsDistinct && !hasDistinctOn(spec.DistinctConfig) && !suppliedByOverride {
			return emptyDistinctOnError(site.where)
		}
		return nil
	})
}

func hasDistinctOn(config *DistinctSpec) bool {
	return config != nil && len(config.DistinctOn) > 0
}

func emptyDistinctOnError(where string) error {
	return fmt.Errorf("configuration error: %s is a distinct operator with no distinct_config.distinct_on; "+
		"name the columns its records are distinct on", where)
}
