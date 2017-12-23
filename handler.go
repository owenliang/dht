package dht

import "net"

func HandlePing(transactionId string, addDict map[string]interface{},  packetFrom *net.UDPAddr) ([]byte, error)  {
	pong := map[string]interface{}{}
	pong["t"] = transactionId
	pong["y"] = "r"
	a := map[string]interface{}{}
	a["id"] = MyNodeId()
	pong["r"] = a
	return Encode(pong)
}
