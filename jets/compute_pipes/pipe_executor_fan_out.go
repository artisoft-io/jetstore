package compute_pipes

import (
	"fmt"
	"log"
)

func (ctx *BuilderContext) startFanOutPipe(spec *PipeSpec, source *InputChannel) {
	var cpErr, err error
	evaluators := make([]PipeTransformationEvaluator, len(spec.Apply))

	defer func() {
		fmt.Println("Closing fan_out PipeTransformationEvaluator")
		for i := range evaluators {
			if evaluators[i] != nil {
				err = evaluators[i].done()
				if err != nil {
					log.Printf("while calling done on PipeTransformationEvaluator (in fan_out): %v", err)
				}
			}
		}
		// Closing the output channels
		oc := make(map[string]bool)
		for i := range spec.Apply {
			oc[spec.Apply[i].Output] = true
		}
		for i := range oc {
			ctx.channelRegistry.CloseChannel(i)
		}
	}()

	for j := range spec.Apply {
		eval, err := ctx.buildPipeTransformationEvaluator(source, &spec.Apply[j])
		if err != nil {
			cpErr = fmt.Errorf("while calling buildPipeTransformationEvaluator for %s: %v", spec.Apply[j].Type, err)
			goto gotError
		}
		evaluators[j] = eval
	}

	fmt.Println("**! start fan_out loop on source:", source.config.Name)
	for inRow := range source.channel {
		fmt.Println("**! fan_out, row from source:", source.config.Name)
		for i := range spec.Apply {
			err = evaluators[i].apply(&inRow)
			if err != nil {
				cpErr = fmt.Errorf("while calling apply on PipeTransformationEvaluator (in fan_out): %v", err)
				goto gotError
			}
		}
	}

	// All good!
	return

gotError:
	log.Println(cpErr)
	ctx.computePipesResultCh <- ComputePipesResult{Err: cpErr}
	close(ctx.done)
}
