package compute_pipes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/jackc/pgx/v4"
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
				err = fmt.Errorf("while BuildTransformationColumnEvaluator (in NewJetrulesTransformationPipe) %v", err)
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

	// Get rule_config, there is 3 sources of rule configuration:
	// 	- cpipes_config_json: This rule configuration is pipeline specific.
	//    This rule configuration is used for all clients and is json encoded.
	//    This rule configuratin is embeded in the cpipes config.

	// 	- Table rule_configv2: This rule configuration is process & client specific and
	//    is used across all pipelines using this process. This configuration is either csv or json encoded.
	//
	// 	- Table pipeline_config: This rule configuration is pipeline & client specific.
	//    This configuration is either csv or json encoded.

	// Get process / client specific rule configuration from rule_configv2
	var ruleConfigJson string
	err = ctx.dbpool.QueryRow(context.Background(),
		`SELECT rule_config_json FROM jetsapi.rule_configv2 WHERE process_name = $1 AND client = $2`,
		ctx.processName, ctx.cpConfig.CommonRuntimeArgs.Client).Scan(&ruleConfigJson)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("while reading rule_config_json from jetsapi.rule_configv2: %v", err)
	}
	if len(ruleConfigJson) > 0 {
		// parse and append to ruleConfig
		config.RuleConfig, err = appendRuleConfig(config.RuleConfig, &ruleConfigJson)
		if err != nil {
			return nil, fmt.Errorf("while parsing and appending rule config from rule_configv2: %v", err)
		}
	}

	// Get pipeline / client specific rule config from pipeline_config
	ruleConfigJson = ""
	err = ctx.dbpool.QueryRow(context.Background(),
		`SELECT rule_config_json FROM jetsapi.pipeline_config WHERE key = $1`, 
		ctx.cpConfig.CommonRuntimeArgs.PipelineConfigKey).Scan(&ruleConfigJson)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("while reading rule_config_json from pipeline_config table: %v", err)
	}
	if len(ruleConfigJson) > 0 {
		// parse and append to ruleConfig
		config.RuleConfig, err = appendRuleConfig(config.RuleConfig, &ruleConfigJson)
		if err != nil {
			return nil, fmt.Errorf("while parsing and appending rule config from pipeline_config table: %v", err)
		}
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

func appendRuleConfig(ruleConfig []map[string]any, configJson *string) ([]map[string]any, error) {
  if len(*configJson) > 0 {
    config := make([]map[string]any, 0)
		if err := json.Unmarshal([]byte(*configJson), &config); err != nil {
			// Assume it's csv
      var err2 error
			config, err2 = parseRuleConfigCsv(config, configJson)
			if err2 != nil {
				return nil, fmt.Errorf("while reading jetsapi.pipeline_config table, invalid rule_config_json\nJSON ERR:%v\nCSV ERR: %v", err, err2)
			}
			log.Println("Got pipeline-specific rule config in csv format")
		} else {
			if len(config) > 0 {
				log.Println("Got pipeline-specific rule config in json format")
			}
		}
    ruleConfig = append(ruleConfig, config...)
  }
  return ruleConfig, nil
}

func parseRuleConfigCsv(config []map[string]any, ruleConfig *string) ([]map[string]any, error) {
	rows, err := jcsv.Parse(*ruleConfig)
	if len(rows) > 1 && len(rows[0]) > 3 && err == nil {
    entities := make(map[string]map[string]any)
		for i := range rows {
			// Skip the header
			if i > 0 {
        // Transform triples:
        // subject:   rows[i][0],
        // predicate: rows[i][1],
        // object:    rows[i][2],
        // rdfType:   rows[i][3],
        // Into json struct:
        // [
        //   {
        //   "usi_sm:clientName": {
        //     "value": "Local 138",
        //     "type": "text"
        //   }
        // ]
        entity := entities[rows[i][0]]
        if entity == nil {
          entity = make(map[string]any)
          entities[rows[i][0]] = entity
        }
        // predicate to value / type
        entity[rows[i][1]] = map[string]any{
          "value": rows[i][2],
          "type": rows[i][3],
        }
			}
		}
    // Get the list of entities into config
    for _, entity := range entities {
      config = append(config, entity)
    }
	}
	return config, err
}
