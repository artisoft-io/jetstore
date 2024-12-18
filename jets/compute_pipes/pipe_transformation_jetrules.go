package compute_pipes

import (
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Jetrules operator. Execute rules for each record group (bundle) recieved from input chan

type JetrulesTransformationPipe struct {
	cpConfig       *ComputePipesConfig
	source         *InputChannel
	reteMetaStore  *rete.ReteMetaStoreFactory
	jrPoolManager  *JrPoolManager
	outputChannels []*JetrulesOutputChan
	spec           *TransformationSpec
	env            map[string]interface{}
	doneCh         chan struct{}
}

type JetrulesOutputChan struct {
	className        string
	columnEvaluators []TransformationColumnEvaluator
	outputCh         *OutputChannel
}

// Implementing interface PipeTransformationEvaluator
// Each call to Apply, the input correspond to a rdf session on which to Apply the jetrules
// see jetrules_pool_worker.go for worker implementation
func (ctx *JetrulesTransformationPipe) Apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in JetrulesTransformationPipe")
	}
	// Send out the input to the worker pool
	select {
	case ctx.jrPoolManager.WorkersTaskCh <- *input:
	case <-ctx.doneCh:
		log.Println("JetrulesTransformationPipe interrupted")
	}
	return nil
}

func (ctx *JetrulesTransformationPipe) Done() error {
	return nil
}

func (ctx *JetrulesTransformationPipe) Finally() {
	// log.Println("Entering JetrulesTransformationPipe.Finally")
	close(ctx.jrPoolManager.WorkersTaskCh)
	// Wait till the pool workers are done
	// This is to avoid to close the output channel too early since the pool workers
	// are writing to the output channel async
	ctx.jrPoolManager.WaitForDone.Wait()
}

func (ctx *BuilderContext) NewJetrulesTransformationPipe(source *InputChannel, _ *OutputChannel, spec *TransformationSpec) (*JetrulesTransformationPipe, error) {
	if spec == nil || spec.JetrulesConfig == nil {
		return nil, fmt.Errorf("error: Jetrules Pipe Transformation spec is missing regex, lookup, and/or keywords definition")
	}
	spec.NewRecord = true
	config := spec.JetrulesConfig

	// Get the rete meta store
	reteMetaStore, err := GetJetrulesFactory(ctx.dbpool, config.ProcessName)
	if err != nil {
		return nil, err
	}

	// Create the output channels for each of the exported rdf type
	jetrulesOutputChan := make([]*JetrulesOutputChan, 0, len(config.OutputChannels))
	for i := range config.OutputChannels {
		outCh, err := ctx.channelRegistry.GetOutputChannel(config.OutputChannels[i].Name)
		if err != nil {
			return nil, err
		}
		if len(outCh.config.ClassName) == 0 {
			return nil, fmt.Errorf("error: missing class name on jetrules output channel named %s",
				config.OutputChannels[i].Name)
		}
		// Make a set of TransformationColumnEvaluator for each of the output channel
		columnEvaluators := make([]TransformationColumnEvaluator, len(spec.Columns))
		for i := range spec.Columns {
			// log.Printf("**& *JETRULES* build TransformationColumn[%d] of type %s for output %s", i, spec.Type, config.OutputChannels[i].Name)
			columnEvaluators[i], err = ctx.BuildTransformationColumnEvaluator(source, outCh, &spec.Columns[i])
			if err != nil {
				err = fmt.Errorf("while BuildTransformationColumnEvaluator (in NewAnalyzeTransformationPipe) %v", err)
				log.Println(err)
				return nil, err
			}
		}
		jetrulesOutputChan = append(jetrulesOutputChan, &JetrulesOutputChan{
			className:        outCh.config.ClassName,
			columnEvaluators: columnEvaluators,
			outputCh:         outCh,
		})
	}

	// Assert current source period to meta graph
	err = AssertSourcePeriodInfo(config, reteMetaStore.MetaGraph, reteMetaStore.ResourceMgr)
	if err != nil {
		return nil, fmt.Errorf("while AssertSourcePeriodInfo: %v", err)
	}

	// Assert rule config to meta graph from the pipeline configuration
	err = AssertRuleConfiguration(reteMetaStore, config)
	if err != nil {
		return nil, fmt.Errorf("while AssertRuleConfiguration: %v", err)
	}

	// Assert metadata source
	err = AssertMetadataSource(reteMetaStore, config, ctx.env)
	if err != nil {
		return nil, fmt.Errorf("while AssertMetadataSource: %v", err)
	}

	// Print rdf session if in debug mode
	if config.IsDebug {
		log.Println("METADATA GRAPH")
		log.Println(strings.Join(reteMetaStore.MetaGraph.ToTriples(), "\n"))
	}

	// Setup a worker pool
	var jrPoolManager *JrPoolManager
	workerResultCh := make(chan JetrulesWorkerResult, 10)
	ctx.chResults.JetrulesWorkerResultCh <- workerResultCh
	jrPoolManager, err = ctx.NewJrPoolManager(config, source, reteMetaStore, jetrulesOutputChan, workerResultCh)
	if err != nil {
		return nil, err
	}
	return &JetrulesTransformationPipe{
		cpConfig:       ctx.cpConfig,
		source:         source,
		reteMetaStore:  reteMetaStore,
		jrPoolManager:  jrPoolManager,
		outputChannels: jetrulesOutputChan,
		spec:           spec,
		env:            ctx.env,
		doneCh:         ctx.done,
	}, nil
}
