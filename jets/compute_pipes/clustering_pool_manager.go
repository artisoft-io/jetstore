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
	config               *ClusteringSpec
	WorkersTaskCh        chan []any
	distributors         []*ClusteringDistributor
	distributionResultCh chan []any
	columnsCorrelation   [][]float64
	analysisLookup       LookupTable
	correlationOutputCh  *OutputChannel
	poolWg               *sync.WaitGroup
	WaitForDone          *sync.WaitGroup
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
		config:               config,
		WorkersTaskCh:        make(chan []any, 1),
		distributors:         make([]*ClusteringDistributor, 0),
		distributionResultCh: make(chan []any, 100),
		correlationOutputCh:  correlationOutputCh,
		analysisLookup:       analysisLookup,
		poolWg:               new(sync.WaitGroup),
		WaitForDone:          new(sync.WaitGroup),
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
		if tag1map[columnTag] {
			columns1Pos[column] = len(columns1)
			columns1 = append(columns1, column)
			poolMgr.distributors = append(poolMgr.distributors, &ClusteringDistributor{
				column1:             &column,
				column1Pos:          source.columns[column],
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
	poolMgr.columnsCorrelation = make([][]float64, len(columns1))
	for i := range columns1 {
		poolMgr.columnsCorrelation[i] = make([]float64, len(columns2))
	}

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
	//        100 * (nbr_distinct_value / total_non_nil_values)
	//     Lower is the ratio, more correlated is column1 with column2.
	//   - The clustering status is calculated as:
	//		   - When all clusters are of size 1 (single member): invalid
	//       - When the average of all correlation values > max_avr_correlation_threshold_pct: invalid
	//       - Otherwise: valid
	//   - When a cluster contains a node with a data_classification contained in
	//     cluster_data_subclassification then each node of the cluster get that
	//     node classification as
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
			for input := range poolMgr.WorkersTaskCh {
				for _, distributor := range poolMgr.distributors {
					if len(input) > distributor.column1Pos {
						value := input[distributor.column1Pos]
						str, ok := value.(string)
						if ok {
							workerCh := distributor.distributionTaskMap[str]
							if workerCh == nil {
								// Got an unseen value, create a worker
								poolMgr.poolWg.Add(1)
								workerCh = make(chan []any, 1)
								distributor.distributionTaskMap[str] = workerCh
								go func() {
									defer poolMgr.poolWg.Done()
									worker := NewClusteringWorker(config, source, distributor.column1,
										value, columns2, correlationOutputCh, ctx.done, ctx.errCh)
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
		col1Pos := correlationOutputCh.columns["column_name_1"]
		col2Pos := correlationOutputCh.columns["column_name_2"]
		countPos := correlationOutputCh.columns["distinct_count"]
		totalNonNilPos := correlationOutputCh.columns["total_non_null_count"]
		// Use this variable as an accumulator to reduce all column1_value
		columnCorrelationAccumulator := make(map[string]*columnCorrelation)
		for correlationresult := range poolMgr.distributionResultCh {
			// save the result so it can be used to determine the clusters
			key := fmt.Sprintf("%v__%v", correlationresult[col1Pos], correlationresult[col2Pos])
			cc := columnCorrelationAccumulator[key]
			if cc == nil {
				cc = &columnCorrelation{
					column1:          correlationresult[col1Pos].(string),
					column2:          correlationresult[col2Pos].(string),
					distinctCount:    correlationresult[countPos].(int),
					totalNonNilCount: correlationresult[totalNonNilPos].(int),
				}
				columnCorrelationAccumulator[key] = cc
			} else {
				cc.distinctCount += correlationresult[countPos].(int)
				cc.totalNonNilCount += correlationresult[totalNonNilPos].(int)
			}
		}
		// Determine the column correlation
		var avrCorrelationPct float64
		var nbrVariables int
		for _, cc := range columnCorrelationAccumulator {
			column1 := columns1Pos[cc.column1]
			column2 := columns2Pos[cc.column2]
			correlationPct := 100 * float64(cc.distinctCount) / float64(cc.totalNonNilCount)
			avrCorrelationPct += correlationPct
			nbrVariables += 1
			poolMgr.columnsCorrelation[column1][column2] = correlationPct
			if config.IsDebug {
				log.Printf("COLUMN CORRELATION: %s -> %s: %v  (%v, %v)\n", cc.column1, cc.column2, correlationPct, cc.distinctCount, cc.totalNonNilCount)
			}
			// Send the correlation result to the output channel so it makes it's way to s3
			correlationresult := make([]any, len(poolMgr.correlationOutputCh.config.Columns))
			correlationresult[col1Pos] = cc.column1
			correlationresult[col2Pos] = cc.column2
			correlationresult[countPos] = cc.distinctCount
			correlationresult[totalNonNilPos] = cc.totalNonNilCount
			select {
			case poolMgr.correlationOutputCh.channel <- correlationresult:
			case <-ctx.done:
				log.Println("Clustering Pool Manager interrupted")
			}
		}
		avrCorrelationPct /= float64(nbrVariables)
		clusterStatus := "valid"
		if int(avrCorrelationPct+0.5) > config.MaxAvrCorrelationThresholdPct {
			log.Printf("Clustering algo failure: avr correlation is %v, exceeding max thresold of %v",
				avrCorrelationPct, config.MaxAvrCorrelationThresholdPct)
			clusterStatus = "invalid"
		} else {
			if config.IsDebug {
				log.Printf("Clustering algo: avr correlation is %v, below max thresold of %v",
					avrCorrelationPct, config.MaxAvrCorrelationThresholdPct)
			}
		}

		if config.IsDebug {
			log.Println("POOL MANAGER - Determine the clusters, clustering status:", clusterStatus)
		}
		// Determine the clusters
		threshold := float64(config.CorrelationThresholdPct)
		if threshold < 1 {
			threshold = 1
		}
		// make a lookup of the transitive data classification
		transitiveDC := make(map[string]bool)
		for _, dc := range config.TransitiveDataClassification {
			transitiveDC[dc] = true
		}
		clusters := make([]map[string]bool, 0)
		var cluster map[string]bool
		for i, column1 := range columns1 {
			c1 := getClusterOf(column1, clusters)
			if c1 < 0 {
				cluster = make(map[string]bool)
				cluster[column1] = true
			} else {
				cluster = clusters[c1]
				clusters = remove(clusters, c1)
			}
			for j, column2 := range columns2 {
				if poolMgr.columnsCorrelation[i][j] > 0 && poolMgr.columnsCorrelation[i][j] <= threshold {
					c2 := getClusterOf(column2, clusters)
					if c2 < 0 || !transitiveDC[column2] {
						// column2 is not yet in a cluster, put it in the current cluster
						cluster[column2] = true
					} else {
						// Merge c2 into cluster, remove c2 from clusters
						cluster = merge(cluster, clusters[c2])
						clusters = remove(clusters, c2)
					}
				}
			}
			// Add cluster into the set of clusters
			clusters = append(clusters, cluster)
		}
		// Validate the cluster structure, make sure the clustering did not breakdown
		maxMembership := 0
		for _, cluster := range clusters {
			c := len(cluster)
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
				columnClassificationMap := make(map[string]string)
				for _, tag := range config.ClusterDataSubclassification {
					for column, b := range cluster {
						if b {
							row, err := poolMgr.analysisLookup.Lookup(&column)
							if err == nil {
								dc, err := poolMgr.analysisLookup.LookupValue(row, poolMgr.config.TargetColumnsLookup.DataClassificationColumn)
								if err == nil {
									dataClassification, ok := dc.(string)
									if ok {
										if dataClassification == tag {
											subClassification = tag
											goto subclassificationDone
										}
										columnClassificationMap[column] = dataClassification
									}
								} else {
									log.Printf("WARNING: ignoring error while calling clustering lookup value for key %s: %v\n", column, err)
								}
							} else {
								log.Printf("WARNING: ignoring error while calling clustering lookup with key %s: %v\n", column, err)
							}
						}
					}
				}
			}
		subclassificationDone:
			for column, b := range cluster {
				if b {
					row := make([]any, len(outputCh.config.Columns))
					row[outputCh.columns["cluster_id"]] = label
					row[outputCh.columns["column_name"]] = column
					row[outputCh.columns["status"]] = clusterStatus
					if len(subClassification) == 0 && len(cluster) == 1 {
						row[outputCh.columns["data_subclassification"]] = "__SOLO__"
					} else {
						row[outputCh.columns["data_subclassification"]] = subClassification
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
		}

		poolMgr.WaitForDone.Done()
		close(workersResultCh)
		log.Println("Clustering Worker Pool Completed")
	}()
	return
}

type columnCorrelation struct {
	column1          string
	column2          string
	distinctCount    int
	totalNonNilCount int
}

func getClusterOf(column string, clusters []map[string]bool) int {
	for i, c := range clusters {
		if c[column] {
			return i
		}
	}
	return -1
}

func remove(s []map[string]bool, i int) []map[string]bool {
	s[len(s)-1], s[i] = nil, s[len(s)-1]
	return s[:len(s)-1]
}

func merge(s1, s2 map[string]bool) map[string]bool {
	for k, v := range s2 {
		s1[k] = v
	}
	return s1
}
