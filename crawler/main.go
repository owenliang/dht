package main

import (
	"github.com/owenliang/dht"
	"os"
	"fmt"
	"context"
	"time"
)

func main()  {
	var (
		krpc *dht.KRPC
		err error
		nodes = make(chan *dht.CompactNode, 10000)
		bootstrap  = "router.bittorrent.com:6881"
	)
	if krpc, err = dht.CreateKPRC(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// 实际上, 做一个爬虫并不需要维护路由表, 而只需要尽快加入到更多节点的路由表中

	// 不停的find_node, 让更多人认识我
	nodes <- &dht.CompactNode{bootstrap, ""}
	for i := 0; i < 3000; i++ {
		go func() {
			var (
				err error
				node *dht.CompactNode
				findNodeReq *dht.FindNodeRequest
				findNodeResp *dht.FindNodeResponse
				compactNode *dht.CompactNode
			)
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
				findNodeReq.Target = dht.GenNodeId()

				if findNodeResp, err = krpc.FindNode(context.Background(), findNodeReq, node.Address); err == nil {
					for _, compactNode = range findNodeResp.Nodes {
						//  没必要维护路由表
						// dht.GetRoutingTable().InsertNode(compactNode)
						select {
						case nodes <- compactNode:
						default:
						}
					}
				}
			}
		}()
	}
	for {
		time.Sleep(1 * time.Second)
	}
}
