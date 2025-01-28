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
	evaluators := make([]PipeTransformationEvaluator, len(spec.Apply))

	defer func() {
		// Catch the panic that might be generated downstream
		if r := recover(); r != nil {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("StartFanOutPipe: recovered error: %v\n", r))
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
		fmt.Println("**!@@ FanOutPipe: Closing Output Channels")
		oc := make(map[string]bool)
		for i := range spec.Apply {
			// Make sure the output channel config is used (eg jetrules don't, it overrides it)
			if len(spec.Apply[i].OutputChannel.Name) > 0 {
				oc[spec.Apply[i].OutputChannel.Name] = true
			}
			if spec.Apply[i].Type == "jetrules" {
				// Get the output channels of jetrules
				for j := range spec.Apply[i].JetrulesConfig.OutputChannels {
					oc[spec.Apply[i].JetrulesConfig.OutputChannels[j].Name] = true
				}
			}
			if spec.Apply[i].Type == "clustering" {
				// Get the output channels of clustering
				oc[spec.Apply[i].ClusteringConfig.CorrelationOutputChannel.Name] = true
			}
		}
		for name := range oc {
			fmt.Println("**!@@ FanOutPipe: Closing Output Channel", name)
			ctx.channelRegistry.CloseChannel(name)
		}
		close(writePartitionsResultCh)
	}()

	for j := range spec.Apply {
		eval, err := ctx.BuildPipeTransformationEvaluator(source, nil, writePartitionsResultCh, &spec.Apply[j])
		if err != nil {
			cpErr = fmt.Errorf("while calling BuildPipeTransformationEvaluator for %s: %v", spec.Apply[j].Type, err)
			goto gotError
		}
		evaluators[j] = eval
	}

	// fmt.Println("**!@@ start fan_out loop on source:", source.name)
	for inRow := range source.channel {
		for i := range spec.Apply {
			err = evaluators[i].Apply(&inRow)
			if err != nil {
				cpErr = fmt.Errorf("while calling Apply on PipeTransformationEvaluator (in fan_out): %v", err)
				goto gotError
			}
		}
	}
	// fmt.Println("Closing fan_out PipeTransformationEvaluator")
	for i := range evaluators {
		if evaluators[i] != nil {
			err = evaluators[i].Done()
			if err != nil {
				cpErr = fmt.Errorf("while calling done on PipeTransformationEvaluator (in fan_out): %v", err)
				log.Println(cpErr)
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
}
