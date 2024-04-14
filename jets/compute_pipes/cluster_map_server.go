package compute_pipes

import (
	"fmt"
	"log"
	"net/rpc"
	"sync"
)

type Peer struct {
	peerAddress string
	client      *rpc.Client
}

// Message used to send records to remote peer
// Sender is nodeId (shardId) or client peer
type PeerRecordMessage struct {
	Sender       int32
	RecordsCount int32
	Records      [][]interface{}
}
type PeerReply struct{}

// The server handling incomming requests from peer nodes
type PeerServer struct {
	nodeId                    int32
	recordCount               map[int]*int64
	peersWg                   *sync.WaitGroup
	remainingPeerInWg         *sync.WaitGroup
	incommingDataCh           chan<- []interface{}
	peersResultClosed         map[int]*bool
	receivedFromPeersResultCh []chan ComputePipesResult
	errCh                     chan error
	done                      chan struct{}
}

func (ps *PeerServer) ClientReady(args *PeerRecordMessage, reply *PeerReply) error {
	*reply = PeerReply{}
	sender := int(args.Sender)
	if args.Sender > ps.nodeId {
		sender -= 1
	}
	if sender < 0 || sender > len(ps.receivedFromPeersResultCh) {
		return fmt.Errorf("error: invalid sender %d, expecting up to %d",
			args.Sender, len(ps.receivedFromPeersResultCh)+1)
	}
	// log.Printf("**!@@ PeerServer: got ClientReady for peer %d", args.Sender)
	ps.peersWg.Add(1)
	ps.remainingPeerInWg.Done()
	return nil
}

func (ps *PeerServer) PushRecords(args *PeerRecordMessage, reply *PeerReply) error {
	var cpErr error
	*reply = PeerReply{}
	sender := int(args.Sender)
	if args.Sender > ps.nodeId {
		sender -= 1
	}
	if sender < 0 || sender > len(ps.receivedFromPeersResultCh) {
		return fmt.Errorf("error: invalid sender %d, expecting up to %d",
			args.Sender, len(ps.receivedFromPeersResultCh)+1)
	}
	// Verify the count is good
	count := int(args.RecordsCount)
	for i := 0; i < count; i++ {
		if len(args.Records[i]) == 0 {
			// cpErr = fmt.Errorf("**!@@ PeerServer ERROR got record of 0-length")
			goto gotError
		}
		select {
		case ps.incommingDataCh <- args.Records[i]:
		case <-ps.done:
			err := fmt.Errorf("PeerServer: writing to incommingDataCh intermediate channel interrupted")
			log.Println(err)
			return err
		}
	}
	*ps.recordCount[sender] += int64(count)
	return nil
gotError:
	log.Println(cpErr)
	ps.errCh <- cpErr
	close(ps.done)
	ps.receivedFromPeersResultCh[sender] <- ComputePipesResult{
		TableName:    fmt.Sprintf("Records received from peer %d (error)", args.Sender),
		CopyRowCount: *ps.recordCount[sender],
	}
	close(ps.receivedFromPeersResultCh[sender])
	*ps.peersResultClosed[sender] = true
	ps.peersWg.Done()
	return cpErr
}

func (ps *PeerServer) ClientDone(args *PeerRecordMessage, reply *PeerReply) error {
	*reply = PeerReply{}
	sender := int(args.Sender)
	if args.Sender > ps.nodeId {
		sender -= 1
	}
	if sender < 0 || sender > len(ps.receivedFromPeersResultCh) {
		//DO WE NEED TO PANIC HERE TO AVOID HANGING???
		return fmt.Errorf("error: invalid sender %d, expecting up to %d",
			args.Sender, len(ps.receivedFromPeersResultCh)+1)
	}
	ps.receivedFromPeersResultCh[sender] <- ComputePipesResult{
		TableName:    fmt.Sprintf("Records received from peer %d", args.Sender),
		CopyRowCount: *ps.recordCount[sender],
	}
	// log.Printf("**!@@ PeerServer: got ClientDone for peer %d", args.Sender)
	close(ps.receivedFromPeersResultCh[sender])
	*ps.peersResultClosed[sender] = true
	ps.peersWg.Done()
	return nil
}
