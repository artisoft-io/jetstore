package compute_pipes

import (
	"fmt"
	"log"
	"sync"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// JrPoolManager manages a pool of JrPoolWorkers for jetrules execution

// JrPoolManager manage a pool of workers to execute rules in parallel
// jrPoolWg is a wait group of the workers.
// The WorkersTaskCh is closed in jetrules operator
type JrPoolManager struct {
	config        *JetrulesSpec
	WorkersTaskCh chan []interface{}
	jrPoolWg      *sync.WaitGroup
	WaitForDone   *sync.WaitGroup
}

// Create the JrPoolManager, it will be set to the receiving BuilderContext
func (ctx *BuilderContext) NewJrPoolManager(
	config *JetrulesSpec, source *InputChannel, reteMetaStore *rete.ReteMetaStoreFactory,
	outputChannels []*JetrulesOutputChan, jetrulesWorkerResultCh chan JetrulesWorkerResult) (jrpm *JrPoolManager, err error) {
	log.Println("Starting the Pool Manager")
	if config.PoolSize < 1 {
		close(jetrulesWorkerResultCh)
		return nil, fmt.Errorf("error: cannot have a worker pool of size less than 1")
	}
	// Create the pool manager
	jrpm = &JrPoolManager{
		config:        config,
		WorkersTaskCh: make(chan []interface{}, 1),
		jrPoolWg:      new(sync.WaitGroup),
		WaitForDone:   new(sync.WaitGroup),
	}
	jrpm.WaitForDone.Add(1)

	// Create a channel for the workers to report results
	workersResultCh := make(chan JetrulesWorkerResult)
	// Collect the results from all the workers
	go func() {
		var sessionCount int64
		var errorCount int64
		var err error
		for workerResult := range workersResultCh {
			sessionCount += workerResult.ReteSessionCount
			errorCount += workerResult.ErrorsCount
			if workerResult.Err != nil {
				err = workerResult.Err
				break
			}
		}
		// Send out the collected result
		select {
		case jetrulesWorkerResultCh <- JetrulesWorkerResult{
			ReteSessionCount: sessionCount,
			ErrorsCount:      errorCount,
			Err:              err}:
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
			log.Printf("Collecting results from JrPoolWorkers interrupted")
		}
		close(jetrulesWorkerResultCh)
	}()

	// Set up all the workers, use a wait group to track when they are all done
	// to close workersResultCh
	go func() {
		log.Println("Starting a Worker Pool of size", config.PoolSize)
		for i := 0; i < config.PoolSize; i++ {
			jrpm.jrPoolWg.Add(1)
			go func() {
				defer jrpm.jrPoolWg.Done()
				worker := NewJrPoolWorker(config, source, reteMetaStore, outputChannels, ctx.done, ctx.errCh)
				worker.DoWork(jrpm, workersResultCh)
			}()
		}
		jrpm.jrPoolWg.Wait()
		jrpm.WaitForDone.Done()
		close(workersResultCh)
		log.Println("Jetrules Worker Pool Completed")
	}()
	return
}
