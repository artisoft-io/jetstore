package pcserv

import "log"


type PeerMessage struct {
	RecordsCount int32
	Records     []interface{}
}

type PeerReply struct {
	RecordsCount int32
	ErrMsg   string
}

type PeerServer struct {
	Prefix string
}

func (ps *PeerServer) PushRecords(args *PeerMessage, reply *PeerReply) error {
	*reply = PeerReply{
		RecordsCount: args.RecordsCount,
	}
	log.Printf("%s got request with %d records", ps.Prefix, args.RecordsCount)
	for i := range args.Records {
		log.Println("GOT Record", args.Records[i])
	}
	return nil
}
