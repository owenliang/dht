package dht

import (
	"time"
	"math/big"
	"sync"
)

const (
 	KNODES = 8	// 每个桶保存8个节点

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

func newBucket(min, max *big.Int) (bucket *Bucket) {
	bucket = &Bucket{}

	bucket.min = min
	bucket.max = max
	bucket.nodes = make(map[string]*Node)
	bucket.lastActive = time.Now().Unix()
	return
}

func (bucket *Bucket) size() int {
	return len(bucket.nodes)
}

func (bucket *Bucket) inRange(nodeId string) bool {
	intId := new(big.Int).SetBytes([]byte(nodeId))
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

func CreateRoutingTable() (rt *RoutingTable) {
	rt = &RoutingTable{}
	rt.buckets = append(rt.buckets, rootBucket())
	return
}

func (rt *RoutingTable) splitBucket() {
	toSplit := rt.buckets[len(rt.buckets) - 1]

	sumRange := new(big.Int).Add(toSplit.min, toSplit.max)
	mid := new(big.Int).Div(sumRange, big.NewInt(2))

	leftBucket := newBucket(toSplit.min, mid)
	rightBucket := newBucket(mid, toSplit.max)

	// 原桶数据分裂
	for nodeId, node := range toSplit.nodes {
		if leftBucket.inRange(nodeId) {
			leftBucket.nodes[nodeId] = node
		} else {
			rightBucket.nodes[nodeId] = node
		}
	}

	if leftBucket.inRange(MyNodeId()) {
		leftBucket, rightBucket = rightBucket, leftBucket
	}
	leftBucket.lastActive = toSplit.lastActive
	rt.buckets[len(rt.buckets) - 1] = leftBucket
	rt.buckets = append(rt.buckets, rightBucket)
}

func (rt *RoutingTable) insertNode(nodeInfo *CompactNode) bool {
	if nodeInfo.Id == MyNodeId() {
		return true
	}
	for i := 0; i < len(rt.buckets); i++ {
		if !rt.buckets[i].inRange(nodeInfo.Id) {
			continue
		}
		if rt.buckets[i].insertNode(nodeInfo) { // bucket没满插入成功
			return true
		}
		if i + 1 != len(rt.buckets) { // bucket不包含自身,无法分裂
			return false
		}
		rt.splitBucket()
		return rt.insertNode(nodeInfo)
	}
	return false
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
