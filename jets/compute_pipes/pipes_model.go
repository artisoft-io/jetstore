package compute_pipes

// import (
// 	"encoding/json"
// )

type ComputePipesConfig struct {
	OutputTables []TableSpec `json:"output_tables"`
	PipesConfig  []PipeSpec  `json:"pipes_config"`
}

type TableSpec struct {
	Type    string            `json:"type"`
	Key     string            `json:"key"`
	Name    string            `json:"name"`
	Columns []TableColumnSpec `json:"columns"`
}

type TableColumnSpec struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	RdfType string `json:"rdf_type"`
}

type PipeSpec struct {
	Type   string               `json:"type"`
	Input  string               `json:"input"`
	Column string               `json:"column"` // splitter column
	Apply  []TransformationSpec `json:"apply"`
}

type TransformationSpec struct {
	Type    string                     `json:"type"`
	Columns []TransformationColumnSpec `json:"columns"`
	Output  string                     `json:"output"`
}

type TransformationColumnSpec struct {
	Name     string           `json:"name"`
	Type     string           `json:"type"`
	Expr     *string          `json:"expr"`
	EvalExpr *ExpressionNode  `json:"eval_expr"`
	Where    *ExpressionNode  `json:"where"`
	CaseExpr []CaseExpression `json:"case_expr"`
	ElseExpr *ExpressionNode  `json:"else_expr"`
}

type ExpressionNode struct {
	Type     *string         `json:"type"`
	Expr     *string         `json:"expr"`
	EvalExpr *ExpressionNode `json:"eval_expr"`
	Lhs      *ExpressionNode `json:"lhs"`
	Op       *string         `json:"op"`
	Rhs      *ExpressionNode `json:"rhs"`
}

type CaseExpression struct {
	When ExpressionNode `json:"when"`
	Then ExpressionNode `json:"then"`
}
