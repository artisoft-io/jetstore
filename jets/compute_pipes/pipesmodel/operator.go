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
// freely. Five of the six original methods are one of those reaches each; the
// rest are withheld with a named reader and a stated reason in Phase 9 §12.6 of
// the `agentic_ai` plan. The short form: the channel registry is withheld
// because an operator that can name any channel can read from or write into one
// it does not own -- the one use it genuinely needs, the error channel, is
// resolved by the builder and handed over on OperatorArgs; the pgx pool and the
// S3 device manager are withheld because admitting them would put those
// dependencies back into a package whose whole purpose is not to have them.
//
// **The seventh method, ReportError, is of a different kind and §22 is where it
// was decided.** The six are accessors; this one is a service. It reads four
// withheld fields -- `peKey`, `nodeId`, `sessionId` and the step id off
// `cpConfig` -- and exposes none of them, which is what lets a site operator
// write a row that joins to its worker row without being handed the node's
// identity to reason about. Widening the interface was the cheaper of the two
// shapes offered: an accessor for `nodeId` would have fixed one column of the
// row and left the site assembling the other eleven.
//
// # Writing a test double
//
// A site that consumes this interface is not broken by a method being added; a
// site that *implements* one is, and a test double is an implementation. **Embed
// OperatorEnv in the fake and override only what the test needs**, so that a
// method added later compiles and panics on use rather than failing the build
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

	// ReportError writes one row-level failure to an error channel, filling the
	// columns that identify the run: the pipeline execution key, the session id,
	// the shard, the step and the operator type.
	//
	// **It does not fail the pipeline.** An operator fails the node by returning
	// an error from Apply; this reports a record the operator could not handle
	// while the pipeline carries on, which is what map_record, jetrules, render
	// and the inference operators do on their own error channels.
	//
	// ch is nil when the step authored no error channel, and nil is a no-op: the
	// absence is the author's choice (see OperatorArgs.ErrorChannel), so a site
	// operator calls this unconditionally rather than guarding it.
	//
	// **It does not count, and does not enforce OperatorArgs.MaxErrorCount.**
	// Every built-in counts its own errors, because the same cap governs the
	// operator's log line as well as its rows and the count belongs where the
	// operator's other state is -- per pipe for map_record and render, shared
	// atomically across a pool for the inference operators. A helper that
	// silently stopped writing at a threshold its caller could not observe would
	// be a worse trap than the one it removes. See the field's own comment.
	ReportError(ch *OutputChannel, e RowLevelError)
}

// RowLevelError is the per-record half of an error row: what this operator could
// not do with this record. The rest of the row -- which run, which shard, which
// step, which operator, which channel -- is JetStore's and is filled by
// OperatorEnv.ReportError.
//
// **The three fields are the ones JetStore's own operators set**, measured
// against every caller of NewProcessError on 2026-09-15: ErrorMessage at all
// eight sites, InputColumn and RowJetsKey at three each (render, and the two
// inference paths). Two further columns of jetsapi.process_errors are
// deliberately absent. `grouping_key` is set by **no** caller, so admitting it
// would be a field offered on nobody's evidence; `rete_session_saved` and
// `rete_session_triples` are the rules engine's, set from inside a RETE session
// that no site operator has.
//
// An empty string is written as SQL NULL, which is what the built-ins do by
// setting the column only when they have a value for it.
type RowLevelError struct {
	// ErrorMessage is what went wrong, and is the only field a caller must set.
	ErrorMessage string

	// InputColumn is the column the failure is about, when the failure is about
	// one. render names its configured input column; the inference operators
	// name the column they were reading.
	InputColumn string

	// RowJetsKey is the record's own key, so a triage query can find the record
	// the row is about rather than only the step it failed in.
	RowJetsKey string
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

	// Outputs are the channels the step's `site_config.output_channels`
	// declared, resolved by the builder and handed over in the order they were
	// authored. It is nil when the step declared none.
	//
	// **This is ErrorChannel's move pluralised, and it preserves the same
	// argument.** §12.6 withholds `channelRegistry` because an operator that can
	// name any channel can read from or write into one it does not own. An
	// operator handed the channels *its own step* declared can name nothing
	// else: the registry is still unreachable, and what arrives here is a
	// function of the document rather than of anything the operator says at run
	// time. The list is resolved once, in siteOperatorArgs, which is the rule
	// the eighteen built-ins follow.
	//
	// # Choosing between output_channel and output_channels
	//
	// They are different containers and both may be declared. `output_channel`
	// is a field of the *step*, shared with every built-in; the builder resolves
	// it before a factory can be reached and it arrives as Output. `output_channels`
	// is a field of `site_config`, so it exists only for a site operator, and it
	// arrives here. **Outputs never contains Output.**
	//
	// Author `output_channel` alone for the ordinary shape -- one operator, one
	// stream of records. Author `output_channels` when one input fans out into
	// several differently-shaped streams, which is what an operator emitting a
	// dozen tables from one row needs.
	//
	// **Declaring both is admitted rather than refused, and the built-ins are
	// the reason.** `clustering` writes its step's output channel and a
	// correlation channel named in its own config; `anonymize` writes its step's
	// output channel and a keys channel named in its own. A rule refusing the
	// pair would be one JetStore's own operators break. What is refused instead
	// is the thing that is never meaningful: **a name repeated inside
	// `output_channels`**, which hands the operator one channel at two indices
	// with nothing to tell them apart. Naming the step's own `output_channel` in
	// the list as well is *not* refused -- that is an author deliberately
	// unifying two containers so that one loop covers them -- and it hands over
	// the same *OutputChannel the registry holds rather than a second one.
	//
	// # Closing them is not the operator's job
	//
	// The graph closes a channel, not the operator that writes it:
	// ChannelRegistry.CloseChannel is on the far side of §12.6's line for the
	// same reason naming is. Write records and return; select every send against
	// OperatorEnv.Done, which is the only termination signal this package hands
	// over, the way every built-in does in Apply.
	Outputs []*OutputChannel

	// ErrorChannel is resolved, and nil when the step configured none. An
	// operator that writes a record here is reporting a row-level failure the
	// way map_record, jetrules and render do.
	//
	// **An operator that wants one must author one.** JetStore synthesises a
	// default error channel for the six built-ins it knows report row-level
	// failures; it cannot do that for an operator whose failure modes it knows
	// nothing about, so for a site operator the absence of this channel is the
	// author's choice rather than an oversight.
	//
	// Write to it with OperatorEnv.ReportError rather than by assembling a row:
	// the columns that identify the run are not reachable from this package, and
	// a hand-assembled row is missing them silently.
	ErrorChannel *OutputChannel

	// MaxErrorCount is the cap the step authored, and **counting against it is
	// the operator's own job**. ReportError does not count; an operator that
	// wants the built-ins' behaviour reports while its own count is at or below
	// this number, says once that it has reached it, and keeps counting so the
	// summary it logs at Done is the true total.
	//
	// **It is zero when the step named none, and zero means none.** That is the
	// one asymmetry with the built-ins worth knowing before authoring a step.
	// Each of the four built-ins that report row-level failures substitutes its
	// own default for a missing or non-positive value -- 20 for map_record,
	// jetrules and render, 50 for the inference operators, measured 2026-09-15 --
	// and a site operator has none, because JetStore does not know how often an
	// operator it knows nothing about will write a row. **So author a
	// max_error_count beside the error_channel**, or the step reports nothing.
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
