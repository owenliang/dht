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
		err error
		nodes = make(chan string, 10000)
		node string
		bootstrap  = "router.bittorrent.com:6881"
		findNodeReq *dht.FindNodeRequest
		findNodeResp *dht.FindNodeResponse
		compactNode *dht.CompactNode
	)
	if krpc, err = dht.CreateKPRC(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// TODO: 对RoutingTable进行定期的node ping和bucket refresh

	// 不停的find_node, 让更多人认识我
	nodes <- bootstrap
	for {
		node = ""
		select {
		case node = <-nodes:
		default:
		}
		if len(node) == 0 {
			node = bootstrap
		}

		findNodeReq = dht.NewFindNodeRequest()
		findNodeReq.Target = dht.GenNodeId()
		if findNodeResp, err = krpc.FindNode(context.Background(), findNodeReq, node); err == nil {
			for _, compactNode = range findNodeResp.Nodes {
				dht.GetRoutingTable().InsertNode(compactNode)
				select {
				case nodes <- compactNode.Address:
					fmt.Println(compactNode)
				default:
					<- nodes
					nodes <- compactNode.Address
				}
			}
		}
	}
}
