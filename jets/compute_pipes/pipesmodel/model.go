// Package pipesmodel is the public surface of the compute pipes operator
// contract: what a site-specific operator must be able to name, and nothing
// else.
//
// **It is 13 types and it imports only the standard library**, which is the
// whole point. `jets/compute_pipes` is 144 files and 51,212 lines and reaches
// Apache Arrow, the AWS SDK, pgx and the jsii runtime; a package implementing a
// three-method interface should inherit none of that. Phase 9's `BC.1` decided
// the surface and `BC.2` moved it.
//
// **Every name here is aliased back into `compute_pipes`**, so no file outside
// that package changed when this one was created, and every reference in
// `tools/cpipes_contract` still resolves.
//
// **The alias does not make the split free, and the plan said it did.** An alias
// keeps code compiling; it says nothing about the line numbers of the file the
// types left. 148 lines came out of `pipes_model.go` and everything below them
// moved up, displacing 88 `file:line` citations across 23 documents and all
// three projects in the parent repository. 78 were repaired at `BC.2`; the rest
// are struck-through text or the rows of a past audit's repair table, where the
// old number is the record. Budget that audit before moving types out of a file
// this documented.
//
// # What is deliberately absent
//
// `TransformationSpec` does not cross. It is a union carrying one pointer per
// built-in operator, so 38 of the 57 types reachable from it are configuration
// a site can neither use nor extend -- and it has no field a site's own
// configuration could occupy (`I-766`).
//
// `OutputChannelConfig` does not cross either, and that is `BC.2`'s correction
// to `BC.1`: a factory receives the *resolved* `*OutputChannel`, never the spec
// the builder resolved it from. Dropping it takes the surface from 19 types to
// 13 and removes `parquet_schema_info.go` from the closure -- which is what
// makes the stdlib-only property reachable at all, because
// `ParquetSchemaInfo` holds an unexported `*arrow.Schema` and cannot cross
// without Apache Arrow coming with it.
//
// `ComputePipesConfig` does not cross: 27 further types against seven fields
// the operator constructors actually read, five of them scalars.
package pipesmodel

import (
	"fmt"
	"strings"
)

// DomainKeysSpec contains the overall information, with overriding hashing method.
// The hashing method is applicable to all object types.
// DomainKeys is a map keyed by the object type.
type DomainKeysSpec struct {
	HashingOverride string                    `json:"hashing_override,omitempty"`
	DomainKeys      map[string]*DomainKeyInfo `json:"domain_keys_info,omitempty"`
}

// DomainKeyInfo associates a domain hashed key made as a composide domain
// key with an optional prep-processing function on each of the column making the key.
// KeyExpr is the original function(column) expression.
// Columns: list of input column name making the domain key
// PreprocessFnc: list of pre-processing functions for the input column (one per column)
// ObjectType: Object type associated with the Domain Key
type DomainKeyInfo struct {
	KeyExpr    []string `json:"key_expr,omitempty"`
	ObjectType string   `json:"object_type,omitempty"`
}

type InputChannel struct {
	Name           string
	Channel        <-chan []any
	Columns        *map[string]int
	DomainKeySpec  *DomainKeysSpec
	Config         *ChannelSpec
	HasGroupedRows bool
}

type OutputChannel struct {
	Name    string
	Channel chan<- []any
	Columns *map[string]int
	Config  *ChannelSpec
}

type PipeTransformationEvaluator interface {
	Apply(input *[]any) error
	Done() error
	Finally()
}

// ChannelSpec specifies the columns of a channel and other properties.
// The columns can be obtained from a domain class from the
// local workspace using class_name.
// In that case, the columns
// that are specified in the slice, are added to the columns of
// the domain class.
// When direct_properties_only is true, only take the data properties
// of the class, not including the properties of the parent classes.
// ClassName is used to get the columns from the local workspace, and get domain key from registry, and is optional.
// Env variables (from mainInputSchemaProvider.Env) can be used in the class_name, e.g., hc:${ENTITY}.
// DomainKeys provide the ability to configure the domain keys in the cpipes config document.
// DomainKeysInfo is obtained from the domain_keys_registry table or derived from DomainKeys - the latter takes precedence when both are available.
// columnsMap is added in StartComputePipes
type ChannelSpec struct {
	Comment              string                `json:"comment,omitempty"` // free text for the reader; ignored by JetStore
	Name                 string                `json:"name"`
	Columns              []string              `json:"columns"`
	ClassName            string                `json:"class_name,omitempty"`
	DirectPropertiesOnly bool                  `json:"direct_properties_only,omitzero"`
	HasDynamicColumns    bool                  `json:"has_dynamic_columns,omitzero"`
	SameColumnsAsInput   bool                  `json:"same_columns_as_input,omitzero"`
	DomainKeys           map[string]any        `json:"domain_keys,omitempty"`
	DomainKeysInfo       *DomainKeysSpec       `json:"domain_keys_spec,omitzero"`
	ColumnEncodings      []*ColumnEncodingSpec `json:"column_encodings,omitzero"`
	columnsMap           *map[string]int
}

// ColumnsMap and SetColumnsMap are how `compute_pipes` reaches a field that
// stays unexported on purpose.
//
// **`columnsMap` is derived state and not configuration**: it is the column
// name to position index, built in `StartComputePipes` from `Columns` or taken
// from the input headers, and read once when the channel is constructed. It has
// no business being set by whoever writes a `.pc.json` or a site operator, and
// exporting the field to let the builder reach it across the package boundary
// would advertise it to exactly the readers who should not see it.
//
// **This is the second type in the contract carrying package-private derived
// state, and the first one is why the surface is 13 types rather than 19**:
// `ParquetSchemaInfo` holds an unexported `*arrow.Schema` and could not cross
// at all without bringing Apache Arrow. That one was removed from the surface;
// this one is reachable from `InputChannel.Config` and had to be kept, so it is
// kept behind two methods instead. `I-767` asks whether the field belongs on
// the spec at all.
func (s *ChannelSpec) ColumnsMap() *map[string]int { return s.columnsMap }

func (s *ChannelSpec) SetColumnsMap(m *map[string]int) { s.columnsMap = m }

// ColumnEncodingSpec is used to specify special encoding for a channel column, e.g., toon or json
// Column is the column name to which the special encoding applies, this is required.
// EntityEncoding is used to specify the encoding of the column: range values: json, toon, briefing_prose (default is json).
// RemoveModelPrefixes is used to remove the model prefixes from the columns, e.g., jets: or rdf: on the output (any prefix up to the character ':').
// ExcludeProperties is used to specify the properties to exclude from the output, e.g., jets:key, rdf:type, etc.
// This is used to exclude properties from the json or toon output.
type ColumnEncodingSpec struct {
	Comment             string   `json:"comment,omitempty"` // free text for the reader; ignored by JetStore
	Column              string   `json:"column"`
	EntityEncoding      string   `json:"entity_encoding,omitempty"`
	RemoveModelPrefixes bool     `json:"remove_model_prefixes,omitzero"`
	ExcludeProperties   []string `json:"exclude_properties,omitempty"`
}

type TransformationColumnSpec struct {
	Comment string `json:"comment,omitempty"` // free text for the reader; ignored by JetStore
	// Type range: select, multi_select, value, eval, map, hash
	// count, distinct_count, sum, min, max, avrg, case,
	// map_reduce, lookup
	// AsRdfType applies to expr with non-aggragate operators: select, multi_select, value
	// AsRdfType applies to expr with aggragate operators: min, max, sum, avrg
	// MaxEnvVarSubstitution applies to expr with env var substitution: select, multi_select, value, lookup
	Name                  string                      `json:"name"`
	Type                  string                      `json:"type"`
	Expr                  *string                     `json:"expr,omitempty"`
	ExprArray             []string                    `json:"expr_array,omitempty"`
	MapExpr               *MapExpression              `json:"map_expr,omitzero"`
	EvalExpr              *ExpressionNode             `json:"eval_expr,omitzero"`
	HashExpr              *HashExpression             `json:"hash_expr,omitzero"`
	Where                 *ExpressionNode             `json:"where,omitzero"`
	CaseExpr              []CaseExpression            `json:"case_expr,omitempty"` // case operator
	ElseExpr              []*TransformationColumnSpec `json:"else_expr,omitempty"` // case operator
	MapOn                 *string                     `json:"map_on,omitzero"`
	AlternateMapOn        []string                    `json:"alternate_map_on,omitempty"`
	ApplyMap              []TransformationColumnSpec  `json:"apply_map,omitempty"`
	ApplyReduce           []TransformationColumnSpec  `json:"apply_reduce,omitempty"`
	LookupName            *string                     `json:"lookup_name,omitzero"`
	LookupKey             []LookupColumnSpec          `json:"key,omitempty"`
	LookupValues          []LookupColumnSpec          `json:"values,omitempty"`
	MaxEnvVarSubstitution int                         `json:"max_env_var_substitution,omitzero"`
	AsRdfType             string                      `json:"as_rdf_type,omitempty"`
}

type LookupColumnSpec struct {
	Comment string `json:"comment,omitempty"` // free text for the reader; ignored by JetStore
	// Type range: select, value
	// MaxEnvVarSubstitution applies to expr with env var substitution: value
	Name                  string  `json:"name,omitempty"`
	Type                  string  `json:"type,omitempty"`
	Expr                  *string `json:"expr,omitzero"`
	MaxEnvVarSubstitution int     `json:"max_env_var_substitution,omitzero"`
}

// Hash using values from columns.
// Case single column, use Expr.
// Case multi column, use CompositeExpr.
// Expr takes precedence if both are populated.
// DomainKey is specified as an object_type. DomainKeysJson provides the
// mapping between domain keys and columns.
// AlternateCompositeExpr is used when Expr or CompositeExpr returns nil or empty.
// MultiStepShardingMode values: 'limited_range', 'full_range' or empty.
// NoPartitions indicated not to assign the hash to a partition (no modulo operation).
// NbrJetsPartitions is the number of partitions to use for the hash operator when NoPartitions is false.
// MaxNbrJetsPartitions use the minimum between the cluster nbr of partitions and this setting provided the NoPartitions is false.
// NbrJetsPartitions takes precedence over MaxNbrJetsPartitions when both are provided.
// ComputeDomainKey flag indicate to compute the domain key rather than a simple hash.
// This consider the hashing algo used and delimitor between the key components.
type HashExpression struct {
	Comment                 string   `json:"comment,omitempty"` // free text for the reader; ignored by JetStore
	Expr                    string   `json:"expr,omitempty"`
	CompositeExpr           []string `json:"composite_expr,omitempty"`
	DomainKey               string   `json:"domain_key,omitempty"`
	NbrJetsPartitionsAny    any      `json:"nbr_jets_partitions,omitzero"`
	MaxNbrJetsPartitionsAny any      `json:"max_nbr_jets_partitions,omitzero"`
	MultiStepShardingMode   string   `json:"multi_step_sharding_mode,omitempty"`
	AlternateCompositeExpr  []string `json:"alternate_composite_expr,omitempty"`
	NoPartitions            bool     `json:"no_partitions,omitzero"`
	ComputeDomainKey        bool     `json:"compute_domain_key,omitzero"`
}

func (h *HashExpression) String() string {
	var b strings.Builder
	b.WriteString("HashExpression(")
	if h.Expr != "" {
		fmt.Fprintf(&b, "Expr: %s, ", h.Expr)
	}
	if len(h.CompositeExpr) > 0 {
		fmt.Fprintf(&b, "CompositeExpr: %v, ", h.CompositeExpr)
	}
	if h.DomainKey != "" {
		fmt.Fprintf(&b, "DomainKey: %s, ", h.DomainKey)
	}
	if h.MultiStepShardingMode != "" {
		fmt.Fprintf(&b, "MultiStepShardingMode: %s, ", h.MultiStepShardingMode)
	}
	if len(h.AlternateCompositeExpr) > 0 {
		fmt.Fprintf(&b, "AlternateCompositeExpr: %v, ", h.AlternateCompositeExpr)
	}
	if h.NoPartitions {
		b.WriteString("NoPartitions: true, ")
	}
	if h.ComputeDomainKey {
		b.WriteString("ComputeDomainKey: true")
	}
	b.WriteString(")")
	return b.String()
}

type MapExpression struct {
	Comment           string            `json:"comment,omitempty"` // free text for the reader; ignored by JetStore
	CleansingFunction string            `json:"cleansing_function,omitempty"`
	Argument          string            `json:"argument,omitempty"`
	Default           string            `json:"default,omitempty"`
	ErrMsg            string            `json:"err_msg,omitempty"`
	CodeValueMapping  map[string]string `json:"code_value_mapping,omitempty"`
	RdfType           string            `json:"rdf_type,omitempty"`
}

type ExpressionNode struct {
	Comment string `json:"comment,omitempty"` // free text for the reader; ignored by JetStore
	// Name is for the special case CaseEnvExpression
	// Type is for leaf nodes: select, value, expr_proxy, function
	// Expr is for leaf nodes, the expression to evaluate:
	// - for Type: select, it is the column name to select or substitute with env var
	//   substitution if it contains the char '$'.
	// - for Type: value, it is the value to use or substitute with env var
	//   substitution if it contains the char '$'.
	// ExprPos is for leaf nodes for Type select, it is the 0-based column position to select,
	// it is an alternative to Expr which is the column name.
	// ExprList is for leaf nodes with multiple values, used for the `in`` operator.
	// MaxEnvVarSubstitution indicates how many loop of env substitution to do for
	// Expr containinng the char '$', default to 3.
	// For non leaf nodes, Op is the operator: and, or, ==, !=, >, >=, <, <=, etc.
	// Special case for type: expr_proxy, it indicates that the expression is a proxy
	// for another expression, the actual expression is specified by one of:
	// - ExprEnvVarProxy: the expression is specified by an env var, the value of
	//   the env var is the actual expression as a json string to evaluate.
	// (more to come)
	// Special case for type: function, it indicates that the expression is a function call,
	// the actual function is specified by Expr, and the arguments are specified by Farg.
	// Default value to use when the evaluation returns error
	Name                  string           `json:"name,omitempty"`
	Type                  string           `json:"type,omitempty"`
	Expr                  string           `json:"expr,omitempty"`
	ExprPos               *int             `json:"expr_pos,omitempty"`
	ExprList              []string         `json:"expr_list,omitempty"`
	MaxEnvVarSubstitution int              `json:"max_env_var_substitution,omitzero"`
	AsRdfType             string           `json:"as_rdf_type,omitempty"`
	Arg                   *ExpressionNode  `json:"arg,omitzero"`
	Lhs                   *ExpressionNode  `json:"lhs,omitzero"`
	Op                    string           `json:"op,omitempty"`
	Rhs                   *ExpressionNode  `json:"rhs,omitzero"`
	ExprEnvVarProxy       string           `json:"expr_env_var_proxy,omitempty"`
	Farg                  []ExpressionNode `json:"function_arguments,omitzero"`
	Default               *ExpressionNode  `json:"default,omitzero"`
}

type CaseExpression struct {
	Comment string                      `json:"comment,omitempty"` // free text for the reader; ignored by JetStore
	When    ExpressionNode              `json:"when"`
	Then    []*TransformationColumnSpec `json:"then"`
}
