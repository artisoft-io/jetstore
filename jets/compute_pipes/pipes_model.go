package compute_pipes

// This file contains the Compute Pipes configuration model
type ComputePipesConfig struct {
	OutputTables []TableSpec   `json:"output_tables"`
	Channels     []ChannelSpec `json:"channels"`
	PipesConfig  []PipeSpec    `json:"pipes_config"`
}

type ChannelSpec struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
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
	// Type range: fan_out, splitter,
	Type   string               `json:"type"`
	Input  string               `json:"input"`
	Column *string              `json:"column"` // splitter column
	Apply  []TransformationSpec `json:"apply"`
}

type TransformationSpec struct {
	// Type range: map_record, aggregate
	Type    string                     `json:"type"`
	Columns []TransformationColumnSpec `json:"columns"`
	Output  string                     `json:"output"`
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
