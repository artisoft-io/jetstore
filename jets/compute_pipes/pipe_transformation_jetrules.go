package compute_pipes

import (
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/jackc/pgx/v4/pgxpool"
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
	className string
	outputCh  *OutputChannel
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
	close(ctx.jrPoolManager.WorkersTaskCh)
	// log.Println("**!@@ ** Send ANALYZE Result to", ctx.outputCh.config.Name, "DONE")
	return nil
}

func (ctx *JetrulesTransformationPipe) Finally() {}

func (ctx *BuilderContext) NewJetrulesTransformationPipe(source *InputChannel, _ *OutputChannel, spec *TransformationSpec) (*JetrulesTransformationPipe, error) {
	if spec == nil || spec.JetrulesConfig == nil {
		return nil, fmt.Errorf("error: Jetrules Pipe Transformation spec is missing regex, lookup, and/or keywords definition")
	}
	spec.NewRecord = true
	config := spec.JetrulesConfig

	// Get the jetrule process info -- the mainRule name or ruleSequence name

	// Get the rete meta store
	reteMetaStore, err := GetJetrulesFactory(ctx.dbpool, config.ProcessName)
	if err != nil {
		return nil, err
	}
	// Create the output channels for each of the exported rdf type
	jetrulesOutputChan := make([]*JetrulesOutputChan, 0, len(config.JetrulesOutput))
	for i := range config.JetrulesOutput {
		name := config.JetrulesOutput[i].OutputChannel.Name
		outCh, err := ctx.channelRegistry.GetOutputChannel(name)
		if err != nil {
			return nil, err
		}
		jetrulesOutputChan = append(jetrulesOutputChan, &JetrulesOutputChan{
			className: config.JetrulesOutput[i].ClassName,
			outputCh:  outCh,
		})
	}

	// Assert current source period to meta graph
	err = AssertSourcePeriodInfo(config, reteMetaStore.MetaGraph, reteMetaStore.ResourceMgr)
	if err != nil {
		return nil, fmt.Errorf("while assertSourcePeriodInfo: %v", err)
	}

	// Assert rule config to meta graph from the pipeline configuration
	err = AssertRuleConfiguration(reteMetaStore, config)
	if err != nil {
		return nil, fmt.Errorf("while assertRuleConfiguration: %v", err)
	}

	// Setup a worker pool
	var jrPoolManager *JrPoolManager
	jrPoolManager, err = ctx.NewJrPoolManager(config, source, reteMetaStore, jetrulesOutputChan)

	return &JetrulesTransformationPipe{
		cpConfig:       ctx.cpConfig,
		source:         source,
		reteMetaStore:  reteMetaStore,
		jrPoolManager:  jrPoolManager,
		outputChannels: jetrulesOutputChan,
		spec:           spec,
		env:            ctx.env,
		doneCh:         ctx.done,
	}, err
}

// Function to get the properties (aka predicates) of the jetrules class
// used for the output channel
func GetJetClassProperties(dbpool *pgxpool.Pool, className, processName string) ([]string, error) {
	metaStore, err := GetJetrulesFactory(dbpool, processName)
	if err != nil {
		return nil, err
	}
	model := metaStore.ReteModelLookup[metaStore.MainRuleFileNames[0]]
	if model == nil {
		return nil, fmt.Errorf("error: bug in getting class properties for class '%s' (GetJetClassProperties)", className)
	}
	for i := range model.Tables {
		if model.Tables[i].ClassName == className {
			result := make([]string, 0, len(model.Tables[i].Columns))
			for j := range model.Tables[i].Columns {
				result = append(result, model.Tables[i].Columns[j].PropertyName)
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf("error: Class '%s' not found in workspace for process name '%s'", className, processName)
}
