package compute_pipes

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
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
	var peersInWg, remainingPeerInWg sync.WaitGroup
	var distributionWg sync.WaitGroup
	var distributionCh []chan []interface{}
	var distributionResultCh chan ComputePipesResult
	var incommingDataCh chan []interface{}
	var server net.Listener
	var outPeers []Peer
	var evaluators []PipeTransformationEvaluator
	var destinationShardId int
	nbrShard := ctx.env["$NBR_SHARDS"].(int)
	shardId := ctx.env["$SHARD_ID"].(int)
	var spliterColumnIdx int
	var ok bool
	var addr string
	defer func() {
		// Closing the output channels
		fmt.Println("**! CLUSTER_MAP: Closing Output Channels")
		oc := make(map[string]bool)
		for i := range spec.Apply {
			oc[spec.Apply[i].Output] = true
		}
		for i := range oc {
			fmt.Println("**! CLUSTER_MAP: Closing Output Channel", i)
			ctx.channelRegistry.CloseChannel(i)
		}
	}()

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
		cpErr = fmt.Errorf("while opening a listener on %s for incomming connection: %v", addr, err)
		goto gotError
	}
	log.Println("**!@@ CLUSTER_MAP *2 Listner started on", addr)
	remainingPeerInWg.Add(nbrShard - 1)
	go ctx.listenForIncomingData(server, incommingDataCh, &peersInWg, &remainingPeerInWg)

	// Note: when evaluatorsWg and source is done, need to call Close() on server to terminate the Accept loop
	// and close intermediate channel incommingDataCh
	err = ctx.registerNode()
	if err != nil {
		cpErr = fmt.Errorf("while calling registerNode: %v", err)
		goto gotError
	}
	outPeers = make([]Peer, len(ctx.peersAddress))
	// Open the client connections with peers -- send data, output sources
	for i, peerAddress := range ctx.peersAddress {
		log.Printf("**!@@ CLUSTER_MAP *3 (%s) connecting to %s", ctx.selfAddress, peerAddress)
		if peerAddress != ctx.selfAddress {
			retry := 0
			for {
				conn, err := net.Dial("tcp", peerAddress)
				if err == nil {
					log.Printf("**!@@ CLUSTER_MAP *3 (%s) CONNECTED to %s on try #%d", ctx.selfAddress, peerAddress, retry)
					outPeers[i] = Peer{
						peerAddress: peerAddress,
						conn:        conn,
					}
					break
				}
				if retry >= 5 {
					cpErr = fmt.Errorf("too many retry while opening conn with peer %d at %s for cluster_map with source channel %s: %v", i, peerAddress, source.config.Name, err)
					goto gotError
				}
				log.Printf("**!@@ CLUSTER_MAP *3 (%s) failed to connect to %s on try #%d, will retry", ctx.selfAddress, peerAddress, retry)
				time.Sleep(1 * time.Second)
				retry++
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
	// log.Printf("**!@@ CLUSTER_MAP *4 WAIT for all incomming PEER conn to be established")
	remainingPeerInWg.Wait()
	// log.Printf("**!@@ CLUSTER_MAP *4 DONE WAIT got all incomming PEER conn established")

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
		// log.Printf("**!@@ CLUSTER_MAP *5 Processing intermediate channel incommingDataCh")
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
		// log.Printf("**!@@ CLUSTER_MAP *5 Processing intermediate channel incommingDataCh - All good!")
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
					err = ctx.sendRow(iWorker, outPeers[iWorker].conn, inRow)
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
		var keyHash uint64
		v := inRow[spliterColumnIdx]
		if v != nil {
			key = toString(v)
		}
		if len(key) > 0 {
			// hash the key, select a peer node
			h := fnv.New64a()
			h.Write([]byte(key))
			keyHash = h.Sum64()
			destinationShardId = int(keyHash % uint64(nbrShard))
		} else {
			// pick random shard
			destinationShardId = rand.Intn(nbrShard)
		}
		// log.Printf("**!@@ CLUSTER_MAP *5 INPUT key: %s, hash: %d => %d", key, keyHash, destinationShardId)
		// consume or send the record via the distribution channels
		select {
		case distributionCh[destinationShardId] <- inRow:
		case <-ctx.done:
			log.Printf("ClusterMap: writing to incommingDataCh intermediate channel interrupted")
			goto doneSource // so we can clean up
		}
	}
doneSource:
	log.Printf("**!@@ CLUSTER_MAP *5 DONE Processing input source channel: %s", source.config.Name)

	// Close the distribution channel to outPeer since processing the source has completed
	for i := range distributionCh {
		if i == shardId {
			// Local shard, correspond to incommingDataCh, will be closed once the incomming peer
			// connections are closed
		} else {
			close(distributionCh[i])
		}
	}

	// All the evaluators are created and no errors so far, we can close the writePartitionsResultCh
	close(writePartitionsResultCh)

	// Wait for the distrubution channels to be completed
	// log.Printf("**!@@ CLUSTER_MAP *7 WAIT on distributionWg so we can close the connection to PEER")
	distributionWg.Wait()
	// log.Printf("**!@@ CLUSTER_MAP *7 DONE WAIT on distributionWg CLOSING connections to PEER")
	// Close the outgoing connection to peer nodes
	for i := range outPeers {
		if outPeers[i].conn != nil {
			outPeers[i].conn.Close()
		}
	}

	// Source channel completed, now wait for the peers with incoming records to complete
	// log.Printf("**!@@ CLUSTER_MAP *8 WAIT on peersInWg - incomming PEER")
	peersInWg.Wait()
	// log.Printf("**!@@ CLUSTER_MAP *8 DONE WAIT on peersInWg - incomming PEER")

	// Close incommingDataCh and server listerner
	close(incommingDataCh)
	server.Close()
	server = nil

	// When the evaluators has completed processing incommingDataCh then close output channels
	evaluatorsWg.Wait()

	// All good!
	return

gotError:
	close(clusterMapResultCh)
	close(writePartitionsResultCh)
	log.Println(cpErr)
	ctx.errCh <- cpErr
	close(ctx.done)
	if server != nil {
		server.Close()
	}
}

func (ctx *BuilderContext) registerNode() error {
	var err, cpErr error
	shardId := ctx.env["$SHARD_ID"].(int)

	// Get the node address and register it with database
	nodePort := strings.Split(ctx.env["$CPIPES_SERVER_ADDR"].(string), ":")[1]
	if ctx.env["$JETSTORE_DEV_MODE"].(bool) {
		ctx.selfAddress = fmt.Sprintf("127.0.0.1:%s", nodePort)
	} else {
		nodeIp, err := awsi.GetPrivateIp()
		if err != nil {
			cpErr = fmt.Errorf("while getting node's IP (in StartComputePipes): %v", err)
			return cpErr
		}
		ctx.selfAddress = fmt.Sprintf("%s:%s", nodeIp, nodePort)
	}
	// Register node to database
	sessionId := ctx.env["$SESSIONID"].(string)
	stmt := fmt.Sprintf(
		"INSERT INTO jetsapi.cpipes_cluster_node_registry (session_id, node_address, shard_id) VALUES ('%s','%s',%d);",
		sessionId, ctx.selfAddress, shardId)
	log.Println(stmt)
	_, err = ctx.dbpool.Exec(context.Background(), stmt)
	if err != nil {
		cpErr = fmt.Errorf("while inserting node's addressin db (in StartComputePipes): %v", err)
		return cpErr
	}
	log.Printf("Node's address %s registered into database", ctx.selfAddress)
	// Get the peers addresses from database
	registrationTimeout := ctx.cpConfig.ClusterConfig.PeerRegistrationTimeout
	if registrationTimeout == 0 {
		registrationTimeout = 60
	}
	nbrNodes := ctx.env["$NBR_SHARDS"].(int)
	stmt = "SELECT node_address FROM jetsapi.cpipes_cluster_node_registry WHERE session_id = $1 ORDER BY shard_id ASC"
	start := time.Now()
	for {
		ctx.peersAddress = make([]string, 0)
		rows, err := ctx.dbpool.Query(context.Background(), stmt, sessionId)
		if err != nil {
			cpErr = fmt.Errorf("while querying peer's address from db (in StartComputePipes): %v", err)
			return cpErr
		}
		for rows.Next() {
			var addr string
			if err := rows.Scan(&addr); err != nil {
				rows.Close()
				cpErr = fmt.Errorf("while scanning node's address from db (in StartComputePipes): %v", err)
				return cpErr
			}
			ctx.peersAddress = append(ctx.peersAddress, addr)
		}
		rows.Close()
		if len(ctx.peersAddress) == nbrNodes {
			log.Printf("Got %d out of %d peer's addresses, done", len(ctx.peersAddress), nbrNodes)
			break
		}
		log.Printf("Got %d out of %d peer's addresses, will try again", len(ctx.peersAddress), nbrNodes)
		if time.Since(start) > time.Duration(registrationTimeout)*time.Second {
			log.Printf("Error, timeout occured while trying to get peer's addresses")
			cpErr = fmt.Errorf("error: timeout while getting peers addresses (in StartComputePipes): %v", err)
			return cpErr
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func (ctx *BuilderContext) listenForIncomingData(server net.Listener, incommingDataCh chan<- []interface{},
	peersWg, remainingPeerInWg *sync.WaitGroup) {
	// server.Close() will be called by the caller to terminate the loop
	log.Println("**!@@ CLUSTER_MAP *2 calling server.Accept()")
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Println("Accept error: ", err, " :: done")
			return
		}
		peersWg.Add(1)
		remainingPeerInWg.Done()
		go ctx.handleIncomingData(conn, incommingDataCh, peersWg)
	}
}

func (ctx *BuilderContext) handleIncomingData(conn net.Conn, incommingDataCh chan<- []interface{}, peersWg *sync.WaitGroup) {
	defer func() {
		conn.Close()
		peersWg.Done()
	}()

	timeoutDuration := 180 * time.Second
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
	tmpSz := 1024
	var n int
	var err error
	for {
		// create a temp buffer
		tmp := make([]byte, tmpSz)
		tmpbuff := new(bytes.Buffer)
		for {
			n, err = conn.Read(tmp)
			if n > 0 {
				tmpbuff.Write(tmp[:n])
			}
			if (n > 0 && n < tmpSz) || err != nil {
				break
			}
		}

		// irow := 0
		bufLen := tmpbuff.Len()
		if bufLen > 0 {
			for {
				// decode the rows received
				row := new([]interface{})
				// creates a decoder object
				gobobj := gob.NewDecoder(tmpbuff)
				// decodes buffer and unmarshals it into a Message struct
				err2 := gobobj.Decode(row)
				if err2 == io.EOF {
					break
				}
				// irow++
				// Send the record to the intermediate channel
				// fmt.Printf("**!@@ Got record LENGTH %d #%d of msg remaining %d of %d\n", len(*row), irow, tmpbuff.Len(), bufLen)
				if len(*row) > 0 {
					select {
					case incommingDataCh <- *row:
					case <-ctx.done:
						log.Printf("handleIncomingData: writing to incommingDataCh intermediate channel interrupted")
						return
					}
				}
			}
		}
		// Check if we're done
		switch {
		case err == nil:
		case errors.Is(err, os.ErrDeadlineExceeded):
			log.Printf("*** PEER Deadline exceeded on read, bailing out")
			return
		case err == io.EOF:
			log.Printf("*** PEER Connect closed by client, done")
			return
		default:
			// read error
			log.Println("*** PEER read error:", err)
			return
		}
	}
}

func (ctx *BuilderContext) sendRow(iWorker int, conn net.Conn, row []interface{}) error {
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
	// log.Printf("**!@@ CLUSTER_MAP *6 Sending ROW of length %d as %d bytes to peer %d - transmitted %d bytes", len(row), bin_buf.Len(), iWorker, n)
	return err
}
