package compute_pipes

import (
	"fmt"
	"log"
)

func (ctx *BuilderContext) startFanOutPipe(spec *PipeSpec, source *InputChannel) {
	var cpErr, err error
	evaluators := make([]PipeTransformationEvaluator, len(spec.Apply))

	defer func() {
		// Closing the output channels
		fmt.Println("**! FanOutPipe: Closing Output Channels")
		oc := make(map[string]bool)
		for i := range spec.Apply {
			oc[spec.Apply[i].Output] = true
		}
		for i := range oc {
			// fmt.Println("**! FanOutPipe: Closing Output Channel",i)
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

	// fmt.Println("**! start fan_out loop on source:", source.config.Name)
	for inRow := range source.channel {
		for i := range spec.Apply {
			err = evaluators[i].apply(&inRow)
			if err != nil {
				cpErr = fmt.Errorf("while calling apply on PipeTransformationEvaluator (in fan_out): %v", err)
				goto gotError
			}
		}
	}
	// fmt.Println("Closing fan_out PipeTransformationEvaluator")
	for i := range evaluators {
		if evaluators[i] != nil {
			err = evaluators[i].done()
			if err != nil {
				log.Printf("while calling done on PipeTransformationEvaluator (in fan_out): %v", err)
			}
		}
	}

	// All good!
	return

gotError:
	log.Println(cpErr)	
	ctx.errCh <- cpErr
	close(ctx.done)
}
