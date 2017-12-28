package dht

import (
	"time"
	"math/big"
	"sync"
	"sort"
)

const (
 	KNODES = 8	// 每个桶保存8个节点
 	MAX_FAIL_TIMES = 3 // 3次连续fail则标记bad

 	// 节点状态
	NODE_STATUS_GOOD = 1
	NODE_STATUS_BAD = 2
)

type Node struct {
	info *CompactNode	// 节点地址
	lastActive int64	// 上次活跃时间
	failTimes int // 连续访问失败的次数, 超过3次就标记为bad
	status int // 状态: good, bad, questionable
}

type Bucket struct {
	nodes map[string]*Node
	min, max *big.Int
	lastActive int64
}

type RoutingTable struct {
	buckets []*Bucket
	mutex sync.Mutex
}

type ClosestNodes struct {
	target string
	nodes []*CompactNode
}

func (closest ClosestNodes) Len() int {
	return len(closest.nodes)
}

func (closest ClosestNodes) Swap(i, j int) {
	closest.nodes[i], closest.nodes[j] = closest.nodes[j], closest.nodes[i]
}

func (closest ClosestNodes) Less(i, j int) bool {
	leftId := nodeId2Int(closest.nodes[i].Id)
	rightId := nodeId2Int(closest.nodes[j].Id)
	targetId := nodeId2Int(closest.target)

	// 计算异或距离, 比较大小
	cmp := new(big.Int).Xor(leftId, targetId).Cmp( new(big.Int).Xor(rightId, targetId) )
	return cmp < 0
}

func newBucket(min, max *big.Int) (bucket *Bucket) {
	bucket = &Bucket{}

	bucket.min = min
	bucket.max = max
	bucket.nodes = make(map[string]*Node)
	bucket.lastActive = time.Now().Unix()
	return
}

func nodeId2Int(nodeId string) *big.Int {
	return new(big.Int).SetBytes([]byte(nodeId))
}

func (bucket *Bucket) size() int {
	return len(bucket.nodes)
}

func (bucket *Bucket) inRange(nodeId string) bool {
	intId := nodeId2Int(nodeId)
	return intId.Cmp(bucket.min) >= 0 && intId.Cmp(bucket.max) < 0
}

func (bucket *Bucket) insertNode(nodeInfo *CompactNode) bool {
	var (
		node *Node
		exist bool
	)
	if node, exist = bucket.nodes[nodeInfo.Id]; exist {
		goto REPLACE
	}
	for nodeId, node := range bucket.nodes {
		if node.status == NODE_STATUS_BAD {
			delete(bucket.nodes, nodeId) // 虽然是bad, 但删1个就好了
			break
		}
	}
	if len(bucket.nodes) == KNODES {
		return false
	}
REPLACE:
	node = &Node{}
	node.info = nodeInfo
	node.status = NODE_STATUS_GOOD
	node.lastActive = time.Now().Unix()
	node.failTimes = 0
	bucket.nodes[nodeInfo.Id] = node
	bucket.lastActive = time.Now().Unix()
	return true
}

func rootBucket() (root *Bucket) {
	minId := big.NewInt(0)
	maxId := new(big.Int).Exp(big.NewInt(2), big.NewInt(160), nil)
	root = newBucket(minId, maxId)
	root.insertNode(&CompactNode{"", MyNodeId()})
	return
}

// 路由表单例
var routingTable *RoutingTable
var initRoutingTableOnce sync.Once

func GetRoutingTable() (*RoutingTable) {
	initRoutingTableOnce.Do(func () {
		routingTable = &RoutingTable{}
		routingTable.buckets = append(routingTable.buckets, rootBucket())
	})
	return routingTable
}

func (rt *RoutingTable) splitBucket(idx int) {
	toSplit := rt.buckets[idx]

	sumRange := new(big.Int).Add(toSplit.min, toSplit.max)
	mid := new(big.Int).Div(sumRange, big.NewInt(2))

	rightBucket := newBucket(mid, toSplit.max)
	toSplit.max = mid

	// 原桶分裂成2个新桶
	for nodeId, node := range toSplit.nodes {
		if !toSplit.inRange(nodeId) {
			delete(toSplit.nodes, nodeId)
			rightBucket.nodes[nodeId] = node
		}
	}
	rightBucket.lastActive = toSplit.lastActive

	// 插入分裂后的桶
	rt.buckets = append(rt.buckets, nil) // 扩容
	insertIdx := idx + 1
	copy(rt.buckets[insertIdx + 1:], rt.buckets[insertIdx:])
	rt.buckets[insertIdx] = rightBucket
}

func (rt *RoutingTable) findBucket(nodeId string) int {
	for i := 0; i < len(rt.buckets); i++ {
		if !rt.buckets[i].inRange(nodeId) {
			continue
		}
		return i
	}
	return -1
}

func (rt *RoutingTable) insertNode(nodeInfo *CompactNode) bool {
	if nodeInfo.Id == MyNodeId() {
		return true
	}

	idx := rt.findBucket(nodeInfo.Id)
	if idx < 0 {
		return false // never reach
	}

	if rt.buckets[idx].insertNode(nodeInfo) { // bucket没满插入成功
		return true
	}
	if !rt.buckets[idx].inRange(MyNodeId()) { // bucket不包含自身,无法分裂
		return false
	}
	rt.splitBucket(idx)
	return rt.insertNode(nodeInfo)
}

func (rt *RoutingTable) InsertNode(nodeInfo *CompactNode) bool {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	return rt.insertNode(nodeInfo)
}

func (rt *RoutingTable) Size() int {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()
	return len(rt.buckets)
}

func (rt *RoutingTable) Fail(nodeId string) {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	if nodeId == MyNodeId() {
		return
	}

	idx := rt.findBucket(nodeId)
	if idx < 0 {
		return
	}

	if node, exist := rt.buckets[idx].nodes[nodeId]; exist {
		if node.status != NODE_STATUS_BAD {
			node.failTimes++
			if node.failTimes >= MAX_FAIL_TIMES {
				node.status = NODE_STATUS_BAD
			}
		}
	}
}

func (rt *RoutingTable) FindNode(nodeId string) *CompactNode {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	// 永远不返回自己
	if nodeId == MyNodeId() {
		return nil
	}

	idx := rt.findBucket(nodeId)
	if idx < 0 {
		return nil
	}

	nodes := rt.buckets[idx].nodes
	if node, exist := nodes[nodeId]; exist {
		return node.info
	}
	return nil
}

func (rt *RoutingTable) ClosestNodes(nodeId string) (nodes []*CompactNode) {
	nodes = make([]*CompactNode, 0)

	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	idx := rt.findBucket(nodeId)
	if idx < 0 {
		return
	}

	for _, node := range rt.buckets[idx].nodes {
		if node.info.Id != nodeId {
			nodes = append(nodes, node.info)
		}
	}

	// 不足8个, 找周边的bucket
	if len(nodes) < KNODES {
		leftIdx := idx - 1
		rightIdx := idx + 1
		for len(nodes) < KNODES && (leftIdx >= 0 || rightIdx < len(rt.buckets)){ // 从左边和右边的邻居桶补一些进来
			if leftIdx >= 0 {
				for _, node := range rt.buckets[leftIdx].nodes {
					nodes = append(nodes, node.info)
				}
			}
			if rightIdx < len(rt.buckets) {
				for _, node := range rt.buckets[rightIdx].nodes {
					nodes = append(nodes, node.info)
				}
			}
			leftIdx--
			rightIdx++
		}
	}

	// 按距离排序
	closestNodes := ClosestNodes{}
	closestNodes.target = nodeId
	closestNodes.nodes = nodes
	sort.Sort(closestNodes)
	// 取最近的8个
	if len(nodes) > KNODES {
		nodes = nodes[:KNODES]
	}
	return
}