package compute_pipes

import (
	"fmt"
	"log"
)

// Clustering operator. Execute rules for each record group (bundle) recieved from input chan

type ClusteringTransformationPipe struct {
	cpConfig            *ComputePipesConfig
	source              *InputChannel
	inputRowCount       int
	poolManager         *ClusteringPoolManager
	correlationOutputCh *OutputChannel
	spec                *TransformationSpec
	channelRegistry     *ChannelRegistry
	env                 map[string]interface{}
	doneCh              chan struct{}
}

// Implementing interface PipeTransformationEvaluator
// Each call to Apply, the input correspond to a row for which we calculate the
// column correlation.
func (ctx *ClusteringTransformationPipe) Apply(input *[]interface{}) error {
	if input == nil {
		return fmt.Errorf("error: unexpected null input arg in ClusteringTransformationPipe")
	}
	c := ctx.spec.ClusteringConfig.MaxInputCount
	if c > 0 && ctx.inputRowCount >= c {
		// Got the max nbr of records, skipping the rest
		return nil
	}
	// Send out the input to the worker pool
	select {
	case ctx.poolManager.WorkersTaskCh <- *input:
		ctx.inputRowCount += 1
	case <-ctx.doneCh:
		log.Println("ClusteringTransformationPipe interrupted")
	}
	return nil
}

func (ctx *ClusteringTransformationPipe) Done() error {
	return nil
}

func (ctx *ClusteringTransformationPipe) Finally() {
	// log.Println("Entering ClusteringTransformationPipe.Finally")
	close(ctx.poolManager.WorkersTaskCh)
	// Wait till the pool workers are done
	// This is to avoid to close the output channel too early since the pool workers
	// are writing to the output channel async
	ctx.poolManager.WaitForDone.Wait()
	if ctx.correlationOutputCh != nil {
		log.Printf("ClusteringTransformationPipe: Closing Correlation Output Channel %s\n",
			ctx.correlationOutputCh.config.Name)
		ctx.channelRegistry.CloseChannel(ctx.correlationOutputCh.config.Name)
	}
}

func (ctx *BuilderContext) NewClusteringTransformationPipe(source *InputChannel, outputCh *OutputChannel,
	spec *TransformationSpec) (*ClusteringTransformationPipe, error) {

	// Some validation
	if spec == nil || spec.ClusteringConfig == nil {
		return nil, fmt.Errorf("error: Clustering Pipe Transformation spec is missing clustering_config element")
	}
	spec.NewRecord = true
	config := spec.ClusteringConfig
	if len(config.TargetColumnsLookup.LookupName) == 0 ||
		len(config.TargetColumnsLookup.DataClassificationColumn) == 0 ||
		len(config.TargetColumnsLookup.Column1ClassificationValues) == 0 ||
		len(config.TargetColumnsLookup.Column2ClassificationValues) == 0 {
		return nil, fmt.Errorf("error: clustering_config is missing lookup_name and/or data_classification_column or values")
	}
	if config.CorrelationThreshold == 0 {
		return nil, fmt.Errorf("error: clustering_config is missing value for correlation_threshold")
	}
	if config.CardinalityThreshold == 0 {
		return nil, fmt.Errorf("error: clustering_config is missing value for cardinality_threshold")
	}

	// Get the output channel for the column correlation.
	var correlationOutputCh *OutputChannel
	var err error
	if config.CorrelationOutputChannel == nil {
		return nil, fmt.Errorf("error: the clustering operator is missing the correlation_output_channel configuration")
	}
	correlationOutputCh, err = ctx.channelRegistry.GetOutputChannel(config.CorrelationOutputChannel.Name)
	if err != nil {
		return nil, err
	}
	// Make sure to have the expected columns in the correlationOutputCh channel
	_, ok := correlationOutputCh.columns["column_name_1"]
	if !ok {
		return nil, fmt.Errorf("error: the clustering operator's correlation_output_channel is missing column 'column_name_1'")
	}
	_, ok = correlationOutputCh.columns["column_name_2"]
	if !ok {
		return nil, fmt.Errorf("error: the clustering operator's correlation_output_channel is missing column 'column_name_2'")
	}
	_, ok = correlationOutputCh.columns["observations_count"]
	if !ok {
		return nil, fmt.Errorf("error: the clustering operator's correlation_output_channel is missing column 'observations_count'")
	}
	_, ok = correlationOutputCh.columns["cramerv"]
	if !ok {
		return nil, fmt.Errorf("error: the clustering operator's correlation_output_channel is missing column 'cramerv'")
	}
	_, ok = correlationOutputCh.columns["cardinality_avr"]
	if !ok {
		return nil, fmt.Errorf("error: the clustering operator's correlation_output_channel is missing column 'cardinality_avr'")
	}
	if config.MinNonNilCount < 3 {
		log.Printf("WARNING: clustering_config with min_non_null_count < 3, defaulting to 3")
		config.MinNonNilCount = 3
	}

	// Setup a worker pool
	var poolManager *ClusteringPoolManager
	clusteringResultCh := make(chan ClusteringResult, 10)
	ctx.chResults.ClusteringResultCh <- clusteringResultCh
	poolManager, err = ctx.NewClusteringPoolManager(config, source, outputCh, correlationOutputCh, clusteringResultCh)
	if err != nil {
		return nil, err
	}
	return &ClusteringTransformationPipe{
		cpConfig:            ctx.cpConfig,
		source:              source,
		poolManager:         poolManager,
		correlationOutputCh: correlationOutputCh,
		spec:                spec,
		channelRegistry:     ctx.channelRegistry,
		env:                 ctx.env,
		doneCh:              ctx.done,
	}, nil
}
