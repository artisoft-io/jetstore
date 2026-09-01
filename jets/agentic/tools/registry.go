// Package tools is the agentic tool layer: one Go registry with two
// exposures (plan section 6, settled Q-4). The Phase-1 loop calls the
// registry in-process — "dispatch to a Go function" — and the stdio MCP
// adapter (jets/agentic/mcp) is a client of this package, not the calling
// convention. The signatures are GENERATED into jets_agentic_tools.json by
// `jets-agentic generate` and embedded here; Go binds behaviour to the
// names, and TestSignaturesAndHandlersAgree fails the suite if the two sets
// drift.
package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed jets_agentic_tools.json
var signaturesJSON []byte

// Signature is one generated tool signature. Reversibility ("na",
// "reversible", "irreversible" — proposal section 6.3) and the minimum
// autonomy tier ride along from day one; retrofitting them across a Phase-2
// catalogue is worse than carrying them with three tools.
type Signature struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Reversibility string          `json:"reversibility"`
	MinTier       string          `json:"min_tier"`
	InputSchema   json.RawMessage `json:"input_schema"`
}

type catalog struct {
	Catalog string      `json:"catalog"`
	Version string      `json:"version"`
	Tools   []Signature `json:"tools"`
}

// Handler is a tool implementation. It takes the resolved workspace handle
// — never a path — and the tool's arguments as JSON; it returns a
// JSON-serialisable result.
type Handler func(ctx context.Context, ws *Workspace, args json.RawMessage) (any, error)

// Tool is a signature bound to its implementation.
type Tool struct {
	Signature
	Handler Handler
}

// Registry is the one interface both exposures share.
type Registry struct {
	tools map[string]*Tool
}

// NewRegistry binds handlers to the embedded signatures by name. Every
// handler must have a signature; signatures without a handler are reported
// by Unbound rather than failing here, so a partial registry is usable in
// tests.
func NewRegistry(handlers map[string]Handler) (*Registry, error) {
	var cat catalog
	if err := json.Unmarshal(signaturesJSON, &cat); err != nil {
		return nil, fmt.Errorf("while parsing embedded tool signatures: %w", err)
	}
	reg := &Registry{tools: map[string]*Tool{}}
	for _, sig := range cat.Tools {
		reg.tools[sig.Name] = &Tool{Signature: sig}
	}
	for name, h := range handlers {
		tool, ok := reg.tools[name]
		if !ok {
			return nil, fmt.Errorf("handler %q has no generated signature", name)
		}
		tool.Handler = h
	}
	return reg, nil
}

// DefaultRegistry is the registry as it stands: the three Phase-0 read-only
// tools, compile_rule_file, which Phase 1 adds as gap 6's second verifier, and
// the two workspace reads J.1 decided in Phase 2 and T.1 built (2026-08-31).
//
// **Six of J.1's seven. All six are read-only and carry reversibility "na".**
// The seventh is propose_workspace_edit, the first tool that changes anything
// and the first that will need a class other than "na"; J.1 sequenced it behind
// K.1's approval record, K.1 is done, and it is still deliberately not here —
// plan §7 declines to take it merely because it became available.
func DefaultRegistry() (*Registry, error) {
	return NewRegistry(map[string]Handler{
		"list_domain_classes":    ListDomainClasses,
		"describe_domain_class":  DescribeDomainClass,
		"validate_cpipes_config": ValidateCpipesConfig,
		"compile_rule_file":      CompileRuleFile,
		"list_workspace_files":   ListWorkspaceFiles,
		"read_workspace_file":    ReadWorkspaceFile,
	})
}

// List returns the signatures in name order.
func (r *Registry) List() []Signature {
	out := make([]Signature, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t.Signature)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Unbound returns the names of signatures with no handler.
func (r *Registry) Unbound() []string {
	var out []string
	for name, t := range r.tools {
		if t.Handler == nil {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// Call dispatches by name — this, not MCP, is how the Phase-1 loop reaches
// a tool.
func (r *Registry) Call(ctx context.Context, ws *Workspace, name string, args json.RawMessage) (any, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("no tool named %q in the registry", name)
	}
	if tool.Handler == nil {
		return nil, fmt.Errorf("tool %q has a signature but no handler bound", name)
	}
	return tool.Handler(ctx, ws, args)
}
