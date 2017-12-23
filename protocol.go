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
	Target *CompactNode
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

func ParseCompactNode(nodeInfo string) (*CompactNode, error) {
	if len(nodeInfo) != 26 {
		return nil, errors.New("compact node invalid")
	}
	compactNode := &CompactNode{}
	compactNode.Id = nodeInfo[0:20]
	port := binary.BigEndian.Uint16([]byte(nodeInfo[24:26]))
	compactNode.Address = fmt.Sprintf("%d:%d:%d:%d:%d", nodeInfo[20], nodeInfo[21], nodeInfo[22], nodeInfo[23], port)
	return compactNode, nil
}

func ParsePeerInfo(peerInfo string) (string, error) {
	if len(peerInfo) != 6 {
		return "", errors.New("compact peer invalid")
	}
	port := binary.BigEndian.Uint16([]byte(peerInfo[4:6]))
	return fmt.Sprintf("%d:%d:%d:%d:%d", peerInfo[0], peerInfo[1], peerInfo[2], peerInfo[3], port), nil
}

func ParsePingResponse(transactionId string, benDict map[string]interface{}) (response *PingResponse, err error) {
	var (
		iField interface{}
		resDict map[string]interface{}
		exist bool
		typeOk bool
	)

	response = &PingResponse{}
	response.TransactionId = transactionId
	response.Type = "r"

	if iField, exist = benDict["r"]; !exist {
		goto ERROR
	}
	if resDict, typeOk = iField.(map[string]interface{}); !typeOk {
		goto ERROR
	}
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

func ParseFindNodeResponse(transactionId string, benDict map[string]interface{}) (response *FindNodeResponse, err error) {
	var (
		iField interface{}
		resDict map[string]interface{}
		exist bool
		typeOk bool
		nodes string
		compactNode *CompactNode
		nodesSplit string
		target string
	)

	response = &FindNodeResponse{}
	response.TransactionId = transactionId
	response.Type = "r"
	response.Nodes = make([]*CompactNode, 0)

	if iField, exist = benDict["r"]; !exist {
		goto ERROR
	}
	if resDict, typeOk = iField.(map[string]interface{}); !typeOk {
		goto ERROR
	}

	if iField, exist = resDict["id"]; !exist {
		goto ERROR
	}
	if response.Id, typeOk = iField.(string); !typeOk {
		goto ERROR
	}

	if iField, exist = resDict["target"]; exist {
		if target, typeOk = iField.(string); !typeOk {
			goto ERROR
		}
		// target解析compactNode
		if compactNode, err = ParseCompactNode(target); err != nil {
			goto ERROR
		}
		response.Target = compactNode
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
				if compactNode, err = ParseCompactNode(nodesSplit); err != nil {
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

func ParseGetPeersResponse(transactionId string, benDict map[string]interface{}) (response *GetPeersResponse, err error) {
	var (
		iField interface{}
		resDict map[string]interface{}
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

	if iField, exist = benDict["r"]; !exist {
		goto ERROR
	}
	if resDict, typeOk = iField.(map[string]interface{}); !typeOk {
		goto ERROR
	}

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
			address, err = ParsePeerInfo(peerInfo)
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
				if compactNode, err = ParseCompactNode(nodesSplit); err != nil {
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

func ParseAnnouncePeerResponse(transactionId string, benDict map[string]interface{}) (response *AnnouncePeerResponse, err error) {
	err = errors.New("not implement")
	return
}

func (response *PingResponse) String() string {
	ret := "---PingResponse---\n"
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
	ret := "---FindNodeResponse---\n"
	ret += "T=" + hex.EncodeToString([]byte(response.TransactionId)) + "\n"
	ret += "Id=" + hex.EncodeToString([]byte(response.Id)) + "\n"
	if response.Target != nil {
		ret += "Target=\n"
		ret += "->"+ response.Target.String()
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


func (response *GetPeersResponse) String() string {
	ret := "---GetPeersResponse---\n"
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
var myNodeId string = ""

func MyNodeId() string {
	if len(myNodeId) == 0 {
		myNodeId = GenNodeId()
	}
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