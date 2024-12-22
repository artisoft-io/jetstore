package compute_pipes

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// ClusteringPoolManager manages a pool of ClusteringPoolWorkers for jetrules execution

// ClusteringPoolManager manage a pool of workers to calculate the columns correlation in parallel
// poolWg is a wait group of the workers.
// The WorkersTaskCh is a channel between the clustering operator and the pool manager (to limit the nbr of rows)
// The distributionTaskCh is used by the pool manager to distribute the input rows to all the workers.
// The distributionResultCh is used to collect the correlation resultts from the workers by the pool manager.
// The correlationOutputCh is to send the correlation results to s3, this is done when is_debug is true
type ClusteringPoolManager struct {
	config               *ClusteringSpec
	WorkersTaskCh        chan []any
	distributionTaskCh   []chan []any
	distributionResultCh chan []any
	columnsCorrelation   [][]float64
	correlationOutputCh  *OutputChannel
	poolWg               *sync.WaitGroup
	WaitForDone          *sync.WaitGroup
}

// Create the ClusteringPoolManager, it will be set to the receiving BuilderContext
func (ctx *BuilderContext) NewClusteringPoolManager(config *ClusteringSpec,
	source *InputChannel, outputCh *OutputChannel, correlationOutputCh *OutputChannel,
	clusteringResultCh chan ClusteringResult) (poolMgr *ClusteringPoolManager, err error) {

	log.Println("Starting the Clustering Pool Manager")
	// Create the pool manager
	poolMgr = &ClusteringPoolManager{
		config:               config,
		WorkersTaskCh:        make(chan []any, 1),
		distributionTaskCh:   make([]chan []any, 0),
		distributionResultCh: make(chan []any, 100),
		correlationOutputCh:  correlationOutputCh,
		poolWg:               new(sync.WaitGroup),
		WaitForDone:          new(sync.WaitGroup),
	}

	// Identify the columns that match column1 and column2 criteria
	targetConfig := &config.TargetColumnsLookup
	columns1 := make([]string, 0)
	columns2 := make([]string, 0)
	columns1Pos := make(map[string]int)
	columns2Pos := make(map[string]int)
	analysisLookup := ctx.lookupTableManager.LookupTableMap[targetConfig.LookupName]
	if analysisLookup == nil {
		return nil, fmt.Errorf("error: clustering operator lookup table %s is not found", targetConfig.LookupName)
	}
	tag1map := make(map[any]bool)
	tag2map := make(map[any]bool)
	for _, tag := range targetConfig.Column1ClassificationValues {
		tag1map[tag] = true
	}
	for _, tag := range targetConfig.Column2ClassificationValues {
		tag2map[tag] = true
	}
	for _, column := range source.config.Columns {
		lkrow, err := analysisLookup.Lookup(&column)
		if err != nil {
			return nil, fmt.Errorf("NewClusteringPoolManager: while looking up key %s from table %s",
				column, targetConfig.LookupName)
		}
		columnTag, err := analysisLookup.LookupValue(lkrow, targetConfig.DataClassificationColumn)
		if err != nil {
			return nil, fmt.Errorf("NewClusteringPoolManager: while getting '%s' lookup row value: %v",
				targetConfig.DataClassificationColumn, err)
		}
		if tag1map[columnTag] {
			columns1 = append(columns1, column)
			columns1Pos[column] = len(columns1)
			distributionCh := make(chan []any, 1)
			poolMgr.distributionTaskCh = append(poolMgr.distributionTaskCh, distributionCh)
		}
		if tag2map[columnTag] {
			columns2 = append(columns2, column)
			columns2Pos[column] = len(columns2)
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
	// For each column with column1_classification_values compute the correlation
	// with the columns with column2_classification_values.
	// The correlation is calculated as the 100 * (nbr_distinct_value / total_non_nil_values)
	// Lower is the ratio, more correlated is column1 with column2.
	go func() {
		log.Println("Starting the clustering Worker Pool")
		for i, column1 := range columns1 {
			poolMgr.poolWg.Add(1)
			go func() {
				defer poolMgr.poolWg.Done()
				worker := NewClusteringWorker(config, source, column1, columns2, correlationOutputCh,
					ctx.done, ctx.errCh)
				worker.DoWork(poolMgr.distributionTaskCh[i], poolMgr.distributionResultCh, workersResultCh)
			}()
		}
		// Distribute the input rows to the workers
		go func() {
			defer func() {
				// Close the intermediate task distribution channels
				for _, taskCh := range poolMgr.distributionTaskCh {
					close(taskCh)
				}
			}()
			for input := range poolMgr.WorkersTaskCh {
				for _, taskCh := range poolMgr.distributionTaskCh {
					// Send the input row to worker's task channel
					select {
					case taskCh <- input:
					case <-ctx.done:
						log.Println("Clustering Pool Manager interrupted while distributing tasks to workers")
					}
				}
			}
		}()
		// Collect the results from the workers
		col1Pos := correlationOutputCh.columns["column_name_1"]
		col2Pos := correlationOutputCh.columns["column_name_2"]
		countPos := correlationOutputCh.columns["distinct_count"]
		countPctPos := correlationOutputCh.columns["distinct_count_pct"]
		for correlationresult := range poolMgr.distributionResultCh {
			if poolMgr.config.IsDebug {
				// Send the correlation result to the output channel so it makes it's way to s3
				select {
				case poolMgr.correlationOutputCh.channel <- correlationresult:
				case <-ctx.done:
					log.Println("Clustering Pool Manager interrupted")
				}
				// save the result so it can be used to determine the clusters
				column1 := correlationresult[col1Pos].(string)
				column2 := correlationresult[col2Pos].(string)
				countPct := correlationresult[countPctPos].(float64)
				if poolMgr.config.IsDebug {
					log.Printf("COLUMN CORRELATION: %s -> %s: (%v, %f)\n", column1, column2, correlationresult[countPos], countPct)
				}
				poolMgr.columnsCorrelation[columns1Pos[column1]][columns2Pos[column2]] = countPct
			}
		}
		poolMgr.poolWg.Wait()

		// Determine the clusters
		threshold := float64(config.CorrelationThresholdPct)
		if threshold < 1 {
			threshold = 1
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
				if poolMgr.columnsCorrelation[i][j] <= threshold {
					c2 := getClusterOf(column2, clusters)
					if c2 < 0 {
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
		// Send out the cluster information
		for i, cluster := range clusters {
			label := fmt.Sprintf("cluster%d", i)
			for column, b := range cluster {
				if b {
					row := make([]any, 2)
					row[0] = label
					row[1] = column
					// Send the cluster membership to output channel
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
