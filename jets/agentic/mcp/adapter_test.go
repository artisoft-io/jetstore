// T.1's premise, checked rather than repeated.
//
// The plan says MCP exposure of a new tool is *free* because the adapter
// enumerates reg.List(). That is a claim about this package, and this package
// had no test — so the claim had been read off NewServer's loop and never run.
// It holds, with one qualification worth more than the confirmation: exposure
// is free **once the handler is bound**, because NewServer refuses a registry
// with any unbound signature. A signature added to toolsig.py without a Go
// handler does not get exposed with a stub; it takes the whole MCP server down.
package mcpadapter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A real client over an in-memory transport, so what is asserted is what a
// client is served rather than what the loop was written to serve.
func TestAdapterServesEveryRegistrySignature(t *testing.T) {
	reg, err := tools.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	// A nil workspace is enough: nothing is called here, and resolving a real
	// one would make this a test of the fixture.
	srv, err := NewServer(reg, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	clientT, serverT := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t1-check", Version: "0"}, nil).
		Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	served := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		served[tool.Name] = tool
	}
	for _, sig := range reg.List() {
		tool, ok := served[sig.Name]
		if !ok {
			t.Errorf("%s is in the registry and is not served over MCP", sig.Name)
			continue
		}
		if tool.Description != sig.Description {
			t.Errorf("%s: the served description is not the generated one", sig.Name)
		}
		if !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s: reversibility %q should give a read-only hint", sig.Name, sig.Reversibility)
		}
	}
	if len(served) != len(reg.List()) {
		t.Errorf("MCP serves %d tools for a registry of %d", len(served), len(reg.List()))
	}
}

// T.2 changed both verifiers' schemas to a `oneOf` over two required sets, and
// the wire is where an MCP client meets them. relaxExternalRefs walks the whole
// document, so a $ref inside a `oneOf` branch is the case that would have been
// missed — asserted here because the two verifiers are exactly the tools whose
// schemas relax.
func TestWireSchemaKeepsTheOneOf(t *testing.T) {
	reg, err := tools.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"compile_rule_file":      {"rule_text", "rule_path"},
		"validate_cpipes_config": {"config", "config_path"},
	}
	for _, sig := range reg.List() {
		alts, ok := want[sig.Name]
		if !ok {
			continue
		}
		wire, err := WireSchema(sig.InputSchema)
		if err != nil {
			t.Fatalf("%s: %v", sig.Name, err)
		}
		var doc struct {
			Properties map[string]json.RawMessage `json:"properties"`
			OneOf      []struct {
				Required []string `json:"required"`
			} `json:"oneOf"`
		}
		if err := json.Unmarshal(wire, &doc); err != nil {
			t.Fatalf("%s: %v", sig.Name, err)
		}
		if len(doc.OneOf) != 2 {
			t.Errorf("%s: the wire schema carries %d oneOf branches, want 2", sig.Name, len(doc.OneOf))
		}
		branches := map[string]bool{}
		for _, b := range doc.OneOf {
			if len(b.Required) == 1 {
				branches[b.Required[0]] = true
			}
		}
		for _, alt := range alts {
			if _, ok := doc.Properties[alt]; !ok {
				t.Errorf("%s: the wire schema has no %s property", sig.Name, alt)
			}
			if !branches[alt] {
				t.Errorf("%s: no oneOf branch requires %s", sig.Name, alt)
			}
		}
	}
}

// The qualification above, as an assertion: a signature with no handler is not
// exposed as a stub, it is refused. This is what makes "exposure is free" a
// statement about a *pair* of changes rather than about the signature alone.
func TestAdapterRefusesAnUnboundSignature(t *testing.T) {
	reg, err := tools.NewRegistry(map[string]tools.Handler{
		"list_domain_classes": func(context.Context, *tools.Workspace, json.RawMessage) (any, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewServer(reg, nil); err == nil {
		t.Error("NewServer built a server for a registry with unbound signatures")
	}
}
