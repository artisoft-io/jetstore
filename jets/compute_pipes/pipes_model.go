package compute_pipes

import "strconv"

// This file contains the Compute Pipes configuration model
type ComputePipesConfig struct {
	RuntimeMetrics []Metric       `json:"runtime_metrics"`
	ClusterConfig  *ClusterSpec   `json:"cluster_config"`
	OutputTables   []TableSpec    `json:"output_tables"`
	Channels       []ChannelSpec  `json:"channels"`
	Context        *[]ContextSpec `json:"context"`
	PipesConfig    []PipeSpec     `json:"pipes_config"`
}

// Config for peer2peer communication
type ClusterSpec struct {
	ReadTimeout             int `json:"read_timeout"`
	WriteTimeout            int `json:"write_timeout"`
	PeerRegistrationTimeout int `json:"peer_registration_timeout"`
}

type Metric struct {
	// Type range: runtime
	// Name values: alloc, total_alloc, sys, nbr_gc
	Type string `json:"type"`
	Name string `json:"name"`
}
type ChannelSpec struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

type ContextSpec struct {
	// Type range: file_key_component
	Type string `json:"type"`
	Key  string `json:"key"`
	Expr string `json:"expr"`
}

type TableSpec struct {
	Key     string            `json:"key"`
	Name    string            `json:"name"`
	Columns []TableColumnSpec `json:"columns"`
}

type TableColumnSpec struct {
	Name    string `json:"name"`
	RdfType string `json:"rdf_type"`
}

type PipeSpec struct {
	// Type range: fan_out, splitter, cluster_map
	Type   string               `json:"type"`
	Input  string               `json:"input"`
	Column *string              `json:"column"` // splitter column
	Apply  []TransformationSpec `json:"apply"`
}

type TransformationSpec struct {
	// Type range: map_record, aggregate, partition_writer
	Type                  string                     `json:"type"`
	PartitionSize         *int                       `json:"partition_size"`
	FilePathSubstitutions *[]PathSubstitution        `json:"file_path_substitutions"`
	Columns               []TransformationColumnSpec `json:"columns"`
	Output                string                     `json:"output"`
}

type PathSubstitution struct {
	Replace string `json:"replace"`
	With    string `json:"with"`
}
type TransformationColumnSpec struct {
	// Type range: select, value, eval, map,
	// (applicable to aggregate) count, distinct_count, sum, min,
	// case, map_reduce
	Name        string                      `json:"name"`
	Type        string                      `json:"type"`
	Expr        *string                     `json:"expr"`
	MapExpr     *MapExpression              `json:"map_expr"`
	EvalExpr    *ExpressionNode             `json:"eval_expr"`
	Where       *ExpressionNode             `json:"where"`
	CaseExpr    []CaseExpression            `json:"case_expr"`
	ElseExpr    *ExpressionNode             `json:"else_expr"`
	MapOn       *string                     `json:"map_on"`
	ApplyMap    *[]TransformationColumnSpec `json:"apply_map"`
	ApplyReduce *[]TransformationColumnSpec `json:"apply_reduce"`
}

type MapExpression struct {
	CleansingFunction *string `json:"cleansing_function"`
	Argument          *string `json:"argument"`
	Default           *string `json:"default"`
	ErrMsg            *string `json:"err_msg"`
	RdfType           string  `json:"rdf_type"`
}

type ExpressionNode struct {
	// Type for leaf node: select, value, eval
	Type     *string         `json:"type"`
	Expr     *string         `json:"expr"`
	EvalExpr *ExpressionNode `json:"eval_expr"`
	Arg      *ExpressionNode `json:"arg"`
	Lhs      *ExpressionNode `json:"lhs"`
	Op       *string         `json:"op"`
	Rhs      *ExpressionNode `json:"rhs"`
}

type CaseExpression struct {
	When ExpressionNode `json:"when"`
	Then ExpressionNode `json:"then"`
}

func toString(v interface{}) string {
	if v != nil {
		// improve this by supporting different types in the splitting column
		switch vv := v.(type) {
		case string:
			return vv
		case int:
			return strconv.Itoa(vv)
		}
	}
	return ""
}
