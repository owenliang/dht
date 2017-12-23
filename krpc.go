package dht

import (
	"net"
	"sync"
	"context"
	"time"
	"errors"
	"fmt"
)

type KRPCContext struct {
	transactionId string // 请求ID
	request interface{} // 请求protocol对象
	encoded []byte // 序列化请求
	requestTo *net.UDPAddr // 目标地址

	response interface{} // 应答protocol对象
	responseFrom *net.UDPAddr // 收到应答的地址

	finishNotify chan byte // 收到应答后唤醒
}

type KRPC struct {
	conn *net.UDPConn

	mutex sync.Mutex
	reqContext map[string]*KRPCContext // 等待应答的请求

	reqQueue chan *KRPCContext // 请求队列
}

func (krpc *KRPC)ReadLoop() {
	var (
		err error
		ctx *KRPCContext

		buffer []byte = make([]byte, 10000)
		bufSize int

		responseFrom *net.UDPAddr
		response interface{}
		respDict map[string]interface{}

		transactionId string
		msgType string

		iField interface{}
		exist bool
		typeOk bool

	)
	for {
		if bufSize, responseFrom, err = krpc.conn.ReadFromUDP(buffer); err != nil || bufSize == 0 {
			continue
		}

		// 反序列化(TODO:多协程并行)
		data := buffer[:bufSize]
		if response, err = Decode(data); err != nil {
			continue
		}

		// 打印应答
		fmt.Println("read:", response)

		// 解析bencode字典第一层中的t(请求ID)、y(类型：请求，应答，错误)
		if respDict, typeOk = response.(map[string]interface{}); !typeOk {
			continue
		}

		if iField, exist = respDict["t"]; !exist { // t字段
			continue
		}
		if transactionId, typeOk = iField.(string); !typeOk {
			continue
		}

		if iField, exist = respDict["y"]; !exist { // y字段
			continue
		}
		if msgType, typeOk = iField.(string); !typeOk {
			continue
		}

		// 应答
		if msgType == "r" {
			// 寻找请求上下文
			{
				krpc.mutex.Lock()
				if ctx, exist = krpc.reqContext[transactionId]; exist {
					delete(krpc.reqContext, transactionId)
				}
				krpc.mutex.Unlock()
			}
			// 唤醒调用者进一步处理
			if ctx != nil {
				ctx.response = response
				ctx.responseFrom = responseFrom
				ctx.finishNotify <- 1
			}
		} else if msgType == "e" { // 错误

		} else if msgType == "q" { // 请求

		} else { // 未知

		}
	}
}

func (krpc *KRPC) SendLoop() {
	var (
		ctx *KRPCContext
	)
	for {
		select {
		case ctx = <-krpc.reqQueue:
			krpc.conn.WriteToUDP(ctx.encoded, ctx.requestTo)
		}
	}
}

func CreateKPRC() (kprc *KRPC, err error){
	krpc := KRPC{}
	addr := net.UDPAddr{net.IPv4(0, 0, 0,0), 0, ""}
	if krpc.conn, err = net.ListenUDP("udp4", &addr); err != nil {
		return nil, err
	}
	krpc.reqContext = make(map[string]*KRPCContext)
	krpc.reqQueue = make(chan *KRPCContext, 100000)
	go krpc.SendLoop()
	go krpc.ReadLoop()
	return &krpc, nil
}

func (krpc *KRPC) BurstRequest(transactionId string, request interface{}, encoded []byte, address string) (ctxt *KRPCContext, err error) {
	var (
		requestTo *net.UDPAddr
		isTimeout bool = false
	)
	// 域名解析
	if requestTo, err = net.ResolveUDPAddr("udp4", address); err != nil {
		return
	}
	// 生成调用上下文
	ctx := &KRPCContext{
		transactionId: transactionId,
		request: request,
		encoded: encoded,
		requestTo: requestTo,
		finishNotify: make(chan byte, 1),
	}
	// 注册调用
	{
		krpc.mutex.Lock()
		krpc.reqContext[transactionId] = ctx
		krpc.mutex.Unlock()
	}
	// 启动超时
	timeoutCtx, _ := context.WithTimeout(context.Background(), time.Duration(5) * time.Second)
	select {
	case krpc.reqQueue <- ctx:  // 排队请求
	case <- timeoutCtx.Done(): // 等待超时
		isTimeout = true
	}
	// 排队成功,等待应答
	if !isTimeout {
		select {
		case <- ctx.finishNotify:
		case <- timeoutCtx.Done():
			isTimeout = true
		}
	}
	if isTimeout {
		{	// 超时取消注册的上下文
			krpc.mutex.Lock()
			if _, exist := krpc.reqContext[transactionId]; exist {
				delete(krpc.reqContext, transactionId)
			}
			krpc.mutex.Unlock()
		}
		return nil, errors.New("request timeout")
	}
	return ctx, nil
}

func (krpc *KRPC) Ping(request *PingRequest) (response *PingResponse, err error) {
	var (
		ctx *KRPCContext
		bytes []byte
	)

	// 序列化
	protobuf := map[string]interface{}{}
	protobuf["t"] = request.TransactionId
	protobuf["y"] = request.Type
	protobuf["q"] = request.Method
	protobuf["a"] = map[string]interface{}{
		"id": MyNodeId(),
	}
	if bytes, err = Encode(protobuf); err != nil {
		return
	}

	if ctx, err = krpc.BurstRequest(request.TransactionId, request, bytes,"router.bittorrent.com:6881"); err != nil {
		return
	}
	// TODO: 反序列化protocol
	ctx = ctx

	return
}

func (krpc *KRPC) FindNode(request *FindNodeRequest) (response *FindNodeResponse, err error) {
	return
}

func (krpc *KRPC) GetPeers(request *GetPeersRequest) (response *GetPeersResponse, err error) {
	return
}

func (krpc *KRPC) AnnouncePeer(request *AnnouncePeerRequest) (response *AnnouncePeerResponse, err error) {
	return
}