package main

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes/tests/pcserv"
)


func main() {
	pcServer := new(pcserv.PeerServer)
	pcServer.Prefix = "SERVER PREFIX"
	rpc.Register(pcServer)
	// Registers an HTTP handler for RPC messages
	rpc.HandleHTTP()
	// Start listening for the requests 
	listener, err := net.Listen("tcp", ":8085")
	if err !=nil {
		log.Fatal("Listener error: ", err)
	}
	// Serve accepts incoming HTTP connections on the listener l, creating 
	// a new service goroutine for each. The service goroutines read requests 
	// and then call handler to reply to them
	go func (){
		err := http.Serve(listener, nil)
		log.Println("server DONE with err", err)
	}()
	log.Println("OK server started on :8085 for 20 sec")
	time.Sleep(20 * time.Second)
	log.Println("OK shutdown server")
	listener.Close()
}
