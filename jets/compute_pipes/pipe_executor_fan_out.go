package compute_pipes

import (
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
)

func (ctx *BuilderContext) StartFanOutPipe(spec *PipeSpec, source *InputChannel, writePartitionsResultCh chan ComputePipesResult) {
	var cpErr, err error
	var lastEvaluatorType string
	isDebugMode := ctx.cpConfig.ClusterConfig.IsDebugMode
	inputSourceName := source.Name
	evaluators := make([]PipeTransformationEvaluator, len(spec.Apply))

	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("StartFanOutPipe[source: %s]: recovered error: %v\n", inputSourceName, r))
			buf.WriteString(string(debug.Stack()))
			cpErr := errors.New(buf.String())
			log.Println(cpErr)
			ctx.errCh <- cpErr
			// Avoid closing a closed channel
			select {
			case <-ctx.done:
			default:
				close(ctx.done)
			}
		}
		// Closing the output channels
		log.Printf("StartFanOutPipe[source: %s]: Closing Output Channels", inputSourceName)
		oc := make(map[string]bool)
		for i := range spec.Apply {
			// Make sure the output channel config is used (eg jetrules don't, it overrides it)
			if len(spec.Apply[i].OutputChannel.Name) > 0 {
				oc[spec.Apply[i].OutputChannel.Name] = true
			}
			// A site operator's `site_config.output_channels`, read through the one
			// function that knows them. Outside the switch for outputChannelConfigs'
			// reason: a site token is by construction none of the cases below, so a
			// case for it would be a case no document can reach.
			//
			// Without this arm a channel declared there is registered by
			// StartComputePipes and resolved onto OperatorArgs.Outputs and then
			// closed by nothing, so every reader of it blocks on a channel that
			// never sees EOF (P9-I13). siteOutputChannelConfigs answers nil when
			// there is no site_config, no list, or the token is a built-in, so a
			// document declaring none takes exactly the path it took before this
			// arm existed.
			for _, siteChannel := range siteOutputChannelConfigs(&spec.Apply[i]) {
				if len(siteChannel.Name) > 0 {
					oc[siteChannel.Name] = true
				}
			}
			// Every operator's error channel, read through the one function that
			// knows them all -- the same one the synthesis and the channel registry
			// read. This was a per-type list here and it had drifted: it named no
			// `render` and no site operator, so a render step's error channel, which
			// SynthesizeDefaultErrorChannels gives it whenever it names none, was
			// closed by nothing, and the process_errors writer it feeds waited for
			// an EOF that never came. The run hung after every other channel had
			// closed (Patient_Profile_Template, 2026-10-11). Outside the switch so
			// the next operator that reports row-level failures is closed without a
			// second list to remember.
			if ec := errorChannelConfig(&spec.Apply[i]); ec != nil && len(ec.Name) > 0 {
				oc[ec.Name] = true
			}
			switch spec.Apply[i].Type {
			case "jetrules":
				// Get the output channels of jetrules
				if spec.Apply[i].JetrulesConfig != nil {
					for j := range spec.Apply[i].JetrulesConfig.OutputChannels {
						oc[spec.Apply[i].JetrulesConfig.OutputChannels[j].Name] = true
					}
				}

			case "clustering":
				// Get the output channels of clustering
				if spec.Apply[i].ClusteringConfig != nil &&
					spec.Apply[i].ClusteringConfig.CorrelationOutputChannel != nil {
					oc[spec.Apply[i].ClusteringConfig.CorrelationOutputChannel.Name] = true
				}
			}
		}
		for name := range oc {
			log.Printf("StartFanOutPipe[source: %s]: Closing Output Channel %s", inputSourceName, name)
			ctx.channelRegistry.CloseChannel(name)
		}
		close(writePartitionsResultCh)
	}()

	for j := range spec.Apply {
		lastEvaluatorType = spec.Apply[j].Type
		if isDebugMode {
			log.Printf("*** StartFanOutPipe[source: %s]: BUILDING PipeTransformationEvaluator for %s - STARTING",
				inputSourceName, lastEvaluatorType)
		}
		eval, err := ctx.BuildPipeTransformationEvaluator(source, nil, writePartitionsResultCh, &spec.Apply[j])
		if err != nil {
			log.Printf("*** StartFanOutPipe[source: %s]: BUILDING PipeTransformationEvaluator for %s - FAILED: %v",
				inputSourceName, lastEvaluatorType, err)
			cpErr = fmt.Errorf("while calling BuildPipeTransformationEvaluator for %s: %v", lastEvaluatorType, err)
			goto gotError
		}
		evaluators[j] = eval
		if isDebugMode {
			log.Printf("*** StartFanOutPipe[source: %s]: BUILDING PipeTransformationEvaluator for %s - DONE",
				inputSourceName, lastEvaluatorType)
		}
	}

	if isDebugMode {
		log.Printf("*** StartFanOutPipe[source: %s]: start fan_out loop on source %s - STARTING",
			inputSourceName, inputSourceName)
	}
	for inRow := range source.Channel {
		for i := range spec.Apply {
			if evaluators[i] != nil {
				err = evaluators[i].Apply(&inRow)
				if err != nil {
					cpErr = fmt.Errorf("while calling Apply on PipeTransformationEvaluator (in fan_out): %v", err)
					goto gotError
				}
			}
		}
	}
	
	if isDebugMode {
		log.Printf("*** StartFanOutPipe[source: %s]: done fan_out loop on source %s - CLOSING EVALUATORS",
			inputSourceName, inputSourceName)
	}
	for i := range evaluators {
		if evaluators[i] != nil {
			err = evaluators[i].Done()
			if err != nil {
				if strings.Contains(err.Error(), "cannot perform file analysis") {
					log.Printf("while calling done on PipeTransformationEvaluator (in fan_out): %v\n", err)
					cpErr = err
				} else {
					cpErr = fmt.Errorf("while calling done on PipeTransformationEvaluator (in fan_out): %v", err)
					log.Println(cpErr)
				}
				goto gotError
			}
		}
	}
	for i := range evaluators {
		if evaluators[i] != nil {
			evaluators[i].Finally()
		}
	}

	// All good!
	log.Printf("*** StartFanOutPipe[source: %s]: exiting SUCCESS", inputSourceName)
	return

gotError:
	for i := range evaluators {
		if evaluators[i] != nil {
			evaluators[i].Finally()
		}
	}
	log.Println(cpErr)
	ctx.errCh <- cpErr
	// Avoid closing a closed channel
	select {
	case <-ctx.done:
	default:
		close(ctx.done)
	}
	log.Printf("*** StartFanOutPipe[source: %s]: exiting WITH ERROR for %s", inputSourceName, lastEvaluatorType)
}
