package main

import (
	"github.com/owenliang/dht"
)

func main()  {
	rt := dht.CreateRoutingTable()
	for   {
		rt.InsertNode(&dht.CompactNode{"", dht.GenNodeId()})
	}
}
