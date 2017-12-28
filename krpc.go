package dht

import (
	"net"
	"sync"
	"context"
	"time"
	"errors"
	"fmt"
	"runtime"
)

type KRPCContext struct {
	transactionId string // 请求ID
	request interface{} // 请求protocol对象
	encoded []byte // 序列化请求
	requestTo *net.UDPAddr // 目标地址

	errCode int // 错误码
	errMsg string // 错误信息
	resDict map[string]interface{} // r字典
	responseFrom *net.UDPAddr // 发送应答的地址

	finishNotify chan byte // 收到应答后唤醒
}

type KRPCResponse struct {
	encoded []byte // 序列化应答
	responseTo *net.UDPAddr // 回复地址
}

type KRPCPacket struct {
	encoded []byte // 序列化的包
	packetFrom *net.UDPAddr // 来源地址
}

type KRPC struct {
	conn *net.UDPConn

	mutex sync.Mutex
	reqContext map[string]*KRPCContext // 等待应答的请求

	reqQueue chan *KRPCContext // 发送请求队列
	resQueue chan *KRPCResponse // 发送应答队列

	procQueue chan *KRPCPacket // 处理外来包队列
	procPending chan byte // 请求处理堆积控制
}

func (krpc *KRPC)HandleResponse(transactionId string, benDict map[string]interface{},  packetFrom *net.UDPAddr) {
	var (
		ctx *KRPCContext
		resDict map[string]interface{}
		iField interface{}
		exist bool
		typeOk bool
	)

	if iField, exist = benDict["r"]; !exist {
		return
	}
	if resDict, typeOk = iField.(map[string]interface{}); !typeOk {
		return
	}

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
		ctx.resDict = resDict
		ctx.responseFrom = packetFrom
		ctx.finishNotify <- 1
	}
}

func (krpc *KRPC)HandleError(transactionId string, benDict map[string]interface{},  packetFrom *net.UDPAddr) {
	var (
		ctx *KRPCContext
		exist bool
		iField interface{}
		iList []interface{}
		typeOk bool

		errCode int
		errMsg string
	)

	fmt.Println("HandleError")

	if iField, exist = benDict["e"]; !exist {
		return
	}
	if iList, typeOk = iField.([]interface{}); !typeOk {
		return
	}
	if len(iList) < 2 {
		return
	}
	if errCode, typeOk = iList[0].(int); !typeOk {
		return
	}
	if errMsg, typeOk = iList[1].(string); !typeOk {
		return
	}

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
		ctx.errCode = errCode
		ctx.errMsg = errMsg
		ctx.resDict = nil
		ctx.responseFrom = packetFrom
		ctx.finishNotify <- 1
	}
}

func (krpc *KRPC)HandleRequest(transactionId string, benDict map[string]interface{},  packetFrom *net.UDPAddr) {
	var (
		iField interface{}
		method string
		exist bool
		typeOk bool
		addDict map[string]interface{}
		respBytes []byte
		err error
	)

	if iField, exist = benDict["q"]; !exist {
		return
	}
	if method, typeOk = iField.(string); !typeOk {
		return
	}

	if iField, exist = benDict["a"]; !exist {
		return
	}
	if addDict, typeOk = iField.(map[string]interface{}); !typeOk {
		return
	}

	select {
		case krpc.procPending <- 1: // 增加1个处理中的请求
		default:
			return
	}
	// 并发协程处理
	go func() {
		fmt.Println("HandleRequest method=" + method)
		if method == "ping" {
			respBytes, err = HandlePing(transactionId, addDict, packetFrom)
		} else if method == "find_node" {
			respBytes, err = HandleFindNode(transactionId, addDict, packetFrom)
		} else if method == "get_peers" {
			respBytes, err = HandleGetPeer(transactionId, addDict, packetFrom)
		} else if method == "announce_peer" {
			respBytes, err = HandleAnnouncePeer(transactionId, addDict, packetFrom)
		} else {
			goto END
		}
		fmt.Println(method, string(respBytes), err)
		if err != nil {
			krpc.resQueue <- &KRPCResponse{encoded: respBytes, responseTo: packetFrom}
		}
		END:
		<- krpc.procPending // 处理完释放计数
	}()
}

func (krpc *KRPC)HandlePacket(data []byte, packetFrom *net.UDPAddr) {
	var (
		err error

		bencode interface{}
		benDict map[string]interface{}

		transactionId string
		msgType string

		iField interface{}
		exist bool
		typeOk bool
	)

	if bencode, err = Decode(data); err != nil {
		goto INVALID
	}

	// 提取: t(请求ID)，y(请求，应答，错误)
	if benDict, typeOk = bencode.(map[string]interface{}); !typeOk {
		goto INVALID
	}

	if iField, exist = benDict["t"]; !exist {
		goto INVALID
	}
	if transactionId, typeOk = iField.(string); !typeOk {
		goto INVALID
	}

	if iField, exist = benDict["y"]; !exist {
		goto INVALID
	}
	if msgType, typeOk = iField.(string); !typeOk {
		goto INVALID
	}

	// 应答
	if msgType == "r" {
		krpc.HandleResponse(transactionId, benDict, packetFrom)
	} else if msgType == "e" { // 错误
		krpc.HandleError(transactionId, benDict, packetFrom)
	} else if msgType == "q" { // 请求
		krpc.HandleRequest(transactionId, benDict, packetFrom)
	} else { // 未知
		goto INVALID
	}
	return

INVALID:
	fmt.Println(data)
}

func (krpc *KRPC)ProcLoop() {
	var (
		packet *KRPCPacket
	)
	for {
		packet = <- krpc.procQueue
		krpc.HandlePacket(packet.encoded, packet.packetFrom)
	}
}

func (krpc *KRPC)ReadLoop() {
	var (
		err error

		packetFrom *net.UDPAddr
		buffer []byte = make([]byte, 10000)
		bufSize int
	)
	for {
		if bufSize, packetFrom, err = krpc.conn.ReadFromUDP(buffer); err != nil || bufSize == 0 {
			continue
		}

		data := make([]byte, bufSize)
		copy(data, buffer[:bufSize])

		packet := &KRPCPacket{encoded: data, packetFrom: packetFrom}

		krpc.procQueue <- packet
	}
}

func (krpc *KRPC) SendLoop() {
	var (
		ctx *KRPCContext
		resp *KRPCResponse
	)
	for {
		select {
		case ctx = <-krpc.reqQueue:
			krpc.conn.WriteToUDP(ctx.encoded, ctx.requestTo)
		case resp = <- krpc.resQueue:
			krpc.conn.WriteToUDP(resp.encoded, resp.responseTo)
		}
	}
}

func CreateKPRC() (krpc *KRPC, err error){
	krpc = &KRPC{}
	addr := net.UDPAddr{net.IPv4(0, 0, 0,0), 6881, ""}
	if krpc.conn, err = net.ListenUDP("udp4", &addr); err != nil {
		return nil, err
	}
	krpc.reqContext = make(map[string]*KRPCContext)
	krpc.reqQueue = make(chan *KRPCContext, 100000)
	krpc.resQueue = make(chan *KRPCResponse, 100000)
	krpc.procQueue = make(chan *KRPCPacket, 100000)
	krpc.procPending = make(chan byte, 100000)
	go krpc.SendLoop()
	go krpc.ReadLoop()
	for i := 0; i < runtime.NumCPU(); i++ {
		go krpc.ProcLoop()
	}
	return krpc, nil
}

func (krpc *KRPC) BurstRequest(userCtx context.Context, transactionId string, request interface{}, encoded []byte, address string) (ctxt *KRPCContext, err error) {
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
	// 启动RPC超时
	timeoutCtx, cancelFunc := context.WithTimeout(userCtx, time.Duration(5) * time.Second)
	defer cancelFunc()
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

func (krpc *KRPC) Ping(userCtx context.Context, request *PingRequest, address string) (response *PingResponse, err error) {
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

	if ctx, err = krpc.BurstRequest(userCtx, request.TransactionId, request, bytes, address); err != nil {
		return
	}
	if ctx.errCode != 0 {
		return nil, errors.New(ctx.errMsg)
	}
	response, err = UnserializePingResponse(ctx.transactionId, ctx.resDict)
	return
}

func (krpc *KRPC) FindNode(userCtx context.Context, request *FindNodeRequest, address string) (response *FindNodeResponse, err error) {
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
		"target": request.Target,
	}
	if bytes, err = Encode(protobuf); err != nil {
		return
	}

	if ctx, err = krpc.BurstRequest(userCtx, request.TransactionId, request, bytes, address); err != nil {
		return
	}
	if ctx.errCode != 0 {
		return nil, errors.New(ctx.errMsg)
	}
	response, err = UnserializeFindNodeResponse(ctx.transactionId, ctx.resDict)
	return
}

func (krpc *KRPC) GetPeers(userCtx context.Context, request *GetPeersRequest, address string) (response *GetPeersResponse, err error) {
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
		"info_hash": request.InfoHash,
	}
	if bytes, err = Encode(protobuf); err != nil {
		return
	}
	if ctx, err = krpc.BurstRequest(userCtx, request.TransactionId, request, bytes, address); err != nil {
		return
	}
	response, err = UnserializeGetPeersResponse(ctx.transactionId, ctx.resDict)
	return
}

func (krpc *KRPC) AnnouncePeer(userCtx context.Context, request *AnnouncePeerRequest, address string) (response *AnnouncePeerResponse, err error) {
	var (
		ctx *KRPCContext
		bytes []byte
		addition map[string]interface{}
	)

	// 序列化
	protobuf := map[string]interface{}{}
	protobuf["t"] = request.TransactionId
	protobuf["y"] = request.Type
	protobuf["q"] = request.Method
	addition = map[string]interface{}{
		"id": MyNodeId(),
		"implied_port": request.ImpliedPort,
		"info_hash": request.InfoHash,
	}
	if request.ImpliedPort != 0 {
		addition["port"] = request.Port
	}
	if len(request.Token) != 0 {
		addition["token"] = request.Token
	}
	protobuf["a"] = addition
	if bytes, err = Encode(protobuf); err != nil {
		return
	}
	if ctx, err = krpc.BurstRequest(userCtx, request.TransactionId, request, bytes, address); err != nil {
		return
	}
	response, err = UnserializeAnnouncePeerResponse(ctx.transactionId, ctx.resDict)
	return
}