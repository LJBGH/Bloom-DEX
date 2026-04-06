## 查看 topic 列表
Write-Host "Listing topics ..."
docker exec -it bld-kafka kafka-topics --bootstrap-server localhost:9092 --list

## 创建 topic trade.order.create 主题
Write-Host "Creating topic trade.order.create (partitions=8, replication-factor=1) ..."
docker exec -it bld-kafka kafka-topics --bootstrap-server localhost:9092 --create --topic trade.order.create --partitions 8 --replication-factor 1

## 查看 topic trade.order.create 分区详情
Write-Host "Describing topic trade.order.create partitions ..."
docker exec -it bld-kafka kafka-topics --bootstrap-server localhost:9092 --describe --topic trade.order.create

## 增加 topic 分区
## Write-Host "Adding partitions to topic trade.order.create (partitions=8) ..."
## docker exec -it bld-kafka kafka-topics --bootstrap-server localhost:9092 --alter --topic trade.order.create --partitions 8