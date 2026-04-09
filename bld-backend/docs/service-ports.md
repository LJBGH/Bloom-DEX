# 端口

## 基础服务

``` 2181  Zookeeper
    2379  etcd
    33060 MySQL（宿主机）
    6379  Redis
    9092  Kafka（宿主机）
    8545  本地 EVM 节点（可选）

    9001  Kafka UI
    9002  etcdkeeper
```

## 微服务

``` 9003  gateway
    9004  userapi   (HTTP)
    9005  walletapi (HTTP)
    9006  ordersapi (HTTP)

    9101  walletapi (gRPC)
    9201  market-ws (socket)
```
