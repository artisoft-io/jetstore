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
			switch spec.Apply[i].Type {
			case "jetrules":
				// Get the output channels of jetrules
				if spec.Apply[i].JetrulesConfig != nil {
					for j := range spec.Apply[i].JetrulesConfig.OutputChannels {
						oc[spec.Apply[i].JetrulesConfig.OutputChannels[j].Name] = true
					}
					// Get the error output channel of jetrules
					if spec.Apply[i].JetrulesConfig.ErrorChannel != nil {
						oc[spec.Apply[i].JetrulesConfig.ErrorChannel.Name] = true
					}
				}

			case "ollama":
				// Get the error output channel of ollama
				if spec.Apply[i].OllamaConfig != nil && spec.Apply[i].OllamaConfig.ErrorChannel != nil {
					oc[spec.Apply[i].OllamaConfig.ErrorChannel.Name] = true
				}

			case "clustering":
				// Get the output channels of clustering
				if spec.Apply[i].ClusteringConfig != nil &&
					spec.Apply[i].ClusteringConfig.CorrelationOutputChannel != nil {
					oc[spec.Apply[i].ClusteringConfig.CorrelationOutputChannel.Name] = true
				}

			case "map_record":
				// Get the error output channel of map_record
				if spec.Apply[i].MapRecordConfig != nil && spec.Apply[i].MapRecordConfig.ErrorChannel != nil {
					oc[spec.Apply[i].MapRecordConfig.ErrorChannel.Name] = true
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
