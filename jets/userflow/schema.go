package userflow

import (
	"embed"
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// The library renders its messages through a printer and panics on a nil one;
// it keeps an unexported default and gives no way to reach it.
var messagePrinter = message.NewPrinter(language.English)

// The emitted schemas, committed here so //go:embed can reach them.
//
// **They are copies, and the copy is deliberate — see I-16.** The source of
// truth is TypeScript, in jetsclient_ide/src/{userflow,actions}, which emits the
// JSON Schema beside itself. //go:embed refuses any path outside its own
// directory, so Go needs the artifact under jets/. A build-time copy would work
// and needs the build to cooperate; a committed copy with a test asserting it
// matches needs nothing, and `go test ./...` is something people run.
//
// That shape is not this task's invention. The repository already ruled it for
// generated assets: jets/workspace_assets/data_model/jets_agentic.schema.json is
// an emitted JSON Schema, committed, carrying a "jetstore-owned-asset … DO NOT
// EDIT" header naming its generator. One convention beats a second good idea.
//
// **The one improvement on the precedent is the guard.** That asset's drift
// check is `jets-agentic generate --check`, a command someone has to remember,
// and this repository has no CI — no .github/workflows — so "fails loudly when
// it drifts" means "fails loudly when someone runs it". Here the guard is a Go
// test. Regenerate with:
//
//	UPDATE_SCHEMA=1 go test ./jets/userflow/
//
//go:embed schema/*.json
var schemaFS embed.FS

const (
	flowSchemaFile   = "schema/userflow.schema.json"
	actionSchemaFile = "schema/action.schema.json"
	formSchemaFile   = "schema/form.schema.json"
)

// ownedAssetComment is injected into each copy so the file says what it is when
// somebody opens it in the IDE, which is the only place a reader will meet it.
// JSON has no comments; JSON Schema has `$comment`, which is what the precedent
// asset uses too.
func ownedAssetComment(source string) string {
	return fmt.Sprintf(
		"jetstore-owned-asset — DO NOT EDIT. Source: %s, emitted by `npm run emit-schema` in "+
			"jetsclient_ide. This copy exists because //go:embed cannot reach outside its own "+
			"directory; TestSchemaCopiesMatchSource fails if the two drift, so a local edit is "+
			"caught rather than silently overwritten.",
		source)
}

func compile(name string) (*jsonschema.Schema, error) {
	raw, err := schemaFS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("reading embedded schema %s: %w", name, err)
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("parsing embedded schema %s: %w", name, err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(name, doc); err != nil {
		return nil, fmt.Errorf("adding %s: %w", name, err)
	}
	return c.Compile(name)
}

// findingsFor turns a schema failure into findings carrying a JSON Pointer.
//
// v6 reports a *tree* of causes; the leaves are the useful part, because an
// `anyOf` failure at the root says only "nothing matched" while its leaves say
// which field. Walking to the leaves is what makes the message worth showing.
func findingsFor(err error) []wsvalidate.Finding {
	var ve *jsonschema.ValidationError
	if !asValidationError(err, &ve) {
		return []wsvalidate.Finding{{
			Severity: wsvalidate.Error,
			Code:     "schema",
			Message:  err.Error(),
		}}
	}
	findings := make([]wsvalidate.Finding, 0, 4)
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			findings = append(findings, wsvalidate.Finding{
				Severity: wsvalidate.Error,
				Code:     "schema",
				Message:  e.ErrorKind.LocalizedString(messagePrinter),
				Path:     jsonPointer(e.InstanceLocation),
			})
			return
		}
		for _, cause := range e.Causes {
			walk(cause)
		}
	}
	walk(ve)
	return findings
}

// jsonPointer builds an RFC 6901 pointer from v6's instance location.
//
// Escaping is applied here rather than left to the caller because these segments
// are *document* keys — a state key, an action name — and a document that is
// being rejected is exactly the one whose keys may contain anything.
func jsonPointer(location []string) string {
	if len(location) == 0 {
		return ""
	}
	var b strings.Builder
	for _, segment := range location {
		b.WriteByte('/')
		b.WriteString(strings.ReplaceAll(strings.ReplaceAll(segment, "~", "~0"), "/", "~1"))
	}
	return b.String()
}

func validateAgainst(schemaFile, content string) []wsvalidate.Finding {
	schema, err := compile(schemaFile)
	if err != nil {
		// A broken embedded schema is a deployment fault, not a user's file.
		return []wsvalidate.Finding{{
			Severity: wsvalidate.Error,
			Code:     "schemaUnavailable",
			Message:  err.Error(),
		}}
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(content))
	if err != nil {
		return []wsvalidate.Finding{{
			Severity: wsvalidate.Error,
			Code:     "notJson",
			Message:  err.Error(),
		}}
	}
	if err := schema.Validate(inst); err != nil {
		return findingsFor(err)
	}
	return nil
}

// ValidateFlowDocument is the .uf.json validator the save path dispatches to.
//
// Schema first, then the reference checks — an unparseable document produces one
// useful finding, and running the reference checks on it would produce a second
// set about a document nobody wrote.
//
// **The reachability policy comes from the environment**, so a deployment that
// set JETS_USERFLOW_STRICT_REACHABILITY gets a refusal here rather than a
// warning. See StrictReachabilityEnvVar.
func ValidateFlowDocument(content string) []wsvalidate.Finding {
	if findings := validateAgainst(flowSchemaFile, content); len(findings) > 0 {
		return findings
	}
	flow, err := ParseFlow([]byte(content))
	if err != nil {
		return []wsvalidate.Finding{{Severity: wsvalidate.Error, Code: "notJson", Message: err.Error()}}
	}
	return ValidateFlow(flow, PolicyFromEnv())
}

// ValidateActionDocument is the .ua.json validator.
//
// Schema only. **Whether an `escape` name resolves cannot be checked here** and
// that is not an omission: the escape registry is compiled into the browser
// bundle and the server has no way to enumerate it. A green save does not mean a
// loadable flow — the client checks it at load, in userflow/store.ts.
func ValidateActionDocument(content string) []wsvalidate.Finding {
	return validateAgainst(actionSchemaFile, content)
}

// ValidateFormDocument is the .form.json validator.
//
// Schema only. A form names a table configuration and an action, and neither can
// be checked here: the table configurations are still compiled into the client
// (A.4), and whether an action name resolves depends on the flow's own action
// document, which this file has not been given. The client checks both at load.
func ValidateFormDocument(content string) []wsvalidate.Finding {
	return validateAgainst(formSchemaFile, content)
}

func asValidationError(err error, target **jsonschema.ValidationError) bool {
	ve, ok := err.(*jsonschema.ValidationError)
	if ok {
		*target = ve
	}
	return ok
}
