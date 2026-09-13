package pipesmodel

import "encoding/json"

// TransformationColumnEvaluator computes one column of an output record.
//
// It lived in `compute_pipes/pipes_runtime_model.go` until Phase 9's `BD.2` and
// moved here because it is `OperatorEnv.ColumnEvaluator`'s return type: a site
// operator that honours the step's authored `columns` has to be able to name
// what it gets back. `compute_pipes` keeps a type alias, so nothing outside this
// package changed.
//
// Initialize and Done are intended for aggregate transformations column evaluators.
type TransformationColumnEvaluator interface {
	Update(currentValue *[]any, input *[]any) error
	Done(currentValue *[]any) error
}

// OperatorEnv is what a site-supplied operator is given while it is being built.
//
// It is an interface JetStore implements and a site consumes, which is what makes
// the bias to admit less nearly free: a method added here is backward compatible
// for every consumer, and a method removed is not.
//
// # What is deliberately absent, and how to read that
//
// `BuilderContext` has 18 fields and the operator constructors reach into them
// freely. Six of those reaches are here; the rest are withheld with a named
// reader and a stated reason in Phase 9 §12.6 of the `agentic_ai` plan. The
// short form: the channel registry is withheld because an operator that can name
// any channel can read from or write into one it does not own -- the one use it
// genuinely needs, the error channel, is resolved by the builder and handed over
// on OperatorArgs; the pgx pool and the S3 device manager are withheld because
// admitting them would put those dependencies back into a package whose whole
// purpose is not to have them.
//
// # Writing a test double
//
// A site that consumes this interface is not broken by a method being added; a
// site that *implements* one is, and a test double is an implementation. **Embed
// OperatorEnv in the fake and override only what the test needs**, so that a
// seventh method compiles and panics on use rather than failing the build
// (`I-781`).
//
//	type fakeEnv struct {
//		pipesmodel.OperatorEnv // nil; any method not overridden panics
//		done chan struct{}
//	}
//	func (f *fakeEnv) Done() <-chan struct{} { return f.done }
type OperatorEnv interface {
	// Done is closed when the node is terminating. An operator that can block
	// on a channel send must select on it, which is what every built-in does
	// in Apply.
	Done() <-chan struct{}

	// Substitute applies the pipeline's environment-variable substitution to a
	// configuration string, the way every built-in does to a path, a bucket, a
	// step id or a prompt.
	Substitute(value string) string

	// EnvValue reads one environment value by name. The map is keyed both bare
	// and dollar-prefixed -- `nbr_partitions` and `$NBR_PARTITIONS` are both
	// written -- so Substitute reaches the second set and this reaches either.
	EnvValue(name string) (any, bool)

	// ColumnEvaluator builds the runtime evaluator for one entry of the
	// operator's `columns`, so that a site operator gets the authored column
	// vocabulary rather than reimplementing it.
	ColumnEvaluator(source *InputChannel, outCh *OutputChannel,
		spec *TransformationColumnSpec) (TransformationColumnEvaluator, error)

	// SessionId is the pipeline execution's correlation id, and is the string
	// every log line in compute_pipes is prefixed with.
	SessionId() string

	// IsDebugMode reports whether the deployment asked for verbose logging.
	IsDebugMode() bool
}

// OperatorArgs is what the builder hands a site factory: the common fields of the
// authored step, the resolved channels, and the site's own configuration verbatim.
//
// It is a struct rather than a parameter list for the same reason OperatorEnv is an
// interface JetStore implements: JetStore writes it and the site reads it, so a
// field added here breaks no site.
//
// Two of `TransformationSpec`'s seven common fields are deliberately not here.
// `when` is evaluated before the factory can be reached and returns a nil
// evaluator when it is false; `conditional_config` is applied in the *starter*
// process, before the document reaches this one. Putting either here would invite
// a site operator to re-implement a decision already taken.
type OperatorArgs struct {
	// Type is the operator token as authored, so one factory can serve several.
	Type      string
	Comment   string
	NewRecord bool

	// Columns is the step's `columns` array, to be turned into evaluators with
	// OperatorEnv.ColumnEvaluator.
	Columns []TransformationColumnSpec

	// Source and Output are resolved. Output is nil when the step named none.
	Source *InputChannel
	Output *OutputChannel

	// ErrorChannel is resolved, and nil when the step configured none. An
	// operator that writes a record here is reporting a row-level failure the
	// way map_record, jetrules and render do.
	//
	// **An operator that wants one must author one.** JetStore synthesises a
	// default error channel for the six built-ins it knows report row-level
	// failures; it cannot do that for an operator whose failure modes it knows
	// nothing about, so for a site operator the absence of this channel is the
	// author's choice rather than an oversight.
	ErrorChannel  *OutputChannel
	MaxErrorCount int

	// Config is the site's own configuration, verbatim, for the factory to
	// decode into whatever type it likes. It is the `site_config.config` object
	// of the authored step, and it is nil when the step named none.
	//
	// Raw rather than decoded because the document crosses a process boundary as
	// JSON twice -- the starter marshals it into `jetsapi.cpipes_execution_status`
	// and the node unmarshals it back -- and json.RawMessage is the one field
	// type that survives both hops without JetStore knowing the site's schema.
	Config json.RawMessage
}

// SiteOperatorFactory builds one instance of a site-supplied operator.
//
// It is called once per pipe instance, from the `default:` branch of the
// builder's dispatch, which is what makes a site token unable to shadow a
// built-in: the eighteen built-in cases are tried first and a collision never
// reaches here.
//
// Returning a nil evaluator and a nil error is an error. The builder uses a nil
// evaluator to mean "this step's `when` condition was false, do not apply it",
// and a factory that returns one would be silently skipped.
type SiteOperatorFactory func(env OperatorEnv, args *OperatorArgs) (PipeTransformationEvaluator, error)
