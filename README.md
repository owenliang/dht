# dht

## 介绍

DHT是去中心化P2P下载的重要技术，它避免了BT下载依赖中心tracker节点来获取拥有资源的节点列表。

DHT通过P2P的方式传播资源的拥有者信息，而不在依靠tracker，而这个传播的算法就是DHT。

DHT并不是下载协议，最终资源下载仍旧是BT协议（种子），DHT是在帮助我们在P2P网络种找到下载地址。

具体参考官方论文：[DHT Protocol](http://www.bittorrent.org/beps/bep_0005.html)

## 计划

分步骤实现一个DHT协议的种子爬虫，因为涉及的知识点比较多，一次性实现也不是很有数，所以暂定一个计划：

* 实现bencode协议的序列化/反序列化（完成开发，文件：bencode.go）
* 创建UDP SOCKET，尝试向大型的DHT节点发送4种协议的请求，并接受应答进行观察
* 实现路由表Routing table，利用UDP请求/应答得到的其他Node，维护自己的亲近朋友列表
* 基于UDP SOCKET，动态更新Routing table
* 基于UDP SOCKET，在响应get peers请求时进行欺诈，将自身作为结果返回
* 基于UDP SOCKET，接受announce peers请求，保存资源的peers

