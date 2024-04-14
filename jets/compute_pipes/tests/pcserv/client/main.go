package main 

import (
	"log"
	"net/rpc"

	"github.com/artisoft-io/jetstore/jets/compute_pipes/tests/pcserv"
)

func main() {
	// Address to this variable will be sent to the RPC server 
	// Type of reply should be same as that specified on server 
	args := &pcserv.PeerMessage{
		RecordsCount: 22,
		Records: []interface{}{"A", "B", "C"},
	}
	reply := pcserv.PeerReply{}
	
	// DialHTTP connects to an HTTP RPC server at the specified network
	client, err := rpc.DialHTTP("tcp", "127.0.0.1:8085")
	if err != nil {
		log.Fatal("Client connection error: ", err)
	}
	log.Println("Client connected, calling method")

	// Invoke the remote function PushRecords
	err = client.Call("PeerServer.PushRecords", args, &reply)
	if err != nil {
		log.Fatal("Client invocation error: ", err)
	}

	// Print the reply from the server 
	log.Printf("Got back a reply: %d Records :: %s", reply.RecordsCount, reply.ErrMsg)
}