package compute_pipes

import (
	"log"
	"sync"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// JrPoolManager manages a pool of JrPoolWorkers for jetrules execution

// JrPoolManager manage a pool of workers to execute rules in parallel
// JrPoolWg is a wait group of the workers.
// The WorkersTaskCh is closed in jetrules operator
type JrPoolManager struct {
	config        *JetrulesSpec
	WorkersTaskCh chan []interface{}
	JrPoolWg      *sync.WaitGroup
}

// Create the JrPoolManager, it will be set to the receiving BuilderContext
func (ctx *BuilderContext) NewJrPoolManager(
	config *JetrulesSpec, source *InputChannel, reteMetaStore *rete.ReteMetaStoreFactory,
	outputChannels []*JetrulesOutputChan, jetrulesWorkerResultCh chan JetrulesWorkerResult) (jrpm *JrPoolManager, err error) {

	// Create the pool manager
	jrpm = &JrPoolManager{
		config:        config,
		WorkersTaskCh: make(chan []interface{}, 1),
		JrPoolWg:      new(sync.WaitGroup),
	}

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
			ErrorsCount: errorCount,
			Err:        err}:
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
		for i := 0; i < ctx.s3DeviceManager.s3WorkerPoolSize; i++ {
			jrpm.JrPoolWg.Add(1)
			go func() {
				defer jrpm.JrPoolWg.Done()
				worker := NewJrPoolWorker(config, source, reteMetaStore, outputChannels, ctx.done, ctx.errCh)
				worker.DoWork(jrpm, workersResultCh)
			}()
		}
		jrpm.JrPoolWg.Wait()
		close(workersResultCh)
	}()
	return
}
