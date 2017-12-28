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
		nodes = make(chan *dht.CompactNode, 10000)
		node *dht.CompactNode
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
	// 作为一个爬虫，其实不太需要维护Routing table的样子
	nodes <- &dht.CompactNode{bootstrap, ""}
	for {
		node = nil
		select {
		case node = <-nodes:
		default:
		}
		if node == nil {
			node = &dht.CompactNode{bootstrap, ""}
		}

		findNodeReq = dht.NewFindNodeRequest()
		if len(node.Id) == 0 {
			findNodeReq.Target = dht.MyNodeId()
		} else {
			findNodeReq.Target = node.Id[:10] + dht.MyNodeId()[10:]
		}

		if findNodeResp, err = krpc.FindNode(context.Background(), findNodeReq, node.Address); err == nil {
			for _, compactNode = range findNodeResp.Nodes {
				dht.GetRoutingTable().InsertNode(compactNode)
				select {
				case nodes <- compactNode:
				default:
					<- nodes
					nodes <- compactNode
				}
			}
		}
	}
}
