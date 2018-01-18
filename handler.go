package dht

import (
	"net"
	"errors"
	"fmt"
	"encoding/hex"
)

func ActiveNode(addDict map[string]interface{},  packetFrom *net.UDPAddr) {
	var (
		iField interface{}
		id string
		exist bool
		typeOk bool
	)
	if iField, exist = addDict["id"]; !exist {
		return
	}
	if id, typeOk = iField.(string); !typeOk {
		return
	}
	GetRoutingTable().InsertNode(NewCompactNode(id, packetFrom))
}

func HandlePing(transactionId string, addDict map[string]interface{},  packetFrom *net.UDPAddr) ([]byte, error)  {
	ActiveNode(addDict, packetFrom)

	resp := &PingResponse{}
	resp.TransactionId = transactionId
	return resp.Serialize()
}

func HandleFindNode(transactionId string, addDict map[string]interface{},  packetFrom *net.UDPAddr) ([]byte, error)  {
	var (
		iField interface{}
		target string
		exist bool
		typeOk bool
		targetNode *CompactNode
	)

	ActiveNode(addDict, packetFrom)

	resp := &FindNodeResponse{}
	resp.TransactionId = transactionId

	if iField, exist = addDict["target"]; !exist {
		return nil, errors.New("missing target field")
	}

	if target, typeOk = iField.(string); !typeOk {
		return nil, errors.New("target type invalid")
	}

	if targetNode = GetRoutingTable().FindNode(target); targetNode != nil {
		resp.Nodes = make([]*CompactNode, 1)
		resp.Nodes[0] = targetNode
	} else {
		resp.Nodes = GetRoutingTable().ClosestNodes(target)
	}
	return resp.Serialize()
}

func HandleGetPeer(transactionId string, addDict map[string]interface{},  packetFrom *net.UDPAddr) ([]byte, error)  {
	var (
		iField interface{}
		infoHash string
		exist bool
		typeOk bool
	)

	ActiveNode(addDict, packetFrom)

	resp := &GetPeersResponse{}
	resp.TransactionId = transactionId

	if iField, exist = addDict["info_hash"]; !exist {
		return nil, errors.New("missing info_hash field")
	}

	if infoHash, typeOk = iField.(string); !typeOk {
		return nil, errors.New("info_hash type invalid")
	}

	// 暂时没保存peer，只能找到nodes
	resp.Nodes = GetRoutingTable().ClosestNodes(infoHash)

	return resp.Serialize()
}

func HandleAnnouncePeer(transactionId string, addDict map[string]interface{},  packetFrom *net.UDPAddr) ([]byte, error)  {
	var (
		iField interface{}
		infoHash string
		token string
		impliedPort int = 0
		port int
		exist bool
		typeOk bool
	)

	ActiveNode(addDict, packetFrom)

	resp := &AnnouncePeerResponse{}
	resp.TransactionId = transactionId

	if iField, exist = addDict["info_hash"]; !exist {
		return nil, errors.New("missing info_hash field")
	}
	if infoHash, typeOk = iField.(string); !typeOk {
		return nil, errors.New("info_hash type invalid")
	}

	if iField, exist = addDict["token"]; !exist {
		return nil, errors.New("missing token field")
	}
	if token, typeOk = iField.(string); !typeOk {
		return nil, errors.New("token type invalid")
	}

	if iField, exist = addDict["implied_port"]; exist {
		if impliedPort, typeOk = iField.(int); !typeOk {
			return nil, errors.New("implied_port type invalid")
		}
	}

	// 解析port
	if impliedPort == 1 {
		if iField, exist = addDict["port"]; !exist {
			return nil, errors.New("missing port field")
		}
		if port, typeOk = iField.(int); !typeOk {
			return nil, errors.New("port type invalid")
		}
	} else {
		port = int(packetFrom.Port)
	}

	// 校验token
	if !GetTokenManager().ValidateToken(token) {
		return nil, errors.New("token invalid")
	}

	// 保存peerinfo, 后续用于抓取种子
	HandlePeerInfo(infoHash, packetFrom.IP, port)

	return resp.Serialize()
}

func HandlePeerInfo(infoHash string, ip net.IP, port int) {
	fmt.Println( "magnet:?xt=urn:btih:" + hex.EncodeToString([]byte(infoHash)), ip, ":", port)
}
