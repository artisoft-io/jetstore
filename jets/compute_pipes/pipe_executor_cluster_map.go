package compute_pipes

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"
)

// Pipe executor that shard the input channel onto the cluster based
// on the shard key. The shard key is hashed and map onto one of the
// cluster node (The number of nodes of the cluster is specified by
// nbrShard in cpipes main, aka loader main).

type Peer struct {
	peerAddress string
	conn        net.Conn
}

// Cluster nodes sharding data using splitter key
func (ctx *BuilderContext) StartClusterMap(spec *PipeSpec, source *InputChannel,
	clusterMapResultCh, writePartitionsResultCh chan chan ComputePipesResult) {
	var cpErr, err error
	var evaluatorsWg sync.WaitGroup
	var peersInWg sync.WaitGroup
	var distributionWg sync.WaitGroup
	var distributionCh []chan []interface{}
	var distributionResultCh chan ComputePipesResult
	var incommingDataCh chan []interface{}
	var server net.Listener
	outPeers := make([]Peer, len(ctx.peersAddress))
	var evaluators []PipeTransformationEvaluator
	var destinationShardId int
	nbrShard := ctx.env["$NBR_SHARDS"].(int)
	shardId := ctx.env["$SHARD_ID"].(int)
	var oc map[string]bool
	var spliterColumnIdx int
	var ok bool
	var addr string

	fmt.Println("**!@@ CLUSTER_MAP *1 Called, shuffle on column", *spec.Column)

	if ctx.cpConfig.ClusterConfig == nil {
		cpErr = fmt.Errorf("error: missing ClusterConfig section in compute_pipes_config")
		goto gotError
	}

	spliterColumnIdx, ok = source.columns[*spec.Column]
	if !ok {
		cpErr = fmt.Errorf("error: invalid column name %s for cluster_map with source channel %s", *spec.Column, source.config.Name)
		goto gotError
	}

	// Open connection with peer nodes
	// With each node, have 2 connections: one to send and the other one to receive.
	// Start the connection listener for the incomming (server) -- receive data, input sources
	// Create an intermediate channel for all the incomming connections to use to forward the
	// input records.
	incommingDataCh = make(chan []interface{}, 1)

	// Handle the incomming connection
	addr = ctx.env["$CPIPES_SERVER_ADDR"].(string)
	server, err = net.Listen("tcp", addr)
	if err != nil {
		cpErr = fmt.Errorf("while opening a listener on port 8085 for incomming connection: %v", err)
		goto gotError
	}
	fmt.Println("**!@@ CLUSTER_MAP *2 Listner started on", addr)
	go ctx.listenForIncomingData(server, incommingDataCh, &peersInWg)

	// Note: when evaluatorsWg and source is done, need to call Close() on server to terminate the Accept loop
	// and close intermediate channel incommingDataCh

	// Open the client connections with peers -- send data, output sources
	for i, peerAddress := range ctx.peersAddress {
		log.Printf("**!@@ CLUSTER_MAP *3 (%s) connecting to %s", ctx.selfAddress, peerAddress)
		if peerAddress != ctx.selfAddress {
			conn, err := net.Dial("tcp", peerAddress)
			if err != nil {
				cpErr = fmt.Errorf("while opening conn with peer %d at %s for cluster_map with source channel %s", i, peerAddress, source.config.Name)
				goto gotError
			}
			outPeers[i] = Peer{
				peerAddress: peerAddress,
				conn:        conn,
			}
		} else {
			log.Printf("**!@@ CLUSTER_MAP *3 (%s) stand-in for %s", ctx.selfAddress, peerAddress)
			// Put a stand-in for self
			outPeers[i] = Peer{
				peerAddress: ctx.selfAddress,
			}
		}
	}
	log.Printf("**!@@ CLUSTER_MAP *3 (%s) All %d peer connections established", ctx.selfAddress, len(ctx.peersAddress))

	// Build the PipeTransformationEvaluators
	evaluators = make([]PipeTransformationEvaluator, len(spec.Apply))
	for j := range spec.Apply {
		partitionResultCh := make(chan ComputePipesResult, 1)
		writePartitionsResultCh <- partitionResultCh
		eval, err := ctx.buildPipeTransformationEvaluator(source, spec.Column, partitionResultCh, &spec.Apply[j])
		if err != nil {
			cpErr = fmt.Errorf("while calling buildPipeTransformationEvaluator in StartClusterMap for %s: %v", spec.Apply[j].Type, err)
			goto gotError
		}
		evaluators[j] = eval
	}

	// Have the evaluators process records from incommingDataCh in a coroutine
	evaluatorsWg.Add(1)
	go func() {
		defer evaluatorsWg.Done()
		// Process the channel
		log.Printf("**!@@ CLUSTER_MAP *5 Processing intermediate channel incommingDataCh")
		for inRow := range incommingDataCh {
			for i := range evaluators {
				err = evaluators[i].apply(&inRow)
				if err != nil {
					cpErr = fmt.Errorf("while calling apply on PipeTransformationEvaluator (in StartClusterMap): %v", err)
					goto gotError
				}
			}
		}
		// Done, close the evaluators
		for i := range spec.Apply {
			if evaluators[i] != nil {
				err = evaluators[i].done()
				if err != nil {
					log.Printf("while calling done on PipeTransformationEvaluator (in StartClusterMap): %v", err)
				}
				evaluators[i].finally()
			}
		}
		// All good!
		log.Printf("**!@@ CLUSTER_MAP *5 Processing intermediate channel incommingDataCh - All good!")
		return

	gotError:
		for i := range spec.Apply {
			if evaluators[i] != nil {
				evaluators[i].finally()
			}
		}
		log.Println(cpErr)
		ctx.errCh <- cpErr
		close(ctx.done)
	}()

	// Process the source channel, distribute the input records on the cluster,
	// The records for this node are sent to incommingDataCh
	// Process the channel
	// Add a layor of intermediate channels so the main loop does not serialize all the sending of inRow.
	// This is to allow sending to peer nodes in parallel
	distributionCh = make([]chan []interface{}, nbrShard)
	for i := range distributionCh {
		if i == shardId {
			// Consume the record locally -- no need for another coroutine, just switch the channel
			distributionCh[i] = incommingDataCh
		} else {
			distributionCh[i] = make(chan []interface{}, 4)
			distributionResultCh = make(chan ComputePipesResult, 1)
			clusterMapResultCh <- distributionResultCh
			distributionWg.Add(1)
			// Send record to peer node
			go func(iWorker int, resultCh chan ComputePipesResult) {
				defer distributionWg.Done()
				log.Printf("**!@@ CLUSTER_MAP *6 Distributing records :: sending to peer %d - starting", iWorker)
				var sentRowCount int64
				for inRow := range distributionCh[iWorker] {
					err = ctx.sendRow(outPeers[iWorker].conn, inRow)
					if err != nil {
						cpErr = fmt.Errorf("while sending row to peer node %d: %v", iWorker, err)
						goto gotError
					}
					sentRowCount += 1
				}
				// All good!
				log.Printf("**!@@ CLUSTER_MAP *6 Distributing records :: sending to peer %d - All good!", iWorker)
				resultCh <- ComputePipesResult{
					TableName:    fmt.Sprintf("Record sent to peer %d", iWorker),
					CopyRowCount: sentRowCount,
				}
				close(resultCh)
				return
			gotError:
				log.Printf("**!@@ CLUSTER_MAP *6 Distributing records :: sending to peer %d - gotError", iWorker)
				log.Println(cpErr)
				ctx.errCh <- cpErr
				close(ctx.done)
				resultCh <- ComputePipesResult{
					TableName:    fmt.Sprintf("Record sent to peer %d (error)", iWorker),
					CopyRowCount: sentRowCount,
					Err:          cpErr,
				}
				close(resultCh)

			}(i, distributionResultCh)
		}
	}
	// All the peers distribution coroutines to sent records are established, can now close clusterMapResultCh
	close(clusterMapResultCh)
	log.Printf("**!@@ CLUSTER_MAP *5 Processing input source channel: %s", source.config.Name)
	for inRow := range source.channel {
		var key string
		v := inRow[spliterColumnIdx]
		if v != nil {
			key = toString(v)
		}
		if len(key) > 0 {
			// hash the key, select a peer node
			h := fnv.New64a()
			h.Write([]byte(key))
			keyHash := h.Sum64()
			destinationShardId = int(keyHash % uint64(nbrShard))
		} else {
			// pick random shard
			destinationShardId = rand.Intn(nbrShard)
		}
		// consume or send the record via the distribution channels
		select {
		case distributionCh[destinationShardId] <- inRow:
		case <-ctx.done:
			log.Printf("ClusterMap: writing to incommingDataCh intermediate channel interrupted")
			return
		}
	}

	// All the evaluators are created and no errors so far, we can close the writePartitionsResultCh
	close(writePartitionsResultCh)

	// Source channel completed, now wait for the peers with incoming records to complete
	peersInWg.Wait()

	// Wait for the distrubution channels to be completed, this will ensure no more data
	// is sent to incommingDataCh, so we can close it
	distributionWg.Wait()

	// Close incommingDataCh and server listerner
	close(incommingDataCh)
	server.Close()

	// When the evaluators has completed processing incommingDataCh then close output channels
	evaluatorsWg.Wait()

	// Closing the output channels
	oc = make(map[string]bool)
	for i := range spec.Apply {
		oc[spec.Apply[i].Output] = true
	}
	for i := range oc {
		fmt.Println("**! SplitterPipe: Closing Output Channel", i)
		ctx.channelRegistry.CloseChannel(i)
	}

	// All good!
	return

gotError:
	close(writePartitionsResultCh)
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.done)

}

func (ctx *BuilderContext) listenForIncomingData(server net.Listener, incommingDataCh chan<- []interface{}, peersWg *sync.WaitGroup) {
	// server, err := net.Listen("tcp", ":8085")
	// if err != nil {

	// }
	// defer server.Close()

	// server.Close() will be called by the caller to terminate the loop
	fmt.Println("**!@@ CLUSTER_MAP *2 calling server.Accept()")
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Println("Accept error: ", err, " :: done")
			return
		}
		peersWg.Add(1)
		go ctx.handleIncomingData(conn, incommingDataCh, peersWg)
	}
}

func (ctx *BuilderContext) handleIncomingData(conn net.Conn, incommingDataCh chan<- []interface{}, peersWg *sync.WaitGroup) {
	defer func() {
		conn.Close()
		peersWg.Done()
	}()

	timeoutDuration := 600 * time.Second
	if ctx.cpConfig.ClusterConfig != nil {
		d := ctx.cpConfig.ClusterConfig.ReadTimeout
		if d > 0 {
			timeoutDuration = time.Duration(d) * time.Second
		}
	}
	fmt.Println("Launching server...")
	conn.SetReadDeadline(time.Now().Add(timeoutDuration))

	remoteAddr := conn.RemoteAddr().String()
	fmt.Println("Client connected from " + remoteAddr)

	// create a temp 10k buffer
	tmp := make([]byte, 10240)
	for {
		_, err := conn.Read(tmp)
		switch {
		case err == nil:
		case errors.Is(err, os.ErrDeadlineExceeded):
			//* add time to deadlime?
			return
		case err == io.EOF:
			// done
			return
		default:
			// read error
			log.Println("*** read error:", err)
			return
		}

		// convert bytes into Buffer (which implements io.Reader/io.Writer)
		tmpbuff := bytes.NewBuffer(tmp)
		row := new([]interface{})

		// creates a decoder object
		gobobj := gob.NewDecoder(tmpbuff)
		// decodes buffer and unmarshals it into a Message struct
		gobobj.Decode(row)

		// Send the record to the intermediate channel
		select {
		case incommingDataCh <- *row:
		case <-ctx.done:
			log.Printf("handleIncomingData: writing to incommingDataCh intermediate channel interrupted")
			return
		}
	}
}

func (ctx *BuilderContext) sendRow(conn net.Conn, row []interface{}) error {
	bin_buf := new(bytes.Buffer)

	// create a encoder object
	gobobje := gob.NewEncoder(bin_buf)
	// encode buffer and marshal it into a gob object
	err := gobobje.Encode(row)
	if err != nil {
		log.Println("error while encoding row:", err)
		return err
	}
	_, err = conn.Write(bin_buf.Bytes())
	return err
}
