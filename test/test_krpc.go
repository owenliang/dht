package main

import (
	"github.com/owenliang/dht"

	"os"
	"fmt"
	"context"
)

func main()  {
	var (
		krpc *dht.KRPC

		pingRequest *dht.PingRequest
		pingResponse *dht.PingResponse

		findNodeRequest *dht.FindNodeRequest
		findNodeResponse *dht.FindNodeResponse

		getPeersRequest *dht.GetPeersRequest
		getPeersResponse *dht.GetPeersResponse

		announcePeerRequest *dht.AnnouncePeerRequest
		announcePeerResponse *dht.AnnouncePeerResponse

		address = "router.bittorrent.com:6881"
		//address = "dht.transmissionbt.com:6881"
		err error
	)

	// krpc
	if krpc, err = dht.CreateKPRC(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for {
		// ping
		pingRequest = dht.NewPingRequest()
		pingResponse, err = krpc.Ping(context.Background(), pingRequest, address)
		fmt.Println("Ping", pingResponse, err)

		// find node
		findNodeRequest = dht.NewFindNodeRequest()
		findNodeRequest.Target = dht.GenNodeId()
		findNodeResponse, err = krpc.FindNode(context.Background(), findNodeRequest, address)
		fmt.Println("FindNode", findNodeResponse, err)

		// get peers
		getPeersRequest = dht.NewGetPeersRequest()
		getPeersRequest.InfoHash = dht.GenNodeId() // 随机仿造一个20字节的info_hash
		getPeersResponse, err = krpc.GetPeers(context.Background(), getPeersRequest, address)
		fmt.Println("GetPeers", getPeersResponse, err)

		// announce peer
		announcePeerRequest = dht.NewAnnouncePeerRequest()
		announcePeerRequest.InfoHash = dht.GenNodeId() // 随机仿造一个20字节的info_hash
		announcePeerRequest.Token = dht.GenNodeId() // 随机伪造一个token
		announcePeerResponse, err = krpc.AnnouncePeer(context.Background(), announcePeerRequest, address)
		fmt.Println("AnnouncePeer", announcePeerResponse, err)
	}
}
