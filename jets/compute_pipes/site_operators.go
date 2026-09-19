package compute_pipes

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/compute_pipes/pipesmodel"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// Site-supplied operators: the registry, the option that carries it in, and the
// adapter that turns a *BuilderContext into the seven methods a site sees.
//
// The registry stays in `compute_pipes` rather than in `pipesmodel`, and the
// reason is narrower than it looks: a site `main` imports `compute_pipes`
// anyway, because it calls CoordinateComputePipes. What the split buys is that
// the site's *operator package*, its unit tests, and any library of operators
// shared between deployments import `pipesmodel` alone and inherit none of
// Arrow, the AWS SDK or pgx.

// SiteOperatorFactory is re-exported so a site names one package rather than two.
type SiteOperatorFactory = pipesmodel.SiteOperatorFactory

// OperatorEnv, OperatorArgs and RowLevelError likewise.
type OperatorEnv = pipesmodel.OperatorEnv
type OperatorArgs = pipesmodel.OperatorArgs
type RowLevelError = pipesmodel.RowLevelError

// CPOption configures a compute pipes node. It is variadic on
// CoordinateComputePipes so that the six existing call sites -- two lambdas, two
// tasks and the local test driver twice -- compile unchanged.
type CPOption func(*cpOptions)

type cpOptions struct {
	siteOperators map[string]SiteOperatorFactory
}

// WithOperators registers this deployment's own operators.
//
// A site `cp_node` main is the stock main plus this one argument, which is the
// whole of the composition:
//
//	err := (&arg).CoordinateComputePipes(ctx, dbpool, jrProxy,
//		compute_pipes.WithOperators(map[string]compute_pipes.SiteOperatorFactory{
//			"cgt_scrub": scrub.New,
//		}))
//
// **A name that collides with a built-in is registered and never reached**: the
// registry is consulted in the `default:` branch of the builder's dispatch, so
// the eighteen built-ins win. That is a deliberate policy rather than an
// accident of ordering -- a site cannot change what an existing `.pc.json`
// means -- and because losing silently is worse than losing loudly, the
// collision is logged here, at registration, rather than left to be inferred
// from a pipeline that behaves like the stock one.
//
// Calling it more than once merges, later calls winning on a repeated name.
func WithOperators(operators map[string]SiteOperatorFactory) CPOption {
	return func(o *cpOptions) {
		if o.siteOperators == nil {
			o.siteOperators = make(map[string]SiteOperatorFactory, len(operators))
		}
		for name, factory := range operators {
			switch {
			case len(name) == 0:
				log.Println("WARNING: WithOperators: an operator registered under an empty name can never be reached; ignored")
				continue
			case factory == nil:
				log.Printf("WARNING: WithOperators: operator %q registered with a nil factory; ignored", name)
				continue
			case reservedOperatorTypes[name]:
				log.Printf(
					"WARNING: WithOperators: %q is a built-in compute pipes operator; the built-in wins and this factory will never be called",
					name)
			}
			o.siteOperators[name] = factory
		}
	}
}

func applyCPOptions(opts []CPOption) *cpOptions {
	o := &cpOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

// builtinOperatorTypes is the set of tokens BuildPipeTransformationEvaluator
// dispatches on. It exists only so WithOperators can warn about a collision, and
// it is a second list of the same thing -- which is exactly how
// `reportsRowLevelFailures` and `errorChannelConfig` could have drifted apart
// and why that pair has a test. This one has the same test:
// TestBuiltinOperatorTypesMatchesTheDispatch parses the switch out of
// pipes_runtime_model.go and compares.
var builtinOperatorTypes = map[string]bool{
	"map_record":       true,
	"aggregate":        true,
	"partition_writer": true,
	"group_by":         true,
	"merge":            true,
	"distinct":         true,
	"filter":           true,
	"sort":             true,
	"jetrules":         true,
	"ollama":           true,
	"embed":            true,
	"vllm":             true,
	"analyze":          true,
	"anonymize":        true,
	"clustering":       true,
	"high_freq":        true,
	"shuffling":        true,
	RenderOperatorType: true,
}

// reservedOperatorTypes is the set of tokens a deployment may not register an
// operator under. It is `builtinOperatorTypes` plus the tokens that are resolved
// away *before* the dispatch ever sees them, and the distinction is the whole
// reason it is a second map rather than an entry in the first.
//
// `builtinOperatorTypes` means "what BuildPipeTransformationEvaluator dispatches
// on", and TestBuiltinOperatorTypesMatchesTheDispatch asserts that in both
// directions against the parsed switch. `infer` is not in that switch and must
// not be added to it: ResolveInferBackend rewrites every `type: infer` into
// `ollama` or `vllm` before the graph is built, deliberately, so that nothing
// downstream knows the abstract type exists.
//
// **But a site cannot have the token either**, and the failure if it does is the
// quiet kind: a factory registered as "infer" is never called and no collision
// warning fires, because the rewrite happens first and the graph the dispatch
// sees contains no such operator. The author gets an ollama step and no message.
// So the collision check and the `site_config` refusal below read this set, and
// the dispatch-matching test keeps reading the narrower one.
//
// A token belongs here when it is a legal `type` in an authored `.pc.json` that
// some earlier pass consumes. There is one today.
var reservedOperatorTypes = func() map[string]bool {
	m := make(map[string]bool, len(builtinOperatorTypes)+1)
	for k := range builtinOperatorTypes {
		m[k] = true
	}
	m["infer"] = true
	return m
}()

// validateSiteOperatorSpec is what a *starter* can say about a site operator
// without an operator registry, which is not much and is worth having.
//
// The startup path runs in cp_sharding_starter and cp_reducing_starter; the
// registry is an argument to CoordinateComputePipes, which runs in cp_node. So a
// mistyped site token cannot be caught at startup and is reported by the
// dispatch inside a running worker instead (`I-779`). What *is* catchable from
// the document alone is a site_config on a token the dispatch will never take it
// for -- the eighteen built-ins -- and leaving that to be inferred from an
// operator silently ignoring its configuration is the failure this whole design
// exists to avoid one layer up.
func validateSiteOperatorSpec(transformationConfig *TransformationSpec) error {
	if transformationConfig.SiteConfig == nil {
		return nil
	}
	if reservedOperatorTypes[transformationConfig.Type] {
		return fmt.Errorf(
			"configuration error: operator '%s' is a built-in and cannot carry a 'site_config'; "+
				"use its own '%s_config' instead",
			transformationConfig.Type, transformationConfig.Type)
	}
	return nil
}

// operatorEnv adapts a *BuilderContext to the seven methods a site operator sees.
//
// It is a separate unexported type rather than a set of methods on
// *BuilderContext, and that is load-bearing rather than tidiness. Were
// *BuilderContext to satisfy OperatorEnv directly, a site operator could recover
// it -- `env.(interface{ FileKey() string })` needs no exported type name -- and
// reach every exported method the builder has. The withheld list of §12.6 would
// then be a convention rather than a boundary. This type has exactly the seven
// methods and nothing else to assert down to.
//
// It is built once per step, in buildSiteOperator, which is why it can carry the
// operator's type: `operator_type` is a triage discriminator on an error row and
// it is filled from the authored spec rather than from anything the site passes,
// so a site operator cannot report a row under another operator's name.
type operatorEnv struct {
	ctx *BuilderContext
	// operatorType is spec.Type, for the error rows ReportError writes.
	operatorType string
}

var _ OperatorEnv = operatorEnv{}

func (e operatorEnv) Done() <-chan struct{} {
	return e.ctx.done
}

func (e operatorEnv) Substitute(value string) string {
	return utils.ReplaceEnvVars(value, e.ctx.env)
}

func (e operatorEnv) EnvValue(name string) (any, bool) {
	if e.ctx.env == nil {
		return nil, false
	}
	v, ok := e.ctx.env[name]
	return v, ok
}

func (e operatorEnv) ColumnEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {
	return e.ctx.BuildTransformationColumnEvaluator(source, outCh, spec)
}

func (e operatorEnv) SessionId() string {
	return e.ctx.sessionId
}

// IsDebugMode is nil-guarded twice, and the second guard is not defensive
// padding: ClusterConfig is a *ClusterSpec, and `&ComputePipesConfig{}` -- which
// is what a dozen tests in this package build -- leaves it nil. The rest of the
// package reads `ctx.cpConfig.ClusterConfig.IsDebugMode` unguarded because
// UnmarshalComputePipesConfig always fills it in; an accessor a site calls is
// reached from places that never went through that decoder.
func (e operatorEnv) IsDebugMode() bool {
	if e.ctx.cpConfig == nil || e.ctx.cpConfig.ClusterConfig == nil {
		return false
	}
	return e.ctx.cpConfig.ClusterConfig.IsDebugMode
}

// ReportError writes one row-level failure to ch, filling from the builder the
// five columns a site operator cannot reach: the pipeline execution key, the
// session id, the shard (ctx.nodeId), the operator type and the step id. It is
// NewProcessError and write2Chan, which are a method on *BuilderContext and an
// unexported method respectively, reached through the one door a site has.
//
// **Three nils are absorbed rather than guarded by the caller**, and they are not
// the same nil. ch is nil when the step authored no error channel, which is the
// author's choice; ch.Config is what write2Chan sizes the row from; ch.Columns is
// what it places columns by. The first site operator written against this
// interface guarded all three by hand, and that guard is the measurable half of
// what this method removes -- the other half is the copy of setColumn it had to
// make, whose comma-ok form is what keeps the discriminator columns additive.
//
// It does not count. See OperatorArgs.MaxErrorCount.
func (e operatorEnv) ReportError(ch *OutputChannel, rowErr RowLevelError) {
	if ch == nil || ch.Config == nil || ch.Columns == nil {
		return
	}
	peRow := e.ctx.NewProcessError(e.operatorType)
	peRow.ErrorMessage = rowErr.ErrorMessage
	// An empty string is written as NULL, which is what the built-ins do by
	// setting these columns only when they have a value for them.
	if len(rowErr.InputColumn) > 0 {
		peRow.InputColumn = sql.NullString{String: rowErr.InputColumn, Valid: true}
	}
	if len(rowErr.RowJetsKey) > 0 {
		peRow.RowJetsKey = sql.NullString{String: rowErr.RowJetsKey, Valid: true}
	}
	peRow.write2Chan(ch, e.ctx.done)
}

// buildSiteOperator is the `default:` branch of BuildPipeTransformationEvaluator.
//
// It reports whether the token was a registered site operator at all, so that an
// unregistered name still produces the message it produced before this existed
// rather than a new one that says something about a registry the author has
// never heard of.
func (ctx *BuilderContext) buildSiteOperator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationSpec) (pipe PipeTransformationEvaluator, registered bool, err error) {
	factory := ctx.siteOperators[spec.Type]
	if factory == nil {
		return nil, false, nil
	}
	args, err := ctx.siteOperatorArgs(source, outCh, spec)
	if err != nil {
		return nil, true, err
	}
	pipe, err = factory(operatorEnv{ctx: ctx, operatorType: spec.Type}, args)
	if err != nil {
		return nil, true, fmt.Errorf("while building site operator '%s': %v", spec.Type, err)
	}
	if pipe == nil {
		// A nil evaluator is how the builder says "this step's `when` was false,
		// do not apply it" (BuildPipeTransformationEvaluator). A factory that
		// returns one would be skipped for the whole run with nothing said.
		return nil, true, fmt.Errorf(
			"error: site operator '%s' returned a nil evaluator and no error", spec.Type)
	}
	return pipe, true, nil
}

// siteOperatorArgs assembles what the factory is handed. The channels are
// resolved here rather than in the factory, which is the same rule the eighteen
// built-ins follow: the builder owns the channel registry and a site operator
// never sees it.
func (ctx *BuilderContext) siteOperatorArgs(source *InputChannel, outCh *OutputChannel,
	spec *TransformationSpec) (*OperatorArgs, error) {
	args := &OperatorArgs{
		Type:      spec.Type,
		Comment:   spec.Comment,
		NewRecord: spec.NewRecord,
		Columns:   spec.Columns,
		Source:    source,
		Output:    outCh,
	}
	siteConfig := spec.SiteConfig
	if siteConfig == nil {
		return args, nil
	}
	args.Config = siteConfig.Config
	args.MaxErrorCount = siteConfig.MaxErrorCount
	if siteConfig.ErrorChannel != nil {
		// The same two checks map_record makes, and for the same reason: an
		// error channel with no name or no spec name is a configuration error
		// that would otherwise surface as a channel lookup failure naming an
		// empty string.
		if len(siteConfig.ErrorChannel.Name) == 0 {
			return nil, fmt.Errorf("error: site_config.error_channel name cannot be empty (operator '%s')", spec.Type)
		}
		if len(siteConfig.ErrorChannel.SpecName) == 0 {
			return nil, fmt.Errorf("error: site_config.error_channel spec name cannot be empty (operator '%s')", spec.Type)
		}
		errorCh, err := ctx.channelRegistry.GetOutputChannel(siteConfig.ErrorChannel.Name)
		if err != nil {
			return nil, fmt.Errorf("while resolving the error channel of site operator '%s': %v", spec.Type, err)
		}
		args.ErrorChannel = errorCh
	}
	if err := ctx.resolveSiteOutputChannels(args, siteConfig, spec.Type); err != nil {
		return nil, err
	}
	return args, nil
}

// resolveSiteOutputChannels turns `site_config.output_channels` into
// OperatorArgs.Outputs.
//
// It is the error channel's resolution pluralised -- the same two emptiness
// checks per entry, then the same GetOutputChannel -- and the same argument
// carries it: the builder owns the registry, resolves what the step named, and
// hands over the result, so the operator can name nothing else (§12.6).
//
// **Authored order is preserved and is the operator's only handle on which is
// which**, since a resolved channel carries its own name and the operator reads
// it off `Config.Name` or off the position it authored. That is why a repeated
// name is refused: it puts one channel at two indices with nothing to tell them
// apart, which is a typo in every case anyone has been able to construct. The
// step's own `output_channel` appearing in the list is deliberately *not*
// refused -- see OperatorArgs.Outputs for why -- and yields the same pointer the
// registry holds, so the operator writing through either sees one channel.
func (ctx *BuilderContext) resolveSiteOutputChannels(args *OperatorArgs,
	siteConfig *SiteOperatorSpec, operatorType string) error {
	if len(siteConfig.OutputChannels) == 0 {
		return nil
	}
	outputs := make([]*OutputChannel, 0, len(siteConfig.OutputChannels))
	declared := make(map[string]int, len(siteConfig.OutputChannels))
	for i := range siteConfig.OutputChannels {
		channelConfig := &siteConfig.OutputChannels[i]
		if len(channelConfig.Name) == 0 {
			return fmt.Errorf("error: site_config.output_channels[%d] name cannot be empty (operator '%s')",
				i, operatorType)
		}
		if len(channelConfig.SpecName) == 0 {
			return fmt.Errorf("error: site_config.output_channels[%d] ('%s') spec name cannot be empty (operator '%s')",
				i, channelConfig.Name, operatorType)
		}
		if first, ok := declared[channelConfig.Name]; ok {
			return fmt.Errorf(
				"error: site_config.output_channels names '%s' twice, at [%d] and [%d]; the operator would be "+
					"handed one channel at two indices with nothing to tell them apart (operator '%s')",
				channelConfig.Name, first, i, operatorType)
		}
		declared[channelConfig.Name] = i
		outputCh, err := ctx.channelRegistry.GetOutputChannel(channelConfig.Name)
		if err != nil {
			return fmt.Errorf("while resolving output channel '%s' of site operator '%s': %v",
				channelConfig.Name, operatorType, err)
		}
		outputs = append(outputs, outputCh)
	}
	args.Outputs = outputs
	return nil
}
