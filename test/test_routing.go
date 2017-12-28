package main

import (
	"github.com/owenliang/dht"
	"fmt"
)

func main()  {
	rt := dht.GetRoutingTable()
	size := 0
	for   {
		rt.InsertNode(&dht.CompactNode{"", dht.GenNodeId()})
		if size != rt.Size() {
			size = rt.Size()
			fmt.Println(size)
			fmt.Println(rt.ClosestNodes(dht.MyNodeId()))
		}
	}
}
