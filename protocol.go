package dht

import (
	"crypto/rand"
	"crypto/sha1"
	"sync/atomic"
	"strconv"
	"sync"
	"time"
	"errors"
	"encoding/binary"
	"fmt"
	"encoding/hex"
	"net"
)

type CompactNode struct {
	Address string
	Id string
}

type ProtocolBase struct {
	TransactionId string // t: 请求唯一ID标识
	Type string // y: 消息类型(q,r,e)
	Id string	// id（request是请求方id, response是应答方id)
}

type BaseRequest struct {
	ProtocolBase
	Method string // 请求方法名
}

type BaseResponse struct {
	ProtocolBase
}

// PING
type PingRequest struct {
	BaseRequest
}

type PingResponse struct {
	BaseResponse
}

// FIND NODE
type FindNodeRequest struct {
	BaseRequest
	Target string
}

type FindNodeResponse struct {
	BaseResponse
	Nodes []*CompactNode
}

// GET PEERS
type GetPeersRequest struct {
	BaseRequest
	InfoHash string
}

type GetPeersResponse struct {
	BaseResponse
	Token string
	Nodes []*CompactNode
	Values []string // ip:port address..
}

// ANNOUNCE PEER
type AnnouncePeerRequest struct {
	BaseRequest
	ImpliedPort int
	InfoHash string
	Port int
	Token string
}

type AnnouncePeerResponse struct {
	BaseResponse
}

func NewPingRequest() (request *PingRequest) {
	request = &PingRequest{}
	request.TransactionId = GenTransactionId()
	request.Type = "q"
	request.Method = "ping"
	request.Id = MyNodeId()
	return request
}

func NewFindNodeRequest() (request *FindNodeRequest) {
	request = &FindNodeRequest{}
	request.TransactionId = GenTransactionId()
	request.Type = "q"
	request.Method = "find_node"
	request.Id = MyNodeId()
	return request
}

func NewGetPeersRequest() (request *GetPeersRequest) {
	request = &GetPeersRequest{}
	request.TransactionId = GenTransactionId()
	request.Type = "q"
	request.Method = "get_peers"
	request.Id = MyNodeId()
	return request
}

func NewAnnouncePeerRequest() (request *AnnouncePeerRequest) {
	request = &AnnouncePeerRequest{}
	request.TransactionId = GenTransactionId()
	request.Type = "q"
	request.Method = "announce_peer"
	request.ImpliedPort = 0
	request.Id = MyNodeId()
	return request
}

func UnserializeCompactNode(nodeInfo string) (*CompactNode, error) {
	if len(nodeInfo) != 26 {
		return nil, errors.New("compact node invalid")
	}
	compactNode := &CompactNode{}
	compactNode.Id = nodeInfo[0:20]
	port := binary.BigEndian.Uint16([]byte(nodeInfo[24:26]))
	compactNode.Address = fmt.Sprintf("%d.%d.%d.%d:%d", nodeInfo[20], nodeInfo[21], nodeInfo[22], nodeInfo[23], port)
	return compactNode, nil
}

func UnserializePeerInfo(peerInfo string) (string, error) {
	if len(peerInfo) != 6 {
		return "", errors.New("compact peer invalid")
	}
	port := binary.BigEndian.Uint16([]byte(peerInfo[4:6]))
	return fmt.Sprintf("%d.%d.%d.%d:%d", peerInfo[0], peerInfo[1], peerInfo[2], peerInfo[3], port), nil
}

func UnserializePingResponse(transactionId string, resDict map[string]interface{}) (response *PingResponse, err error) {
	var (
		iField interface{}
		exist bool
		typeOk bool
	)

	response = &PingResponse{}
	response.TransactionId = transactionId
	response.Type = "r"

	if iField, exist = resDict["id"]; !exist {
		goto ERROR
	}
	if response.Id, typeOk = iField.(string); !typeOk {
		goto ERROR
	}
	return response, nil
ERROR:
	return nil, errors.New("invalid ping response")
}

func UnserializeFindNodeResponse(transactionId string, resDict map[string]interface{}) (response *FindNodeResponse, err error) {
	var (
		iField interface{}
		exist bool
		typeOk bool
		nodes string
		compactNode *CompactNode
		nodesSplit string
	)

	response = &FindNodeResponse{}
	response.TransactionId = transactionId
	response.Type = "r"
	response.Nodes = make([]*CompactNode, 0)

	if iField, exist = resDict["id"]; !exist {
		goto ERROR
	}
	if response.Id, typeOk = iField.(string); !typeOk {
		goto ERROR
	}

	if iField, exist = resDict["nodes"]; exist {
		if nodes, typeOk = iField.(string); !typeOk {
			goto ERROR
		}
		if len(nodes) % 26 != 0 {
			goto ERROR
		}
		for i := 0; i <= len(nodes); i++  {
			if i % 26 == 0 && i != 0 {
				nodesSplit = nodes[i - 26:i]
				// closest nodes解析compactNode
				if compactNode, err = UnserializeCompactNode(nodesSplit); err != nil {
					goto ERROR
				}
				response.Nodes = append(response.Nodes, compactNode)
			}
		}
	}
	return response, nil
ERROR:
	return nil, errors.New("invalid find_node response")
}

func UnserializeGetPeersResponse(transactionId string, resDict map[string]interface{}) (response *GetPeersResponse, err error) {
	var (
		iField interface{}
		exist bool
		typeOk bool
		nodes string
		nodesSplit string
		compactNode *CompactNode
		peers []interface{}
		peerInfo string
		address string
	)

	response = &GetPeersResponse{}
	response.TransactionId = transactionId
	response.Type = "r"
	response.Nodes = make([]*CompactNode, 0)
	response.Values = make([]string, 0)

	if iField, exist = resDict["id"]; !exist {
		goto ERROR
	}
	if response.Id, typeOk = iField.(string); !typeOk {
		goto ERROR
	}

	if iField, exist = resDict["values"]; exist {
		if peers, typeOk = iField.([]interface{}); !typeOk {
			goto ERROR
		}
		for i := 0; i < len(peers); i++ {
			if peerInfo, typeOk = peers[i].(string); !typeOk {
				goto ERROR
			}
			address, err = UnserializePeerInfo(peerInfo)
			if err != nil {
				goto ERROR
			}
			response.Values = append(response.Values, address)
		}
	}

	if iField, exist = resDict["nodes"]; exist {
		if nodes, typeOk = iField.(string); !typeOk {
			goto ERROR
		}
		if len(nodes) % 26 != 0 {
			goto ERROR
		}
		for i := 0; i <= len(nodes); i++  {
			if i % 26 == 0 && i != 0 {
				nodesSplit = nodes[i - 26:i]
				// target解析compactNode
				if compactNode, err = UnserializeCompactNode(nodesSplit); err != nil {
					goto ERROR
				}
				response.Nodes = append(response.Nodes, compactNode)
			}
		}
	}
	return response, nil
ERROR:
	return nil, errors.New("invalid find_node response")
}

func UnserializeAnnouncePeerResponse(transactionId string, benDict map[string]interface{}) (response *AnnouncePeerResponse, err error) {
	err = errors.New("not implement")
	return
}

func (node *CompactNode) Serialize() (bytes []byte, err error) {
	var (
		addr *net.UDPAddr
	)
	if addr, err = net.ResolveUDPAddr("udp", node.Address); err != nil {
		return
	}
	bytes = make([]byte, 26)
	copy(bytes, node.Id)
	copy(bytes[20:24], addr.IP)
	binary.BigEndian.PutUint16(bytes[24:26], uint16(addr.Port))
	return bytes, nil
}

func (response *PingResponse) Serialize() ([]byte, error){
	resp := map[string]interface{}{}

	resp["t"] = response.TransactionId
	resp["y"] = "r"

	r := map[string]interface{}{}
	r["id"] = MyNodeId()

	resp["r"] = r
	return Encode(resp)
}

func (response *FindNodeResponse) Serialize() (bytes []byte, err error){
	var (
		compactNode *CompactNode
		compactNodeBytes []byte
		resp = map[string]interface{}{}
		r = map[string]interface{}{}
		nodesBytes []byte = nil
	)

	resp["t"] = response.TransactionId
	resp["y"] = "r"

	r["id"] = MyNodeId()
	for _, compactNode = range response.Nodes {
		if compactNodeBytes, err = compactNode.Serialize(); err == nil {
			nodesBytes = append(	nodesBytes, compactNodeBytes...)
		}
	}
	r["nodes"] = string(nodesBytes)

	resp["r"] = r
	return Encode(resp)
}

func (response *GetPeersResponse) Serialize() (bytes []byte, err error) {
	var (
		addr *net.UDPAddr
		compactNode *CompactNode
		compactNodeBytes []byte
		resp = map[string]interface{}{}
		r = map[string]interface{}{}
		nodesBytes []byte = nil
		peerInfos  = make([]string, 0)
		peerInfo string
	)
	resp["t"] = response.TransactionId
	resp["y"] = "r"

	var compactPeerInfo [6]byte
	for _, peerInfo = range response.Values {
		if addr, err = net.ResolveUDPAddr("udp", peerInfo); err != nil {
			return
		}
		copy(compactPeerInfo[0:4], addr.IP)
		binary.BigEndian.PutUint16(bytes[4:6], uint16(addr.Port))
		peerInfos = append(peerInfos, string(compactPeerInfo[:]))
	}
	if len(peerInfos) > 0 {
		r["values"] = peerInfos
	} else {
		for _, compactNode = range response.Nodes {
			if compactNodeBytes, err = compactNode.Serialize(); err == nil {
				nodesBytes = append(	nodesBytes, compactNodeBytes...)
			}
		}
		r["nodes"] = string(nodesBytes)
	}

	r["id"] = MyNodeId()
	r["token"] = GetTokenManager().GetToken()
	return Encode(resp)
}

func (response *AnnouncePeerResponse) Serialize() ([]byte, error) {
	var (
		resp = map[string]interface{}{}
		r = map[string]interface{}{}
	)
	resp["t"] = response.TransactionId
	resp["y"] = "r"
	r["id"] = MyNodeId()
	return Encode(resp)
}

func (response *PingResponse) String() string {
	ret := "\n---PingResponse---\n"
	ret += "T=" + hex.EncodeToString([]byte(response.TransactionId)) + "\n"
	ret += "Id=" + hex.EncodeToString([]byte(response.Id)) + "\n"
	ret += "---------------------\n"
	return ret
}

func (compactNode *CompactNode) String() string {
	ret := "ID=" + hex.EncodeToString([]byte(compactNode.Id)) + " Addr=" + compactNode.Address + "\n"
	return ret
}

func (response *FindNodeResponse) String() string {
	ret := "\n---FindNodeResponse---\n"
	ret += "T=" + hex.EncodeToString([]byte(response.TransactionId)) + "\n"
	ret += "Id=" + hex.EncodeToString([]byte(response.Id)) + "\n"
	if len(response.Nodes) != 0 {
		ret += "Nodes=\n"
		for _, node := range response.Nodes {
			ret += "->" + node.String()
		}
	}
	ret += "---------------------\n"
	return ret
}


func (response *GetPeersResponse) String() string {
	ret := "\n---GetPeersResponse---\n"
	ret += "T=" + hex.EncodeToString([]byte(response.TransactionId)) + "\n"
	ret += "Id=" + hex.EncodeToString([]byte(response.Id)) + "\n"
	if len(response.Values) != 0 {
		ret += "Values=\n"
		for _, address := range response.Values {
			ret += "->" + address
		}
	}
	if len(response.Nodes) != 0 {
		ret += "Nodes=\n"
		for _, node := range response.Nodes {
			ret += "->" + node.String()
		}
	}
	ret += "---------------------\n"
	return ret
}

// 生成随机DHT NODE ID
func GenNodeId() string {
	randBytes := make([]byte, 160) // 随机160字节, 然后sha1计算20字节二进制ID
	for {
		if _, err := rand.Read(randBytes); err == nil {
			sha1Bytes := sha1.Sum(randBytes)
			return string(sha1Bytes[:])
		}
	}
}

// 我的DHT NODE ID
var myNodeId string
var initMyNodeId sync.Once

func MyNodeId() string {
	initMyNodeId.Do(func() {
		myNodeId = GenNodeId()
	})
	return myNodeId
}

// 生成请求唯一ID
var myTransactionId int64 = 0
var initTransactionId sync.Once

func GenTransactionId() string {
	initTransactionId.Do(func() {
		myTransactionId = time.Now().UnixNano()
	})
	return strconv.Itoa(int(atomic.AddInt64(&myTransactionId, 1)))
}
