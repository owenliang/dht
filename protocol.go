package dht

import (
	"crypto/rand"
	"crypto/sha1"
	"sync/atomic"
	"strconv"
	"sync"
	"time"
)

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
	Id string
	Target string
}

type FindNodeResponse struct {
	BaseResponse
	Nodes string
	ClosestNodes []string
}

// GET PEERS
type GetPeersRequest struct {
	BaseRequest
	InfoHash string
}

type GetPeersResponse struct {
	BaseResponse
	Token string
	InfoHash string
	Nodes string
	Values []string
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