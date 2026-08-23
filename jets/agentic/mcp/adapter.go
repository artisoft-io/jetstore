// Package mcpadapter is the stdio exposure of the agentic tool registry —
// an adapter over jets/agentic/tools, not the calling convention (plan
// section 6, settled Q-4). The Phase-1 loop calls the registry in-process;
// this package exists so an MCP client (Claude Code, criterion 12) can
// drive the same tools over stdio — whichever ones the registry carries,
// which is the point of the adapter and the reason this sentence no longer
// names a count. The modelcontextprotocol/go-sdk
// dependency is confined here: the registry stays testable with no
// transport, and an HTTP exposure — which no identified consumer needs — is
// one new file later, behind its own security decision.
//
// Stdio only, deliberately. The adapter, not the registry, resolves the
// workspace path: tools take a resolved handle, so the registry interface
// never acquires the stdio adapter's local-disk shape.
package mcpadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer builds an MCP server exposing every bound tool of the registry
// against the resolved workspace.
func NewServer(reg *tools.Registry, ws *tools.Workspace) (*mcp.Server, error) {
	if unbound := reg.Unbound(); len(unbound) != 0 {
		return nil, fmt.Errorf("registry has signatures with no handler: %v", unbound)
	}
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "jetstore-agentic",
		Title:   "JetStore agentic tools",
		Version: "0.1.0",
	}, nil)
	readOnly := true
	for _, sig := range reg.List() {
		name := sig.Name
		wireSchema, err := relaxExternalRefs(sig.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("while preparing %s's schema for the wire: %w", name, err)
		}
		srv.AddTool(&mcp.Tool{
			Name:        name,
			Description: sig.Description,
			InputSchema: wireSchema,
			// The three Phase-0 tools are read-only by design; the day-one
			// metadata rides in _meta for clients that look.
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: sig.Reversibility == "na" && readOnly},
			Meta: mcp.Meta{
				"jetstore.reversibility": sig.Reversibility,
				"jetstore.min_tier":      sig.MinTier,
			},
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.Params.Arguments
			if len(args) == 0 {
				args = json.RawMessage(`{}`)
			}
			result, err := reg.Call(ctx, ws, name, args)
			if err != nil {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				}, nil
			}
			raw, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return nil, err
			}
			return &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
				StructuredContent: result,
			}, nil
		})
	}
	return srv, nil
}

// Run serves over stdio until the client disconnects.
func Run(ctx context.Context, reg *tools.Registry, ws *tools.Workspace) error {
	srv, err := NewServer(reg, ws)
	if err != nil {
		return err
	}
	return srv.Run(ctx, &mcp.StdioTransport{})
}

// relaxExternalRefs makes a generated signature self-contained for the
// wire: a cross-file `$ref` (the emitter's typed truth — for example
// validate_cpipes_config's payload referencing the item-2b schema) cannot
// be resolved by an MCP client, so it is replaced by a permissive object
// schema whose description names the authoritative schema file. Internal
// `#/...` refs pass through untouched.
// WireSchema is relaxExternalRefs under an exported name, for callers that
// must send the registry's schemas to a model over a wire that cannot resolve
// a cross-file $ref — the tool-conformance measurement of J.2 among them.
//
// **Exported so the measurement cannot drift from the adapter.** J.2 asks
// whether a local model can drive the same catalogue Claude drives; a harness
// that relaxed the refs its own way would be measuring a schema no client is
// ever served, and the discrepancy would be invisible in the result.
func WireSchema(schema json.RawMessage) (json.RawMessage, error) {
	return relaxExternalRefs(schema)
}

func relaxExternalRefs(schema json.RawMessage) (json.RawMessage, error) {
	var node any
	if err := json.Unmarshal(schema, &node); err != nil {
		return nil, err
	}
	return json.Marshal(relaxNode(node))
}

func relaxNode(node any) any {
	switch v := node.(type) {
	case map[string]any:
		if ref, ok := v["$ref"].(string); ok && !strings.HasPrefix(ref, "#") {
			out := map[string]any{"type": "object"}
			desc, _ := v["description"].(string)
			if desc != "" {
				desc += " "
			}
			out["description"] = desc + fmt.Sprintf("(Validated against the full schema at %s.)",
				strings.SplitN(ref, "#", 2)[0])
			return out
		}
		out := make(map[string]any, len(v))
		for k, sub := range v {
			out[k] = relaxNode(sub)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, sub := range v {
			out[i] = relaxNode(sub)
		}
		return out
	default:
		return node
	}
}
