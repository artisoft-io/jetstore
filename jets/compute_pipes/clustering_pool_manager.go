package compute_pipes

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// ClusteringPoolManager manages a set of ClusteringDistributors, each having a pool of ClusteringPoolWorkers
// for the clustering algorithm

// ClusteringDistributors distribute row by unique value of column1 to an associated ClusteringPoolWorkers.
// ClusteringPoolWorkers calculate the columns correlation in parallel.
// poolWg is a wait group of the workers.
// The WorkersTaskCh is a channel between the clustering operator and the pool manager (to limit the nbr of rows)
// The distributionTaskCh is used by the pool manager to distribute the input rows to all the workers.
// The distributionResultCh is used to collect the correlation resultts from the workers by the pool manager.
// The correlationOutputCh is to send the correlation results to s3, this is done when is_debug is true
type ClusteringPoolManager struct {
	config                  *ClusteringSpec
	WorkersTaskCh           chan []any
	distributors            []*ClusteringDistributor
	distributionResultCh    chan []any
	columnsCorrelation      []*ColumnCorrelation
	analysisLookup          LookupTable
	columnClassificationMap map[string]string
	correlationOutputCh     *OutputChannel
	poolWg                  *sync.WaitGroup
	WaitForDone             *sync.WaitGroup
}

type ColumnCorrelation struct {
	column1          string
	column2          string
	distinct1Count   int
	distinct2Count   int
	observationCount int
}

func NewColumnCorrelation(c1, c2 string, dc1, dc2, oc int) *ColumnCorrelation {
	return &ColumnCorrelation{
		column1:          c1,
		column2:          c2,
		distinct1Count:   dc1,
		distinct2Count:   dc2,
		observationCount: oc,
	}
}

type ClusteringDistributor struct {
	column1             *string
	column1Pos          int
	distributionTaskMap map[string]chan []any
}

// Create the ClusteringPoolManager, it will be set to the receiving BuilderContext
func (ctx *BuilderContext) NewClusteringPoolManager(config *ClusteringSpec,
	source *InputChannel, outputCh *OutputChannel, correlationOutputCh *OutputChannel,
	clusteringResultCh chan ClusteringResult) (poolMgr *ClusteringPoolManager, err error) {

	defer func() {
		if err != nil {
			close(clusteringResultCh)
		}
	}()
	log.Println("Starting the Clustering Pool Manager")
	targetConfig := &config.TargetColumnsLookup
	analysisLookup := ctx.lookupTableManager.LookupTableMap[targetConfig.LookupName]
	if analysisLookup == nil {
		err = fmt.Errorf("error: clustering operator lookup table %s is not found", targetConfig.LookupName)
		return
	}

	// Create the pool manager
	poolMgr = &ClusteringPoolManager{
		config:                  config,
		WorkersTaskCh:           make(chan []any, 1),
		distributors:            make([]*ClusteringDistributor, 0),
		distributionResultCh:    make(chan []any, 100),
		correlationOutputCh:     correlationOutputCh,
		analysisLookup:          analysisLookup,
		columnClassificationMap: make(map[string]string),
		poolWg:                  new(sync.WaitGroup),
		WaitForDone:             new(sync.WaitGroup),
	}

	// Identify the columns that match column1 and column2 criteria
	columns1 := make([]string, 0)
	columns2 := make([]string, 0)
	columns1Pos := make(map[string]int)
	columns2Pos := make(map[string]int)
	tag1map := make(map[any]bool)
	tag2map := make(map[any]bool)
	for _, tag := range targetConfig.Column1ClassificationValues {
		tag1map[tag] = true
	}
	for _, tag := range targetConfig.Column2ClassificationValues {
		tag2map[tag] = true
	}
	for _, column := range source.config.Columns {
		lkrow, err2 := analysisLookup.Lookup(&column)
		if err2 != nil {
			err = fmt.Errorf("NewClusteringPoolManager: while looking up key %s from table %s: %v",
				column, targetConfig.LookupName, err2)
			return
		}
		columnTag, err2 := analysisLookup.LookupValue(lkrow, targetConfig.DataClassificationColumn)
		if err2 != nil {
			err = fmt.Errorf("NewClusteringPoolManager: while getting '%s' lookup row value: %v",
				targetConfig.DataClassificationColumn, err2)
			return
		}
		dataClassification, ok := columnTag.(string)
		if ok {
			poolMgr.columnClassificationMap[column] = dataClassification
		}
		if tag1map[columnTag] {
			columns1Pos[column] = len(columns1)
			columns1 = append(columns1, column)
			poolMgr.distributors = append(poolMgr.distributors, &ClusteringDistributor{
				column1:             &column,
				column1Pos:          (*source.columns)[column],
				distributionTaskMap: make(map[string]chan []any),
			})
		}
		if tag2map[columnTag] {
			columns2Pos[column] = len(columns2)
			columns2 = append(columns2, column)
		}
	}
	if config.IsDebug {
		log.Printf("Got for column1: %s\n", strings.Join(columns1, ","))
		log.Printf("Got for column2: %s\n", strings.Join(columns2, ","))
	}
	poolMgr.WaitForDone.Add(1)

	// Create a channel for the workers to report results
	workersResultCh := make(chan ClusteringResult)

	// Collect the results from all the workers
	go func() {
		var err error
		for workerResult := range workersResultCh {
			if workerResult.Err != nil {
				err = workerResult.Err
				break
			}
		}
		if config.IsDebug {
			log.Println("POOL MANAGER - Done collecting results from workers, err?", err)
		}
		// Send out the collected result
		select {
		case clusteringResultCh <- ClusteringResult{Err: err}:
			if err != nil {
				// Interrupt the whole process, there's been an error while executing rules
				// Avoid closing a closed channel
				select {
				case <-ctx.done:
				default:
					close(ctx.done)
				}
			}
		case <-ctx.done:
			log.Printf("Collecting results from ClusteringPoolWorkers interrupted")
		}
		close(clusteringResultCh)
	}()

	// Set up all the workers, use a wait group to track when they are all done
	// to close workersResultCh
	// Clustering algo:
	//   - For each column with column1_classification_values compute the correlation
	//     with the columns having column2_classification_values.
	//   - The correlation is calculated for aggregated value of column1 as
	//        (nbr_distinct_value / total_non_nil_values)
	//     Lower is the ratio, more correlated is column1 with column2.
	//   - The clustering status is calculated as:
	//		   - When all clusters are of size 1 (single member): invalid
	//       - When the average of all correlation values > max_avr_correlation_threshold_pct: invalid
	//       - Otherwise: valid
	//   - When a cluster contains a node with a data_classification contained in
	//     cluster_data_subclassification then each node of the cluster get that
	//     node classification as
	workerOutputCh := &OutputChannel{
		columns: &map[string]int{},
		config: &ChannelSpec{
			Name: "workers_out",
			Columns: []string{
				"column_name_1",
				"column_name_2",
				"distinct_count",
				"total_non_nil_count",
			},
		},
	}
	for i, c := range workerOutputCh.config.Columns {
		(*workerOutputCh.columns)[c] = i
	}
	go func() {
		if config.IsDebug {
			log.Println("Starting the clustering Worker Pool")
		}
		// Distribute the input rows to the distributors
		go func() {
			defer func() {
				// Close the intermediate task distribution channels
				for _, distributor := range poolMgr.distributors {
					for _, workerCh := range distributor.distributionTaskMap {
						close(workerCh)
					}
				}
				// log.Println("POOL MANAGER - Waiting on workers to finish (poolWg)")
				poolMgr.poolWg.Wait()
				// log.Println("POOL MANAGER - Waiting on workers to finish (poolWg) DONE")
				close(poolMgr.distributionResultCh)
			}()
			// Distribute the input rows to the distributors
			for input := range poolMgr.WorkersTaskCh {
				for _, distributor := range poolMgr.distributors {
					if len(input) > distributor.column1Pos {
						value := input[distributor.column1Pos]
						str, ok := value.(string)
						if ok && len(str) > 0 {
							workerCh := distributor.distributionTaskMap[str]
							if workerCh == nil {
								// Got an unseen value, create a worker
								poolMgr.poolWg.Add(1)
								workerCh = make(chan []any, 1)
								distributor.distributionTaskMap[str] = workerCh
								go func() {
									defer poolMgr.poolWg.Done()
									worker := NewClusteringWorker(config, source, distributor.column1,
										columns2, workerOutputCh, ctx.done, ctx.errCh)
									worker.DoWork(workerCh, poolMgr.distributionResultCh, workersResultCh)
								}()
							}
							// Send the input row to worker's task channel
							select {
							case workerCh <- input:
							case <-ctx.done:
								log.Println("Clustering Pool Manager interrupted while distributing tasks to workers")
							}
						}
					}
				}
			}
			if config.IsDebug {
				log.Println("POOL MANAGER - Done distributing input to workers")
			}
		}()

		// Collect the results from the workers
		// Worker's output columns positions
		wName1 := (*workerOutputCh.columns)["column_name_1"]
		wName2 := (*workerOutputCh.columns)["column_name_2"]
		WCount := (*workerOutputCh.columns)["distinct_count"]
		WTotal := (*workerOutputCh.columns)["total_non_nil_count"]
		// Manager's output columns positions
		col1Pos := (*correlationOutputCh.columns)["column_name_1"]
		col2Pos := (*correlationOutputCh.columns)["column_name_2"]
		distinct1Pos := (*correlationOutputCh.columns)["distinct_column_1_count"]
		distinct2Pos := (*correlationOutputCh.columns)["distinct_column_2_count"]
		totalPos := (*correlationOutputCh.columns)["observations_count"]
		// Use this variable as an accumulator to reduce all column1_value
		columnCorrelationAccumulator := make(map[string]*ClusterCorrelation)
		for correlationresult := range poolMgr.distributionResultCh {
			// save the result so it can be used to determine the clusters
			key := fmt.Sprintf("%v__%v", correlationresult[wName1], correlationresult[wName2])
			// //***
			// fmt.Printf("Got Worker Result: %v, %v distinct_count: %v, total: %v\n",
			// correlationresult[wName1],
			// correlationresult[wName2],
			// correlationresult[WCount],
			// correlationresult[WTotal])
			cc := columnCorrelationAccumulator[key]
			if cc == nil {
				cc = NewClusterCorrelation(correlationresult[wName1].(string),
					correlationresult[wName2].(string), config.MinColumn2NonNilCount)
				columnCorrelationAccumulator[key] = cc
			}
			cc.AddObservation(correlationresult[WCount].(int), correlationresult[WTotal].(int))
		}

		// Determine the column correlation
		poolMgr.columnsCorrelation = make([]*ColumnCorrelation, 0, len(columns1)*len(columns2))
		for _, cc := range columnCorrelationAccumulator {
			distinctC2Count, totalCount := cc.CumulatedCounts()
			// Get the column positions in slice columns1
			column1 := columns1Pos[cc.column1]
			// Send the correlation result to the output channel so it makes it's way to s3
			distinctC1Count := len(poolMgr.distributors[column1].distributionTaskMap)
			correlationresult := make([]any, len(poolMgr.correlationOutputCh.config.Columns))
			correlationresult[col1Pos] = cc.column1
			correlationresult[col2Pos] = cc.column2
			correlationresult[distinct1Pos] = distinctC1Count
			correlationresult[distinct2Pos] = distinctC2Count
			correlationresult[totalPos] = totalCount
			select {
			case poolMgr.correlationOutputCh.channel <- correlationresult:
			case <-ctx.done:
				log.Println("Clustering Pool Manager interrupted")
			}
			if config.IsDebug {
				log.Printf("COLUMN CORRELATION: %s -> %s: (%v, %v, %v)\n",
					correlationresult[col1Pos], correlationresult[col2Pos], correlationresult[distinct1Pos],
					correlationresult[distinct2Pos], correlationresult[totalPos])
			}
			// Save the result to determine the clusters
			if totalCount >= config.MinColumn1NonNilCount {
				poolMgr.columnsCorrelation = append(poolMgr.columnsCorrelation, &ColumnCorrelation{
					column1:          cc.column1,
					column2:          cc.column2,
					distinct1Count:   distinctC1Count,
					distinct2Count:   distinctC2Count,
					observationCount: totalCount,
				})
			}
		}
		if config.IsDebug {
			log.Println("POOL MANAGER - Determine the clusters:")
		}
		clusters := MakeClusters(poolMgr.columnsCorrelation, poolMgr.columnClassificationMap, config)

		// Validate the cluster structure, make sure the clustering did not breakdown
		clusterStatus := "valid"
		maxMembership := 0
		for _, cluster := range clusters {
			c := len(cluster.membership)
			if c > maxMembership {
				maxMembership = c
			}
		}
		if maxMembership == 1 {
			log.Println("Clustering algo failure: all cluster are of size 1")
			clusterStatus = "invalid"
		}
		// Send out the cluster information
		var subClassification string
		for i, cluster := range clusters {
			label := fmt.Sprintf("cluster%d", i)
			// Determine cluster member's subclassification
			subClassification = ""
			if clusterStatus == "valid" {
				if len(cluster.membership) == 1 {
					for _, tag := range config.SoloDataSubclassification {
						for column := range cluster.membership {
							if poolMgr.columnClassificationMap[column] == tag {
								subClassification = tag
								goto subclassificationDone
							}
						}
					}
				} else {
					//*TODO Could there be more than one tag?
					for tag := range cluster.clusterTags {
						subClassification = tag
						goto subclassificationDone
					}
				}
			}
		subclassificationDone:
			for column := range cluster.membership {
				row := make([]any, len(outputCh.config.Columns))
				row[(*outputCh.columns)["cluster_id"]] = label
				row[(*outputCh.columns)["column_name"]] = column
				row[(*outputCh.columns)["status"]] = clusterStatus
				if len(subClassification) == 0 && len(cluster.membership) == 1 {
					row[(*outputCh.columns)["data_subclassification"]] = "__SOLO__"
				} else {
					row[(*outputCh.columns)["data_subclassification"]] = subClassification
				}
				// Send the cluster membership to output channel
				if config.IsDebug {
					log.Printf("Cluster '%s' (%s), member: '%s', subsclassification: '%s'\n", label, clusterStatus,
						column, subClassification)
				}
				select {
				case outputCh.channel <- row:
				case <-ctx.done:
					log.Println("Clustering Pool Manager sending cluster membership interrupted")
				}
			}
		}

		poolMgr.WaitForDone.Done()
		close(workersResultCh)
		log.Println("Clustering Worker Pool Completed")
	}()
	return
}
